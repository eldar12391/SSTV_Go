/*
server.go реализует HTTP-уровень приложения SSTV: раздачу встроенной статики,
прием файлов на кодирование/декодирование и формирование ответов WAV/MP3/PNG.
Файл валидирует multipart-запросы, декодирует входные медиа через пакет codec,
применяет пользовательские параметры (sample_rate, audio_format, mp3_bitrate,
out_width, out_height), выполняет ресемплинг при необходимости и при наличии
ffmpeg может транскодировать аудио в MP3 и принимать MP3 на вход декодера.
*/
package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	_ "image/gif"
	_ "image/jpeg"

	"github.com/h1nezo/sstv/internal/codec"
)

func New(staticFS http.FileSystem) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(staticFS))
	mux.HandleFunc("/api/encode", handleEncode)
	mux.HandleFunc("/api/decode", handleDecode)
	return mux
}

func handleEncode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing field 'image'", http.StatusBadRequest)
		return
	}
	defer file.Close()
	log.Printf("encode: %s (%d B)", hdr.Filename, hdr.Size)

	img, _, err := image.Decode(file)
	if err != nil {
		http.Error(w, "cannot decode image: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	audioFormat := strings.ToLower(strings.TrimSpace(r.FormValue("audio_format")))
	if audioFormat == "" {
		audioFormat = "wav"
	}
	if audioFormat != "wav" && audioFormat != "mp3" {
		http.Error(w, "bad request: audio_format must be wav or mp3", http.StatusBadRequest)
		return
	}

	outSampleRate := parseUint32Clamped(r.FormValue("sample_rate"), codec.SampleRate, 8000, 96000)
	mp3Bitrate := parseIntClamped(r.FormValue("mp3_bitrate"), 192, 64, 320)

	samples := codec.Encode(img)
	if outSampleRate != codec.SampleRate {
		samples = resample(samples, codec.SampleRate, outSampleRate)
	}

	if audioFormat == "wav" {
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Content-Disposition", `attachment; filename="sstv_signal.wav"`)
		if err := codec.WriteWAV(w, samples, outSampleRate); err != nil {
			log.Printf("encode write wav: %v", err)
		}
		return
	}

	mp3Data, err := encodeMP3(samples, outSampleRate, mp3Bitrate)
	if err != nil {
		if errorsIsExecNotFound(err) {
			http.Error(w, "mp3 conversion requires ffmpeg installed on server", http.StatusNotImplemented)
			return
		}
		http.Error(w, "mp3 conversion failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Disposition", `attachment; filename="sstv_signal.mp3"`)
	_, _ = w.Write(mp3Data)
}

func handleDecode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("audio")
	if err != nil {
		http.Error(w, "missing field 'audio'", http.StatusBadRequest)
		return
	}
	defer file.Close()
	log.Printf("decode: %s (%d B)", hdr.Filename, hdr.Size)

	audioData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "cannot read audio: "+err.Error(), http.StatusBadRequest)
		return
	}

	samples, sr, err := codec.ReadWAV(bytes.NewReader(audioData))
	if err != nil {
		samples, sr, err = decodeAudioViaFFmpeg(audioData)
		if err != nil {
			if errorsIsExecNotFound(err) {
				http.Error(w, "cannot read audio: WAV only unless ffmpeg is installed", http.StatusNotImplemented)
				return
			}
			http.Error(w, "cannot read audio: "+err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}
	if sr != codec.SampleRate {
		log.Printf("decode: resampling %d -> %d Hz", sr, codec.SampleRate)
		samples = resample(samples, sr, codec.SampleRate)
	}

	img := codec.Decode(samples)
	outW := parseIntClamped(r.FormValue("out_width"), codec.Width, 64, 2048)
	outH := parseIntClamped(r.FormValue("out_height"), codec.Height, 64, 2048)
	if outW != codec.Width || outH != codec.Height {
		img = scaleImage(img, outW, outH)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		http.Error(w, "png encode: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", `attachment; filename="sstv_decoded.png"`)
	w.Write(buf.Bytes())
}

func resample(src []float32, fromRate, toRate uint32) []float32 {
	if len(src) == 0 || fromRate == toRate {
		return src
	}
	if fromRate == 0 || toRate == 0 {
		return src
	}

	ratio := float64(fromRate) / float64(toRate)
	n := int(float64(len(src)) / ratio)
	dst := make([]float32, n)
	for i := range dst {
		x := float64(i) * ratio
		j0 := int(x)
		j1 := j0 + 1
		if j0 < 0 {
			j0 = 0
		}
		if j1 >= len(src) {
			j1 = len(src) - 1
		}
		frac := float32(x - float64(j0))
		dst[i] = src[j0]*(1-frac) + src[j1]*frac
	}
	return dst
}

func encodeMP3(samples []float32, sampleRate uint32, bitrateKbps int) ([]byte, error) {
	var wav bytes.Buffer
	if err := codec.WriteWAV(&wav, samples, sampleRate); err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "wav",
		"-i", "pipe:0",
		"-f", "mp3",
		"-codec:a", "libmp3lame",
		"-b:a", fmt.Sprintf("%dk", bitrateKbps),
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(wav.Bytes())
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		if len(strings.TrimSpace(errOut.String())) > 0 {
			return nil, fmt.Errorf(strings.TrimSpace(errOut.String()))
		}
		return nil, err
	}

	return out.Bytes(), nil
}

func decodeAudioViaFFmpeg(audioData []byte) ([]float32, uint32, error) {
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-f", "f32le",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", codec.SampleRate),
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(audioData)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		if len(strings.TrimSpace(errOut.String())) > 0 {
			return nil, 0, fmt.Errorf(strings.TrimSpace(errOut.String()))
		}
		return nil, 0, err
	}

	raw := out.Bytes()
	if len(raw) < 4 {
		return nil, 0, fmt.Errorf("ffmpeg returned empty audio")
	}
	n := len(raw) / 4
	samples := make([]float32, n)
	for i := 0; i < n; i++ {
		u := binary.LittleEndian.Uint32(raw[i*4 : i*4+4])
		samples[i] = math.Float32frombits(u)
	}

	return samples, codec.SampleRate, nil
}

func errorsIsExecNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*exec.Error)
	return ok
}

func parseIntClamped(raw string, def, minV, maxV int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func parseUint32Clamped(raw string, def, minV, maxV uint32) uint32 {
	if raw == "" {
		return def
	}
	v, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return def
	}
	out := uint32(v)
	if out < minV {
		return minV
	}
	if out > maxV {
		return maxV
	}
	return out
}

func scaleImage(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	b := src.Bounds()
	sw := b.Dx()
	sh := b.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := b.Min.X + x*sw/w
			sy := b.Min.Y + y*sh/h
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

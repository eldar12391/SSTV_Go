/*
wav_reader.go отвечает за чтение WAV-файлов в PCM-массив float32 для декодера SSTV.
Файл проверяет базовую структуру RIFF/WAVE, извлекает параметры потока и находит data-чанк.
Поддерживается 16-битный PCM; при многоканальном аудио каналы сводятся в моно усреднением,
после чего значения нормализуются в диапазон около [-1, 1]. Возвращаются сэмплы и частота
дискретизации исходного файла для последующего приведения к рабочей частоте кодека.
*/
package codec

import (
	"encoding/binary"
	"fmt"
	"io"
)

func ReadWAV(r io.Reader) ([]float32, uint32, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, err
	}
	if len(data) < 44 {
		return nil, 0, fmt.Errorf("file too short to be a WAV")
	}

	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("not a WAV file")
	}

	channels := binary.LittleEndian.Uint16(data[22:24])
	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	bitsPerSample := binary.LittleEndian.Uint16(data[34:36])

	pos := 12
	for pos+8 < len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		pos += 8
		if chunkSize < 0 {
			return nil, 0, fmt.Errorf("invalid WAV chunk size")
		}
		if pos+chunkSize > len(data) {
			return nil, 0, fmt.Errorf("invalid WAV chunk size: exceeds file bounds")
		}
		if chunkID == "data" {
			pcmData := data[pos : pos+chunkSize]
			samples, err := decodePCM(pcmData, channels, bitsPerSample)
			return samples, sampleRate, err
		}
		pos += chunkSize
		if chunkSize%2 == 1 && pos < len(data) {
			pos++
		}
	}

	return nil, 0, fmt.Errorf("no data chunk found")
}

func decodePCM(data []byte, channels, bits uint16) ([]float32, error) {
	if bits != 16 {
		return nil, fmt.Errorf("only 16-bit WAV supported, got %d", bits)
	}

	frameSize := int(channels) * 2
	nFrames := len(data) / frameSize
	samples := make([]float32, nFrames)

	for i := 0; i < nFrames; i++ {
		offset := i * frameSize
		var sum float64
		for ch := 0; ch < int(channels); ch++ {
			v := int16(binary.LittleEndian.Uint16(data[offset+ch*2:]))
			sum += float64(v)
		}
		samples[i] = float32(sum / float64(channels) / 32768.0)
	}
	return samples, nil
}

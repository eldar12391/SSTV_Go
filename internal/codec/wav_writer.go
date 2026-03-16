/*
wav_writer.go выполняет упаковку PCM-сэмплов float32 в WAV-контейнер 16-bit PCM mono.
Файл формирует RIFF/WAVE заголовок, записывает fmt/data чанки и последовательно сериализует
аудиоданные в little-endian формате. Перед записью значения ограничиваются допустимым
диапазоном амплитуды, чтобы избежать переполнения при преобразовании к int16.
Этот модуль используется сервером при отдаче результата кодирования изображения в аудио.
*/
package codec

import (
	"encoding/binary"
	"io"
)

func WriteWAV(w io.Writer, samples []float32, sampleRate uint32) error {
	dataLen := uint32(len(samples) * 2)
	wr := func(v any) error { return binary.Write(w, binary.LittleEndian, v) }

	io.WriteString(w, "RIFF"); wr(36 + dataLen)
	io.WriteString(w, "WAVE")
	io.WriteString(w, "fmt "); wr(uint32(16)); wr(uint16(1)); wr(uint16(1))
	wr(sampleRate); wr(sampleRate * 2); wr(uint16(2)); wr(uint16(16))
	io.WriteString(w, "data"); wr(dataLen)

	for _, s := range samples {
		if s > 1 {
			s = 1
		}
		if s < -1 { s = -1 }
		v := int16(s * 32767)
		if err := wr(v); err != nil {
			return err
		}
	}
	return nil
}

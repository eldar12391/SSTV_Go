/*
encoder.go реализует прямое кодирование изображения в аудиосигнал Martin M1.
Алгоритм масштабирует входной кадр к целевому формату 320x256, формирует VIS-заголовок,
после чего построчно генерирует синхроимпульс и три цветовых канала в порядке G/B/R.
Для каждого пикселя вычисляется частота в рабочем диапазоне SSTV, а непрерывная фаза
синусоиды сохраняется между сегментами, чтобы избежать спектральных разрывов и упростить
стабильный декод на принимающей стороне.
*/
package codec

import (
	"image"
	"math"
)

func tone(dst []float32, phase *float64, freq, ms float64) []float32 {
	n := msToSamples(ms)
	step := 2 * math.Pi * freq / SampleRate
	for i := 0; i < n; i++ {
		dst = append(dst, float32(math.Sin(*phase)))
		*phase += step
		if *phase > 2*math.Pi {
			*phase -= 2 * math.Pi
		}
	}
	return dst
}

func Encode(img image.Image) []float32 {
	rgba := scaleRGBA(img, Width, Height)
	phase := 0.0

	pcm := make([]float32, 0, SampleRate*120)

	pcm = tone(pcm, &phase, 1900, 300)
	pcm = tone(pcm, &phase, 1200, 10)
	pcm = tone(pcm, &phase, 1900, 300)
	pcm = tone(pcm, &phase, 1200, 30)

	for b := 0; b < 7; b++ {
		if (VISCode>>b)&1 == 1 {
			pcm = tone(pcm, &phase, 1100, 30)
		} else {
			pcm = tone(pcm, &phase, 1300, 30)
		}
	}
	pcm = tone(pcm, &phase, 1200, 30)

	pxMs := ChannelMs / float64(Width)

	for y := 0; y < Height; y++ {
		pcm = tone(pcm, &phase, FreqSync, LineSyncMs)
		pcm = tone(pcm, &phase, FreqSep, SepMs)

		for _, ch := range [3]int{1, 2, 0} {
			for x := 0; x < Width; x++ {
				px := rgba.Pix[(y*Width+x)*4+ch]
				pcm = tone(pcm, &phase, PixToFreq(px), pxMs)
			}
			pcm = tone(pcm, &phase, FreqSep, SepMs)
		}
	}

	return pcm
}

/*
constants.go хранит базовые параметры протокола Martin M1 и преобразования между
уровнем яркости пикселя и несущей частотой. Здесь заданы геометрия кадра, частоты
синхроимпульсов и видеодиапазона, длительности элементов строки и VIS-код режима.
Также файл содержит вспомогательные функции для прямого и обратного маппинга
яркости в частоту и пересчета времени из миллисекунд в количество PCM-сэмплов.
Эти константы являются опорной моделью для кодера и декодера.
*/
package codec

import "math"

const (
	Width  = 320
	Height = 256
	SampleRate = 44100
	FreqBlack = 1500.0
	FreqWhite = 2300.0
	FreqSync = 1200.0
	FreqSep  = 1500.0
	LineSyncMs  = 4.862
	SepMs       = 0.572
	ChannelMs   = 146.432
	VISCode = 44
)

func PixToFreq(v uint8) float64 {
	return FreqBlack + (FreqWhite-FreqBlack)*(float64(v)/255.0)
}

func FreqToPix(f float64) uint8 {
	v := math.Round(255.0 * (f - FreqBlack) / (FreqWhite - FreqBlack))
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func msToSamples(ms float64) int {
	return int(math.Round(SampleRate * ms / 1000.0))
}

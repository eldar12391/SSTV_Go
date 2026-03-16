/*
decoder.go выполняет обратное преобразование Martin M1 SSTV из PCM-сэмплов в изображение.
Файл решает три задачи: находит начало видеолиний после VIS-секции, стабилизирует синхронизацию
каждой строки с поправкой на дрейф тайминга и извлекает яркость пикселей по частоте в диапазоне
1500-2300 Гц для каналов G/B/R. Для устойчивости применяются два способа оценки частоты
(автокорреляция как основной и zero-crossing как fallback), а также построчная фильтрация одиночных
цветовых выбросов. Итогом работы является RGBA-кадр 320x256, где альфа-канал заполняется полностью.
*/
package codec

import (
	"image"
	"math"
)

func blockFreq(samples []float32, offset, length int) float64 {
	if length <= 1 || offset < 0 || offset >= len(samples)-1 {
		return 0
	}

	crossings := 0
	end := offset + length
	if end > len(samples)-1 {
		end = len(samples) - 1
	}
	if end-offset <= 1 {
		return 0
	}
	for i := offset; i < end; i++ {
		if (samples[i] >= 0) != (samples[i+1] >= 0) {
			crossings++
		}
	}

	cycles := float64(crossings) * 0.5
	return cycles * SampleRate / float64(end-offset)
}

func blockFreqCentered(samples []float32, center, length int) float64 {
	if length < 8 {
		length = 8
	}
	offset := center - length/2
	if offset < 0 {
		offset = 0
	}
	if offset+length >= len(samples) {
		offset = len(samples) - length - 1
	}
	if offset < 0 {
		return 0
	}
	return blockFreq(samples, offset, length)
}

func blockFreqAuto(samples []float32, center, length int, minF, maxF float64) float64 {
	if length < 24 {
		length = 24
	}
	offset := center - length/2
	if offset < 0 {
		offset = 0
	}
	if offset+length >= len(samples) {
		offset = len(samples) - length - 1
	}
	if offset < 0 {
		return 0
	}

	minLag := int(math.Floor(SampleRate / maxF))
	maxLag := int(math.Ceil(SampleRate / minF))
	if minLag < 1 {
		minLag = 1
	}
	if maxLag >= length-2 {
		maxLag = length - 3
	}
	if minLag > maxLag {
		return 0
	}

	bestLag := minLag
	bestScore := -math.MaxFloat64
	for lag := minLag; lag <= maxLag; lag++ {
		var sum float64
		for i := 0; i < length-lag; i++ {
			a := float64(samples[offset+i])
			b := float64(samples[offset+i+lag])
			sum += a * b
		}
		if sum > bestScore {
			bestScore = sum
			bestLag = lag
		}
	}

	if bestLag <= 0 {
		return 0
	}

	return SampleRate / float64(bestLag)
}

func lineStartScore(samples []float32, pos, syncSamples, sepSamples int) float64 {
	if pos < 0 || pos+syncSamples+sepSamples >= len(samples) {
		return math.MaxFloat64
	}
	fSync := blockFreq(samples, pos, syncSamples)
	fSep := blockFreq(samples, pos+syncSamples, sepSamples)
	return math.Abs(fSync-FreqSync) + 0.6*math.Abs(fSep-FreqSep)
}

func refineLineStart(samples []float32, predicted, syncSamples, sepSamples, radius int) int {
	if predicted < 0 {
		predicted = 0
	}
	start := predicted - radius
	if start < 0 {
		start = 0
	}
	end := predicted + radius
	maxPos := len(samples) - (syncSamples + sepSamples + 1)
	if end > maxPos {
		end = maxPos
	}
	if start >= end {
		return predicted
	}

	bestPos := predicted
	bestScore := math.MaxFloat64
	for p := start; p <= end; p += 2 {
		score := lineStartScore(samples, p, syncSamples, sepSamples)
		if score < bestScore {
			bestScore = score
			bestPos = p
		}
	}
	return bestPos
}

func stabilizeInitialStart(samples []float32, coarse, lineSamples, syncSamples, sepSamples int) int {
	radius := syncSamples * 16
	start := coarse - radius
	if start < 0 {
		start = 0
	}
	end := coarse + radius
	maxPos := len(samples) - (syncSamples + sepSamples + 1)
	if end > maxPos {
		end = maxPos
	}
	if start >= end {
		return coarse
	}

	bestPos := coarse
	bestScore := math.MaxFloat64
	probeLines := 6
	for p := start; p <= end; p += 2 {
		score := 0.0
		valid := true
		for k := 0; k < probeLines; k++ {
			lp := p + k*lineSamples
			if lp+syncSamples+sepSamples >= len(samples) {
				valid = false
				break
			}
			score += lineStartScore(samples, lp, syncSamples, sepSamples)
		}
		if !valid {
			continue
		}
		if score < bestScore {
			bestScore = score
			bestPos = p
		}
	}

	return bestPos
}

func despike(line []uint8) {
	if len(line) < 3 {
		return
	}
	for i := 1; i < len(line)-1; i++ {
		l := int(line[i-1])
		c := int(line[i])
		r := int(line[i+1])
		avg := (l + r) / 2
		if absInt(c-avg) > 48 && absInt(l-r) < 28 {
			line[i] = uint8(avg)
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func smoothTopRows(img *image.RGBA, rows int) {
	b := img.Bounds()
	h := b.Dy()
	w := b.Dx()
	if h < 3 || rows <= 0 {
		return
	}
	if rows > h-2 {
		rows = h - 2
	}

	for y := 0; y < rows; y++ {
		y1 := y + 1
		y2 := y + 2
		for x := 0; x < w; x++ {
			i0 := img.PixOffset(x, y)
			i1 := img.PixOffset(x, y1)
			i2 := img.PixOffset(x, y2)
			for ch := 0; ch < 3; ch++ {
				v0 := int(img.Pix[i0+ch])
				v1 := int(img.Pix[i1+ch])
				v2 := int(img.Pix[i2+ch])
				if absInt(v0-v1) > 42 && absInt(v1-v2) < 24 {
					img.Pix[i0+ch] = uint8((v1 + v2) / 2)
				}
			}
		}
	}
}

func findLineStart(samples []float32, syncSamples, sepSamples int) int {
	visMs := 300.0 + 10.0 + 300.0 + 30.0 + 7*30.0 + 30.0
	expected := msToSamples(visMs)

	start := expected - int(0.20*SampleRate)
	if start < 0 {
		start = 0
	}
	end := expected + int(2.0*SampleRate)
	if end > len(samples)-(syncSamples+sepSamples) {
		end = len(samples) - (syncSamples + sepSamples)
	}

	bestPos := expected
	bestScore := math.MaxFloat64
	for i := start; i < end; i += 20 {
		score := lineStartScore(samples, i, syncSamples, sepSamples)
		if score < bestScore {
			bestScore = score
			bestPos = i
		}
	}

	if bestScore < 180 {
		return bestPos
	}

	searchBlock := int(0.03 * SampleRate)
	for i := 0; i < min(len(samples)-searchBlock, SampleRate*3); i += 100 {
		f := blockFreq(samples, i, searchBlock)
		if f > 1150 && f < 1250 {
			return i
		}
	}

	if expected < len(samples) {
		return expected
	}
	return 0
}

func Decode(samples []float32) *image.RGBA {
	pxSamples := msToSamples(ChannelMs / float64(Width))
	chanSamples := pxSamples * Width
	syncSamples := msToSamples(LineSyncMs)
	sepSamples := msToSamples(SepMs)
	lineSamples := syncSamples + sepSamples + (chanSamples+sepSamples)*3

	cursor := findLineStart(samples, syncSamples, sepSamples)
	cursor = stabilizeInitialStart(samples, cursor, lineSamples, syncSamples, sepSamples)

	img := image.NewRGBA(image.Rect(0, 0, Width, Height))

	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			img.Pix[img.PixOffset(x, y)+3] = 255
		}
	}

	freqWin := pxSamples * 4
	if freqWin < 64 {
		freqWin = 64
	}

	lineCh := [3][]uint8{
		make([]uint8, Width),
		make([]uint8, Width),
		make([]uint8, Width),
	}

	for y := 0; y < Height; y++ {
		expected := cursor
		radius := syncSamples * 8
		if y == 0 {
			radius = syncSamples * 14
		} else {
			expected = cursor + lineSamples
		}
		cursor = refineLineStart(samples, expected, syncSamples, sepSamples, radius)

		if cursor+lineSamples > len(samples) {
			break
		}

		pos := cursor + syncSamples + sepSamples

		for _, ch := range [3]int{1, 2, 0} {
			for x := 0; x < Width; x++ {
				center := pos + pxSamples/2
				f := blockFreqAuto(samples, center, freqWin, FreqBlack, FreqWhite)
				if f == 0 {
					f = blockFreqCentered(samples, center, freqWin)
				}
				lineCh[ch][x] = FreqToPix(f)
				pos += pxSamples
			}
			pos += sepSamples
		}

		for _, ch := range [3]int{0, 1, 2} {
			despike(lineCh[ch])
		}

		for x := 0; x < Width; x++ {
			idx := img.PixOffset(x, y)
			img.Pix[idx+0] = lineCh[0][x]
			img.Pix[idx+1] = lineCh[1][x]
			img.Pix[idx+2] = lineCh[2][x]
		}
	}

	smoothTopRows(img, 8)

	return img
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

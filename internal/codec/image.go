/*
image.go содержит утилиту масштабирования изображений для кодека SSTV.
Функция выполняет преобразование исходного image.Image в image.RGBA заданного размера
через nearest-neighbour, что обеспечивает предсказуемое и быстрое поведение без внешних
зависимостей. Этот шаг используется перед кодированием, чтобы входные данные строго
соответствовали геометрии режима Martin M1.
*/
package codec

import (
	"image"
)

func scaleRGBA(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := b.Min.X + x*sw/w
			sy := b.Min.Y + y*sh/h
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

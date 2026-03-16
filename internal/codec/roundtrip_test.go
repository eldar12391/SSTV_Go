/*
roundtrip_test.go содержит интеграционную проверку полного цикла кодирования и
декодирования SSTV внутри пакета codec. Тест генерирует синтетический градиент,
прогоняет его через Encode/Decode и убеждается, что результат сохраняет
пространственную вариативность по горизонтали и вертикали, а также не
вырождается в выраженный однотонный сине-фиолетовый артефакт.
*/
package codec

import (
	"image"
	"image/color"
	"testing"
)

func TestRoundtripHasSignalVariance(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, Width, Height))
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			r := uint8((x * 255) / (Width - 1))
			g := uint8((y * 255) / (Height - 1))
			b := uint8(((x + y) * 255) / (Width + Height - 2))
			src.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	pcm := Encode(src)
	dec := Decode(pcm)

	left := dec.RGBAAt(0, Height/2)
	right := dec.RGBAAt(Width-1, Height/2)
	top := dec.RGBAAt(Width/2, 0)
	bottom := dec.RGBAAt(Width/2, Height-1)

	if left == right {
		t.Fatalf("decoded image has no horizontal variance: left=%+v right=%+v", left, right)
	}
	if top == bottom {
		t.Fatalf("decoded image has no vertical variance: top=%+v bottom=%+v", top, bottom)
	}

	if left.B > left.R+80 && left.B > left.G+80 && right.B > right.R+80 {
		t.Fatalf("decoded image appears heavily blue/purple biased: left=%+v right=%+v", left, right)
	}
}

package utils

import (
	"image"
	"image/color"
	"testing"

	"github.com/kovidgoyal/imaging"
)

func TestImageProcessingPaletteBounds(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		name := "valid palette"
		if invalid {
			name = "out of range palette index"
		}
		t.Run(name, func(t *testing.T) {
			red := color.NRGBA{R: 255, A: 255}
			img := image.NewPaletted(image.Rect(0, 0, 3, 2), color.Palette{red})
			want := red
			if invalid {
				// CVE-2023-36308: decoded pixels may reference a missing palette entry.
				for i := range img.Pix {
					img.Pix[i] = 255
				}
				want = color.NRGBA{}
			}
			for operation, result := range map[string]image.Image{
				"group photo crop": cropToSquare(img),
				"media resize":     imaging.Resize(img, 2, 2, imaging.Lanczos),
			} {
				if result.Bounds() != image.Rect(0, 0, 2, 2) {
					t.Fatalf("%s: unexpected bounds %v", operation, result.Bounds())
				}
				for y := 0; y < 2; y++ {
					for x := 0; x < 2; x++ {
						if got := color.NRGBAModel.Convert(result.At(x, y)); got != want {
							t.Fatalf("%s: pixel (%d,%d) = %v, want %v", operation, x, y, got, want)
						}
					}
				}
			}
		})
	}
}

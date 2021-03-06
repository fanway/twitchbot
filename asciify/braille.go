package asciify

import (
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func Braille(img image.Image, maxW int, reverse bool, thMult float32) string {
	b := img.Bounds()
	imageWidth := b.Max.X
	imageHeight := b.Max.Y
	var w, h int
	ratio := float32(imageHeight) / float32(imageWidth)
	// scale image to fit into given width and braille unicode character (1 braille symbol is 2x4)
	if imageWidth != maxW*2 {
		w = 2 * maxW
		h = 4 * int((float32(w) * ratio / 4))
	} else {
		w = imageWidth
		h = imageHeight
	}
	rect := image.Rect(0, 0, w, h)
	img1 := image.NewRGBA(rect)

	hRatio := float32(imageHeight) / float32(h)
	wRatio := float32(imageWidth) / float32(w)

	var th uint32

	// grayscale an image and count a threshold as an average value of the pixel intensity
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			r, g, b, _ := img.At(int(float32(x)*wRatio), int(float32(y)*hRatio)).RGBA()
			// r*0.299 + g*0.587 + b*0.114
			r = uint32(0.299 * float32(r))
			g = uint32(0.587 * float32(g))
			b = uint32(0.114 * float32(b))
			rgb := (r + g + b) >> 8
			th += rgb
			img1.Set(x, y, color.Gray{uint8(rgb)})
		}
	}

	th /= uint32(w * h)
	th = uint32(float32(th) * thMult)

	// braillify/asciify algorithm:
	// https://en.wikipedia.org/wiki/Braille_Patterns
	// We are going top to bottom, left to right
	// Considering each patter as a binary number, turn on the current bit if
	// the intensity in the pixel is more or equal (less) than a threshold
	// After that, the binary number represents an offset starting from 0x2800 (first braille unicode symbol)
	output := ""
	for imgY := 0; imgY < h; imgY += 4 {
		for imgX := 0; imgX < w; imgX += 2 {
			curr := 0
			currIdx := 1
			for y := 0; y < 4; y++ {
				if y == 3 {
					currIdx = 64
				}
				for x := 0; x < 2; x++ {
					r, g, b, _ := img1.At(imgX+x, imgY+y).RGBA()
					score := (r + g + b) / (3 * 256)
					condition := false
					if reverse {
						condition = score < th
					} else {
						condition = score >= th
					}
					if condition {
						if y != 3 {
							curr |= currIdx << (x * 3)
						} else {
							curr |= currIdx << x
						}
					}
				}
				currIdx <<= 1
			}
			output += string(0x2800 + curr)
		}
		output += " "
	}
	return output
}

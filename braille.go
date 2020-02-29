package main

import (
	"fmt"
	"image"
	_ "image/png"
	"log"
	"os"
)

func Braille() {
	reader, err := os.Open("4H.png")
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	img, _, err := image.Decode(reader)
	if err != nil {
		log.Fatal(err)
	}
	b := img.Bounds()
	maxW := 30
	imageWidth := b.Max.X
	imageHeight := b.Max.Y
	var w, h int
	ratio := float32(imageHeight) / float32(imageWidth)
	if imageWidth != maxW*2 {
		w = 2 * maxW
		h = 4 * int((float32(w) * ratio / 4))
	} else {
		w = imageWidth
		h = imageHeight
	}
	fmt.Println(w, h)

	rect := image.Rect(0, 0, w, h)
	img1 := image.NewRGBA(rect)

	hRatio := float32(imageHeight) / float32(h)
	wRatio := float32(imageWidth) / float32(w)
	fmt.Println(hRatio, wRatio)

	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			p := img.At(int(float32(x)*wRatio), int(float32(y)*hRatio))
			img1.Set(x, y, p)
		}
	}

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
					avg := (r + g + b) / (3 * 255)
					if avg > 150 {
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
		output += "\n"
	}

	fmt.Println(output)
}

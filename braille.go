package main

import (
	"database/sql"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func EmoteCache(reverse bool, url string, width int, rewrite bool, thMult float32) string {
	db := ConnectDb()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
	}
	defer tx.Rollback()
	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS emoteCache(url TEXT NOT NULL,image TEXT NOT NULL, UNIQUE (url) ON CONFLICT REPLACE);")
	if err != nil {
		fmt.Println(err)
	}
	var str string
	if err := tx.QueryRow("SELECT image FROM emoteCache WHERE url=$1;", url).Scan(&str); err == sql.ErrNoRows || rewrite {
		str = AsciifyRequest(url, width, reverse, thMult)
		_, err = tx.Exec("INSERT INTO emoteCache(url, image) VALUES($1,$2);", url, str)
		if err != nil {
			fmt.Println(err)
		}
	}
	tx.Commit()
	return str
}

func Braille(img image.Image, maxW int, reverse bool, thMult float32) string {
	b := img.Bounds()
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
	rect := image.Rect(0, 0, w, h)
	img1 := image.NewRGBA(rect)

	hRatio := float32(imageHeight) / float32(h)
	wRatio := float32(imageWidth) / float32(w)

	var th uint32

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

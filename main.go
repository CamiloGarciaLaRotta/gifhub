package main

import (
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"log"
	"math/rand"
	"os"
	"time"
)

func main() {
	h, w := 1000, 1000
	numFrames := 5

	var delays = make([]int, numFrames)
	var frames = make([]*image.Paletted, numFrames)
	for i := 0; i < numFrames; i++ {
		delays[i] = 100
		frames[i] = randomFrame(h, w)
	}

	g := gif.GIF{Delay: delays, Image: frames}

	f, err := os.Create("yyy.gif")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := gif.EncodeAll(f, &g); err != nil {
		log.Fatal(err)
	}
}

func randomFrame(w, h int) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, w, h), palette.Plan9)
	draw.Draw(img, img.Rect, &image.Uniform{randomColor()}, image.ZP, draw.Src)
	return img
}

func randomColor() color.RGBA {
	rand.Seed(time.Now().UnixNano())
	r, g, b := rand.Intn(255), rand.Intn(255), rand.Intn(255)
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}

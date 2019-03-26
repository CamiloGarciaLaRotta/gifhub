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

	"github.com/llgcode/draw2d/draw2dimg"
	//NOTE: gfx is not using modules, so you'll have to work inside your $GOPATH
	"github.com/peterhellberg/gfx"
)

func main() {
	// DONE
	// - create GIFs via image stdlib
	// - create GIFs via gfx & draw2d
	// TODO
	// - write text in a GIF
	//   see:
	//   https://stackoverflow.com/questions/38299930/how-to-add-a-simple-text-label-to-an-image-in-go
	//   https://godoc.org/github.com/golang/freetype
	//
	// - benchmark which approach is fastest to create gifhub GIFs

	// generate gif via peterhellberg/gfx
	stdlibGIF("stdlib.gif")

	// generate gif via peterhellberg/gfx
	gfxGIF("gfx.gif")

	// generate gif via llgcode/draw2d
	draw2dGIF("draw2d.gif")
}

func gfxGIF(output string) {
	rand.Seed(time.Now().UnixNano())

	var en4 = gfx.PaletteEN4

	a := &gfx.Animation{Delay: 15}

	// add circles and a polygon to a paletted image
	// then add them to the gfx animation
	for i := 0; i < 10; i++ {
		p := gfx.Polygon{
			{rand.Float64() * 1000, rand.Float64() * 1000},
			{rand.Float64() * 1000, rand.Float64() * 1000},
			{rand.Float64() * 1000, rand.Float64() * 1000},
			{rand.Float64() * 1000, rand.Float64() * 1000},
		}

		m := gfx.NewPaletted(1000, 1000, en4, en4.Color(7))
		gfx.DrawPolygon(m, p, 0, en4.Color(2))

		gfx.DrawCircleFilled(m, gfx.V(rand.Float64()*1000, rand.Float64()*1000), 25, en4.Color(0))
		gfx.DrawCircleFilled(m, gfx.V(rand.Float64()*1000, rand.Float64()*1000), 25, en4.Color(0))
		gfx.DrawCircleFilled(m, gfx.V(rand.Float64()*1000, rand.Float64()*1000), 25, en4.Color(1))
		gfx.DrawCircleFilled(m, gfx.V(rand.Float64()*1000, rand.Float64()*1000), 25, en4.Color(1))

		a.AddPalettedImage(m)
	}

	a.SaveGIF(output)
}

func draw2dGIF(output string) {
	lightGreen := color.RGBA{123, 201, 111, 0xff}
	darkGreen := color.RGBA{108, 178, 103, 0xff}

	// draw the images using an RGBA image and the graphic context of draw2d library
	dest1 := image.NewRGBA(image.Rect(0, 0, 1000, 1000))
	gcImg1 := draw2dimg.NewGraphicContext(dest1)

	dest2 := image.NewRGBA(image.Rect(0, 0, 1000, 1000))
	gcImg2 := draw2dimg.NewGraphicContext(dest2)

	// draw polygon
	gcImg1.SetStrokeColor(lightGreen)
	gcImg1.SetFillColor(lightGreen)
	gcImg1.MoveTo(150, 500)
	gcImg1.LineTo(500, 850)
	gcImg1.LineTo(850, 500)
	gcImg1.LineTo(500, 150)
	gcImg1.Close()
	gcImg1.FillStroke()

	// draw axis
	gcImg1.SetStrokeColor(darkGreen)
	gcImg1.SetLineWidth(10)
	gcImg1.BeginPath()
	gcImg1.MoveTo(100, 500)
	gcImg1.LineTo(900, 500)
	gcImg1.MoveTo(500, 100)
	gcImg1.LineTo(500, 900)
	gcImg1.FillStroke()

	gcImg2.SetStrokeColor(darkGreen)
	gcImg2.SetLineWidth(10)
	gcImg2.BeginPath()
	gcImg2.MoveTo(100, 500)
	gcImg2.LineTo(900, 500)
	gcImg2.MoveTo(500, 100)
	gcImg2.LineTo(500, 900)
	gcImg2.FillStroke()

	// create a paletted image from the RGBA images
	img1 := image.NewPaletted(dest1.Bounds(), palette.Plan9)
	draw.Draw(img1, img1.Rect, dest1, dest1.Bounds().Min, draw.Src)

	img2 := image.NewPaletted(dest2.Bounds(), palette.Plan9)
	draw.Draw(img2, img2.Rect, dest2, dest2.Bounds().Min, draw.Src)

	// encode the GIF
	anim := gif.GIF{Delay: []int{25, 25}, Image: []*image.Paletted{img1, img2}}

	f, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	gif.EncodeAll(f, &anim)
	f.Close()
}

func stdlibGIF(output string) {
	h, w := 1000, 1000
	numFrames := 5

	var delays = make([]int, numFrames)
	var frames = make([]*image.Paletted, numFrames)
	for i := 0; i < numFrames; i++ {
		delays[i] = 100
		frames[i] = randomFrame(h, w)
	}

	g := gif.GIF{Delay: delays, Image: frames}

	f, err := os.Create(output)
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

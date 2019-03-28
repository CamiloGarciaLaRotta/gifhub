package main

import (
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/gobold"

	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/fogleman/gg"
	"github.com/llgcode/draw2d/draw2dimg"
	//NOTE: gfx is not using modules, so you'll have to work inside your $GOPATH
	"github.com/peterhellberg/gfx"
)

func main() {
	// DONE
	// - create GIFs via image stdlib
	// - create GIFs via gfx & draw2d & gg
	// TODO
	// - write text in all GIFs
	//   see:
	//   https://stackoverflow.com/questions/38299930/how-to-add-a-simple-text-label-to-an-image-in-go
	//   https://godoc.org/github.com/golang/freetype
	//
	// - benchmark which lib (gfx, draw2d, gg) is fastest to create gifhub GIFs

	const (
		w = 1000
		h = 1100
	)
	// generate gif via peterhellberg/gfx
	stdlibGIF(w, h, "stdlib.gif")

	// generate gif via peterhellberg/gfx
	gfxGIF(w, h, "gfx.gif")

	// generate gif via llgcode/draw2d
	draw2dGIF(w, h, "draw2d.gif")

	// generate gif via
	ggGIF(w, h, "gg.gif")
}

func gfxGIF(w, h int, output string) {
	rand.Seed(time.Now().UnixNano())

	var en4 = gfx.PaletteEN4

	a := &gfx.Animation{Delay: 15}

	// add circles and a polygon to a paletted image
	// then add them to the gfx animation
	for i := 0; i < 10; i++ {
		p := gfx.Polygon{
			{rand.Float64() * float64(w), rand.Float64() * float64(h)},
			{rand.Float64() * float64(w), rand.Float64() * float64(h)},
			{rand.Float64() * float64(w), rand.Float64() * float64(h)},
			{rand.Float64() * float64(w), rand.Float64() * float64(h)},
		}

		m := gfx.NewPaletted(w, h, en4, en4.Color(7))
		gfx.DrawPolygon(m, p, 0, en4.Color(2))

		gfx.DrawCircleFilled(m, gfx.V(rand.Float64()*float64(w), rand.Float64()*float64(h)), 25, en4.Color(0))
		gfx.DrawCircleFilled(m, gfx.V(rand.Float64()*float64(w), rand.Float64()*float64(h)), 25, en4.Color(0))
		gfx.DrawCircleFilled(m, gfx.V(rand.Float64()*float64(w), rand.Float64()*float64(h)), 25, en4.Color(1))
		gfx.DrawCircleFilled(m, gfx.V(rand.Float64()*float64(w), rand.Float64()*float64(h)), 25, en4.Color(1))

		a.AddPalettedImage(m)
	}

	a.SaveGIF(output)
}

func draw2dGIF(w, h int, output string) {
	lightGreen := color.RGBA{123, 201, 111, 0xff}
	darkGreen := color.RGBA{108, 178, 103, 0xff}

	// draw the images using an RGBA image and the graphic context of draw2d library
	dest1 := image.NewRGBA(image.Rect(0, 0, w, h))
	gcImg1 := draw2dimg.NewGraphicContext(dest1)

	dest2 := image.NewRGBA(image.Rect(0, 0, w, h))
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

	GIF(output, 25, dest1, dest2)
}

func ggGIF(w, h int, output string) {
	font, err := truetype.Parse(gobold.TTF)
	if err != nil {
		log.Fatal(err)
	}
	label := truetype.NewFace(font, &truetype.Options{Size: 48})
	value := truetype.NewFace(font, &truetype.Options{Size: 36})

	rand.Seed(time.Now().UnixNano())

	lightGreen := color.RGBA{123, 201, 111, 0xff}
	darkGreen := color.RGBA{108, 178, 103, 0xff}

	imgs := []image.Image{}
	for i := 0; i < 5; i++ {
		dc := gg.NewContext(w, h)
		dc.SetColor(color.White)
		dc.Clear()

		cr := float64(rand.Intn(250)) + 250
		is := float64(rand.Intn(250)) + 500
		pr := float64(rand.Intn(250)) + 500
		cm := float64(rand.Intn(250)) + 250

		// draw polygon
		dc.SetColor(lightGreen)
		dc.MoveTo(cr, 500)
		dc.LineTo(500, is)
		dc.LineTo(pr, 500)
		dc.LineTo(500, cm)
		dc.Fill()

		// draw axis
		dc.SetLineWidth(5)
		dc.SetColor(darkGreen)
		dc.DrawLine(250, 500, 750, 500)
		dc.DrawLine(500, 250, 500, 750)
		dc.Stroke()

		// draw circles
		circle(darkGreen, color.White, 8, cr, 500, dc)
		circle(darkGreen, color.White, 8, 500, is, dc)
		circle(darkGreen, color.White, 8, pr, 500, dc)
		circle(darkGreen, color.White, 8, 500, cm, dc)

		// draw text
		dc.SetFontFace(label)
		dc.SetRGB(149, 157, 165)
		dc.DrawStringAnchored("Handle", 500, 950, 0.5, 0.5)
		dc.DrawStringAnchored("year", 500, 1000, 0.5, 0.5)
		dc.DrawStringAnchored("Code Review", 500, 180, 0.5, 0.5)
		dc.DrawStringAnchored("Issues", 870, 520, 0.5, 0.5)
		dc.DrawStringAnchored("Pull Requests", 500, 850, 0.5, 0.5)
		dc.DrawStringAnchored("Commits", 130, 520, 0.5, 0.5)

		dc.SetFontFace(value)
		dc.SetRGB(88, 96, 105)
		dc.DrawStringAnchored("10%", 500, 130, 0.5, 0.5)
		dc.DrawStringAnchored("20%", 880, 475, 0.5, 0.5)
		dc.DrawStringAnchored("50%", 500, 800, 0.5, 0.5)
		dc.DrawStringAnchored("90%", 130, 475, 0.5, 0.5)

		imgs = append(imgs, dc.Image())
	}

	GIF(output, 35, imgs...)
}

// TODO non hardcoded inner radius
func circle(outerColor, innerColor color.Color, r, x, y float64, dc *gg.Context) {
	dc.SetColor(color.White)
	dc.DrawCircle(x, y, r)
	dc.FillPreserve()
	dc.SetColor(outerColor)
	dc.SetLineWidth(4)
	dc.Stroke()
	dc.SetColor(innerColor)
}

// GIF creates an output gif file with the input images as frames
func GIF(output string, delay int, images ...image.Image) {
	numFrames := len(images)
	palettedImgs := []*image.Paletted{}
	for _, img := range images {
		paletted := image.NewPaletted(img.Bounds(), palette.Plan9)
		draw.Draw(paletted, paletted.Rect, img, img.Bounds().Min, draw.Src)
		palettedImgs = append(palettedImgs, paletted)
	}

	var delays = make([]int, numFrames)
	for i := 0; i < numFrames; i++ {
		delays[i] = delay
	}

	anim := gif.GIF{Delay: delays, Image: palettedImgs}

	f, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	gif.EncodeAll(f, &anim)
	f.Close()
}
func stdlibGIF(w, h int, output string) {
	numFrames := 5

	var delays = make([]int, numFrames)
	var frames = make([]*image.Paletted, numFrames)
	for i := 0; i < numFrames; i++ {
		delays[i] = 50
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

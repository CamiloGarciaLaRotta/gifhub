package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/fogleman/gg"
	"github.com/urfave/cli/v2"
)

// activity contains GitHub's tracked user activity percentages for a given year
type activity struct {
	Handle, Year                      string
	Commits, Issues, Prs, CodeReviews int
}

// coords contains the X,Y coordinates of the activities in an activity graph.
// Aswell as measurements used to calculate margins and offsets
type coords struct {
	W, H, Mid, Factor, AxisMargin,
	CodeReviewY, IssuesX, PrsY, CommitsX float64
}

// style contains the style attributes of the graph such as font, colors, and size of markers
type style struct {
	LabelColor, ValueColor, AxisColor, PolyColor color.Color
	LabelFont, ValueFont                         font.Face
	MarkerRadius                                 float64
}

// graph contains all information to build the graph of a user's activity for a given year
type graph struct {
	Data   activity
	Coords coords
}

// activityImage contains the image encoding of an activity graph
// as well as the year of the graph for identification and sorting
type activityImage struct {
	Img  image.Image
	Year string
}

func main() {
	app := cli.NewApp()
	app.Name = "gifhub"
	app.Usage = "Create GIFs from a people's GitHub activity graph"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "years",
			Aliases: []string{"y"},
			Value:   "all",
			Usage:   "Scrape activityfrom years `2016,2017,2019`",
		},
		&cli.StringFlag{
			Name:    "out-dir",
			Aliases: []string{"o"},
			Usage:   "Save the GIF in the output directory `./dir`",
			Value:   "./out",
		},
		&cli.StringFlag{
			Name:    "delay",
			Aliases: []string{"d"},
			Usage:   "Set the transition delay of the GIF to `50`ms",
			Value:   "100",
		},
	}
	app.Action = generateGIF

	cli.AppHelpTemplate = `NAME:
	 {{.Name}} - {{.Usage}}

USAGE:
   {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}} GitHub-username

GLOBAL OPTIONS:{{if .VisibleFlags}}
{{range .VisibleFlags}}{{.}}
{{end}}{{end}}
`

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

// generateGIF creates a GIF of the activities of the input user
func generateGIF(c *cli.Context) error {
	var userHandle string
	if c.NArg() > 0 {
		userHandle = c.Args().Get(0)
	} else {
		return cli.ShowAppHelp(c)
	}

	outputDir := c.String("out-dir")
	delay := c.Int("delay")
	specificYears, err := parseYearFlag(c.String("years"), userHandle)
	if err != nil {
		return err
	}
	if len(specificYears) == 0 {
		return errors.New("failed to parse any years")
	}

	chanSize := len(specificYears)

	// pipeline source
	yearc := genYears(specificYears, chanSize)

	// processing pipeline
	actc := genActivities(userHandle, yearc, chanSize)
	graphc := genGraph(actc, chanSize)
	imgc := genImg(graphc, chanSize)

	// pipeline sink
	imgs := bundleImgs(imgc)
	if len(imgs) == 0 {
		return fmt.Errorf("Failed to create a single image for %s", userHandle)
	}

	gif, err := encodeGIF(imgs, outputDir, userHandle, delay)
	if err != nil {
		return fmt.Errorf("GIF: %v", err)
	}

	log.Printf("Created: %s\n", gif)

	return nil
}

// genYears fans out every year to scrape the activity into a channel
func genYears(years []string, size int) <-chan string {
	var out = make(chan string, size)
	go func() {
		defer close(out)
		for _, y := range years {
			out <- y
		}
	}()
	return out
}

// genActivities creates and passes activities into a channel for every year in the input channel
func genActivities(handle string, in <-chan string, size int) <-chan activity {
	var out = make(chan activity, size)
	var wg sync.WaitGroup
	wg.Add(size)
	go func() {
		for year := range in {
			go func(year string) {
				defer wg.Done()
				act, err := parseActivity(handle, year)
				if err != nil {
					log.Printf("scrape activity for %s: %v\n", year, err)
					return
				}
				log.Printf("Activity: %+v\n", act)
				out <- act
			}(year)
		}
	}()
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// genGraph creates and passes graphs into a channel for every activity in the input channel
func genGraph(in <-chan activity, size int) <-chan graph {
	var out = make(chan graph, size)
	go func() {
		defer close(out)
		for act := range in {
			out <- graph{act, coordinates(act)}
		}
	}()
	return out
}

// genImg creates and passes images into a channel for every graph description in the input channel
func genImg(in <-chan graph, size int) <-chan activityImage {
	font, err := truetype.Parse(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}
	labelColor := color.RGBA{88, 96, 105, 0xff}
	valueColor := color.RGBA{149, 157, 165, 0xff}
	axisColor := color.RGBA{108, 178, 103, 0xff}
	polyColor := color.RGBA{123, 201, 111, 0xff}

	var out = make(chan activityImage, size)
	var wg sync.WaitGroup
	wg.Add(size)
	activeGoRoutines := 0
	go func() {
		for g := range in {
			activeGoRoutines++
			go func(g graph) {
				defer wg.Done()
				s := style{
					MarkerRadius: 6,
					LabelColor:   labelColor,
					ValueColor:   valueColor,
					AxisColor:    axisColor,
					PolyColor:    polyColor,
					LabelFont:    truetype.NewFace(font, &truetype.Options{Size: 24}),
					ValueFont:    truetype.NewFace(font, &truetype.Options{Size: 22}),
				}
				out <- activityImage{img(g, s), g.Data.Year}
			}(g)
		}
		// when input channel is closed, reduce the waitgroup counter
		// by the number of goroutines that were expected but not created
		for i := 0; i < size-activeGoRoutines; i++ {
			wg.Done()
		}
	}()
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// bundleImgs collects and sorts all the activity images in the input channel
func bundleImgs(in <-chan activityImage) []image.Image {
	// receive all activity images
	unsortedImgs := []activityImage{}
	for i := range in {
		unsortedImgs = append(unsortedImgs, i)
	}
	sort.Slice(unsortedImgs, func(i, j int) bool {
		return unsortedImgs[i].Year < unsortedImgs[j].Year
	})

	// output sorted images
	numFrames := len(unsortedImgs)
	sortedImgs := make([]image.Image, numFrames)
	for i := 0; i < numFrames; i++ {
		sortedImgs[i] = unsortedImgs[i].Img
	}

	return sortedImgs
}

// encodeGIF bundles the frames to create <userhandle>.gif in the output directory
func encodeGIF(frames []image.Image, outputDir, userHandle string, delay int) (string, error) {
	switch {
	case len(frames) == 0:
		return "", errors.New("GIF: no images to bundle")
	case delay == 0:
		return "", errors.New("GIF: no transition delay given")
	}

	// create appropriate image type for GIF encoding
	numFrames := len(frames)
	palettedImgs := []*image.Paletted{}
	for _, f := range frames {
		paletted := image.NewPaletted(f.Bounds(), palette.Plan9)
		draw.Draw(paletted, paletted.Rect, f, f.Bounds().Min, draw.Src)
		palettedImgs = append(palettedImgs, paletted)
	}

	var delays = make([]int, numFrames)
	for i := 0; i < numFrames; i++ {
		delays[i] = delay
	}

	anim := gif.GIF{Delay: delays, Image: palettedImgs}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err := os.Mkdir(outputDir, os.ModePerm); err != nil {
			return "", nil
		}
	}
	fileName := fmt.Sprintf("%s.gif", userHandle)
	file := filepath.Join(".", outputDir, fileName)
	f, err := os.Create(file)
	if err != nil {
		log.Fatal(err)
	}

	if err := gif.EncodeAll(f, &anim); err != nil {
		return "", err
	}

	return f.Name(), f.Close()
}

// html GETs the HTML text of a URL
func html(url string) (body []byte, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(
		"User-Agent",
		"gifhub v0.0 https://www.github.com/camilogarcialarotta/gifhub - This bot generates GIFs from the user's yearly activity graph",
	)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		cerr := res.Body.Close()
		if err == nil {
			err = cerr
		}
	}()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("GET status: %s: %s", res.Status, url)
	}

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// img generates an image from graph values g with the styles defined in s
func img(g graph, s style) image.Image {
	// to reduce cognitive load, unpack most used variables
	w := g.Coords.W
	h := g.Coords.H
	mid := g.Coords.Mid
	factor := g.Coords.Factor
	axisMargin := g.Coords.AxisMargin

	dc := gg.NewContext(int(w), int(h))
	dc.SetColor(color.White)
	dc.Clear()

	// draw polygon
	dc.SetColor(s.PolyColor)
	dc.SetLineWidth(10)
	dc.MoveTo(mid, g.Coords.CodeReviewY)
	dc.LineTo(g.Coords.IssuesX, mid)
	dc.LineTo(mid, g.Coords.PrsY)
	dc.LineTo(g.Coords.CommitsX, mid)
	dc.ClosePath()
	dc.StrokePreserve()
	dc.Fill()

	// draw axis
	dc.SetLineWidth(4)
	dc.SetColor(s.AxisColor)
	dc.DrawLine(axisMargin, mid, w-axisMargin, mid)
	dc.DrawLine(mid, axisMargin, mid, w-axisMargin)
	dc.Stroke()

	// draw circles
	if g.Data.CodeReviews > 0 {
		circle(s.AxisColor, color.White, s.MarkerRadius, mid, g.Coords.CodeReviewY, dc)
	}
	if g.Data.Issues > 0 {
		circle(s.AxisColor, color.White, s.MarkerRadius, g.Coords.IssuesX, mid, dc)
	}
	if g.Data.Prs > 0 {
		circle(s.AxisColor, color.White, s.MarkerRadius, mid, g.Coords.PrsY, dc)
	}
	if g.Data.Commits > 0 {
		circle(s.AxisColor, color.White, s.MarkerRadius, g.Coords.CommitsX, mid, dc)
	}

	// draw text
	dc.SetFontFace(s.LabelFont)
	dc.SetColor(s.LabelColor)
	dc.DrawStringAnchored(g.Data.Handle, mid, h-1.25*factor, 0.5, 0.5)
	dc.DrawStringAnchored(g.Data.Year, mid, h-0.75*factor, 0.5, 0.5)
	dc.DrawStringAnchored("Code Review", mid, 1.5*factor, 0.5, 0.5)
	dc.DrawStringAnchored("Issues", w-1.25*factor, mid+0.25*factor, 0.5, 0.5)
	dc.DrawStringAnchored("Pull Requests", mid, w-1.25*factor, 0.5, 0.5)
	dc.DrawStringAnchored("Commits", 1.25*factor, mid+0.25*factor, 0.5, 0.5)

	dc.SetFontFace(s.ValueFont)
	dc.SetColor(s.ValueColor)
	dc.DrawStringAnchored(fmt.Sprintf("%d%%", g.Data.CodeReviews), mid, factor, 0.5, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("%d%%", g.Data.Issues), w-1.25*factor, mid-0.25*factor, 0.5, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("%d%%", g.Data.Prs), mid, w-1.75*factor, 0.5, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("%d%%", g.Data.Commits), 1.25*factor, mid-0.25*factor, 0.5, 0.5)

	return dc.Image()
}

// circle creates a circle with outer radius r and inner radius r/2
// in the x,y coordinates of the image context
func circle(outerColor, innerColor color.Color, r, x, y float64, dc *gg.Context) {
	dc.SetColor(innerColor)
	dc.DrawCircle(x, y, r)
	dc.FillPreserve()
	dc.SetColor(outerColor)
	dc.SetLineWidth(r / 2)
	dc.Stroke()
}

// parseActivity returns an activity for a GitHub user on a given year
func parseActivity(userHandle, year string) (activity, error) {
	url := fmt.Sprintf("https://github.com/%[1]s?tab=overview&from=%[2]s-01-01&to=%[2]s-12-31", userHandle, year)
	body, err := html(url)
	if err != nil {
		return activity{}, err
	}

	a, err := scrapeActivity(body)
	if err != nil {
		return activity{}, err
	}
	a.Handle = userHandle
	a.Year = year

	return a, nil
}

// scrapeActivity returns an activity from a GitHub homepage HTML text
func scrapeActivity(html []byte) (activity, error) {
	activity := activity{}         // the struct to return
	activities := map[string]int{} // the temporary map to store scrapped activities

	// tokens to match in the html
	activityAttr := []byte("data-percentages=\"")
	activityKeys := map[string][]byte{
		"commits":     []byte("Commits:"),
		"issues":      []byte("Issues:"),
		"prs":         []byte("Pull requests:"),
		"codeReviews": []byte("Code review:"),
	}

	closingTag := []byte("\">")
	quoteUnicode := []byte("&quot;")
	comma := []byte(",")
	closingBracket := []byte("}")

	// extract the activity container from the HTML text
	rawActivity, err := extractBetween(html, activityAttr, closingTag)
	if err != nil {
		return activity, err
	}

	cleanActivity := bytes.Replace(rawActivity, quoteUnicode, []byte(""), -1)

	// figure out which activity appears last
	// in order to extractBetween with the appropriate token (})
	var lastActivity string
	activityIdx := -1
	for k := range activityKeys {
		if idx := bytes.Index(cleanActivity, activityKeys[k]); idx > activityIdx {
			activityIdx = idx
			lastActivity = k
		}
	}
	if activityIdx == -1 {
		return activity, fmt.Errorf("bytes.Index: did not find any activityKeys in: %s", cleanActivity)
	}

	// extract individual activityKeys
	for k, token := range activityKeys {
		var value []byte
		if k == lastActivity {
			value, err = extractBetween(cleanActivity, token, closingBracket)
		} else {
			value, err = extractBetween(cleanActivity, token, comma)
		}
		if err != nil {
			return activity, err
		}

		// to avoid unnecessary computations, only store if non-zero percentage
		if num, err := strconv.Atoi(string(value)); err != nil {
			return activity, err
		} else if num != 0 {
			activities[k] = num
		}
	}

	activity.Commits = activities["commits"]
	activity.Issues = activities["issues"]
	activity.Prs = activities["prs"]
	activity.CodeReviews = activities["codeReviews"]

	return activity, nil
}

// parseYearFlag returns the years passed to the -y flag
// if no flag is passed, it defaults to all years
func parseYearFlag(rawFlag, handle string) ([]string, error) {
	if rawFlag == "all" {
		body, err := html(fmt.Sprintf("https://github.com/%s", handle))
		if err != nil {
			return nil, fmt.Errorf("parse year flag: %v", err)
		}

		return scrapeYears(body)
	}

	cleanYearFlag := strings.Trim(rawFlag, ", ")
	return strings.Split(cleanYearFlag, ","), nil
}

// scrapeYears returns all available activity years from a GitHub homepage HTML text
// the years are returned in chronological order
func scrapeYears(html []byte) ([]string, error) {
	startList := []byte("<ul class=\"filter-list small\">")
	endList := []byte("</ul>")
	startLink := []byte("<a")
	startYear := []byte("id=\"year-link-")
	quote := []byte("\"")

	rawYearList, err := extractBetween(html, startList, endList)
	if err != nil {
		return nil, fmt.Errorf("extractBetween: %v", err)
	}

	rawYears := bytes.Split(rawYearList, startLink)
	rawYears = rawYears[1:] // drop first slice, it only contains <li>

	years := []string{}
	for _, rawYear := range rawYears {
		year, err := extractBetween(rawYear, startYear, quote)
		if err != nil {
			log.Printf("extractBetween: %v", err)
			continue
		}
		years = append(years, string(year))
	}

	sort.Strings(years)

	return years, nil
}

// coordinates computes the coords forming the path of the activity polygon
func coordinates(activity activity) coords {
	const thresh = 0.8
	w, h := 500.0, 560.0
	mid := w / 2
	factor := w / 10
	axisOffset := 2.35
	axisMargin := axisOffset * factor
	axisLength := mid - axisMargin

	return coords{
		W:           w,
		H:           h,
		Mid:         mid,
		AxisMargin:  axisMargin,
		Factor:      factor,
		CodeReviewY: mid - cappedDelta(float64(activity.CodeReviews), axisLength, thresh),
		IssuesX:     mid + cappedDelta(float64(activity.Issues), axisLength, thresh),
		PrsY:        mid + cappedDelta(float64(activity.Prs), axisLength, thresh),
		CommitsX:    mid - cappedDelta(float64(activity.Commits), axisLength, thresh),
	}
}

// cappedDelta will return a delta with magnitude based on n and proportionate to m
// if the magnitude is greater than thresh, the delta is bumped to m
func cappedDelta(n, m, thresh float64) float64 {
	// wolfram compatile notation: 1-e^{-n/50}
	// see: https://www.desmos.com/calculator/8pcvpgftdv
	delta := 1.0 - math.Pow(math.E, n/-50.0)
	if delta > thresh {
		delta = 1.0
	}
	return m * delta
}

// extractBetween will return the characters in s between the left and right tokens
func extractBetween(s, left, right []byte) ([]byte, error) {
	leftIdx := bytes.Index(s, left)
	if leftIdx == -1 {
		return nil, patternNotFound(left)
	}

	leftOffset := leftIdx + len(left)
	if leftOffset > len(s) {
		return nil, fmt.Errorf("bytes.Index: left offset larger than s: %s", left)
	}

	rightIdx := bytes.Index(s[leftOffset:], right)
	if rightIdx == -1 {
		return nil, patternNotFound(right)
	}

	return s[leftOffset : leftOffset+rightIdx], nil
}

func patternNotFound(pattern []byte) error {
	return fmt.Errorf("bytes.Index: could not find %s", pattern)
}

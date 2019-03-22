package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/urfave/cli"
)

// activity contains GitHub's tracked user activity percentages
type activity struct {
	Handle, Year                      string
	Commits, Issues, Prs, CodeReviews int
}

// coords contains the X,Y coordinates of the activities in an activity graph
type coords struct{ CodeReviewY, IssuesX, PrsY, CommitsX float64 }

// graph contains all information to build the graph of a user's activity for a given year
type graph struct {
	Data   activity
	Coords coords
}

func main() {
	app := cli.NewApp()
	app.Name = "Activity Giffer"
	app.Usage = "Create GIFs from a people's GitHub activity graph"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "years, y",
			Value: "all",
			Usage: "Scrape activityfrom years `2016,2017,2019`",
		},
		cli.StringFlag{
			Name:  "out-dir, o",
			Usage: "Save the GIF in the output directory `./dir`",
			Value: "./out",
		},
		cli.StringFlag{
			Name:  "speed, s",
			Usage: "Set the transition speed of the GIF to `50`ms",
			Value: "100",
		},
		cli.StringFlag{
			Name:  "resize, r",
			Usage: "Set the resizing percentage of the GIF to `60%`",
			Value: "50%",
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
		cli.ShowAppHelp(c)
		return nil
	}

	outputDir := c.String("out-dir")
	gifSpeed := c.String("speed")
	gifResize := c.String("resize")
	specificYears, err := parseYearFlag(c.String("years"), userHandle)
	if err != nil {
		return err
	}
	if len(specificYears) == 0 {
		return errors.New("failed to parse any years")
	}

	lock := &sync.RWMutex{}
	garbageCollector := []string{}
	defer cleanUp(&garbageCollector)

	chanSize := len(specificYears)

	// processing pipeline
	yearc := genYears(specificYears, chanSize)
	actc := genActivities(userHandle, yearc, chanSize)
	graphc := genGraph(actc, chanSize)
	svgc := genSVG(graphc, chanSize, outputDir, &garbageCollector, lock)
	jpgc := genJPG(svgc, chanSize, &garbageCollector, lock)

	// pipeline sink
	jpgs := bundleJPGs(jpgc)
	if len(jpgs) == 0 {
		return fmt.Errorf("Failed to create a single JPG for %s", userHandle)
	}

	gif, err := gif(jpgs, userHandle, gifSpeed, gifResize)
	if err != nil {
		return fmt.Errorf("GIF: %v", err)
	}

	log.Printf("Created: %s\n", gif)

	return nil
}

// genYears fans out every year into a channel
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
			coord, err := coordinates(act)
			if err != nil {
				log.Printf("coordinates for %s: %v\n", act.Year, err)
				continue
			}
			out <- graph{act, coord}
		}
	}()
	return out
}

// genSVG creates and passes SVG filenames into a channel for every graph in the input channel
func genSVG(in <-chan graph, size int, dir string, garbage *[]string, lock *sync.RWMutex) <-chan string {
	var out = make(chan string, size)
	go func() {
		defer close(out)
		for graph := range in {
			svg, err := svg(graph, dir)
			lock.Lock()
			*garbage = append(*garbage, svg)
			lock.Unlock()
			if err != nil {
				log.Printf("SVG: %v\n", err)
				continue
			}
			out <- svg
		}
	}()
	return out
}

// genJPG creates and passes JPG filenames into a channel for every SVG in the input channel
func genJPG(in <-chan string, size int, garbage *[]string, lock *sync.RWMutex) <-chan string {
	var out = make(chan string, size)
	var wg sync.WaitGroup
	wg.Add(size)
	go func() {
		for svg := range in {
			go func(svg string) {
				defer wg.Done()
				jpg, err := jpg(svg)
				lock.Lock()
				*garbage = append(*garbage, jpg)
				lock.Unlock()
				if err != nil {
					log.Printf("JPG: %v\n", err)
					return
				}
				out <- jpg
			}(svg)
		}
	}()
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// bundleJPGs aggreagates all the JPG filenames in the input channel and sorts it
func bundleJPGs(in <-chan string) []string {
	jpgs := []string{}
	for jpg := range in {
		jpgs = append(jpgs, jpg)
	}
	sort.Strings(jpgs)
	return jpgs
}

// html GETs the HTML text of a URL
func html(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(
		"User-Agent",
		"Activity Giffer v0.0	https://www.github.com/camilogarcialarotta/Activity-Giffer - This bot generates GIFs from the user's yearly activity graph",
	)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("GET status: %s: %s", res.Status, url)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// svg creates an SVG of the user's activity in the output directory
// the file will be created as <outputDir>/<userHandle>-<year>.jpg
// if the output directory does not exist, svg will create it

func svg(graph graph, outputDir string) (string, error) {
	tmpl, err := template.ParseFiles("svg.tmpl")
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err := os.Mkdir(outputDir, os.ModePerm); err != nil {
			return "", nil
		}
	}

	fileName := fmt.Sprintf("%s-%s-*.svg", graph.Data.Handle, graph.Data.Year)
	f, err := ioutil.TempFile(outputDir, fileName)
	if err != nil {
		return f.Name(), err
	}
	defer f.Close()

	err = tmpl.Execute(f, graph)
	if err != nil {
		return f.Name(), err
	}

	return f.Name(), nil
}

// jpg converts an SVG to JPG via ImageMagick
// it creates the JPG inside the same directory as the SVG
func jpg(svg string) (string, error) {
	jpg := strings.Replace(svg, ".svg", ".jpg", 1)
	cmd := exec.Command("convert", "-density", "1000", "-resize", "1000x", svg, jpg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("JPG: %v", err)
	}
	return jpg, nil
}

// gif bundles the JPGs to create <userhandle>.gif via ImageMagick
// it creates the GIF inside the same directory as the JPGs
func gif(jpgs []string, userHandle, speed, resize string) (string, error) {
	switch {
	case len(jpgs) == 0:
		return "", errors.New("GIF: no JPGs to bundle")
	case speed == "":
		return "", errors.New("GIF: no transition speed given")
	case resize == "":
		return "", errors.New("GIF: no resize speed given")
	}

	outputDir := filepath.Dir(jpgs[0])
	fileName := fmt.Sprintf("%s.gif", userHandle)
	gif := filepath.Join(".", outputDir, fileName)
	args := []string{"-resize", resize, "-delay", speed, "-loop", "0"}
	args = append(args, jpgs...)
	args = append(args, gif)
	cmd := exec.Command("convert", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return gif, nil
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

		// to avoid unecessary computations, only store if non-zero percentage
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

// coordinates computes the coords forming the SVG path of the activity percentages
func coordinates(activity activity) (coords, error) {
	const (
		xAxis          = 137.5 // x position of the axis Commits - Issues
		yAxis          = 127.5 // y position of the axis CodeReviews - Prs
		activityLength = 67.5  // length of a single activity axis
		thresh         = 0.8   // threshold to cap the actiity delta
	)

	coords := coords{
		CodeReviewY: yAxis - cappedDelta(float64(activity.CodeReviews), activityLength, thresh),
		IssuesX:     xAxis + cappedDelta(float64(activity.Issues), activityLength, thresh),
		PrsY:        yAxis + cappedDelta(float64(activity.Prs), activityLength, thresh),
		CommitsX:    xAxis - cappedDelta(float64(activity.Commits), activityLength, thresh),
	}
	return coords, nil
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

// cleanUp deletes all the files passed as input
func cleanUp(files *[]string) {
	log.Println("Cleaning up")
	for _, file := range *files {
		if err := os.Remove(file); err != nil {
			log.Printf("Cleanup: %v", err)
			continue
		}
	}
}

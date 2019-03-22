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

	specificYears, err := parseYearFlag(c.String("years"), userHandle)
	if err != nil {
		return err
	}
	outputDir := c.String("out-dir")
	gifSpeed := c.String("speed")
	gifResize := c.String("resize")

	garbageCollector := []string{}
	defer cleanUp(&garbageCollector)

	log.Println("Scraping Activities")
	activityGraphs := []graph{}
	for _, year := range specificYears {
		activity, err := parseActivity(userHandle, year)
		if err != nil {
			log.Printf("scrape activity for %s: %v\n", year, err)
			continue
		}

		coords, err := coordinates(activity)
		if err != nil {
			log.Printf("coordinates for %s: %v\n", year, err)
			continue
		}

		log.Printf("Activity: %+v\n", activity)

		activityGraphs = append(activityGraphs, graph{activity, coords})
	}

	if len(activityGraphs) == 0 {
		return fmt.Errorf("Failed to scrape any activities for %s", userHandle)
	}

	log.Println("Converting activities to SVGs")
	svgs := []string{}
	for _, graph := range activityGraphs {
		svgName, err := svg(graph, outputDir)
		garbageCollector = append(garbageCollector, svgName)
		if err != nil {
			log.Printf("SVG: %v\n", err)
			continue
		}
		//  we don't use the idx to directly store the svg filename
		// because in case of failure for any of the years,
		// its easier to just append the succesful ones
		// e.g. append(svgs, svgName) instead of svgs[idx] = svgName
		svgs = append(svgs, svgName)
	}

	if len(svgs) == 0 {
		return fmt.Errorf("Failed to create a single SVG for %s", userHandle)
	}

	log.Println("Converting SVGs to JPGs")
	jpgs := []string{}
	for _, svg := range svgs {
		jpg, err := jpg(svg)
		garbageCollector = append(garbageCollector, jpg)
		if err != nil {
			log.Println(err)
			continue
		}

		jpgs = append(jpgs, jpg)
	}

	if len(jpgs) == 0 {
		return fmt.Errorf("Failed to create a single JPG for %s", userHandle)
	}

	log.Println("Bundling JPGs to single GIF")
	gif, err := gif(jpgs, userHandle, gifSpeed, gifResize)
	if err != nil {
		return fmt.Errorf("GIF: %v", err)
	}

	log.Printf("Created: %s\n", gif)

	return nil
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

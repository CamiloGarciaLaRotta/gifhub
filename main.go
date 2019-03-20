package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/urfave/cli"
)

// githubActivity contains GitHub's tracked user activity percentages
type githubActivity struct {
	Handle, Year                      string
	Commits, Issues, Prs, CodeReviews int
}

func main() {
	app := cli.NewApp()
	app.Name = "Activity Giffer"
	app.Usage = "Create GIFs from a people's GitHub activity graph"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "years, y",
			Usage: "Scrape activityfrom years `2016,2017,2019`",
		},
		cli.StringFlag{
			Name:  "out-dir, o",
			Usage: "Save the GIF in the output directory `./dir`",
			Value: "./out",
		},
	}
	app.Action = generateGIF

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

	specificYears := parseYearFlag(c.String("years"))
	outputDir := c.String("out-dir")

	garbageCollector := []string{}
	defer cleanUp(&garbageCollector)

	log.Println("Scraping Activities")
	activities := []githubActivity{}
	for _, year := range specificYears {
		activity, err := activity(userHandle, year)
		if err != nil {
			log.Printf("scrape activity: %v\n", err)
			continue
		}

		log.Printf("Activity: %+v\n", activity)
		activities = append(activities, activity)
	}

	if len(activities) == 0 {
		return fmt.Errorf("Failed to scrape any activities for %s", userHandle)
	}

	log.Println("Converting activities to SVGs")
	svgs := []string{}
	for _, activity := range activities {
		svgName, err := toSVG(activity, outputDir)
		if err != nil {
			log.Printf("SVG: %v\n", err)
			continue
		}
		//  we don't use the idx to directly store the svg filename
		// because in case of failure for any of the years,
		// its easier to just append the succesful ones
		// e.g. append(svgs, svgName) instead of svgs[idx] = svgName
		svgs = append(svgs, svgName)
		garbageCollector = append(garbageCollector, svgName)
	}

	if len(svgs) == 0 {
		return fmt.Errorf("Failed to create a single SVG for %s", userHandle)
	}

	log.Println("Converting SVGs to JPGs")
	jpgs := []string{}
	for _, svg := range svgs {
		jpg, err := toJPG(svg)
		if err != nil {
			log.Println(err)
			continue
		}
		jpgs = append(jpgs, jpg)
		garbageCollector = append(garbageCollector, jpg)
	}

	if len(jpgs) == 0 {
		return fmt.Errorf("Failed to create a single JPG for %s", userHandle)
	}

	log.Println("Bundling JPGs to single GIF")
	gif, err := toGIF(jpgs, userHandle)
	if err != nil {
		return fmt.Errorf("GIF: %v", err)
	}

	log.Printf("Created: %s\n", gif)

	return nil
}

// parseYearFlag returns the years passed to the -y flag
// if no flag is passed, it defaults to current year
func parseYearFlag(rawFlag string) []string {
	cleanYearFlag := strings.Trim(rawFlag, ", ")

	var specificYears []string
	if cleanYearFlag == "" {
		currentYear := fmt.Sprintf("%d", time.Now().Year())
		specificYears = append(specificYears, currentYear)
	} else {
		specificYears = strings.Split(cleanYearFlag, ",")
	}

	return specificYears
}

// activity scrapes the user activity for a GitHub user for a given year
func activity(userHandle, year string) (githubActivity, error) {
	body, err := getHomePage(userHandle, year)
	if err != nil {
		return githubActivity{}, err
	}

	activity, err := extractActivity(body)
	if err != nil {
		return githubActivity{}, err
	}
	activity.Handle = userHandle
	activity.Year = year

	return activity, nil
}

// getHomePage GETs the HTML text of a GitHub's user homepage overview for a given year
func getHomePage(userHandle, year string) ([]byte, error) {
	homePage := fmt.Sprintf("https://github.com/%[1]s?tab=overview&from=%[2]s-01-01&to=%[2]s-12-31", userHandle, year)
	req, err := http.NewRequest("GET", homePage, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(
		"User-Agent",
		"Activity Giffer v0.0	https://www.github.com/camilogarcialarotta/Activity-Giffer - This bot generates GIFs from the user's yearly activity graph")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("GET status: %s: %s", res.Status, homePage)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// extractActivity returns a githubActivity from a GitHub homepage html text
func extractActivity(html []byte) (githubActivity, error) {
	activity := githubActivity{}   // the struct to return
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

// toSVG creates an SVG of the user's activity in the output directory
// the file will be created as <outputDir>/<userHandle>-<year>.jpg
// if the output directory does not exist, toSVG will create it

func toSVG(activity githubActivity, outputDir string) (string, error) {
	tmpl, err := template.ParseFiles("svg.tmpl")
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err := os.Mkdir(outputDir, os.ModePerm); err != nil {
			return "", nil
		}
	}

	fileName := fmt.Sprintf("%s-%s-*.svg", activity.Handle, activity.Year)
	f, err := ioutil.TempFile(outputDir, fileName)
	if err != nil {
		return "", err
	}
	defer f.Close()

	err = tmpl.Execute(f, activity)
	if err != nil {
		return "", err
	}

	return f.Name(), nil
}

// toJPG converts an SVG to JPG via ImageMagick
// it creates the JPG inside the same directory as the SVG
func toJPG(svg string) (string, error) {
	jpg := strings.Replace(svg, ".svg", ".jpg", 1)
	cmd := exec.Command("convert", "-density", "1000", "-resize", "1000x", svg, jpg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("JPG: %v", err)
	}
	return jpg, nil
}

// toGIF bundles the JPGs to create <userhandle>.gif via ImageMagick
// it creates the GIF inside the same directory as the JPGs
func toGIF(jpgs []string, userHandle string) (string, error) {
	if len(jpgs) == 0 {
		return "", errors.New("GIF: no JPGs to bundle")
	}

	outputDir := filepath.Dir(jpgs[0])
	fileName := fmt.Sprintf("%s.gif", userHandle)
	gif := filepath.Join(".", outputDir, fileName)
	args := []string{"-resize", "50%", "-delay", "50", "-loop", "0"}
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

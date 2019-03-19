package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
			Value: "",
			Usage: "Scrape activityfrom years `2016,2017,2019`",
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

	for _, year := range specificYears {
		activity, err := activity(userHandle, year)
		if err != nil {
			log.Printf("scrape activity: %v\n", err)
			continue
		}

		log.Printf("Activity: %+v\n", activity)

		if err := toSVG(activity); err != nil {
			log.Printf("SVG: %v\n", err)
			continue
		}
	}
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

// toSVG creates an SVG of the user's activity named after the user
func toSVG(activity githubActivity) error {
	tmpl, err := template.ParseFiles("svg.tmpl")
	if err != nil {
		return err
	}

	f, err := os.Create(fmt.Sprintf("%s-%s.svg", activity.Handle, activity.Year))
	if err != nil {
		return err
	}
	defer f.Close()

	err = tmpl.Execute(f, activity)
	if err != nil {
		return err
	}

	return nil
}

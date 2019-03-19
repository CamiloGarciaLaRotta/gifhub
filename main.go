package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"text/template"

	"github.com/urfave/cli"
)

// githubActivity contains GitHub's tracked user activity percentages
type githubActivity struct {
	Handle                            string
	Commits, Issues, Prs, CodeReviews int
}

func main() {
	app := cli.NewApp()
	app.Name = "Activity Giffer"
	app.Usage = "Create GIFs from a people's GitHub activity graph"
	app.Action = generateGIF

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

// generateGIF creates a GIF of the activities of the input user
func generateGIF(c *cli.Context) error {
	if len(os.Args) == 1 {
		cli.ShowAppHelp(c)
		return nil
	}

	userHandle := c.Args().Get(0)
	activity, err := activity(userHandle)
	if err != nil {
		return err
	}

	fmt.Printf("Activity: %+v\n", activity)

	if err := toSVG(activity); err != nil {
		return err
	}

	return nil
}

// activity scrapes the user activity for a GitHub user
func activity(userHandle string) (githubActivity, error) {
	body, err := getHomePage(userHandle)
	if err != nil {
		return githubActivity{}, err
	}

	activity, err := extractActivity(body)
	if err != nil {
		return githubActivity{}, err
	}
	activity.Handle = userHandle

	return activity, nil
}

// getHomePage GETs the HTML text of a GitHub's user homepage
func getHomePage(userHandle string) ([]byte, error) {
	homePage := fmt.Sprintf("https://github.com/%s", userHandle)
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
		return nil, fmt.Errorf("GET status: %d %s", res.StatusCode, res.Status)
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

	fmt.Printf("Scrapped: %s\n", string(cleanActivity))

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

	f, err := os.Create(fmt.Sprintf("%s.svg", activity.Handle))
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

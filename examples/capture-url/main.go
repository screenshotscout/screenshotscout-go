package main

import (
	"fmt"
	"log"

	screenshotscout "github.com/screenshotscout/screenshotscout-go"
)

func main() {
	// Reads SCREENSHOTSCOUT_ACCESS_KEY, and SCREENSHOTSCOUT_SECRET_KEY if it's set
	client, err := screenshotscout.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	fullPage := true
	blockAds := true
	captureURL, err := client.BuildCaptureURL(
		"https://example.com",
		&screenshotscout.CaptureOptions{
			FullPage: &fullPage,
			BlockAds: &blockAds,
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(captureURL)
}

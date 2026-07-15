package main

import (
	"fmt"
	"log"
	"os"

	screenshotscout "github.com/screenshotscout/screenshotscout-go"
)

func main() {
	accessKey := os.Getenv("SCREENSHOTSCOUT_ACCESS_KEY")
	if accessKey == "" {
		log.Fatal("set SCREENSHOTSCOUT_ACCESS_KEY first")
	}
	client, err := screenshotscout.NewClient(accessKey, &screenshotscout.ClientOptions{
		SecretKey: os.Getenv("SCREENSHOTSCOUT_SECRET_KEY"),
	})
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

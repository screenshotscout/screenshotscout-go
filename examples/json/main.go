package main

import (
	"context"
	"fmt"
	"log"

	screenshotscout "github.com/screenshotscout/screenshotscout-go"
)

func main() {
	client, err := screenshotscout.NewClient() // Reads SCREENSHOTSCOUT_ACCESS_KEY
	if err != nil {
		log.Fatal(err)
	}

	responseType := screenshotscout.CaptureResponseTypeJSON
	cache := true
	response, err := client.Capture(
		context.Background(),
		"https://example.com",
		&screenshotscout.CaptureOptions{
			ResponseType: &responseType,
			Cache:        &cache,
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	if response.JSON == nil {
		log.Fatal("Screenshot Scout returned a non-JSON response")
	}
	result := response.JSON.Result
	if result.ScreenshotURL != nil {
		fmt.Println(*result.ScreenshotURL)
	}
	if result.CacheStatus != nil {
		fmt.Println(*result.CacheStatus)
	}
}

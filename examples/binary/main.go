package main

import (
	"context"
	"log"
	"os"

	screenshotscout "github.com/screenshotscout/screenshotscout-go"
)

func main() {
	client, err := screenshotscout.NewClient() // Reads SCREENSHOTSCOUT_ACCESS_KEY
	if err != nil {
		log.Fatal(err)
	}

	fullPage := true
	response, err := client.Capture(
		context.Background(),
		"https://example.com",
		&screenshotscout.CaptureOptions{FullPage: &fullPage},
	)
	if err != nil {
		log.Fatal(err)
	}
	if response.Binary == nil {
		log.Fatal("Screenshot Scout returned a non-binary response")
	}
	if err := os.WriteFile("screenshot.png", response.Binary.Bytes, 0o600); err != nil {
		log.Fatal(err)
	}
}

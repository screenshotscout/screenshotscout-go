package screenshotscout_test

import (
	"fmt"

	screenshotscout "github.com/screenshotscout/screenshotscout-go"
)

func ExampleClient_BuildCaptureURL() {
	client, err := screenshotscout.NewClient(
		"ak_test",
		&screenshotscout.ClientOptions{BaseURL: "https://api.example"},
	)
	if err != nil {
		panic(err)
	}
	fullPage := true
	captureURL, err := client.BuildCaptureURL(
		"https://example.com",
		&screenshotscout.CaptureOptions{FullPage: &fullPage},
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(captureURL)
	// Output:
	// https://api.example/v1/capture?access_key=ak_test&url=https%3A%2F%2Fexample.com&full_page=true
}

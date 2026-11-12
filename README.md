# Screenshot Scout Go SDK

The official Go SDK for the [Screenshot Scout](https://screenshotscout.com/) screenshot API.

## Requirements

- Go 1.25 or newer

## Installation

```sh
go get github.com/screenshotscout/screenshotscout-go
```

## Get your API credentials

Before using the SDK, [sign up for Screenshot Scout](https://screenshotscout.com/auth/signup) or sign in to your account. Open the [API Keys page](https://screenshotscout.com/app/api-keys) and copy the access key. You only need the secret key for [signed requests](#signed-requests).

Set the access key as the `SCREENSHOTSCOUT_ACCESS_KEY` environment variable, or [pass it in code](#pass-the-keys-yourself).

## Capture a screenshot

`Capture` waits until Screenshot Scout returns the final screenshot response.

```go
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
```

`Capture` uses POST by default, or GET when the client has a secret key. An omitted `ResponseType`, or an explicit `CaptureResponseTypeBinary`, expects a binary response.

## Request a JSON result

```go
responseType := screenshotscout.CaptureResponseTypeJSON
response, err := client.Capture(
	context.Background(),
	"https://example.com",
	&screenshotscout.CaptureOptions{
		ResponseType: &responseType,
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
```

`CaptureResponse.Kind` is either `CaptureResponseKindBinary` or `CaptureResponseKindJSON`, and exactly one of `CaptureResponse.Binary` and `CaptureResponse.JSON` is non-nil.

## Use GET

POST is the default, or GET when the client has a secret key. Pass `CaptureHTTPMethodGET` as the final argument to use GET for a call:

```go
format := screenshotscout.CaptureFormatWEBP
response, err := client.Capture(
	context.Background(),
	"https://example.com",
	&screenshotscout.CaptureOptions{Format: &format},
	screenshotscout.CaptureHTTPMethodGET,
)
```

The method selector is separate from `CaptureOptions`. Supplying any value other than `CaptureHTTPMethodGET` or `CaptureHTTPMethodPOST` returns a `SerializationError`.

## Build a capture URL

`BuildCaptureURL` creates a URL without making an HTTP request. Use it when another application or an HTML `<img>` element needs to load the screenshot directly:

```go
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
```

The generated URL contains the access key. A configured secret key signs it automatically; without a secret key, the URL is unsigned. Treat generated URLs as sensitive. Before exposing one to browsers or users, configure a secret key and enable **Require signed requests** on the [API Keys page](https://screenshotscout.com/app/api-keys).

## Signed requests

Set the API key's secret key as the `SCREENSHOTSCOUT_SECRET_KEY` environment variable. The client then signs its capture requests and the capture URLs it builds, with no change to your code:

```go
client, err := screenshotscout.NewClient() // Reads SCREENSHOTSCOUT_ACCESS_KEY and SCREENSHOTSCOUT_SECRET_KEY
if err != nil {
	log.Fatal(err)
}
response, err := client.Capture(context.Background(), "https://example.com", nil)
```

The secret key is used locally and is never transmitted. See the [signed requests guide](https://screenshotscout.com/docs/signed-requests) for details.

## Pass the keys yourself

To use keys from somewhere other than the environment, such as a secrets manager or per-customer settings, pass them to `NewClient`:

```go
client, err := screenshotscout.NewClient(
	screenshotscout.WithAccessKey(config.AccessKey),
	screenshotscout.WithSecretKey(config.SecretKey), // optional
)
if err != nil {
	log.Fatal(err)
}
```

If you pass either key, the client reads neither environment variable. A blank key, such as `WithSecretKey("")`, is an error, so leave out `WithSecretKey` when you have no secret key.

## Capture options

The target URL is the required second argument to `Capture` and the first argument to `BuildCaptureURL`. `CaptureOptions` provides the supported screenshot options:

- Output: `Format`, `ResponseType`
- Network and location: `Country`, `Proxy`, `GeolocationLatitude`, `GeolocationLongitude`, `GeolocationAccuracy`
- Cookies and webpage headers: `Cookies`, `Headers`
- Timing: `Timeout`, `WaitUntil`, `NavigationTimeout`, `Delay`
- Device emulation: `Device`, `DeviceViewportWidth`, `DeviceViewportHeight`, `DeviceScaleFactor`, `DeviceIsMobile`, `DeviceHasTouch`, `DeviceUserAgent`
- Page behavior: `Timezone`, `MediaType`, `ColorScheme`, `ReducedMotion`
- Full page: `FullPage`, `FullPagePreScroll`, `FullPagePreScrollStep`, `FullPagePreScrollStepDelay`, `FullPageMaxHeight`
- Blocking: `BlockCookieBanners`, `BlockAds`, `BlockChatWidgets`
- DOM changes: `HideSelectors`, `ClickSelectors`, `ClickAllSelectors`, `InjectCSS`, `InjectJS`, `BypassCSP`
- Framing: `Selector`, `ClipX`, `ClipY`, `ClipWidth`, `ClipHeight`
- Image output: `ImageWidth`, `ImageHeight`, `ImageMode`, `ImageAnchor`, `ImageAllowUpscale`, `ImageBackground`, `ImageQuality`
- PDF: `PDFPaperFormat`, `PDFLandscape`, `PDFPrintBackground`, `PDFMargin`, `PDFMarginTop`, `PDFMarginRight`, `PDFMarginBottom`, `PDFMarginLeft`, `PDFScale`, `PDFPageRanges`
- Caching: `Cache`, `CacheTTL`, `CacheKey`
- Storage: `StorageMode`, `StorageEndpoint`, `StorageBucket`, `StorageRegion`, `StorageObjectKey`

Scalar fields are pointers. `nil` omits an option, while pointers to `""`, `false`, or zero transmit those values exactly. Empty repeated slices are omitted; non-empty slices preserve caller order and duplicates.

Documented string values are available as constants. You can also pass a value that does not have a constant by using an explicit conversion:

```go
documented := screenshotscout.CaptureFormatWEBP
future := screenshotscout.CaptureFormat("future-format")
```

Screenshot Scout validates target URLs and option values. Invalid or incompatible values are returned as API errors. See the [screenshot option reference](https://screenshotscout.com/docs/screenshot-options).

## Timeouts and cancellation

Use goroutines for concurrent captures. For timing and cancellation, two independent controls are available:

- `CaptureOptions.Timeout` is the Screenshot Scout service-side capture budget in seconds.
- The required `context.Context` controls each call and is the preferred place for call-specific deadlines or cancellation.

```go
serviceTimeout := 180
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

response, err := client.Capture(
	ctx,
	"https://example.com",
	&screenshotscout.CaptureOptions{Timeout: &serviceTimeout},
)
```

These controls are independent. Cancellation returns `context.Canceled` or `context.DeadlineExceeded`, which can be checked with `errors.Is`.

## Custom HTTP client

Pass `WithHTTPClient` to send requests through your own `*http.Client`. SDK requests do not follow redirects. An injected client's transport, cookie jar, and timeout remain in use, and the caller-owned client is not modified.

```go
httpClient := &http.Client{Timeout: 5 * time.Minute}
// Reads SCREENSHOTSCOUT_ACCESS_KEY
client, err := screenshotscout.NewClient(screenshotscout.WithHTTPClient(httpClient))
```

`Capture` does not retry failed requests automatically or fetch a returned screenshot URL.

## Raw responses and errors

Every successful `CaptureResponse` includes `RawResponse`, with the HTTP status, headers, content type, and body bytes. `APIError` and `ResponseDecodingError` provide the same response details.

Five error categories are available as concrete types and `errors.Is` sentinels:

- `ConfigurationError` / `ErrConfiguration`
- `SerializationError` / `ErrSerialization`
- `TransportError` / `ErrTransport`
- `APIError` / `ErrAPI`
- `ResponseDecodingError` / `ErrResponseDecoding`

```go
response, err := client.Capture(context.Background(), "https://example.com", nil)
if err != nil {
	var apiError *screenshotscout.APIError
	var transportError *screenshotscout.TransportError
	switch {
	case errors.As(err, &apiError):
		fmt.Println(apiError.StatusCode)
		if apiError.ErrorCode != nil {
			fmt.Println(*apiError.ErrorCode)
		}
		if apiError.ErrorMessage != nil {
			fmt.Println(*apiError.ErrorMessage)
		}
		fmt.Println(apiError.Errors, apiError.ResponseBody)
		fmt.Println(apiError.RawResponse.Headers)
	case errors.As(err, &transportError):
		fmt.Println(errors.Unwrap(transportError))
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		fmt.Println("capture canceled:", err)
	default:
		fmt.Println(err)
	}
}
```

`TransportError` unwraps the native HTTP cause. `APIError.ResponseBody` retains the complete decoded JSON value when the failure body is valid JSON, including unrecognized fields.

## License

MIT License. See [LICENSE](LICENSE).

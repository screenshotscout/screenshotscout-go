# Screenshot Scout Go SDK

The official Go SDK for the [Screenshot Scout](https://screenshotscout.com/) screenshot API.

The SDK is a small, synchronous wrapper around the inline `GET` and `POST /v1/capture` operation. `Capture` blocks until Screenshot Scout returns the final screenshot response. Applications that want concurrent calls can use ordinary goroutines and contexts.

## Requirements

- Go 1.25 or newer

The initial support policy covers the latest two stable Go releases. Supported versions are rechecked before publication.

## Installation

```sh
go get github.com/screenshotscout/screenshotscout-go
```

## Get your API credentials

Before using the SDK, [sign up for Screenshot Scout](https://screenshotscout.com/auth/signup) or sign in to your account. Open the [API Keys page](https://screenshotscout.com/app/api-keys), then copy your access key and secret key.

The access key is required by `NewClient`. The optional secret key enables [signed requests](#signed-requests). The SDK never reads either credential from environment variables on its own; applications pass credentials explicitly.

## Capture a screenshot

```go
package main

import (
	"context"
	"log"
	"os"

	screenshotscout "github.com/screenshotscout/screenshotscout-go"
)

func main() {
	accessKey := os.Getenv("SCREENSHOTSCOUT_ACCESS_KEY")
	if accessKey == "" {
		log.Fatal("set SCREENSHOTSCOUT_ACCESS_KEY first")
	}

	client, err := screenshotscout.NewClient(accessKey, nil)
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

`Capture` uses POST by default. An omitted `ResponseType`, or an explicit `CaptureResponseTypeBinary`, expects a binary response.

## Request a JSON result

```go
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
```

`CaptureResponse.Kind` is either `CaptureResponseKindBinary` or `CaptureResponseKindJSON`, and exactly one of `CaptureResponse.Binary` and `CaptureResponse.JSON` is non-nil.

## Use GET

POST is the default. Supply one closed `CaptureHTTPMethod` value to select GET for a call:

```go
format := screenshotscout.CaptureFormatWEBP
response, err := client.Capture(
	context.Background(),
	"https://example.com",
	&screenshotscout.CaptureOptions{Format: &format},
	screenshotscout.CaptureHTTPMethodGET,
)
```

The method selector is separate from `CaptureOptions`. Supplying any value other than `CaptureHTTPMethodGET` or `CaptureHTTPMethodPOST` returns a `SerializationError`. The SDK never changes methods after an error.

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

Pass the API key's secret key through `ClientOptions`. The secret is used locally and is never transmitted:

```go
client, err := screenshotscout.NewClient(
	"YOUR_ACCESS_KEY",
	&screenshotscout.ClientOptions{SecretKey: "YOUR_SECRET_KEY"},
)
```

The client signs its first and only GET or POST attempt and every generated capture URL using Screenshot Scout's current HMAC-SHA256 canonical-query algorithm. See the [signed requests guide](https://screenshotscout.com/docs/signed-requests).

## Capture options

The target URL is the required second argument to `Capture` and the first argument to `BuildCaptureURL`. `CaptureOptions` contains every current service option using idiomatic Go names:

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
- PDF: `PDFPaperFormat`, `PDFLandscape`, `PDFPrintBackground`, `PDFMargin`, `PDFMarginTop`, `PDFMarginRight`, `PDFMarginBottom`, `PDFMarginLeft`, `PDFScale`
- Caching: `Cache`, `CacheTTL`, `CacheKey`
- Storage: `StorageMode`, `StorageEndpoint`, `StorageBucket`, `StorageRegion`, `StorageObjectKey`

Scalar fields are pointers. `nil` omits an option, while pointers to `""`, `false`, or zero transmit those values exactly. Empty repeated slices are omitted; non-empty slices preserve caller order and duplicates.

Documented service values have named string types and constants for discoverability, while future or raw strings remain possible through explicit conversion:

```go
documented := screenshotscout.CaptureFormatWEBP
future := screenshotscout.CaptureFormat("future-format")
```

The SDK does not validate target URLs, service enums, ranges, option dependencies, plan permissions, selectors, proxies, or storage settings. Screenshot Scout remains authoritative for those rules. See the [screenshot option reference](https://screenshotscout.com/docs/screenshot-options).

## Timeouts and cancellation

Three independent controls are available:

- `CaptureOptions.Timeout` is the Screenshot Scout service-side capture budget in seconds.
- `ClientOptions.RequestTimeout` is an optional SDK-owned caller-side deadline applied to every request.
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

The SDK never derives the service timeout from a context or request timeout. Context cancellation returns `context.Canceled` or `context.DeadlineExceeded` directly rather than wrapping it in a transport error.

## Client configuration and custom HTTP transport

`ClientOptions` supports:

- `SecretKey`: optional signing secret
- `BaseURL`: optional base URL; defaults to `https://api.screenshotscout.com`
- `RequestTimeout`: optional per-call caller-side deadline; zero disables the SDK-owned deadline
- `HTTPClient`: optional reusable `*http.Client`

The built-in client does not follow redirects. An injected `http.Client` remains caller-owned and follows its own transport, timeout, and redirect configuration.

```go
httpClient := &http.Client{Timeout: 5 * time.Minute}
client, err := screenshotscout.NewClient(
	"YOUR_ACCESS_KEY",
	&screenshotscout.ClientOptions{HTTPClient: httpClient},
)
```

The SDK adds no application-level retries, caching, polling, persistence, method switching, logging, or telemetry, and it never follows a returned screenshot URL automatically.

## Raw responses and errors

Every successful `CaptureResponse` includes `RawResponse`, with the status code and status text, multi-value headers, exact content type, and buffered body bytes. `APIError` and `ResponseDecodingError` retain the same raw response.

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

## Development

```sh
go test ./...
go vet ./...
```

## License

MIT License. See [LICENSE](LICENSE).

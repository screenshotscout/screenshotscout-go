package screenshotscout

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type recordedRequest struct {
	method string
	url    string
	header http.Header
	body   []byte
}

func recordingHTTPClient(
	t *testing.T,
	responseFactory func() *http.Response,
) (*http.Client, *[]recordedRequest) {
	t.Helper()
	requests := &[]recordedRequest{}
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body []byte
		if request.Body != nil {
			var err error
			body, err = io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
		}
		*requests = append(*requests, recordedRequest{
			method: request.Method,
			url:    request.URL.String(),
			header: request.Header.Clone(),
			body:   body,
		})
		return responseFactory(), nil
	})}, requests
}

func response(statusCode int, status, contentType string, body []byte, headers http.Header) *http.Response {
	if headers == nil {
		headers = make(http.Header)
	} else {
		headers = headers.Clone()
	}
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: statusCode,
		Status:     status,
		Header:     headers,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func pointer[T any](value T) *T { return &value }

func TestNewClientValidatesConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		accessKey string
		options   *ClientOptions
	}{
		{name: "blank access key", accessKey: "   "},
		{name: "invalid bearer credential", accessKey: "key with spaces"},
		{name: "blank secret key", accessKey: "key", options: &ClientOptions{SecretKey: "   "}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client, err := NewClient(test.accessKey, test.options)
			if client != nil {
				t.Fatal("expected no client")
			}
			if !errors.Is(err, ErrConfiguration) {
				t.Fatalf("expected configuration category, got %v", err)
			}
			var configurationError *ConfigurationError
			if !errors.As(err, &configurationError) {
				t.Fatalf("expected ConfigurationError, got %T", err)
			}
		})
	}

	client, err := NewClient("key", nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	built, err := client.BuildCaptureURL("https://example.com", nil)
	if err != nil {
		t.Fatalf("BuildCaptureURL: %v", err)
	}
	if !strings.HasPrefix(built, captureEndpointURL+"?") {
		t.Fatalf("unexpected capture endpoint: %s", built)
	}
}

func TestCapturePOSTSerializesCompleteOptionSurface(t *testing.T) {
	t.Parallel()

	httpClient, requests := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusOK, "200 OK", "image/png", []byte{1}, nil)
	})
	client, err := NewClient("access-key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	format := CaptureFormat("future-format")
	responseType := CaptureResponseType("future-response")
	waitUntil := CaptureWaitUntil("future-wait-state")
	mediaType := CaptureMediaType("future-media")
	colorScheme := CaptureColorScheme("future-scheme")
	imageMode := CaptureImageMode("future-image-mode")
	imageAnchor := CaptureImageAnchor("future-anchor")
	paperFormat := CapturePDFPaperFormat("future-paper")
	storageMode := CaptureStorageMode("future-storage")
	empty := ""
	zeroInt := 0
	zeroFloat := 0.0
	falseValue := false

	options := &CaptureOptions{
		Format:                     &format,
		ResponseType:               &responseType,
		Country:                    &empty,
		Proxy:                      &empty,
		GeolocationLatitude:        &zeroFloat,
		GeolocationLongitude:       &zeroFloat,
		GeolocationAccuracy:        &zeroFloat,
		Cookies:                    []string{"session=a", "session=a", ""},
		Headers:                    []string{"X-Test:one", "X-Test:one", ""},
		Timeout:                    &zeroInt,
		WaitUntil:                  &waitUntil,
		NavigationTimeout:          &zeroInt,
		Delay:                      &zeroInt,
		Device:                     &empty,
		DeviceViewportWidth:        &zeroInt,
		DeviceViewportHeight:       &zeroInt,
		DeviceScaleFactor:          &zeroFloat,
		DeviceIsMobile:             &falseValue,
		DeviceHasTouch:             &falseValue,
		DeviceUserAgent:            &empty,
		Timezone:                   &empty,
		MediaType:                  &mediaType,
		ColorScheme:                &colorScheme,
		ReducedMotion:              &falseValue,
		FullPage:                   &falseValue,
		FullPagePreScroll:          &falseValue,
		FullPagePreScrollStep:      &zeroInt,
		FullPagePreScrollStepDelay: &zeroInt,
		FullPageMaxHeight:          &zeroInt,
		BlockCookieBanners:         &falseValue,
		BlockAds:                   &falseValue,
		BlockChatWidgets:           &falseValue,
		HideSelectors:              []string{".same", ".same", ""},
		ClickSelectors:             []string{"#first", "#first", ""},
		ClickAllSelectors:          []string{".all", ".all", ""},
		InjectCSS:                  []string{"", "body { color: red; }"},
		InjectJS:                   []string{"", "document.title = 'x';"},
		BypassCSP:                  &falseValue,
		Selector:                   &empty,
		ClipX:                      &zeroInt,
		ClipY:                      &zeroInt,
		ClipWidth:                  &zeroInt,
		ClipHeight:                 &zeroInt,
		ImageWidth:                 &zeroInt,
		ImageHeight:                &zeroInt,
		ImageMode:                  &imageMode,
		ImageAnchor:                &imageAnchor,
		ImageAllowUpscale:          &falseValue,
		ImageBackground:            &empty,
		ImageQuality:               &zeroInt,
		PDFPaperFormat:             &paperFormat,
		PDFLandscape:               &falseValue,
		PDFPrintBackground:         &falseValue,
		PDFMargin:                  &empty,
		PDFMarginTop:               &empty,
		PDFMarginRight:             &empty,
		PDFMarginBottom:            &empty,
		PDFMarginLeft:              &empty,
		PDFScale:                   &zeroFloat,
		Cache:                      &falseValue,
		CacheTTL:                   &zeroInt,
		CacheKey:                   &empty,
		StorageMode:                &storageMode,
		StorageEndpoint:            &empty,
		StorageBucket:              &empty,
		StorageRegion:              &empty,
		StorageObjectKey:           &empty,
	}

	captureResponse, err := client.Capture(context.Background(), "", options)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if captureResponse.Kind != CaptureResponseKindBinary {
		t.Fatalf("unexpected response kind: %s", captureResponse.Kind)
	}
	if len(*requests) != 1 {
		t.Fatalf("expected one request, got %d", len(*requests))
	}
	recorded := (*requests)[0]
	if recorded.method != http.MethodPost {
		t.Fatalf("expected POST, got %s", recorded.method)
	}
	if recorded.url != captureEndpointURL {
		t.Fatalf("unexpected URL: %s", recorded.url)
	}
	if recorded.header.Get("Authorization") != "Bearer access-key" {
		t.Fatalf("unexpected authorization: %s", recorded.header.Get("Authorization"))
	}
	if recorded.header.Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content type: %s", recorded.header.Get("Content-Type"))
	}

	var actual map[string]any
	if err := json.Unmarshal(recorded.body, &actual); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	expected := map[string]any{
		"url": "", "format": "future-format", "response_type": "future-response",
		"country": "", "proxy": "", "geolocation_latitude": float64(0),
		"geolocation_longitude": float64(0), "geolocation_accuracy": float64(0),
		"cookies": []any{"session=a", "session=a", ""},
		"headers": []any{"X-Test:one", "X-Test:one", ""},
		"timeout": float64(0), "wait_until": "future-wait-state",
		"navigation_timeout": float64(0), "delay": float64(0), "device": "",
		"device_viewport_width": float64(0), "device_viewport_height": float64(0),
		"device_scale_factor": float64(0), "device_is_mobile": false,
		"device_has_touch": false, "device_user_agent": "", "timezone": "",
		"media_type": "future-media", "color_scheme": "future-scheme", "reduced_motion": false,
		"full_page": false, "full_page_pre_scroll": false,
		"full_page_pre_scroll_step": float64(0), "full_page_pre_scroll_step_delay": float64(0),
		"full_page_max_height": float64(0), "block_cookie_banners": false,
		"block_ads": false, "block_chat_widgets": false,
		"hide_selectors":      []any{".same", ".same", ""},
		"click_selectors":     []any{"#first", "#first", ""},
		"click_all_selectors": []any{".all", ".all", ""},
		"inject_css":          []any{"", "body { color: red; }"},
		"inject_js":           []any{"", "document.title = 'x';"},
		"bypass_csp":          false, "selector": "", "clip_x": float64(0), "clip_y": float64(0),
		"clip_width": float64(0), "clip_height": float64(0), "image_width": float64(0),
		"image_height": float64(0), "image_mode": "future-image-mode",
		"image_anchor": "future-anchor", "image_allow_upscale": false,
		"image_background": "", "image_quality": float64(0), "pdf_paper_format": "future-paper",
		"pdf_landscape": false, "pdf_print_background": false, "pdf_margin": "",
		"pdf_margin_top": "", "pdf_margin_right": "", "pdf_margin_bottom": "",
		"pdf_margin_left": "", "pdf_scale": float64(0), "cache": false,
		"cache_ttl": float64(0), "cache_key": "", "storage_mode": "future-storage",
		"storage_endpoint": "", "storage_bucket": "", "storage_region": "",
		"storage_object_key": "",
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected POST body\nactual: %#v\nexpected: %#v", actual, expected)
	}
	if _, present := actual["access_key"]; present {
		t.Fatal("executed request body must not contain access_key")
	}
	if _, present := actual["signature"]; present {
		t.Fatal("unsigned request must not contain signature")
	}
}

func TestOmittedOptionsDoNotInsertServiceDefaults(t *testing.T) {
	t.Parallel()

	httpClient, requests := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusOK, "200 OK", "image/png", []byte{1}, nil)
	})
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Capture(context.Background(), "https://example.com", &CaptureOptions{
		Cookies:           []string{},
		Headers:           []string{},
		HideSelectors:     []string{},
		ClickSelectors:    []string{},
		ClickAllSelectors: []string{},
		InjectCSS:         []string{},
		InjectJS:          []string{},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal((*requests)[0].body, &body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if !reflect.DeepEqual(body, map[string]any{"url": "https://example.com"}) {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestGETAndCaptureURLPreserveOrderAndUseWHATWGEncoding(t *testing.T) {
	t.Parallel()

	httpClient, requests := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusOK, "200 OK", "image/png", []byte{1}, nil)
	})
	client, err := NewClient("query-key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	zero := 0
	falseValue := false
	empty := ""
	options := &CaptureOptions{
		Cookies:  []string{"a=1", "a=1"},
		Headers:  []string{"X-Test:one", "X-Test:one"},
		Delay:    &zero,
		FullPage: &falseValue,
		CacheKey: &empty,
	}
	target := "https://example.com/a path?x=~*"

	if _, err := client.Capture(context.Background(), target, options, CaptureHTTPMethodGET); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("expected one request, got %d", len(*requests))
	}
	recorded := (*requests)[0]
	if recorded.method != http.MethodGet {
		t.Fatalf("expected GET, got %s", recorded.method)
	}
	if len(recorded.body) != 0 {
		t.Fatalf("GET request unexpectedly had a body: %q", recorded.body)
	}
	if recorded.header.Get("Content-Type") != "" {
		t.Fatalf("GET request unexpectedly had Content-Type: %s", recorded.header.Get("Content-Type"))
	}
	const expectedQuery = "url=https%3A%2F%2Fexample.com%2Fa+path%3Fx%3D%7E*&cookies=a%3D1&cookies=a%3D1&headers=X-Test%3Aone&headers=X-Test%3Aone&delay=0&full_page=false&cache_key="
	if !strings.HasSuffix(recorded.url, "?"+expectedQuery) {
		t.Fatalf("unexpected GET URL: %s", recorded.url)
	}
	if strings.Contains(recorded.url, "access_key=") {
		t.Fatal("executed GET must not contain access_key")
	}

	built, err := client.BuildCaptureURL(target, options)
	if err != nil {
		t.Fatalf("BuildCaptureURL: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatal("BuildCaptureURL called the transport")
	}
	const expectedBuiltQuery = "access_key=query-key&" + expectedQuery
	if !strings.HasSuffix(built, "?"+expectedBuiltQuery) {
		t.Fatalf("unexpected built URL: %s", built)
	}
}

func TestSigningMatchesCurrentAPIAlgorithm(t *testing.T) {
	t.Parallel()

	httpClient, requests := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusOK, "200 OK", "application/json", []byte("{}"), nil)
	})
	client, err := NewClient("ak_test", &ClientOptions{SecretKey: "sk_test", HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	responseType := CaptureResponseTypeJSON
	falseValue := false
	zero := 0
	options := &CaptureOptions{
		ResponseType: &responseType,
		FullPage:     &falseValue,
		Delay:        &zero,
		Headers:      []string{"X-Test:one", "X-Test:two"},
	}
	target := "https://example.com/a b?x=1&y=2"
	const expectedSignature = "0c4928ba691575903f27b911b8ea1a536604ca070d60d886e10c127c05e236fc"

	if _, err := client.Capture(context.Background(), target, options, CaptureHTTPMethodGET); err != nil {
		t.Fatalf("signed GET: %v", err)
	}
	if _, err := client.Capture(context.Background(), target, options, CaptureHTTPMethodPOST); err != nil {
		t.Fatalf("signed POST: %v", err)
	}
	built, err := client.BuildCaptureURL(target, options)
	if err != nil {
		t.Fatalf("signed capture URL: %v", err)
	}

	getURL, err := http.NewRequest(http.MethodGet, (*requests)[0].url, nil)
	if err != nil {
		t.Fatalf("parse recorded GET: %v", err)
	}
	if getURL.URL.Query().Get("signature") != expectedSignature {
		t.Fatalf("unexpected GET signature: %s", getURL.URL.Query().Get("signature"))
	}
	var postBody map[string]any
	if err := json.Unmarshal((*requests)[1].body, &postBody); err != nil {
		t.Fatalf("decode signed POST: %v", err)
	}
	if postBody["signature"] != expectedSignature {
		t.Fatalf("unexpected POST signature: %v", postBody["signature"])
	}
	builtRequest, err := http.NewRequest(http.MethodGet, built, nil)
	if err != nil {
		t.Fatalf("parse built URL: %v", err)
	}
	if builtRequest.URL.Query().Get("signature") != expectedSignature {
		t.Fatalf("unexpected built URL signature: %s", builtRequest.URL.Query().Get("signature"))
	}
	if builtRequest.URL.Query().Get("access_key") != "ak_test" {
		t.Fatalf("unexpected built URL access key: %s", builtRequest.URL.Query().Get("access_key"))
	}
	if strings.Contains(built, "sk_test") {
		t.Fatal("built URL exposed the secret key")
	}
	if getURL.URL.Query().Has("access_key") {
		t.Fatal("executed GET included access_key")
	}
	if _, present := postBody["access_key"]; present {
		t.Fatal("executed POST included access_key")
	}
}

func TestBinaryResponseRetainsBytesMetadataAndRawResponse(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	headers.Add("Set-Cookie", "a=1; Path=/")
	headers.Add("Set-Cookie", "b=2; Path=/")
	headers.Set("Screenshot-Scout-Screenshot-URL", "https://cdn.example/screenshot.png")
	headers.Set("Screenshot-Scout-Screenshot-URL-Expires-At", "2026-07-14T00:00:00Z")
	headers.Set("Screenshot-Scout-Cache-Status", "HIT")
	httpClient, _ := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusCreated, "201 Created", "image/png", []byte{0, 1, 2, 255}, headers)
	})
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	result, err := client.Capture(context.Background(), "https://example.com", nil)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if result.Kind != CaptureResponseKindBinary || result.Binary == nil || result.JSON != nil {
		t.Fatalf("invalid discriminated response: %#v", result)
	}
	if !bytes.Equal(result.Binary.Bytes, []byte{0, 1, 2, 255}) {
		t.Fatalf("unexpected bytes: %v", result.Binary.Bytes)
	}
	if result.Binary.ScreenshotURL == nil || *result.Binary.ScreenshotURL != "https://cdn.example/screenshot.png" {
		t.Fatalf("unexpected screenshot URL: %v", result.Binary.ScreenshotURL)
	}
	if result.Binary.ScreenshotURLExpiresAt == nil || *result.Binary.ScreenshotURLExpiresAt != "2026-07-14T00:00:00Z" {
		t.Fatalf("unexpected expiry: %v", result.Binary.ScreenshotURLExpiresAt)
	}
	if result.Binary.CacheStatus == nil || *result.Binary.CacheStatus != "HIT" {
		t.Fatalf("unexpected cache status: %v", result.Binary.CacheStatus)
	}
	if result.RawResponse.StatusCode != http.StatusCreated || result.RawResponse.Status != "201 Created" {
		t.Fatalf("unexpected status: %#v", result.RawResponse)
	}
	if result.RawResponse.ContentType != "image/png" || !bytes.Equal(result.RawResponse.Body, []byte{0, 1, 2, 255}) {
		t.Fatalf("unexpected raw response: %#v", result.RawResponse)
	}
	if !reflect.DeepEqual(result.RawResponse.Headers.Values("Set-Cookie"), []string{"a=1; Path=/", "b=2; Path=/"}) {
		t.Fatalf("unexpected multi-value headers: %#v", result.RawResponse.Headers)
	}
}

func TestJSONResponseRetainsKnownAndAdditionalFields(t *testing.T) {
	t.Parallel()

	body := []byte(`{"screenshot_url":"https://cdn.example/screenshot.png","cache_status":"miss","request_id":"request-123","future":{"nested":true}}`)
	httpClient, _ := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusOK, "200 OK", "application/vnd.screenshotscout+json; charset=utf-8", body, nil)
	})
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	responseType := CaptureResponseTypeJSON

	result, err := client.Capture(context.Background(), "https://example.com", &CaptureOptions{ResponseType: &responseType})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if result.Kind != CaptureResponseKindJSON || result.JSON == nil || result.Binary != nil {
		t.Fatalf("invalid discriminated response: %#v", result)
	}
	captureResult := result.JSON.Result
	if captureResult.ScreenshotURL == nil || *captureResult.ScreenshotURL != "https://cdn.example/screenshot.png" {
		t.Fatalf("unexpected screenshot URL: %v", captureResult.ScreenshotURL)
	}
	if captureResult.CacheStatus == nil || *captureResult.CacheStatus != "miss" {
		t.Fatalf("unexpected cache status: %v", captureResult.CacheStatus)
	}
	if captureResult.ScreenshotURLExpiresAt != nil {
		t.Fatalf("absent expiry was manufactured: %v", captureResult.ScreenshotURLExpiresAt)
	}
	expectedAdditional := map[string]any{
		"request_id": "request-123",
		"future":     map[string]any{"nested": true},
	}
	if !reflect.DeepEqual(captureResult.AdditionalFields, expectedAdditional) {
		t.Fatalf("unexpected additional fields: %#v", captureResult.AdditionalFields)
	}
	if !bytes.Equal(result.RawResponse.Body, body) {
		t.Fatal("raw response body was not retained")
	}
}

func TestKnownResponseTypesRejectMismatchedSuccessfulMediaTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		options     *CaptureOptions
		contentType string
		body        []byte
	}{
		{name: "default binary receives JSON", contentType: "application/json", body: []byte(`{"screenshot_url":"x"}`)},
		{name: "JSON receives binary", options: &CaptureOptions{ResponseType: pointer(CaptureResponseTypeJSON)}, contentType: "image/png", body: []byte{1, 2, 3}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			httpClient, _ := recordingHTTPClient(t, func() *http.Response {
				return response(http.StatusOK, "200 OK", test.contentType, test.body, nil)
			})
			client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			_, err = client.Capture(context.Background(), "https://example.com", test.options)
			if !errors.Is(err, ErrResponseDecoding) {
				t.Fatalf("expected response decoding category, got %v", err)
			}
			var decodingError *ResponseDecodingError
			if !errors.As(err, &decodingError) {
				t.Fatalf("expected ResponseDecodingError, got %T", err)
			}
			if decodingError.RawResponse.ContentType != test.contentType || !bytes.Equal(decodingError.RawResponse.Body, test.body) {
				t.Fatalf("raw response not retained: %#v", decodingError.RawResponse)
			}
		})
	}
}

func TestDynamicResponseTypeUsesActualMediaType(t *testing.T) {
	t.Parallel()

	httpClient, _ := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusOK, "200 OK", "application/json", []byte(`{"screenshot_url":"https://cdn.example/a.png"}`), nil)
	})
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	dynamic := CaptureResponseType("future-response")
	result, err := client.Capture(context.Background(), "https://example.com", &CaptureOptions{ResponseType: &dynamic})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if result.Kind != CaptureResponseKindJSON || result.JSON == nil {
		t.Fatalf("unexpected dynamic response: %#v", result)
	}
}

func TestMalformedSuccessfulJSONIsResponseDecodingError(t *testing.T) {
	t.Parallel()

	tests := []string{"{", "[]", `{"screenshot_url":123}`}
	for _, body := range tests {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			httpClient, _ := recordingHTTPClient(t, func() *http.Response {
				return response(http.StatusOK, "200 OK", "application/json", []byte(body), nil)
			})
			client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			responseType := CaptureResponseTypeJSON
			_, err = client.Capture(context.Background(), "https://example.com", &CaptureOptions{ResponseType: &responseType})
			var decodingError *ResponseDecodingError
			if !errors.As(err, &decodingError) || !errors.Is(err, ErrResponseDecoding) {
				t.Fatalf("expected response decoding error, got %v", err)
			}
			if string(decodingError.RawResponse.Body) != body {
				t.Fatalf("raw body not retained: %q", decodingError.RawResponse.Body)
			}
		})
	}
}

func TestAPIFailureRetainsParsedFieldsAndRawResponse(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error_code":"invalid_options","error_message":"One or more options are invalid.","errors":[{"option":"format","message":"Unsupported."}],"request_id":"request-456"}`)
	headers := http.Header{"X-Request-Id": []string{"request-456"}}
	httpClient, _ := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusBadRequest, "400 Bad Request", "application/json; charset=utf-8", body, headers)
	})
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Capture(context.Background(), "not-semantically-validated", nil)
	if !errors.Is(err, ErrAPI) {
		t.Fatalf("expected API category, got %v", err)
	}
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiError.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", apiError.StatusCode)
	}
	if apiError.ErrorCode == nil || *apiError.ErrorCode != "invalid_options" {
		t.Fatalf("unexpected error code: %v", apiError.ErrorCode)
	}
	if apiError.ErrorMessage == nil || *apiError.ErrorMessage != "One or more options are invalid." {
		t.Fatalf("unexpected error message: %v", apiError.ErrorMessage)
	}
	if len(apiError.Errors) != 1 {
		t.Fatalf("unexpected errors: %#v", apiError.Errors)
	}
	responseObject, ok := apiError.ResponseBody.(map[string]any)
	if !ok || responseObject["request_id"] != "request-456" {
		t.Fatalf("additional API fields were not retained: %#v", apiError.ResponseBody)
	}
	if apiError.RawResponse.Status != "400 Bad Request" || apiError.RawResponse.Headers.Get("X-Request-Id") != "request-456" || !bytes.Equal(apiError.RawResponse.Body, body) {
		t.Fatalf("raw response not retained: %#v", apiError.RawResponse)
	}
}

func TestDefaultClientDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	var redirectedCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/capture":
			writer.Header().Set("Location", "/redirected")
			writer.WriteHeader(http.StatusFound)
			_, _ = writer.Write([]byte("redirect"))
		case "/redirected":
			redirectedCalls.Add(1)
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient("key", nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.endpoint = server.URL + "/v1/capture"
	_, err = client.Capture(context.Background(), "https://example.com", nil)
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != http.StatusFound {
		t.Fatalf("expected raw redirect API error, got %v", err)
	}
	if redirectedCalls.Load() != 0 {
		t.Fatalf("redirect was followed %d times", redirectedCalls.Load())
	}
}

func TestInjectedHTTPClientDoesNotFollowRedirectsOrMutateCallerPolicy(t *testing.T) {
	t.Parallel()

	var redirectedCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/capture":
			writer.Header().Set("Location", "/redirected")
			writer.WriteHeader(http.StatusFound)
			_, _ = writer.Write([]byte("redirect"))
		case "/redirected":
			redirectedCalls.Add(1)
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var callerRedirectPolicyCalls atomic.Int32
	httpClient := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			callerRedirectPolicyCalls.Add(1)
			return nil
		},
	}
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.endpoint = server.URL + "/v1/capture"

	_, err = client.Capture(context.Background(), "https://example.com", nil)
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != http.StatusFound {
		t.Fatalf("expected raw redirect API error, got %v", err)
	}
	if redirectedCalls.Load() != 0 {
		t.Fatalf("SDK request followed redirect %d times", redirectedCalls.Load())
	}
	if callerRedirectPolicyCalls.Load() != 0 {
		t.Fatalf("SDK request used caller redirect policy %d times", callerRedirectPolicyCalls.Load())
	}

	response, err := httpClient.Get(server.URL + "/v1/capture")
	if err != nil {
		t.Fatalf("caller-owned client: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("caller-owned client status: %d", response.StatusCode)
	}
	if callerRedirectPolicyCalls.Load() != 1 || redirectedCalls.Load() != 1 {
		t.Fatalf(
			"caller-owned redirect behavior changed: policy calls=%d, redirected calls=%d",
			callerRedirectPolicyCalls.Load(),
			redirectedCalls.Load(),
		)
	}
}

func TestTransportFailureRetainsNativeCauseAndIsNotRetried(t *testing.T) {
	t.Parallel()

	nativeCause := errors.New("socket closed")
	var calls atomic.Int32
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, nativeCause
	})}
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Capture(context.Background(), "https://example.com", nil)
	if !errors.Is(err, ErrTransport) || !errors.Is(err, nativeCause) {
		t.Fatalf("transport category or native cause missing: %v", err)
	}
	var transportError *TransportError
	if !errors.As(err, &transportError) {
		t.Fatalf("expected TransportError, got %T", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("request was attempted %d times", calls.Load())
	}
}

func TestContextCancellationAndDeadlineAreNotWrapped(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Capture(canceled, "https://example.com", nil)
	if err != context.Canceled {
		t.Fatalf("expected direct context.Canceled, got %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("pre-canceled context called the transport")
	}

	deadlineContext, deadlineCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer deadlineCancel()
	_, err = client.Capture(deadlineContext, "https://example.com", nil)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected direct context.DeadlineExceeded, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("unexpected transport calls: %d", calls.Load())
	}
}

func TestUnsafeValuesAndInvalidMethodsFailBeforeTransport(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(http.StatusOK, "200 OK", "image/png", []byte{1}, nil), nil
	})}
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	nonFinite := math.Inf(1)
	tests := []struct {
		name    string
		options *CaptureOptions
		method  []CaptureHTTPMethod
	}{
		{name: "non-finite number", options: &CaptureOptions{PDFScale: &nonFinite}},
		{name: "invalid method", method: []CaptureHTTPMethod{CaptureHTTPMethod("get")}},
		{name: "multiple methods", method: []CaptureHTTPMethod{CaptureHTTPMethodGET, CaptureHTTPMethodPOST}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.Capture(context.Background(), "https://example.com", test.options, test.method...)
			if !errors.Is(err, ErrSerialization) {
				t.Fatalf("expected serialization category, got %v", err)
			}
			var serializationError *SerializationError
			if !errors.As(err, &serializationError) {
				t.Fatalf("expected SerializationError, got %T", err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid calls reached transport %d times", calls.Load())
	}
}

type failingReadCloser struct{ cause error }

func (reader *failingReadCloser) Read([]byte) (int, error) { return 0, reader.cause }

func (reader *failingReadCloser) Close() error { return nil }

func TestResponseBodyReadFailureIsTransportError(t *testing.T) {
	t.Parallel()

	nativeCause := errors.New("body stream failed")
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       &failingReadCloser{cause: nativeCause},
		}, nil
	})}
	client, err := NewClient("key", &ClientOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Capture(context.Background(), "https://example.com", nil)
	if !errors.Is(err, ErrTransport) || !errors.Is(err, nativeCause) {
		t.Fatalf("expected transport error retaining native cause, got %v", err)
	}
}

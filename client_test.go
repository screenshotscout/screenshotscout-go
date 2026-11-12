package screenshotscout

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
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

// fakeEnvironment returns a lookup over values that records the names it reads.
func fakeEnvironment(values map[string]string) (func(string) (string, bool), *[]string) {
	names := &[]string{}
	return func(name string) (string, bool) {
		*names = append(*names, name)
		value, ok := values[name]
		return value, ok
	}, names
}

// bothVariables returns an environment whose keys a client must not use when
// keys are passed.
func bothVariables() map[string]string {
	return map[string]string{
		"SCREENSHOTSCOUT_ACCESS_KEY": "ak_env",
		"SCREENSHOTSCOUT_SECRET_KEY": "sk_env",
	}
}

func configurationErrorMessage(t *testing.T, err error) string {
	t.Helper()
	if !errors.Is(err, ErrConfiguration) {
		t.Fatalf("expected configuration category, got %v", err)
	}
	var configurationError *ConfigurationError
	if !errors.As(err, &configurationError) {
		t.Fatalf("expected ConfigurationError, got %T", err)
	}
	return configurationError.Message
}

func querySignature(t *testing.T, rawURL string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %s: %v", rawURL, err)
	}
	return parsed.Query().Get("signature")
}

// signedCaptureTarget and signedCaptureOptions, signed with ak_test and
// sk_test, give signedCaptureSignature.
const (
	signedCaptureTarget    = "https://example.com/a b?x=1&y=2"
	signedCaptureSignature = "6419f2ac8c0440f1e341b547630606c9dc74dacfbb2df8e122fc9ae26831d893"
)

func signedCaptureOptions() *CaptureOptions {
	return &CaptureOptions{
		ResponseType: pointer(CaptureResponseTypeJSON),
		FullPage:     pointer(false),
		Delay:        pointer(0),
		Headers:      []string{"X-Test:one", "X-Test:two"},
	}
}

func jsonResponse() *http.Response {
	return response(http.StatusOK, "200 OK", "application/json", []byte("{}"), nil)
}

func TestNewClientValidatesConfiguration(t *testing.T) {
	t.Parallel()

	const (
		missingAccessKey = "A non-blank Screenshot Scout access key is required."
		invalidAccessKey = "The Screenshot Scout access key is not a valid Bearer credential."
		invalidSecretKey = "The Screenshot Scout secret key must be a non-blank UTF-8 string when provided."
	)
	tests := []struct {
		name    string
		options []Option
		message string
	}{
		{name: "nil option", options: []Option{nil}, message: "NewClient options must not be nil."},
		{name: "empty access key", options: []Option{WithAccessKey("")}, message: missingAccessKey},
		{name: "blank access key", options: []Option{WithAccessKey("   ")}, message: missingAccessKey},
		{name: "secret key without access key", options: []Option{WithSecretKey("sk_code")}, message: missingAccessKey},
		{name: "invalid bearer credential", options: []Option{WithAccessKey("key with spaces")}, message: invalidAccessKey},
		{name: "empty secret key", options: []Option{WithAccessKey("key"), WithSecretKey("")}, message: invalidSecretKey},
		{name: "blank secret key", options: []Option{WithAccessKey("key"), WithSecretKey("   ")}, message: invalidSecretKey},
		{name: "invalid UTF-8 secret key", options: []Option{WithAccessKey("key"), WithSecretKey("\xff")}, message: invalidSecretKey},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			lookupEnv, names := fakeEnvironment(bothVariables())
			client, err := NewClient(append([]Option{withLookupEnv(lookupEnv)}, test.options...)...)
			if client != nil {
				t.Fatal("expected no client")
			}
			if message := configurationErrorMessage(t, err); message != test.message {
				t.Fatalf("message = %q, want %q", message, test.message)
			}
			if len(*names) != 0 {
				t.Fatalf("read environment variables %q", *names)
			}
		})
	}

	client, err := NewClient(WithAccessKey("key"))
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

func TestNewClientReadsKeysFromTheEnvironment(t *testing.T) {
	t.Parallel()

	const missingVariable = "A non-blank Screenshot Scout access key is required. Pass one, or set SCREENSHOTSCOUT_ACCESS_KEY."
	tests := []struct {
		name          string
		values        map[string]string
		wantAccessKey string
		wantSecretKey string
		wantMessage   string
	}{
		{
			name:          "both variables",
			values:        map[string]string{"SCREENSHOTSCOUT_ACCESS_KEY": "ak_env", "SCREENSHOTSCOUT_SECRET_KEY": "sk_env"},
			wantAccessKey: "ak_env",
			wantSecretKey: "sk_env",
		},
		{
			name:          "access key variable only",
			values:        map[string]string{"SCREENSHOTSCOUT_ACCESS_KEY": "ak_env"},
			wantAccessKey: "ak_env",
		},
		{
			name:          "empty secret key variable",
			values:        map[string]string{"SCREENSHOTSCOUT_ACCESS_KEY": "ak_env", "SCREENSHOTSCOUT_SECRET_KEY": ""},
			wantAccessKey: "ak_env",
		},
		{
			name:        "no variables",
			values:      map[string]string{},
			wantMessage: missingVariable,
		},
		{
			name:        "empty access key variable",
			values:      map[string]string{"SCREENSHOTSCOUT_ACCESS_KEY": ""},
			wantMessage: missingVariable,
		},
		{
			name:        "secret key variable only",
			values:      map[string]string{"SCREENSHOTSCOUT_SECRET_KEY": "sk_env"},
			wantMessage: missingVariable,
		},
		{
			name:        "blank access key variable",
			values:      map[string]string{"SCREENSHOTSCOUT_ACCESS_KEY": "  "},
			wantMessage: "SCREENSHOTSCOUT_ACCESS_KEY is set but blank.",
		},
		{
			name:        "blank secret key variable",
			values:      map[string]string{"SCREENSHOTSCOUT_ACCESS_KEY": "ak_env", "SCREENSHOTSCOUT_SECRET_KEY": " \t"},
			wantMessage: "SCREENSHOTSCOUT_SECRET_KEY is set but blank.",
		},
		{
			name:        "invalid access key variable",
			values:      map[string]string{"SCREENSHOTSCOUT_ACCESS_KEY": "key with spaces"},
			wantMessage: "The Screenshot Scout access key is not a valid Bearer credential.",
		},
		{
			name:        "invalid UTF-8 secret key variable",
			values:      map[string]string{"SCREENSHOTSCOUT_ACCESS_KEY": "ak_env", "SCREENSHOTSCOUT_SECRET_KEY": "\xff"},
			wantMessage: "The Screenshot Scout secret key must be a non-blank UTF-8 string when provided.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			lookupEnv, names := fakeEnvironment(test.values)
			client, err := NewClient(withLookupEnv(lookupEnv))
			if test.wantMessage != "" {
				if client != nil {
					t.Fatal("expected no client")
				}
				if message := configurationErrorMessage(t, err); message != test.wantMessage {
					t.Fatalf("message = %q, want %q", message, test.wantMessage)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if client.accessKey != test.wantAccessKey || client.secretKey != test.wantSecretKey {
				t.Fatalf("keys = %q, %q; want %q, %q", client.accessKey, client.secretKey, test.wantAccessKey, test.wantSecretKey)
			}
			if client.secretKeyFromEnvironment != (test.wantSecretKey != "") {
				t.Fatalf("secretKeyFromEnvironment = %t", client.secretKeyFromEnvironment)
			}
			if want := []string{"SCREENSHOTSCOUT_ACCESS_KEY", "SCREENSHOTSCOUT_SECRET_KEY"}; !slices.Equal(*names, want) {
				t.Fatalf("read %q, want %q", *names, want)
			}
		})
	}
}

// Setting variables affects the whole process, so this test doesn't run in
// parallel.
func TestNewClientReadsTheProcessEnvironmentByDefault(t *testing.T) {
	t.Setenv("SCREENSHOTSCOUT_ACCESS_KEY", "ak_process")
	t.Setenv("SCREENSHOTSCOUT_SECRET_KEY", "sk_process")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.accessKey != "ak_process" || client.secretKey != "sk_process" || !client.secretKeyFromEnvironment {
		t.Fatalf("unexpected keys: %q, %q (from environment: %t)", client.accessKey, client.secretKey, client.secretKeyFromEnvironment)
	}
}

func TestPassingEitherKeyMeansTheEnvironmentIsNotRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		keys       []Option
		wantMethod string
	}{
		{name: "access key", keys: []Option{WithAccessKey("ak_code")}, wantMethod: http.MethodPost},
		{name: "both keys", keys: []Option{WithAccessKey("ak_code"), WithSecretKey("sk_code")}, wantMethod: http.MethodGet},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			lookupEnv, names := fakeEnvironment(bothVariables())
			httpClient, requests := recordingHTTPClient(t, func() *http.Response {
				return response(http.StatusOK, "200 OK", "image/png", []byte{1}, nil)
			})
			client, err := NewClient(append([]Option{withLookupEnv(lookupEnv), WithHTTPClient(httpClient)}, test.keys...)...)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if _, err := client.Capture(context.Background(), "https://example.com", nil); err != nil {
				t.Fatalf("Capture: %v", err)
			}
			recorded := (*requests)[0]
			if recorded.method != test.wantMethod {
				t.Fatalf("method = %s, want %s", recorded.method, test.wantMethod)
			}
			if recorded.header.Get("Authorization") != "Bearer ak_code" {
				t.Fatalf("unexpected authorization: %s", recorded.header.Get("Authorization"))
			}
			if client.secretKeyFromEnvironment {
				t.Fatal("a passed secret key was reported as coming from the environment")
			}
			if len(*names) != 0 {
				t.Fatalf("read environment variables %q", *names)
			}
		})
	}
}

func TestFormattingAClientHidesItsKeys(t *testing.T) {
	t.Parallel()

	signing, err := NewClient(WithAccessKey("ak_hidden"), WithSecretKey("sk_hidden"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	unsigned, err := NewClient(WithAccessKey("ak_hidden"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		for _, value := range []any{signing, *signing, unsigned, *unsigned} {
			formatted := fmt.Sprintf(verb, value)
			if strings.Contains(formatted, "ak_hidden") || strings.Contains(formatted, "sk_hidden") {
				t.Errorf("%s of %T revealed a key: %s", verb, value, formatted)
			}
		}
	}
	if got, want := fmt.Sprint(signing), `screenshotscout.Client{accessKey:"[redacted]", secretKey:"[redacted]"}`; got != want {
		t.Errorf("signing client = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(unsigned), `screenshotscout.Client{accessKey:"[redacted]", secretKey:""}`; got != want {
		t.Errorf("unsigned client = %s, want %s", got, want)
	}
}

func TestCapturePOSTSerializesCompleteOptionSurface(t *testing.T) {
	t.Parallel()

	httpClient, requests := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusOK, "200 OK", "image/png", []byte{1}, nil)
	})
	client, err := NewClient(WithAccessKey("access-key"), WithHTTPClient(httpClient))
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
		PDFPageRanges:              &empty,
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
		"pdf_margin_left": "", "pdf_scale": float64(0), "pdf_page_ranges": "", "cache": false,
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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

func TestGETAndCaptureURLPreserveOrderAndUseRFC3986Encoding(t *testing.T) {
	t.Parallel()

	httpClient, requests := recordingHTTPClient(t, func() *http.Response {
		return response(http.StatusOK, "200 OK", "image/png", []byte{1}, nil)
	})
	client, err := NewClient(WithAccessKey("query-key"), WithHTTPClient(httpClient))
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
	const expectedQuery = "url=https%3A%2F%2Fexample.com%2Fa%20path%3Fx%3D~%2A&cookies=a%3D1&cookies=a%3D1&headers=X-Test%3Aone&headers=X-Test%3Aone&delay=0&full_page=false&cache_key="
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

	httpClient, requests := recordingHTTPClient(t, jsonResponse)
	client, err := NewClient(WithAccessKey("ak_test"), WithSecretKey("sk_test"), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	options := signedCaptureOptions()

	if _, err := client.Capture(context.Background(), signedCaptureTarget, options, CaptureHTTPMethodGET); err != nil {
		t.Fatalf("signed GET: %v", err)
	}
	built, err := client.BuildCaptureURL(signedCaptureTarget, options)
	if err != nil {
		t.Fatalf("signed capture URL: %v", err)
	}

	getURL, err := http.NewRequest(http.MethodGet, (*requests)[0].url, nil)
	if err != nil {
		t.Fatalf("parse recorded GET: %v", err)
	}
	if getURL.URL.Query().Get("signature") != signedCaptureSignature {
		t.Fatalf("unexpected GET signature: %s", getURL.URL.Query().Get("signature"))
	}
	builtRequest, err := http.NewRequest(http.MethodGet, built, nil)
	if err != nil {
		t.Fatalf("parse built URL: %v", err)
	}
	if builtRequest.URL.Query().Get("signature") != signedCaptureSignature {
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
}

func TestOmittedMethodWithSecretKeySendsSignedGET(t *testing.T) {
	t.Parallel()

	httpClient, requests := recordingHTTPClient(t, jsonResponse)
	client, err := NewClient(WithAccessKey("ak_test"), WithSecretKey("sk_test"), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.Capture(context.Background(), signedCaptureTarget, signedCaptureOptions()); err != nil {
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
	if signature := querySignature(t, recorded.url); signature != signedCaptureSignature {
		t.Fatalf("unexpected GET signature: %s", signature)
	}
}

func TestEnvironmentKeysAreReadOnceAndSignAnOmittedMethod(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"SCREENSHOTSCOUT_ACCESS_KEY": "ak_test",
		"SCREENSHOTSCOUT_SECRET_KEY": "sk_test",
	}
	lookupEnv, names := fakeEnvironment(values)
	httpClient, requests := recordingHTTPClient(t, jsonResponse)
	client, err := NewClient(withLookupEnv(lookupEnv), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	values["SCREENSHOTSCOUT_ACCESS_KEY"] = "changed"
	values["SCREENSHOTSCOUT_SECRET_KEY"] = "changed"

	if _, err := client.Capture(context.Background(), signedCaptureTarget, signedCaptureOptions()); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	recorded := (*requests)[0]
	if recorded.method != http.MethodGet {
		t.Fatalf("expected GET, got %s", recorded.method)
	}
	if recorded.header.Get("Authorization") != "Bearer ak_test" {
		t.Fatalf("unexpected authorization: %s", recorded.header.Get("Authorization"))
	}
	if signature := querySignature(t, recorded.url); signature != signedCaptureSignature {
		t.Fatalf("unexpected GET signature: %s", signature)
	}
	if want := []string{"SCREENSHOTSCOUT_ACCESS_KEY", "SCREENSHOTSCOUT_SECRET_KEY"}; !slices.Equal(*names, want) {
		t.Fatalf("read %q, want %q", *names, want)
	}
}

func TestPOSTWithSecretKeyFailsBeforeTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		keys        []Option
		environment map[string]string
		message     string
	}{
		{
			name:    "secret key passed",
			keys:    []Option{WithAccessKey("ak_test"), WithSecretKey("sk_test")},
			message: "POST requests can't be signed. This client has a secret key: send this capture with method GET, or use a client without a secret key and an API key that doesn't require signed requests.",
		},
		{
			name: "secret key from the environment",
			environment: map[string]string{
				"SCREENSHOTSCOUT_ACCESS_KEY": "ak_test",
				"SCREENSHOTSCOUT_SECRET_KEY": "sk_test",
			},
			message: "POST requests can't be signed. This client has a secret key from SCREENSHOTSCOUT_SECRET_KEY: send this capture with method GET, or use a client without a secret key and an API key that doesn't require signed requests.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return response(http.StatusOK, "200 OK", "image/png", []byte{1}, nil), nil
			})}
			lookupEnv, _ := fakeEnvironment(test.environment)
			client, err := NewClient(append([]Option{withLookupEnv(lookupEnv), WithHTTPClient(httpClient)}, test.keys...)...)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			_, err = client.Capture(context.Background(), "https://example.com", nil, CaptureHTTPMethodPOST)
			if message := configurationErrorMessage(t, err); message != test.message {
				t.Fatalf("message = %q, want %q", message, test.message)
			}
			if calls.Load() != 0 {
				t.Fatalf("POST with a secret key reached transport %d times", calls.Load())
			}
		})
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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
			client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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
			client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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

	client, err := NewClient(WithAccessKey("key"))
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
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
	client, err := NewClient(WithAccessKey("key"), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Capture(context.Background(), "https://example.com", nil)
	if !errors.Is(err, ErrTransport) || !errors.Is(err, nativeCause) {
		t.Fatalf("expected transport error retaining native cause, got %v", err)
	}
}

package screenshotscout

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const defaultBaseURL = "https://api.screenshotscout.com"

var accessKeyPattern = regexp.MustCompile(`^[A-Za-z0-9\-._~+/]+=*$`)

// Client is a reusable Screenshot Scout API client.
type Client struct {
	accessKey      string
	secretKey      string
	endpoint       string
	requestTimeout time.Duration
	httpClient     *http.Client
}

// NewClient creates a reusable client. The access key is configured once and
// is sent in the Authorization header for Capture calls.
func NewClient(accessKey string, options *ClientOptions) (*Client, error) {
	if strings.TrimSpace(accessKey) == "" {
		return nil, &ConfigurationError{
			Message: "A non-blank Screenshot Scout access key is required.",
		}
	}
	if !accessKeyPattern.MatchString(accessKey) {
		return nil, &ConfigurationError{
			Message: "The Screenshot Scout access key is not a valid Bearer credential.",
		}
	}

	configuration := ClientOptions{}
	if options != nil {
		configuration = *options
	}
	if configuration.SecretKey != "" {
		if strings.TrimSpace(configuration.SecretKey) == "" || !utf8.ValidString(configuration.SecretKey) {
			return nil, &ConfigurationError{
				Message: "The Screenshot Scout secret key must be a non-blank UTF-8 string when provided.",
			}
		}
	}
	if configuration.RequestTimeout < 0 {
		return nil, &ConfigurationError{
			Message: "RequestTimeout must not be negative.",
		}
	}

	baseURL := configuration.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	endpoint, err := captureEndpoint(baseURL)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{}
	if configuration.HTTPClient != nil {
		clonedHTTPClient := *configuration.HTTPClient
		httpClient = &clonedHTTPClient
	}
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &Client{
		accessKey:      accessKey,
		secretKey:      configuration.SecretKey,
		endpoint:       endpoint,
		requestTimeout: configuration.RequestTimeout,
		httpClient:     httpClient,
	}, nil
}

// Capture performs one inline Screenshot Scout request and waits for the final
// buffered response. POST is used when method is omitted. At most one method
// may be supplied, and it must be CaptureHTTPMethodGET or CaptureHTTPMethodPOST.
// Cancellation follows the supplied context's normal semantics.
func (client *Client) Capture(
	ctx context.Context,
	targetURL string,
	options *CaptureOptions,
	method ...CaptureHTTPMethod,
) (*CaptureResponse, error) {
	if ctx == nil {
		return nil, &SerializationError{
			Message: "A non-nil context.Context is required.",
			Option:  "context",
		}
	}

	serialized, err := serializeCaptureOptions(targetURL, options)
	if err != nil {
		return nil, err
	}
	captureMethod, err := resolveCaptureHTTPMethod(method)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	requestContext := ctx
	cancel := func() {}
	if client.requestTimeout > 0 {
		requestContext, cancel = context.WithTimeout(ctx, client.requestTimeout)
	}
	defer cancel()
	if err := requestContext.Err(); err != nil {
		return nil, err
	}

	signature := client.createSignature(serialized.pairs)
	requestURL := client.endpoint
	var body io.Reader
	if captureMethod == CaptureHTTPMethodGET {
		pairs := append([]wirePair(nil), serialized.pairs...)
		if signature != "" {
			pairs = append(pairs, wirePair{name: "signature", value: signature})
		}
		requestURL += "?" + encodeQueryPairs(pairs)
	} else {
		bodyValues := make(map[string]any, len(serialized.body)+1)
		for name, value := range serialized.body {
			bodyValues[name] = value
		}
		if signature != "" {
			bodyValues["signature"] = signature
		}
		encodedBody, encodeErr := encodeJSONBody(bodyValues)
		if encodeErr != nil {
			return nil, encodeErr
		}
		body = bytes.NewReader(encodedBody)
	}

	request, err := http.NewRequestWithContext(requestContext, string(captureMethod), requestURL, body)
	if err != nil {
		return nil, &SerializationError{
			Message: "The Screenshot Scout HTTP request could not be constructed.",
			Cause:   err,
		}
	}
	request.Header.Set("Authorization", "Bearer "+client.accessKey)
	if captureMethod == CaptureHTTPMethodPOST {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		if contextErr := requestContext.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, &TransportError{
			Message: "Screenshot Scout request failed before an HTTP response body was received.",
			Cause:   err,
		}
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		if contextErr := requestContext.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, &TransportError{
			Message: "Screenshot Scout request failed before a complete HTTP response body was received.",
			Cause:   err,
		}
	}
	rawResponse := RawResponse{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Headers:    response.Header.Clone(),
		Body:       responseBody,
	}
	if contentType := firstHeader(response.Header, "Content-Type"); contentType != nil {
		rawResponse.ContentType = *contentType
	}

	if rawResponse.StatusCode < 200 || rawResponse.StatusCode > 299 {
		return nil, createAPIError(rawResponse)
	}
	return decodeSuccessResponse(rawResponse, expectedKind(options))
}

// BuildCaptureURL constructs a sensitive GET capture URL without making an
// HTTP request. The access key is included in the query, and a configured
// secret key signs the URL automatically.
func (client *Client) BuildCaptureURL(targetURL string, options *CaptureOptions) (string, error) {
	serialized, err := serializeCaptureOptions(targetURL, options)
	if err != nil {
		return "", err
	}
	signature := client.createSignature(serialized.pairs)
	pairs := make([]wirePair, 0, len(serialized.pairs)+2)
	pairs = append(pairs, wirePair{name: "access_key", value: client.accessKey})
	pairs = append(pairs, serialized.pairs...)
	if signature != "" {
		pairs = append(pairs, wirePair{name: "signature", value: signature})
	}
	return client.endpoint + "?" + encodeQueryPairs(pairs), nil
}

func (client *Client) createSignature(pairs []wirePair) string {
	if client.secretKey == "" {
		return ""
	}
	canonicalQuery := buildCanonicalQuery(pairs, client.accessKey)
	mac := hmac.New(sha256.New, []byte(client.secretKey))
	_, _ = mac.Write([]byte(canonicalQuery))
	return hex.EncodeToString(mac.Sum(nil))
}

func captureEndpoint(baseURL string) (string, error) {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(baseURL) != baseURL {
		return "", &ConfigurationError{
			Message: "BaseURL must be a non-blank absolute HTTP or HTTPS URL.",
		}
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", &ConfigurationError{
			Message: "BaseURL must be a non-blank absolute HTTP or HTTPS URL.",
			Cause:   err,
		}
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Opaque != "" {
		return "", &ConfigurationError{
			Message: "BaseURL must be a non-blank absolute HTTP or HTTPS URL.",
		}
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", &ConfigurationError{
			Message: "BaseURL must not contain a query string or fragment.",
		}
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1/capture"
	parsed.RawPath = ""
	endpoint := parsed.String()
	if endpoint == "" {
		return "", &ConfigurationError{
			Message: "BaseURL could not be converted to a capture endpoint.",
			Cause:   fmt.Errorf("empty endpoint"),
		}
	}
	return endpoint, nil
}

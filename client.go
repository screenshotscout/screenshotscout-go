package screenshotscout

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

const captureEndpointURL = "https://api.screenshotscout.com/v1/capture"

var accessKeyPattern = regexp.MustCompile(`^[A-Za-z0-9\-._~+/]+=*$`)

// Client is a reusable Screenshot Scout API client.
type Client struct {
	accessKey                string
	secretKey                string
	secretKeyFromEnvironment bool
	endpoint                 string
	httpClient               *http.Client
}

// NewClient creates a reusable client. Without WithAccessKey or WithSecretKey,
// it reads SCREENSHOTSCOUT_ACCESS_KEY and, if it's set,
// SCREENSHOTSCOUT_SECRET_KEY. Passing either key means neither variable is
// read. The access key is sent in the Authorization header for Capture calls,
// and a secret key signs capture requests and capture URLs.
func NewClient(options ...Option) (*Client, error) {
	configuration := clientOptions{lookupEnv: os.LookupEnv}
	for _, option := range options {
		if option == nil {
			return nil, &ConfigurationError{Message: "NewClient options must not be nil."}
		}
		option(&configuration)
	}
	credentials, err := resolveCredentials(configuration.accessKey, configuration.secretKey, configuration.lookupEnv)
	if err != nil {
		return nil, err
	}

	accessKey := ""
	if credentials.accessKey != nil {
		accessKey = *credentials.accessKey
	}
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

	secretKey := ""
	if credentials.secretKey != nil {
		secretKey = *credentials.secretKey
		if strings.TrimSpace(secretKey) == "" || !utf8.ValidString(secretKey) {
			return nil, &ConfigurationError{
				Message: "The Screenshot Scout secret key must be a non-blank UTF-8 string when provided.",
			}
		}
	}
	httpClient := &http.Client{}
	if configuration.httpClient != nil {
		clonedHTTPClient := *configuration.httpClient
		httpClient = &clonedHTTPClient
	}
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &Client{
		accessKey:                accessKey,
		secretKey:                secretKey,
		secretKeyFromEnvironment: credentials.fromEnvironment && secretKey != "",
		endpoint:                 captureEndpointURL,
		httpClient:               httpClient,
	}, nil
}

// String describes the client without revealing its keys.
func (client Client) String() string {
	return "screenshotscout.Client{accessKey:" + redactKey(client.accessKey) +
		", secretKey:" + redactKey(client.secretKey) + "}"
}

// GoString returns the same description as String, for the %#v verb.
func (client Client) GoString() string {
	return client.String()
}

func redactKey(key string) string {
	if key == "" {
		return `""`
	}
	return `"[redacted]"`
}

// Capture performs one inline Screenshot Scout request and waits for the final
// buffered response. When method is omitted, Capture uses POST, or GET when the
// client has a secret key. At most one method may be supplied, and it must be
// CaptureHTTPMethodGET or CaptureHTTPMethodPOST. A configured secret key signs
// GET requests. POST requests can't be signed, so a client with a secret key
// returns a ConfigurationError for CaptureHTTPMethodPOST without sending a
// request. Cancellation follows the supplied context's normal semantics.
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
	// Without a method, a client that signs sends GET, because POST requests can't be signed.
	defaultMethod := CaptureHTTPMethodPOST
	if client.secretKey != "" {
		defaultMethod = CaptureHTTPMethodGET
	}
	captureMethod, err := resolveCaptureHTTPMethod(method, defaultMethod)
	if err != nil {
		return nil, err
	}
	if captureMethod == CaptureHTTPMethodPOST && client.secretKey != "" {
		source := ""
		if client.secretKeyFromEnvironment {
			source = " from " + secretKeyVariable
		}
		return nil, &ConfigurationError{
			Message: "POST requests can't be signed. This client has a secret key" + source + ": send this capture with method GET, or use a client without a secret key and an API key that doesn't require signed requests.",
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	requestURL := client.endpoint
	var body io.Reader
	if captureMethod == CaptureHTTPMethodGET {
		pairs := append([]wirePair(nil), serialized.pairs...)
		if signature := client.createSignature(serialized.pairs); signature != "" {
			pairs = append(pairs, wirePair{name: "signature", value: signature})
		}
		requestURL += "?" + encodeQueryPairs(pairs)
	} else {
		encodedBody, encodeErr := encodeJSONBody(serialized.body)
		if encodeErr != nil {
			return nil, encodeErr
		}
		body = bytes.NewReader(encodedBody)
	}

	request, err := http.NewRequestWithContext(ctx, string(captureMethod), requestURL, body)
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
		if contextErr := ctx.Err(); contextErr != nil {
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
		if contextErr := ctx.Err(); contextErr != nil {
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

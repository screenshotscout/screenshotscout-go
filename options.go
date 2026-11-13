package screenshotscout

import "net/http"

// Option configures a Client. Pass options to NewClient.
type Option func(*clientOptions)

type clientOptions struct {
	accessKey  *string
	secretKey  *string
	baseURL    string
	httpClient *http.Client
	lookupEnv  func(string) (string, bool)
}

// WithAccessKey sets the access key instead of reading
// SCREENSHOTSCOUT_ACCESS_KEY. Passing either key means NewClient reads neither
// environment variable. A blank key is a ConfigurationError.
func WithAccessKey(accessKey string) Option {
	return func(options *clientOptions) { options.accessKey = &accessKey }
}

// WithSecretKey sets the secret key that signs capture requests and capture
// URLs, instead of reading SCREENSHOTSCOUT_SECRET_KEY. Passing either key means
// NewClient reads neither environment variable. A blank key is a
// ConfigurationError.
func WithSecretKey(secretKey string) Option {
	return func(options *clientOptions) { options.secretKey = &secretKey }
}

// WithBaseURL sets the base URL of another server to send requests to, such as
// your own proxy or a mock server. Captures go to the base URL plus
// /v1/capture, and BuildCaptureURL builds URLs on it. Without it, or with "",
// NewClient reads SCREENSHOTSCOUT_BASE_URL, and without that the client uses
// the Screenshot Scout API.
func WithBaseURL(baseURL string) Option {
	return func(options *clientOptions) { options.baseURL = baseURL }
}

// WithHTTPClient sets the http.Client that sends API requests. Without it, or
// with nil, the client uses a new http.Client. Requests don't follow redirects,
// and the passed client isn't modified.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(options *clientOptions) { options.httpClient = httpClient }
}

// withLookupEnv replaces os.LookupEnv, so tests can run in parallel.
func withLookupEnv(lookupEnv func(string) (string, bool)) Option {
	return func(options *clientOptions) { options.lookupEnv = lookupEnv }
}

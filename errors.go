package screenshotscout

import (
	"errors"
	"fmt"
)

var (
	// ErrConfiguration identifies invalid client configuration.
	ErrConfiguration = errors.New("screenshotscout: configuration error")
	// ErrSerialization identifies values that cannot be serialized safely.
	ErrSerialization = errors.New("screenshotscout: serialization error")
	// ErrTransport identifies failures before a complete HTTP response body is received.
	ErrTransport = errors.New("screenshotscout: transport error")
	// ErrAPI identifies non-2xx Screenshot Scout HTTP responses.
	ErrAPI = errors.New("screenshotscout: API error")
	// ErrResponseDecoding identifies invalid or representation-mismatched 2xx responses.
	ErrResponseDecoding = errors.New("screenshotscout: response decoding error")
)

// ConfigurationError reports invalid client configuration.
type ConfigurationError struct {
	Message string
	Cause   error
}

func (e *ConfigurationError) Error() string { return e.Message }

func (e *ConfigurationError) Unwrap() error { return e.Cause }

func (e *ConfigurationError) Is(target error) bool { return target == ErrConfiguration }

// SerializationError reports a value that cannot be represented safely on the wire.
type SerializationError struct {
	Message string
	Option  string
	Cause   error
}

func (e *SerializationError) Error() string { return e.Message }

func (e *SerializationError) Unwrap() error { return e.Cause }

func (e *SerializationError) Is(target error) bool { return target == ErrSerialization }

// TransportError reports a failure before a complete HTTP response body is
// available. Unwrap exposes the native transport cause.
type TransportError struct {
	Message string
	Cause   error
}

func (e *TransportError) Error() string { return e.Message }

func (e *TransportError) Unwrap() error { return e.Cause }

func (e *TransportError) Is(target error) bool { return target == ErrTransport }

// APIError reports a non-2xx Screenshot Scout response while retaining the
// parsed service fields, complete parsed body when available, and raw response.
type APIError struct {
	StatusCode   int
	ErrorCode    *string
	ErrorMessage *string
	Errors       []any
	ResponseBody any
	RawResponse  RawResponse
}

func (e *APIError) Error() string {
	if e.ErrorMessage != nil {
		return *e.ErrorMessage
	}
	return fmt.Sprintf("Screenshot Scout API request failed with status %d.", e.StatusCode)
}

func (e *APIError) Is(target error) bool { return target == ErrAPI }

// ResponseDecodingError reports an invalid or representation-mismatched 2xx
// response while retaining the raw response.
type ResponseDecodingError struct {
	Message     string
	RawResponse RawResponse
	Cause       error
}

func (e *ResponseDecodingError) Error() string { return e.Message }

func (e *ResponseDecodingError) Unwrap() error { return e.Cause }

func (e *ResponseDecodingError) Is(target error) bool { return target == ErrResponseDecoding }

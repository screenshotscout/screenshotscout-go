package screenshotscout

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type expectedResponseKind string

const (
	expectedResponseBinary expectedResponseKind = "binary"
	expectedResponseJSON   expectedResponseKind = "json"
)

func expectedKind(options *CaptureOptions) expectedResponseKind {
	if options == nil || options.ResponseType == nil || *options.ResponseType == CaptureResponseTypeBinary {
		return expectedResponseBinary
	}
	if *options.ResponseType == CaptureResponseTypeJSON {
		return expectedResponseJSON
	}
	return ""
}

func createAPIError(rawResponse RawResponse) *APIError {
	apiError := &APIError{
		StatusCode:  rawResponse.StatusCode,
		RawResponse: rawResponse,
	}

	var parsed any
	if err := json.Unmarshal(rawResponse.Body, &parsed); err != nil {
		return apiError
	}
	apiError.ResponseBody = parsed
	object, ok := parsed.(map[string]any)
	if !ok {
		return apiError
	}
	if value, ok := object["error_code"].(string); ok {
		apiError.ErrorCode = stringPointer(value)
	}
	if value, ok := object["error_message"].(string); ok {
		apiError.ErrorMessage = stringPointer(value)
	}
	if value, ok := object["errors"].([]any); ok {
		apiError.Errors = value
	}
	return apiError
}

func decodeSuccessResponse(rawResponse RawResponse, expected expectedResponseKind) (*CaptureResponse, error) {
	actual := expectedResponseBinary
	if isJSONMediaType(rawResponse.ContentType) {
		actual = expectedResponseJSON
	}

	if expected != "" && actual != expected {
		return nil, &ResponseDecodingError{
			Message: fmt.Sprintf(
				"Screenshot Scout returned a successful %s response when %s was requested.",
				actual,
				expected,
			),
			RawResponse: rawResponse,
			Cause:       fmt.Errorf("expected a %s response but received %s", expected, actual),
		}
	}

	if actual == expectedResponseJSON {
		return decodeJSONResponse(rawResponse)
	}
	return decodeBinaryResponse(rawResponse), nil
}

func decodeBinaryResponse(rawResponse RawResponse) *CaptureResponse {
	binary := &BinaryCaptureResponse{
		Bytes:                  rawResponse.Body,
		ScreenshotURL:          firstHeader(rawResponse.Headers, "Screenshot-Scout-Screenshot-URL"),
		ScreenshotURLExpiresAt: firstHeader(rawResponse.Headers, "Screenshot-Scout-Screenshot-URL-Expires-At"),
		CacheStatus:            firstHeader(rawResponse.Headers, "Screenshot-Scout-Cache-Status"),
	}
	return &CaptureResponse{
		Kind:        CaptureResponseKindBinary,
		Binary:      binary,
		RawResponse: rawResponse,
	}
}

func decodeJSONResponse(rawResponse RawResponse) (*CaptureResponse, error) {
	var parsed any
	if err := json.Unmarshal(rawResponse.Body, &parsed); err != nil {
		return nil, &ResponseDecodingError{
			Message:     "Screenshot Scout returned a successful JSON response that could not be decoded.",
			RawResponse: rawResponse,
			Cause:       err,
		}
	}
	object, ok := parsed.(map[string]any)
	if !ok {
		return nil, &ResponseDecodingError{
			Message:     "Screenshot Scout returned a successful JSON response that was not an object.",
			RawResponse: rawResponse,
			Cause:       fmt.Errorf("expected a JSON object"),
		}
	}

	screenshotURL, err := readOptionalString(object, "screenshot_url", rawResponse)
	if err != nil {
		return nil, err
	}
	screenshotURLExpiresAt, err := readOptionalString(object, "screenshot_url_expires_at", rawResponse)
	if err != nil {
		return nil, err
	}
	cacheStatus, err := readOptionalString(object, "cache_status", rawResponse)
	if err != nil {
		return nil, err
	}

	additionalFields := make(map[string]any)
	for key, value := range object {
		switch key {
		case "screenshot_url", "screenshot_url_expires_at", "cache_status":
			continue
		default:
			additionalFields[key] = value
		}
	}

	return &CaptureResponse{
		Kind: CaptureResponseKindJSON,
		JSON: &JSONCaptureResponse{
			Result: CaptureResult{
				ScreenshotURL:          screenshotURL,
				ScreenshotURLExpiresAt: screenshotURLExpiresAt,
				CacheStatus:            cacheStatus,
				AdditionalFields:       additionalFields,
			},
		},
		RawResponse: rawResponse,
	}, nil
}

func readOptionalString(object map[string]any, key string, rawResponse RawResponse) (*string, error) {
	value, present := object[key]
	if !present {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, &ResponseDecodingError{
			Message:     fmt.Sprintf("Screenshot Scout returned a non-string %q JSON field.", key),
			RawResponse: rawResponse,
			Cause:       fmt.Errorf("expected %q to be a string", key),
		}
	}
	return stringPointer(text), nil
}

func firstHeader(headers http.Header, name string) *string {
	for headerName, values := range headers {
		if strings.EqualFold(headerName, name) && len(values) > 0 {
			return stringPointer(values[0])
		}
	}
	return nil
}

func stringPointer(value string) *string {
	return &value
}

func isJSONMediaType(contentType string) bool {
	mediaType := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	mediaType = strings.ToLower(mediaType)
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

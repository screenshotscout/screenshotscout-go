package screenshotscout

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxJSONSafeInteger int64 = 9_007_199_254_740_991

type wirePair struct {
	name  string
	value string
}

type serializedCaptureOptions struct {
	pairs []wirePair
	body  map[string]any
}

type captureSerializer struct {
	pairs []wirePair
	body  map[string]any
}

func serializeCaptureOptions(targetURL string, options *CaptureOptions) (serializedCaptureOptions, error) {
	if !utf8.ValidString(targetURL) {
		return serializedCaptureOptions{}, &SerializationError{
			Message: "The capture target URL must contain valid UTF-8 text.",
			Option:  "url",
		}
	}

	serializer := &captureSerializer{
		pairs: []wirePair{{name: "url", value: targetURL}},
		body:  map[string]any{"url": targetURL},
	}
	if options == nil {
		return serializedCaptureOptions{pairs: serializer.pairs, body: serializer.body}, nil
	}

	for _, add := range []func() error{
		func() error { return appendStringOption(serializer, "Format", "format", options.Format) },
		func() error {
			return appendStringOption(serializer, "ResponseType", "response_type", options.ResponseType)
		},
		func() error { return appendStringOption(serializer, "Country", "country", options.Country) },
		func() error { return appendStringOption(serializer, "Proxy", "proxy", options.Proxy) },
		func() error {
			return serializer.appendFloat("GeolocationLatitude", "geolocation_latitude", options.GeolocationLatitude)
		},
		func() error {
			return serializer.appendFloat("GeolocationLongitude", "geolocation_longitude", options.GeolocationLongitude)
		},
		func() error {
			return serializer.appendFloat("GeolocationAccuracy", "geolocation_accuracy", options.GeolocationAccuracy)
		},
		func() error { return serializer.appendRepeated("Cookies", "cookies", options.Cookies) },
		func() error { return serializer.appendRepeated("Headers", "headers", options.Headers) },
		func() error { return serializer.appendInt("Timeout", "timeout", options.Timeout) },
		func() error { return appendStringOption(serializer, "WaitUntil", "wait_until", options.WaitUntil) },
		func() error {
			return serializer.appendInt("NavigationTimeout", "navigation_timeout", options.NavigationTimeout)
		},
		func() error { return serializer.appendInt("Delay", "delay", options.Delay) },
		func() error { return appendStringOption(serializer, "Device", "device", options.Device) },
		func() error {
			return serializer.appendInt("DeviceViewportWidth", "device_viewport_width", options.DeviceViewportWidth)
		},
		func() error {
			return serializer.appendInt("DeviceViewportHeight", "device_viewport_height", options.DeviceViewportHeight)
		},
		func() error {
			return serializer.appendFloat("DeviceScaleFactor", "device_scale_factor", options.DeviceScaleFactor)
		},
		func() error {
			serializer.appendBool("DeviceIsMobile", "device_is_mobile", options.DeviceIsMobile)
			return nil
		},
		func() error {
			serializer.appendBool("DeviceHasTouch", "device_has_touch", options.DeviceHasTouch)
			return nil
		},
		func() error {
			return appendStringOption(serializer, "DeviceUserAgent", "device_user_agent", options.DeviceUserAgent)
		},
		func() error { return appendStringOption(serializer, "Timezone", "timezone", options.Timezone) },
		func() error { return appendStringOption(serializer, "MediaType", "media_type", options.MediaType) },
		func() error {
			return appendStringOption(serializer, "ColorScheme", "color_scheme", options.ColorScheme)
		},
		func() error {
			serializer.appendBool("ReducedMotion", "reduced_motion", options.ReducedMotion)
			return nil
		},
		func() error { serializer.appendBool("FullPage", "full_page", options.FullPage); return nil },
		func() error {
			serializer.appendBool("FullPagePreScroll", "full_page_pre_scroll", options.FullPagePreScroll)
			return nil
		},
		func() error {
			return serializer.appendInt("FullPagePreScrollStep", "full_page_pre_scroll_step", options.FullPagePreScrollStep)
		},
		func() error {
			return serializer.appendInt("FullPagePreScrollStepDelay", "full_page_pre_scroll_step_delay", options.FullPagePreScrollStepDelay)
		},
		func() error {
			return serializer.appendInt("FullPageMaxHeight", "full_page_max_height", options.FullPageMaxHeight)
		},
		func() error {
			serializer.appendBool("BlockCookieBanners", "block_cookie_banners", options.BlockCookieBanners)
			return nil
		},
		func() error { serializer.appendBool("BlockAds", "block_ads", options.BlockAds); return nil },
		func() error {
			serializer.appendBool("BlockChatWidgets", "block_chat_widgets", options.BlockChatWidgets)
			return nil
		},
		func() error {
			return serializer.appendRepeated("HideSelectors", "hide_selectors", options.HideSelectors)
		},
		func() error {
			return serializer.appendRepeated("ClickSelectors", "click_selectors", options.ClickSelectors)
		},
		func() error {
			return serializer.appendRepeated("ClickAllSelectors", "click_all_selectors", options.ClickAllSelectors)
		},
		func() error { return serializer.appendRepeated("InjectCSS", "inject_css", options.InjectCSS) },
		func() error { return serializer.appendRepeated("InjectJS", "inject_js", options.InjectJS) },
		func() error { serializer.appendBool("BypassCSP", "bypass_csp", options.BypassCSP); return nil },
		func() error { return appendStringOption(serializer, "Selector", "selector", options.Selector) },
		func() error { return serializer.appendInt("ClipX", "clip_x", options.ClipX) },
		func() error { return serializer.appendInt("ClipY", "clip_y", options.ClipY) },
		func() error { return serializer.appendInt("ClipWidth", "clip_width", options.ClipWidth) },
		func() error { return serializer.appendInt("ClipHeight", "clip_height", options.ClipHeight) },
		func() error { return serializer.appendInt("ImageWidth", "image_width", options.ImageWidth) },
		func() error { return serializer.appendInt("ImageHeight", "image_height", options.ImageHeight) },
		func() error { return appendStringOption(serializer, "ImageMode", "image_mode", options.ImageMode) },
		func() error {
			return appendStringOption(serializer, "ImageAnchor", "image_anchor", options.ImageAnchor)
		},
		func() error {
			serializer.appendBool("ImageAllowUpscale", "image_allow_upscale", options.ImageAllowUpscale)
			return nil
		},
		func() error {
			return appendStringOption(serializer, "ImageBackground", "image_background", options.ImageBackground)
		},
		func() error { return serializer.appendInt("ImageQuality", "image_quality", options.ImageQuality) },
		func() error {
			return appendStringOption(serializer, "PDFPaperFormat", "pdf_paper_format", options.PDFPaperFormat)
		},
		func() error { serializer.appendBool("PDFLandscape", "pdf_landscape", options.PDFLandscape); return nil },
		func() error {
			serializer.appendBool("PDFPrintBackground", "pdf_print_background", options.PDFPrintBackground)
			return nil
		},
		func() error { return appendStringOption(serializer, "PDFMargin", "pdf_margin", options.PDFMargin) },
		func() error {
			return appendStringOption(serializer, "PDFMarginTop", "pdf_margin_top", options.PDFMarginTop)
		},
		func() error {
			return appendStringOption(serializer, "PDFMarginRight", "pdf_margin_right", options.PDFMarginRight)
		},
		func() error {
			return appendStringOption(serializer, "PDFMarginBottom", "pdf_margin_bottom", options.PDFMarginBottom)
		},
		func() error {
			return appendStringOption(serializer, "PDFMarginLeft", "pdf_margin_left", options.PDFMarginLeft)
		},
		func() error { return serializer.appendFloat("PDFScale", "pdf_scale", options.PDFScale) },
		func() error { serializer.appendBool("Cache", "cache", options.Cache); return nil },
		func() error { return serializer.appendInt("CacheTTL", "cache_ttl", options.CacheTTL) },
		func() error { return appendStringOption(serializer, "CacheKey", "cache_key", options.CacheKey) },
		func() error {
			return appendStringOption(serializer, "StorageMode", "storage_mode", options.StorageMode)
		},
		func() error {
			return appendStringOption(serializer, "StorageEndpoint", "storage_endpoint", options.StorageEndpoint)
		},
		func() error {
			return appendStringOption(serializer, "StorageBucket", "storage_bucket", options.StorageBucket)
		},
		func() error {
			return appendStringOption(serializer, "StorageRegion", "storage_region", options.StorageRegion)
		},
		func() error {
			return appendStringOption(serializer, "StorageObjectKey", "storage_object_key", options.StorageObjectKey)
		},
	} {
		if err := add(); err != nil {
			return serializedCaptureOptions{}, err
		}
	}

	return serializedCaptureOptions{pairs: serializer.pairs, body: serializer.body}, nil
}

func appendStringOption[T ~string](serializer *captureSerializer, property, wireName string, value *T) error {
	if value == nil {
		return nil
	}
	text := string(*value)
	if !utf8.ValidString(text) {
		return &SerializationError{
			Message: fmt.Sprintf("The capture option %q must contain valid UTF-8 text.", property),
			Option:  property,
		}
	}
	serializer.pairs = append(serializer.pairs, wirePair{name: wireName, value: text})
	serializer.body[wireName] = text
	return nil
}

func (serializer *captureSerializer) appendRepeated(property, wireName string, values []string) error {
	if len(values) == 0 {
		return nil
	}
	copyOfValues := make([]string, len(values))
	for index, value := range values {
		if !utf8.ValidString(value) {
			return &SerializationError{
				Message: fmt.Sprintf("The capture option %q must contain only valid UTF-8 strings.", property),
				Option:  property,
			}
		}
		copyOfValues[index] = value
		serializer.pairs = append(serializer.pairs, wirePair{name: wireName, value: value})
	}
	serializer.body[wireName] = copyOfValues
	return nil
}

func (serializer *captureSerializer) appendInt(property, wireName string, value *int) error {
	if value == nil {
		return nil
	}
	integer := int64(*value)
	if integer < -maxJSONSafeInteger || integer > maxJSONSafeInteger {
		return &SerializationError{
			Message: fmt.Sprintf("The capture option %q must use a JSON-safe integer.", property),
			Option:  property,
		}
	}
	serializer.pairs = append(serializer.pairs, wirePair{name: wireName, value: strconv.FormatInt(integer, 10)})
	serializer.body[wireName] = *value
	return nil
}

func (serializer *captureSerializer) appendFloat(property, wireName string, value *float64) error {
	if value == nil {
		return nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) {
		return &SerializationError{
			Message: fmt.Sprintf("The capture option %q must be a finite number.", property),
			Option:  property,
		}
	}
	serializer.pairs = append(serializer.pairs, wirePair{name: wireName, value: ecmaScriptNumberString(*value)})
	bodyValue := *value
	if bodyValue == 0 {
		bodyValue = 0 // Match JavaScript query and JSON serialization for negative zero.
	}
	serializer.body[wireName] = bodyValue
	return nil
}

func (serializer *captureSerializer) appendBool(_ string, wireName string, value *bool) {
	if value == nil {
		return
	}
	serializer.pairs = append(serializer.pairs, wirePair{name: wireName, value: strconv.FormatBool(*value)})
	serializer.body[wireName] = *value
}

func resolveCaptureHTTPMethod(methods []CaptureHTTPMethod) (CaptureHTTPMethod, error) {
	if len(methods) > 1 {
		return "", &SerializationError{
			Message: "Capture accepts at most one HTTP method selector.",
			Option:  "method",
		}
	}
	method := CaptureHTTPMethodPOST
	if len(methods) == 1 {
		method = methods[0]
	}
	if method != CaptureHTTPMethodGET && method != CaptureHTTPMethodPOST {
		return "", &SerializationError{
			Message: "The capture method must be CaptureHTTPMethodGET or CaptureHTTPMethodPOST.",
			Option:  "method",
		}
	}
	return method, nil
}

func buildCanonicalQuery(pairs []wirePair, accessKey string) string {
	entries := make([]wirePair, 0, len(pairs)+1)
	entries = append(entries, pairs...)
	entries = append(entries, wirePair{name: "access_key", value: accessKey})
	sort.SliceStable(entries, func(left, right int) bool {
		return entries[left].name < entries[right].name
	})
	return encodeQueryPairs(entries)
}

func encodeQueryPairs(pairs []wirePair) string {
	var builder strings.Builder
	for index, pair := range pairs {
		if index > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(formEncodeComponent(pair.name))
		builder.WriteByte('=')
		builder.WriteString(formEncodeComponent(pair.value))
	}
	return builder.String()
}

func formEncodeComponent(value string) string {
	const hex = "0123456789ABCDEF"
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		char := value[index]
		switch {
		case (char >= 'A' && char <= 'Z') ||
			(char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			char == '*' || char == '-' || char == '.' || char == '_':
			builder.WriteByte(char)
		case char == ' ':
			builder.WriteByte('+')
		default:
			builder.WriteByte('%')
			builder.WriteByte(hex[char>>4])
			builder.WriteByte(hex[char&0x0f])
		}
	}
	return builder.String()
}

func ecmaScriptNumberString(value float64) string {
	if value == 0 {
		return "0"
	}
	absolute := math.Abs(value)
	if absolute >= 1e-6 && absolute < 1e21 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}

	representation := strconv.FormatFloat(value, 'e', -1, 64)
	exponentIndex := strings.IndexByte(representation, 'e')
	if exponentIndex < 0 {
		return representation
	}
	exponent, err := strconv.Atoi(representation[exponentIndex+1:])
	if err != nil {
		return representation
	}
	mantissa := strings.TrimSuffix(representation[:exponentIndex], ".0")
	if exponent >= 0 {
		return fmt.Sprintf("%se+%d", mantissa, exponent)
	}
	return fmt.Sprintf("%se%d", mantissa, exponent)
}

func encodeJSONBody(body map[string]any) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, &SerializationError{
			Message: "The POST capture body could not be serialized as JSON.",
			Cause:   err,
		}
	}
	return encoded, nil
}

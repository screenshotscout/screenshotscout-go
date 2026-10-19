package screenshotscout

import "net/http"

// CaptureHTTPMethod is the closed, SDK-owned HTTP method selector for Capture.
type CaptureHTTPMethod string

const (
	CaptureHTTPMethodGET  CaptureHTTPMethod = "GET"
	CaptureHTTPMethodPOST CaptureHTTPMethod = "POST"
)

// CaptureFormat is an open string type for Screenshot Scout output formats.
type CaptureFormat string

const (
	CaptureFormatGIF  CaptureFormat = "gif"
	CaptureFormatJPEG CaptureFormat = "jpeg"
	CaptureFormatJPG  CaptureFormat = "jpg"
	CaptureFormatPDF  CaptureFormat = "pdf"
	CaptureFormatPNG  CaptureFormat = "png"
	CaptureFormatTIFF CaptureFormat = "tiff"
	CaptureFormatWEBP CaptureFormat = "webp"
)

// CaptureResponseType is an open string type for successful response representations.
type CaptureResponseType string

const (
	CaptureResponseTypeBinary CaptureResponseType = "binary"
	CaptureResponseTypeJSON   CaptureResponseType = "json"
)

// CaptureWaitUntil is an open string type for navigation completion conditions.
type CaptureWaitUntil string

const (
	CaptureWaitUntilDOMContentLoaded CaptureWaitUntil = "domcontentloaded"
	CaptureWaitUntilLoad             CaptureWaitUntil = "load"
	CaptureWaitUntilNetworkIdle0     CaptureWaitUntil = "networkidle0"
	CaptureWaitUntilNetworkIdle2     CaptureWaitUntil = "networkidle2"
)

// CaptureMediaType is an open string type for emulated CSS media types.
type CaptureMediaType string

const (
	CaptureMediaTypePrint  CaptureMediaType = "print"
	CaptureMediaTypeScreen CaptureMediaType = "screen"
)

// CaptureColorScheme is an open string type for emulated color schemes.
type CaptureColorScheme string

const (
	CaptureColorSchemeAuto  CaptureColorScheme = "auto"
	CaptureColorSchemeDark  CaptureColorScheme = "dark"
	CaptureColorSchemeLight CaptureColorScheme = "light"
)

// CaptureImageMode is an open string type for image resize modes.
type CaptureImageMode string

const (
	CaptureImageModeFill    CaptureImageMode = "fill"
	CaptureImageModeFit     CaptureImageMode = "fit"
	CaptureImageModeStretch CaptureImageMode = "stretch"
)

// CaptureImageAnchor is an open string type for image resize anchors.
type CaptureImageAnchor string

const (
	CaptureImageAnchorBottom      CaptureImageAnchor = "bottom"
	CaptureImageAnchorBottomLeft  CaptureImageAnchor = "bottom_left"
	CaptureImageAnchorBottomRight CaptureImageAnchor = "bottom_right"
	CaptureImageAnchorCenter      CaptureImageAnchor = "center"
	CaptureImageAnchorLeft        CaptureImageAnchor = "left"
	CaptureImageAnchorRight       CaptureImageAnchor = "right"
	CaptureImageAnchorTop         CaptureImageAnchor = "top"
	CaptureImageAnchorTopLeft     CaptureImageAnchor = "top_left"
	CaptureImageAnchorTopRight    CaptureImageAnchor = "top_right"
)

// CapturePDFPaperFormat is an open string type for PDF paper formats.
type CapturePDFPaperFormat string

const (
	CapturePDFPaperFormatA3      CapturePDFPaperFormat = "a3"
	CapturePDFPaperFormatA4      CapturePDFPaperFormat = "a4"
	CapturePDFPaperFormatA5      CapturePDFPaperFormat = "a5"
	CapturePDFPaperFormatA6      CapturePDFPaperFormat = "a6"
	CapturePDFPaperFormatLegal   CapturePDFPaperFormat = "legal"
	CapturePDFPaperFormatLetter  CapturePDFPaperFormat = "letter"
	CapturePDFPaperFormatTabloid CapturePDFPaperFormat = "tabloid"
)

// CaptureStorageMode is an open string type for screenshot storage modes.
type CaptureStorageMode string

const (
	CaptureStorageModeExternal CaptureStorageMode = "external"
	CaptureStorageModeManaged  CaptureStorageMode = "managed"
)

// CaptureOptions contains Screenshot Scout service options. URL, access_key,
// signature, and the request HTTP method are deliberately separate.
//
// Pointer fields preserve the distinction between omission and explicit zero,
// false, or an empty string. Empty repeated slices are omitted.
type CaptureOptions struct {
	// Target and output.
	Format       *CaptureFormat
	ResponseType *CaptureResponseType

	// Network and location.
	Country              *string
	Proxy                *string
	GeolocationLatitude  *float64
	GeolocationLongitude *float64
	GeolocationAccuracy  *float64

	// Cookies and webpage request headers. Order and duplicates are preserved.
	Cookies []string
	Headers []string

	// Navigation and service-side timing.
	Timeout           *int
	WaitUntil         *CaptureWaitUntil
	NavigationTimeout *int
	Delay             *int

	// Viewport and device emulation.
	Device               *string
	DeviceViewportWidth  *int
	DeviceViewportHeight *int
	DeviceScaleFactor    *float64
	DeviceIsMobile       *bool
	DeviceHasTouch       *bool
	DeviceUserAgent      *string

	// Page behavior and preferences.
	Timezone      *string
	MediaType     *CaptureMediaType
	ColorScheme   *CaptureColorScheme
	ReducedMotion *bool

	// Full-page capture and pre-scroll behavior.
	FullPage                   *bool
	FullPagePreScroll          *bool
	FullPagePreScrollStep      *int
	FullPagePreScrollStepDelay *int
	FullPageMaxHeight          *int

	// Blocking.
	BlockCookieBanners *bool
	BlockAds           *bool
	BlockChatWidgets   *bool

	// DOM interactions and injections. Order and duplicates are preserved.
	HideSelectors     []string
	ClickSelectors    []string
	ClickAllSelectors []string
	InjectCSS         []string
	InjectJS          []string
	BypassCSP         *bool

	// Framing and selection.
	Selector   *string
	ClipX      *int
	ClipY      *int
	ClipWidth  *int
	ClipHeight *int

	// Image resizing and quality.
	ImageWidth        *int
	ImageHeight       *int
	ImageMode         *CaptureImageMode
	ImageAnchor       *CaptureImageAnchor
	ImageAllowUpscale *bool
	ImageBackground   *string
	ImageQuality      *int

	// PDF.
	PDFPaperFormat     *CapturePDFPaperFormat
	PDFLandscape       *bool
	PDFPrintBackground *bool
	PDFMargin          *string
	PDFMarginTop       *string
	PDFMarginRight     *string
	PDFMarginBottom    *string
	PDFMarginLeft      *string
	PDFScale           *float64
	PDFPageRanges      *string

	// Caching.
	Cache    *bool
	CacheTTL *int
	CacheKey *string

	// Storage.
	StorageMode      *CaptureStorageMode
	StorageEndpoint  *string
	StorageBucket    *string
	StorageRegion    *string
	StorageObjectKey *string
}

// ClientOptions configures signing and HTTP transport for a reusable Client.
type ClientOptions struct {
	SecretKey  string
	HTTPClient *http.Client
}

// CaptureResponseKind identifies which CaptureResponse member is populated.
type CaptureResponseKind string

const (
	CaptureResponseKindBinary CaptureResponseKind = "binary"
	CaptureResponseKindJSON   CaptureResponseKind = "json"
)

// RawResponse contains the exact buffered HTTP response data retained on
// successes and API failures.
type RawResponse struct {
	StatusCode  int
	Status      string
	Headers     http.Header
	ContentType string
	Body        []byte
}

// CaptureResult contains documented JSON result metadata and any additional
// response fields returned by the service.
type CaptureResult struct {
	ScreenshotURL          *string
	ScreenshotURLExpiresAt *string
	CacheStatus            *string
	AdditionalFields       map[string]any
}

// BinaryCaptureResponse contains a buffered binary capture result.
type BinaryCaptureResponse struct {
	Bytes                  []byte
	ScreenshotURL          *string
	ScreenshotURLExpiresAt *string
	CacheStatus            *string
}

// JSONCaptureResponse contains a decoded JSON capture result.
type JSONCaptureResponse struct {
	Result CaptureResult
}

// CaptureResponse is the discriminated Go response shape. Exactly one of
// Binary and JSON is non-nil on a successful return.
type CaptureResponse struct {
	Kind        CaptureResponseKind
	Binary      *BinaryCaptureResponse
	JSON        *JSONCaptureResponse
	RawResponse RawResponse
}

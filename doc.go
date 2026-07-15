// Package screenshotscout provides a small, synchronous Go client for the
// inline Screenshot Scout capture API.
//
// Client.Capture waits for the final buffered response. A required context
// controls cancellation, and callers can use ordinary goroutines for
// concurrent I/O. Client.BuildCaptureURL constructs a sensitive capture URL
// without performing network I/O.
package screenshotscout

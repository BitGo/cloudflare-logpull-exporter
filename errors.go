package main

const (
	// ErrKindHTTPProto should be used to signal an error which occurred
	// when trying to speak HTTP, whether due to a network or protocol
	// error.
	errKindHTTPProto = "http_protocol"

	// ErrKindHTTPStatus should be used to signal that an unexpected HTTP
	// status was received from an API response
	errKindHTTPStatus = "http_status"

	// ErrKindJSONParse should be used to signal that an unexpected error
	// occurred while parsing the JSON body of an API response
	errKindJSONParse = "json_parse"
)

// retryableAPIError is used to express that a given error was the result of
// something abnormal happening with the Cloudflare API. It may have aborted
// whatever processing we were doing with the API response, but not in any
// irrecoverable way. We may simply retry.
type retryableAPIError struct {
	error
	kind      string
	operation string
}

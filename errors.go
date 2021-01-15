package main

const (
	// An error when trying to speak HTTP, either a network or protocol error
	ErrKindHTTPProto = "http_protocol"

	// An unexpected HTTP status was received from an API response
	ErrKindHTTPStatus = "http_status"

	// An error occurred while parsing the JSON body of an API response
	ErrKindJSONParse = "json_parse"
)

type RetryableAPIError struct {
	error
	Kind      string
	Operation string
}

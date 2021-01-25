package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

// LogEntry contains all of the fields we care about from Cloudflare Logpull
// API response data. It is the target type of JSON unmarshaling and is safe to
// use as a map key.
type LogEntry struct {
	ClientRequestHost    string `json:"ClientRequestHost"`
	EdgeResponseStatus   int    `json:"EdgeResponseStatus"`
	OriginResponseStatus int    `json:"OriginResponseStatus"`
}

// GetLogEntries makes a request to Cloudflare's Logpull API, requesting
// LogEntries for the given zoneID between the given start and end time. Each
// entry is parsed into a LogEntry and passed to the given function. If any
// error occurs, it is returned to the caller. If the error is presumably safe
// to retry (i.e., non-fatal), it will have the type RetryableAPIError.
func GetLogEntries(api *cloudflare.API, zoneID string, start, end time.Time, fn func(LogEntry)) error {
	fields := []string{
		"ClientRequestHost",
		"EdgeResponseStatus",
		"OriginResponseStatus",
	}

	url := api.BaseURL + "/zones/" + zoneID + "/logs/received"
	url += "?start=" + start.Format(time.RFC3339)
	url += "&end=" + end.Format(time.RFC3339)
	url += "&fields=" + strings.Join(fields, ",")

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("Error constructing logpull request: %s", err.Error())
	}

	if api.APIToken != "" {
		req.Header.Add("Authorization", "Bearer "+api.APIToken)
	} else if api.APIEmail != "" && api.APIKey != "" {
		req.Header.Add("X-Auth-Email", api.APIEmail)
		req.Header.Add("X-Auth-Key", api.APIKey)
	} else {
		return errors.New("Unsupported auth scheme")
	}

	client := new(http.Client)
	resp, err := client.Do(req)

	operation := "getLogEntries"

	if err != nil {
		return RetryableAPIError{
			error:     err,
			Operation: operation,
			Kind:      ErrKindHTTPProto,
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return RetryableAPIError{
			error:     fmt.Errorf("Received unexpected HTTP status code: %d", resp.StatusCode),
			Operation: operation,
			Kind:      ErrKindHTTPStatus,
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return RetryableAPIError{
				error:     err,
				Operation: operation,
				Kind:      ErrKindJSONParse,
			}
		}
		fn(entry)
	}

	return nil
}

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

// logEntry contains all of the fields we care about from Cloudflare Logpull
// API response data. It is the target type of JSON unmarshaling and is safe to
// use as a map key.
type logEntry struct {
	ClientRequestHost    string `json:"ClientRequestHost"`
	EdgeResponseStatus   int    `json:"EdgeResponseStatus"`
	OriginResponseStatus int    `json:"OriginResponseStatus"`
}

// getLogEntries makes a request to Cloudflare's Logpull API, requesting log
// entries for the given zoneID between the given start and end time. Each
// entry is parsed into a logEntry struct and passed to the given handler
// function. If any error occurs, it is returned to the caller. If the error
// is presumably safe to retry (i.e., non-fatal), it will have the type
// RetryableAPIError.
func getLogEntries(api *cloudflare.API, zoneID string, start, end time.Time, fn func(logEntry)) error {
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
		return retryableAPIError{
			error:     err,
			operation: operation,
			kind:      errKindHTTPProto,
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Received unexpected HTTP status code: %d", resp.StatusCode)

		if resp.StatusCode == http.StatusBadRequest {
			err = fmt.Errorf("Logpull retention must be enabled: %w", err)
		}

		return retryableAPIError{
			error:     err,
			operation: operation,
			kind:      errKindHTTPStatus,
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		var entry logEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return retryableAPIError{
				error:     err,
				operation: operation,
				kind:      errKindJSONParse,
			}
		}
		fn(entry)
	}

	return nil
}

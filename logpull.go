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

type LogEntry struct {
	ClientRequestHost    string `json:"ClientRequestHost"`
	EdgeResponseStatus   int    `json:"EdgeResponseStatus"`
	OriginResponseStatus int    `json:"OriginResponseStatus"`
}

// Get log entries from Cloudflare's Logpull API
func getLogEntries(api *cloudflare.API, zoneID string, start, end time.Time, fn func(LogEntry)) error {
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
	defer resp.Body.Close()

	operation := "getLogEntries"

	if err != nil {
		return RetryableAPIError{
			error:     err,
			Operation: operation,
			Kind:      ErrKindHTTPProto,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return RetryableAPIError{
			error:     err,
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
		} else {
			fn(entry)
		}
	}

	return nil
}

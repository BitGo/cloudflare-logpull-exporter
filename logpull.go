package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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
// function.
func getLogEntries(api *cloudflare.API, zoneID string, start, end time.Time, handler func(logEntry) error) error {
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
		return fmt.Errorf("creating api request: %w", err)
	}

	if api.APIToken != "" {
		req.Header.Add("Authorization", "Bearer "+api.APIToken)
	} else if api.APIEmail != "" && api.APIKey != "" {
		req.Header.Add("X-Auth-Email", api.APIEmail)
		req.Header.Add("X-Auth-Key", api.APIKey)
	} else {
		return errors.New("creating api request: unusable auth parameters")
	}

	client := new(http.Client)
	resp, err := client.Do(req)

	if err != nil {
		return fmt.Errorf("performing api request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("reading api response body: %w", err)
		} else {
			err = fmt.Errorf("unexpected api response: %s: %s", resp.Status, respBody)
		}
		return err
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		var entry logEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return fmt.Errorf("json: %w", err)
		}
		if err := handler(entry); err != nil {
			return fmt.Errorf("handler: %w", err)
		}
	}

	return nil
}

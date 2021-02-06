package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// defaultBaseURL is the base URL for all API calls, unless explicitly
// overridden by the client.
const defaultBaseURL = "https://api.cloudflare.com/client/v4"

// authType represents the various Cloudflare API authentication schemes
type authType int

const (
	// authKeyEmail specifies that we should authenticate with API key and email address
	authKeyEmail authType = iota
	// authUserService specifies that we should authenticate with a User-Service key
	authUserService
	// authToken specifies that we should authenticate with an API token
	authToken
)

// logEntry contains all of the fields we care about from Cloudflare Logpull
// API response data. It is the target type of JSON unmarshaling and is safe to
// use as a map key.
type logEntry struct {
	ClientRequestHost    string `json:"ClientRequestHost"`
	EdgeResponseStatus   int    `json:"EdgeResponseStatus"`
	OriginResponseStatus int    `json:"OriginResponseStatus"`
}

// logpullAPI is a minimal Cloudflare API client to handle Cloudflare's Logpull
// API endpoint. This is needed because the official Cloudflare API client does
// not support this endpoint yet.
type logpullAPI struct {
	httpClient     *http.Client
	baseURL        string
	authType       authType
	apiKey         string
	apiEmail       string
	apiToken       string
	apiUserService string
}

// newLogpullAPI creates a new Logpull API client from an API key and email
// address.
func newLogpullAPI(key, email string) *logpullAPI {
	return &logpullAPI{
		httpClient: http.DefaultClient,
		baseURL:    defaultBaseURL,
		authType:   authKeyEmail,
		apiKey:     key,
		apiEmail:   email,
	}
}

// newLogpullAPIWithToken creates a new Logpull API client from an API token.
func newLogpullAPIWithToken(token string) *logpullAPI {
	return &logpullAPI{
		httpClient: http.DefaultClient,
		baseURL:    defaultBaseURL,
		authType:   authToken,
		apiToken:   token,
	}
}

// newLogpullAPIWithUserServiceKey creates a new Logpull API client from a
// User-Service key.
func newLogpullAPIWithUserServiceKey(key string) *logpullAPI {
	return &logpullAPI{
		httpClient:     http.DefaultClient,
		baseURL:        defaultBaseURL,
		authType:       authUserService,
		apiUserService: key,
	}
}

// setAPIProperties may be used to set a nonstandard base URL for API requests
// and/or a custom HTTP client. If either parameter is set to its zero value,
// the default is used.
func (api *logpullAPI) setAPIProperties(baseURL string, httpClient *http.Client) {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	api.baseURL = baseURL
	api.httpClient = httpClient
}

// logHandler is a function which is called by pullLogEntries for each parsed
// log entry.
type logHandler func(logEntry) error

// pullLogEntries makes a request to Cloudflare's Logpull API, requesting log
// entries for the given zoneID between the given start and end time. Each
// entry is parsed into a logEntry struct and passed to the given logHandler.
func (api *logpullAPI) pullLogEntries(zoneID string, start, end time.Time, handler logHandler) error {
	// The API will only return the requested fields; thus, if we add or
	// remove fields from the logEntry struct definition, we'll also want
	// to make sure we update this list to ask the API for the same.
	fields := []string{
		"ClientRequestHost",
		"EdgeResponseStatus",
		"OriginResponseStatus",
	}

	url := api.baseURL + "/zones/" + zoneID + "/logs/received"
	url += "?start=" + start.Format(time.RFC3339)
	url += "&end=" + end.Format(time.RFC3339)
	url += "&fields=" + strings.Join(fields, ",")

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating api request: %w", err)
	}

	req.Header.Add("Accept", "application/json")

	if api.authType == authToken {
		req.Header.Add("Authorization", "Bearer "+api.apiToken)
	}

	if api.authType == authKeyEmail {
		req.Header.Add("X-Auth-Key", api.apiKey)
		req.Header.Add("X-Auth-Email", api.apiEmail)
	}

	if api.authType == authUserService {
		req.Header.Add("X-Auth-User-Service-Key", api.apiUserService)
	}

	resp, err := api.httpClient.Do(req)
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

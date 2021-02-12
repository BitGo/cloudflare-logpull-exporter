package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

var (
	goodToken          = "good-token"
	goodKey            = "good-key"
	goodUserServiceKey = "good-user-service-key"
	goodEmail          = "good@example.org"

	goodZoneID                 = "good-zone-id"
	nonexistentZoneID          = "nonexistent-zone-id"
	unauthorizedZoneID         = "unauthorized-zone-id"
	logRetentionDisabledZoneID = "log-retention-disabled-zone-id"

	goodEnd        = time.Date(2021, time.January, 1, 12, 0, 0, 0, time.UTC)
	goodStart      = goodEnd.Add(-1 * time.Minute)
	tooEarlyEnd    = time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	tooEarlyStart  = tooEarlyEnd.Add(-1 * time.Minute)
	tooRecentEnd   = time.Date(2021, time.January, 1, 18, 0, 0, 0, time.UTC)
	tooRecentStart = tooRecentEnd.Add(-1 * time.Minute)

	logEntryJSON     = []byte(`{"ClientRequestHost": "example.org", "EdgeResponseStatus": 200, "OriginResponseStatus": 200}`)
	expectedLogEntry = logEntry{ClientRequestHost: "example.org", EdgeResponseStatus: 200, OriginResponseStatus: 200}

	nopLogHandler = func(logEntry) error { return nil }
)

// mockHandlerFunc allows us to write HTTP handler functions that return
// errors. If an error is returned, it is passed to t.Fatal.
func mockHandlerFunc(t *testing.T, h func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			t.Fatal(err)
		}
	})
}

func mockLogpullHandler(w http.ResponseWriter, r *http.Request) error {
	pathRegexp := regexp.MustCompile(`/zones/(.+)/logs/received`)

	if !pathRegexp.MatchString(r.URL.Path) {
		return fmt.Errorf("called unexpected endpoint: %s", r.URL.Path)
	}

	zoneID := pathRegexp.FindStringSubmatch(r.URL.Path)[1]

	authOk := r.Header.Get("Authorization") == "Bearer "+goodToken
	authOk = authOk || r.Header.Get("X-Auth-Key") == goodKey && r.Header.Get("X-Auth-Email") == goodEmail
	authOk = authOk || r.Header.Get("X-Auth-User-Service-Key") == goodUserServiceKey
	if !authOk {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`))
		return err
	}

	if zoneID == nonexistentZoneID || zoneID == unauthorizedZoneID {
		w.WriteHeader(http.StatusForbidden)
		_, err := w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`))
		return err
	}

	if zoneID == logRetentionDisabledZoneID {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("Retention is not turned on. Please enable log retention"))
		return err
	}

	start, err := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("bad query: error parsing start time: must be unix timestamp or rfc3339 string"))
		return err
	}

	end, err := time.Parse(time.RFC3339, r.URL.Query().Get("end"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("bad query: error parsing end time: must be unix timestamp or rfc3339 string"))
		return err
	}

	if end.Before(start) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("bad query: error parsing time: invalid time range: start not before end"))
		return err
	}

	if start == tooEarlyStart || end == tooEarlyEnd {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("bad query: error parsing time: invalid time range: too early: logs older than 168h0m0s are not available"))
		return err
	}

	if start == tooRecentStart || end == tooRecentEnd {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("bad query: error parsing time: invalid time range: too recent: minimum delay in serving logs is 1m0s"))
		return err
	}

	_, err = w.Write(logEntryJSON)
	return err
}

// TestPullLogEntries will attempt to pull logs from a mock Cloudflare API
// server using sentinel 'good' parameters. It fails if the parsed logEntry
// does not match or expected value or if pullLogEntries returns an error.
func TestPullLogEntries(t *testing.T) {
	ts := httptest.NewServer(mockHandlerFunc(t, mockLogpullHandler))
	defer ts.Close()

	api := newLogpullAPI(goodKey, goodEmail)
	api.setAPIProperties(ts.URL, ts.Client())

	if err := api.pullLogEntries(goodZoneID, goodStart, goodEnd, func(entry logEntry) error {
		if entry != expectedLogEntry {
			t.Error("parsed log entry did not match expected value")
		}
		return nil
	}); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

// TestPullLogEntriesLiveEndpoint will attempt to pull the last minute of logs
// against an actual Cloudflare zone with log retention enabled. It fails if
// pullLogEntries returns an error.
//
// This test is skipped unless the EXPORTER_TEST_LIVE_ENDPOINT environment
// variable is non-empty, and requires CLOUDFLARE_TEST_API_TOKEN and
// CLOUDFLARE_TEST_ZONE_NAME to be set appropriately.
func TestPullLogEntriesLiveEndpoint(t *testing.T) {
	if os.Getenv("EXPORTER_TEST_LIVE_ENDPOINT") == "" {
		t.Skip("skipping test of live API endpoint")
	}

	token := os.Getenv("CLOUDFLARE_TEST_API_TOKEN")
	if token == "" {
		t.Fatal("CLOUDFLARE_TEST_API_TOKEN must be specified")
	}

	zoneName := os.Getenv("CLOUDFLARE_TEST_ZONE_NAME")
	if zoneName == "" {
		t.Fatal("CLOUDFLARE_TEST_ZONE_NAME must be specified")
	}

	cfapi, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		t.Fatalf("creating cfapi client: %s", err)
	}

	zoneID, err := cfapi.ZoneIDByName(strings.TrimSpace(zoneName))
	if err != nil {
		t.Fatalf("zone id lookup: %s", err)
	}

	end := time.Now().Add(-1 * time.Minute)
	start := end.Add(-1 * time.Minute)

	lpapi := newLogpullAPIWithToken(token)
	err = lpapi.pullLogEntries(zoneID, start, end, nopLogHandler)
	if err != nil {
		t.Error(err)
	}
}

// TestPullLogEntriesErrors attempts to pull logs from a mock Cloudflare API
// server with combinations of valid and invalid parameters. It fails when
// pullLogEntries returns an error when an error isn't expected, or the
// inverse.
func TestPullLogEntriesErrors(t *testing.T) {
	testCases := []struct {
		condition         string
		isErrorExpected   bool
		authType          authType
		apiKey            string
		apiEmail          string
		apiUserServiceKey string
		apiToken          string
		zoneID            string
		start             time.Time
		end               time.Time
	}{
		{"with valid API key and email", false, authKeyEmail, goodKey, goodEmail, "", "", goodZoneID, goodStart, goodEnd},
		{"with invalid API key", true, authKeyEmail, "garbage", goodEmail, "", "", goodZoneID, goodStart, goodEnd},
		{"with invalid API email", true, authKeyEmail, goodKey, "garbage", "", "", goodZoneID, goodStart, goodEnd},

		{"with valid user service key", false, authUserService, "", "", goodUserServiceKey, "", goodZoneID, goodStart, goodEnd},
		{"with invalid user service key", true, authUserService, "", "", "garbage", "", goodZoneID, goodStart, goodEnd},

		{"with valid API token", false, authToken, "", "", "", goodToken, goodZoneID, goodStart, goodEnd},
		{"with invalid API token", true, authToken, "", "", "", "garbage", goodZoneID, goodStart, goodEnd},

		{"with valid zone ID", false, authKeyEmail, goodKey, goodEmail, "", "", goodZoneID, goodStart, goodEnd},
		{"with nonexistent zone ID", true, authKeyEmail, goodKey, goodEmail, "", "", nonexistentZoneID, goodStart, goodEnd},
		{"with unauthorized zone ID", true, authKeyEmail, goodKey, goodEmail, "", "", unauthorizedZoneID, goodStart, goodEnd},
		{"with log retention disabled for zone ID", true, authKeyEmail, goodKey, goodEmail, "", "", logRetentionDisabledZoneID, goodStart, goodEnd},

		{"with valid time parameters", false, authKeyEmail, goodKey, goodEmail, "", "", goodZoneID, goodStart, goodEnd},
		{"with too early time parameters", true, authKeyEmail, goodKey, goodEmail, "", "", goodZoneID, tooEarlyStart, tooEarlyEnd},
		{"with too recent time parameters", true, authKeyEmail, goodKey, goodEmail, "", "", goodZoneID, tooRecentStart, tooRecentEnd},
	}

	for _, c := range testCases {
		t.Run(c.condition, func(t *testing.T) {
			ts := httptest.NewServer(mockHandlerFunc(t, mockLogpullHandler))
			defer ts.Close()

			var api *logpullAPI
			switch c.authType {
			case authKeyEmail:
				api = newLogpullAPI(c.apiKey, c.apiEmail)
			case authUserService:
				api = newLogpullAPIWithUserServiceKey(c.apiUserServiceKey)
			case authToken:
				api = newLogpullAPIWithToken(c.apiToken)
			}
			api.setAPIProperties(ts.URL, ts.Client())

			err := api.pullLogEntries(c.zoneID, c.start, c.end, nopLogHandler)
			if err == nil && c.isErrorExpected {
				t.Errorf("expected error when called %s", c.condition)
			} else if err != nil && !c.isErrorExpected {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}

// TestPullLogEntriesAPIErrorContext attempts to pull logs from a mock
// Cloudflare API which will intentionally return non-successful responses. The
// expectation is that the response body will be returned in an error message
// to the caller.
func TestPullLogEntriesAPIErrorContext(t *testing.T) {
	msg := "the server's on fire"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(msg)); err != nil {
			t.Fatal(err)
		}
	}))
	defer ts.Close()

	api := newLogpullAPI(goodKey, goodEmail)
	api.setAPIProperties(ts.URL, ts.Client())

	err := api.pullLogEntries(goodZoneID, goodStart, goodEnd, nopLogHandler)
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Error("expected an error containing the response body from the server")
	}
}

package main

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/prometheus/client_golang/prometheus"
)

// LogPeriod is the window of time for which we want to receive logs, relative
// to one minute ago. At one minute, this expresses that we want the logs
// beginning two minutes ago and ending one minute ago.
const LogPeriod = 1 * time.Minute

var (
	httpResponseDesc = prometheus.NewDesc(
		"cloudflare_logs_http_responses",
		"Cloudflare HTTP responses, obtained via Logpull API",
		[]string{
			"client_request_host",
			"edge_response_status",
			"origin_response_status",
		},
		prometheus.Labels{
			"period": promDurationString(LogPeriod),
		},
	)

	retryableAPIErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudflare_logs_api_errors_total",
			Help: "The total number of retryable Cloudflare API errors",
		},
		[]string{"operation", "kind"},
	)
)

func init() {
	prometheus.MustRegister(retryableAPIErrors)
}

// LogpullCollector is an implementation of prometheus.Collector which reads
// from Cloudflare's Logpull API and produces aggregated metrics.
type LogpullCollector struct {
	api     *cloudflare.API
	zoneIDs []string
}

// NewLogpullCollector creates a new LogpullCollector based on the provided
// *cloudflare.API and zoneIDs.
func NewLogpullCollector(api *cloudflare.API, zoneIDs []string) *LogpullCollector {
	return &LogpullCollector{api, zoneIDs}
}

// Describe is a required method of the prometheus.Collector interface. It is
// used to validate that there are no metric collisions when the collector is
// registered.
func (c *LogpullCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- httpResponseDesc
}

// Collect is a required method of the prometheus.Collector interface. It is
// called by the Prometheus registry whenever a new set of metrics are to be
// collected. If any retryable errors are encountered during this process, they
// are logged and counted in the 'cloudflare_logs_api_errors_total' metric. If
// a non-retryable error occurs here, we log it and exit non-zero.
func (c *LogpullCollector) Collect(ch chan<- prometheus.Metric) {
	// The Logpull API docs say that we must go back at least one full minute.
	end := time.Now().Add(-1 * time.Minute)
	start := end.Add(-1 * LogPeriod)

	var wg sync.WaitGroup

	for _, zoneID := range c.zoneIDs {
		wg.Add(1)

		go func(zoneID string) {
			httpResponses := make(HTTPResponseAggregator)
			err := GetLogEntries(c.api, zoneID, start, end, httpResponses.Inc)
			httpResponses.Collect(ch)

			if err != nil {
				if rerr, ok := err.(RetryableAPIError); ok {
					labels := prometheus.Labels{
						"operation": rerr.Operation,
						"kind":      rerr.Kind,
					}
					retryableAPIErrors.With(labels).Inc()
					log.Println(err)
				} else {
					log.Fatal(err)
				}
			}

			wg.Done()

		}(zoneID)
	}

	wg.Wait()
}

// HTTPResponseAggregator is used to count the number of times a given LogEntry
// has been seen. It implements the prometheus.Collector interface, but in
// order to be used as such it must be 'driven' externally by calling 'Inc'.
type HTTPResponseAggregator map[LogEntry]float64

// Inc increments the number of times that a given LogEntry has been observed.
func (m HTTPResponseAggregator) Inc(entry LogEntry) {
	prev, _ := m[entry]
	m[entry] = prev + 1
}

// Describe is a required method of the prometheus.Collector interface. It is
// used to validate that there are no metric collisions when the collector is
// registered. Although HTTPResponseAggregator can be registered as a
// collector, it may not be registered at the same time as a LogpullCollector
// since both ship the same metrics.
func (m HTTPResponseAggregator) Describe(ch chan<- *prometheus.Desc) {
	ch <- httpResponseDesc
}

// Collect is a required method of the prometheus.Collector interface. When
// registered, it is called by the Prometheus registry whenever a new set of
// metrics are to be collected.
func (m HTTPResponseAggregator) Collect(ch chan<- prometheus.Metric) {
	for entry, value := range m {
		ch <- prometheus.MustNewConstMetric(
			httpResponseDesc,
			prometheus.GaugeValue,
			value,
			entry.ClientRequestHost,
			strconv.Itoa(entry.EdgeResponseStatus),
			strconv.Itoa(entry.OriginResponseStatus),
		)
	}
}

// promDurationString turns a `time.Duration` into a string in Prometheus'
// standard format.
func promDurationString(d time.Duration) string {
	s := ""
	if int(d.Hours()) > 0 {
		s += strconv.Itoa(int(d.Hours())) + "h"
		d -= time.Duration(d.Hours()) * time.Hour
	}
	if int(d.Minutes()) > 0 {
		s += strconv.Itoa(int(d.Minutes())) + "m"
		d -= time.Duration(d.Minutes()) * time.Minute
	}
	if int(d.Seconds()) > 0 {
		s += strconv.Itoa(int(d.Seconds())) + "s"
		d -= time.Duration(d.Seconds()) * time.Second
	}
	if d.Milliseconds() > 0 {
		s += strconv.Itoa(int(d.Milliseconds())) + "ms"
	}
	return s
}

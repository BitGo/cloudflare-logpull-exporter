package main

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/prometheus/client_golang/prometheus"
)

// logPeriod is the window of time for which we want to receive logs, relative
// to one minute ago. At one minute, this expresses that we want the logs
// beginning two minutes ago and ending one minute ago.
const logPeriod = 1 * time.Minute

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
			"period": promDurationString(logPeriod),
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

// logpullCollector is an implementation of prometheus.Collector which reads
// from Cloudflare's Logpull API and produces aggregated metrics.
type logpullCollector struct {
	api     *cloudflare.API
	zoneIDs []string
}

// newLogpullCollector creates a new logpullCollector based on the provided
// *cloudflare.API and zoneIDs.
func newLogpullCollector(api *cloudflare.API, zoneIDs []string) *logpullCollector {
	return &logpullCollector{api, zoneIDs}
}

// Describe is a required method of the prometheus.Collector interface. It is
// used to validate that there are no metric collisions when the collector is
// registered.
func (c *logpullCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- httpResponseDesc
}

// Collect is a required method of the prometheus.Collector interface. It is
// called by the Prometheus registry whenever a new set of metrics are to be
// collected. If any retryable errors are encountered during this process, they
// are logged and counted in the 'cloudflare_logs_api_errors_total' metric. If
// a non-retryable error occurs here, we log it and exit non-zero.
func (c *logpullCollector) Collect(ch chan<- prometheus.Metric) {
	// The Logpull API docs say that we must go back at least one full minute.
	end := time.Now().Add(-1 * time.Minute)
	start := end.Add(-1 * logPeriod)

	var wg sync.WaitGroup

	for _, zoneID := range c.zoneIDs {
		wg.Add(1)

		go func(zoneID string) {
			logEntries := make(logEntryAggregator)
			err := getLogEntries(c.api, zoneID, start, end, logEntries.Inc)
			logEntries.Collect(ch)

			if err != nil {
				if rerr, ok := err.(retryableAPIError); ok {
					labels := prometheus.Labels{
						"operation": rerr.operation,
						"kind":      rerr.kind,
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

// logEntryAggregator is used to count the number of times a given LogEntry has
// been seen. It implements the prometheus.Collector interface, but in order to
// be used as such it must be 'driven' externally by calling 'Inc'.
type logEntryAggregator map[logEntry]float64

// Inc increments the number of times that a given LogEntry has been observed.
func (m logEntryAggregator) Inc(entry logEntry) {
	prev, _ := m[entry]
	m[entry] = prev + 1
}

// Describe is a required method of the prometheus.Collector interface. It is
// used to validate that there are no metric collisions when the collector is
// registered. Although logEntryAggregator can be registered as a collector, it
// may not be registered at the same time as a logpullCollector since both ship
// the same metrics.
func (m logEntryAggregator) Describe(ch chan<- *prometheus.Desc) {
	ch <- httpResponseDesc
}

// Collect is a required method of the prometheus.Collector interface. When
// registered, it is called by the Prometheus registry whenever a new set of
// metrics are to be collected.
func (m logEntryAggregator) Collect(ch chan<- prometheus.Metric) {
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

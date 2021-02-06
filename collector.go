package main

import (
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

	errorCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cloudflare_logs_errors_total",
		Help: "The number of errors that have occurred while collecting metrics",
	})
)

func init() {
	prometheus.MustRegister(errorCounter)
}

type collector struct {
	api          *cloudflare.API
	zoneIDs      []string
	errorHandler func(error)
}

func newCollector(api *cloudflare.API, zoneIDs []string, errorHandler func(error)) *collector {
	return &collector{api, zoneIDs, errorHandler}
}

// Describe is a required method of the prometheus.Collector interface. It is
// used to validate that there are no metric collisions when the collector is
// registered.
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- httpResponseDesc
}

// Collect is a required method of the prometheus.Collector interface. It is
// called by the Prometheus registry whenever a new set of metrics are to be
// collected.
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	// The Logpull API docs say that we must go back at least one full minute.
	end := time.Now().Add(-1 * time.Minute)
	start := end.Add(-1 * logPeriod)

	var wg sync.WaitGroup

	for _, zoneID := range c.zoneIDs {
		wg.Add(1)

		go func(zoneID string) {
			responses := make(map[logEntry]float64)

			if err := getLogEntries(c.api, zoneID, start, end, func(entry logEntry) error {
				responses[entry]++
				return nil
			}); err != nil {
				errorCounter.Inc()
				c.errorHandler(err)
			}

			for entry, count := range responses {
				ch <- prometheus.MustNewConstMetric(
					httpResponseDesc,
					prometheus.GaugeValue,
					count,
					entry.ClientRequestHost,
					strconv.Itoa(entry.EdgeResponseStatus),
					strconv.Itoa(entry.OriginResponseStatus),
				)
			}

			wg.Done()

		}(zoneID)
	}

	wg.Wait()
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

package main

import (
	"log"
	"strconv"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/prometheus/client_golang/prometheus"
)

const LOG_PERIOD = 1 * time.Minute

var (
	HTTPResponseDesc = prometheus.NewDesc(
		"cloudflare_logs_http_responses",
		"Cloudflare HTTP responses, obtained via Logpull API",
		[]string{
			"client_request_host",
			"edge_response_status",
			"origin_response_status",
		},
		prometheus.Labels{
			"period": promDurationString(LOG_PERIOD),
		},
	)

	RetryableAPIErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudflare_logs_api_errors_total",
			Help: "The total number of retryable Cloudflare API errors",
		},
		[]string{"operation", "kind"},
	)
)

func init() {
	prometheus.MustRegister(RetryableAPIErrors)
}

type LogpullCollector struct {
	api    *cloudflare.API
	zoneID string
}

func NewLogpullCollector(api *cloudflare.API, zoneID string) *LogpullCollector {
	return &LogpullCollector{
		api:    api,
		zoneID: zoneID,
	}
}

func (c *LogpullCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- HTTPResponseDesc
}

func (c *LogpullCollector) Collect(ch chan<- prometheus.Metric) {
	httpResponses := make(HTTPResponseAggregator)

	// The Logpull API docs say that we must go back at least one full minute.
	end := time.Now().Add(-1 * time.Minute)
	start := end.Add(-1 * LOG_PERIOD)

	err := getLogEntries(c.api, c.zoneID, start, end, func(entry LogEntry) {
		httpResponses.Inc(entry)
	})

	if err != nil {
		if rerr, ok := err.(RetryableAPIError); ok {
			labels := prometheus.Labels{
				"operation": rerr.Operation,
				"kind":      rerr.Kind,
			}

			RetryableAPIErrors.With(labels).Inc()
			log.Println(err)
		} else {
			log.Fatal(err)
		}
	}

	httpResponses.Collect(ch)
}

type HTTPResponseAggregator map[LogEntry]float64

func (m HTTPResponseAggregator) Inc(entry LogEntry) {
	prev, _ := m[entry]
	m[entry] = prev + 1
}

func (m HTTPResponseAggregator) Describe(ch chan<- *prometheus.Desc) {
	ch <- HTTPResponseDesc
}

func (m HTTPResponseAggregator) Collect(ch chan<- prometheus.Metric) {
	for entry, value := range m {
		ch <- prometheus.MustNewConstMetric(
			HTTPResponseDesc,
			prometheus.GaugeValue,
			value,
			entry.ClientRequestHost,
			strconv.Itoa(entry.EdgeResponseStatus),
			strconv.Itoa(entry.OriginResponseStatus),
		)
	}
}

// Turn a `time.Duration` into a Prometheus' format duration string
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

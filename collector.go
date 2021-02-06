package main

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/prometheus/client_golang/prometheus"
	prommodel "github.com/prometheus/common/model"
)

// The Cloudflare API docs specify that 'start' must be no more than seven days
// earlier from now, and that 'end' must be at least one minute earlier than
// now. Thus, logPeriod must be smaller than seven days, less one minute to
// account for the one minute offset.
// https://developers.cloudflare.com/logs/logpull-api/requesting-logs#parameters
const logPeriodRange = 7*24*time.Hour - time.Minute

type collector struct {
	api          *cloudflare.API
	zoneIDs      []string
	logPeriod    time.Duration
	responseDesc *prometheus.Desc
	errorCounter prometheus.Counter
	errorHandler func(error)
}

// newCollector creates a new Logpull collector. Returns an error if any
// parameters are invalid.
func newCollector(api *cloudflare.API, zoneIDs []string, logPeriod time.Duration, errorHandler func(error)) (*collector, error) {
	if api == nil {
		return nil, errors.New("invalid parameter: api must not be nil")
	}

	if len(zoneIDs) == 0 {
		return nil, errors.New("invalid parameter: zoneIDs must not be empty")
	}

	if logPeriod >= logPeriodRange {
		return nil, errors.New("invalid parameter: logPeriod out of acceptable range")
	}

	responseDesc := prometheus.NewDesc(
		"cloudflare_logs_http_responses",
		"Cloudflare HTTP responses, obtained via Logpull API",
		[]string{
			"client_request_host",
			"edge_response_status",
			"origin_response_status",
		},
		prometheus.Labels{
			"period": prommodel.Duration(logPeriod).String(),
		},
	)

	errorCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cloudflare_logs_errors_total",
		Help: "The number of errors that have occurred while collecting metrics",
	})

	return &collector{
		api,
		zoneIDs,
		logPeriod,
		responseDesc,
		errorCounter,
		errorHandler,
	}, nil
}

// Describe is a required method of the prometheus.Collector interface. It is
// used to validate that there are no metric collisions when the collector is
// registered.
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.responseDesc
	c.errorCounter.Describe(ch)
}

// Collect is a required method of the prometheus.Collector interface. It is
// called by the Prometheus registry whenever a new set of metrics are to be
// collected.
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	// The Cloudflare API docs specify that 'end' must be at least one
	// minute earlier than now.
	// https://developers.cloudflare.com/logs/logpull-api/requesting-logs#parameters,
	end := time.Now().Add(-1 * time.Minute)
	start := end.Add(-1 * c.logPeriod)

	var wg sync.WaitGroup
	defer wg.Wait()

	for _, zoneID := range c.zoneIDs {
		wg.Add(1)
		go func(zoneID string) {
			defer wg.Done()

			responses := make(map[logEntry]float64)

			if err := pullLogEntries(c.api, zoneID, start, end, func(entry logEntry) error {
				responses[entry]++
				return nil
			}); err != nil {
				c.errorCounter.Inc()
				c.errorHandler(err)
			}

			for entry, count := range responses {
				ch <- prometheus.MustNewConstMetric(
					c.responseDesc,
					prometheus.GaugeValue,
					count,
					entry.ClientRequestHost,
					strconv.Itoa(entry.EdgeResponseStatus),
					strconv.Itoa(entry.OriginResponseStatus),
				)
			}

			c.errorCounter.Collect(ch)
		}(zoneID)
	}
}

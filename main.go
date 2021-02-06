package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	addr := os.Getenv("EXPORTER_LISTEN_ADDR")
	if addr == "" {
		addr = ":9299"
	}

	apiEmail := os.Getenv("CLOUDFLARE_API_EMAIL")
	apiKey := os.Getenv("CLOUDFLARE_API_KEY")
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	zoneNames := os.Getenv("CLOUDFLARE_ZONE_NAMES")

	if apiToken == "" && apiKey == "" {
		log.Fatal("Neither CLOUDFLARE_API_TOKEN nor CLOUDFLARE_API_KEY were specified. Use one or the other.")
	}

	if apiToken != "" && apiKey != "" {
		log.Fatal("Both CLOUDFLARE_API_TOKEN and CLOUDFLARE_API_KEY specified. Use one or the other.")
	}

	if apiKey != "" && apiEmail == "" {
		log.Fatal("CLOUDFLARE_API_KEY specified without CLOUDFLARE_API_EMAIL. Both must be provided.")
	}

	if zoneNames == "" {
		log.Fatal("A comma-separated list of zone names must be specified in CLOUDFLARE_ZONE_NAMES")
	}

	var api *cloudflare.API
	var err error

	if apiToken != "" {
		api, err = cloudflare.NewWithAPIToken(apiToken)
	} else {
		api, err = cloudflare.New(apiKey, apiEmail)
	}

	if err != nil {
		log.Fatalf("creating api client: %s", err)
	}

	zoneIDs := make([]string, 0)
	for _, zoneName := range strings.Split(zoneNames, ",") {
		id, err := api.ZoneIDByName(strings.TrimSpace(zoneName))
		if err != nil {
			log.Fatalf("zone id lookup: %s", err)
		}
		zoneIDs = append(zoneIDs, id)
	}

	collectorErrorHandler := func(err error) {
		log.Printf("collector: %s", err)
	}

	collector, err := newCollector(api, zoneIDs, time.Minute, collectorErrorHandler)
	if err != nil {
		log.Fatalf("creating collector: %w", err)
	}

	prometheus.MustRegister(collector)
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

package main

import (
	"log"
	"net/http"
	"os"
	"strings"

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
		log.Fatalf("Error creating API client: %s", err.Error())
	}

	zones := make(map[string]string)
	zoneIDs := make([]string, 0)

	for _, name := range strings.Split(zoneNames, ",") {
		name = strings.TrimSpace(name)
		id, err := api.ZoneIDByName(name)
		if err != nil {
			log.Fatalf("Error looking up zone ID for zone %s: %s", name, err.Error())
		}
		zones[name] = id
		zoneIDs = append(zoneIDs, id)
	}

	prometheus.MustRegister(newLogpullCollector(api, zoneIDs))

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

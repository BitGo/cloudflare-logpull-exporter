# cloudflare-logpull-exporter

A Prometheus exporter to aggregate logs from Cloudflare's Logpull* API.

*NOTE: This API is only available to Enterprise customers*

## Building

```console
$ docker build -t cloudflare-logpull-exporter .
```

## Running

All configuration is done through the following environment variables:

| Name                    | Required                                            |
|-------------------------|-----------------------------------------------------|
| `CLOUDFLARE_API_TOKEN`  | **Yes**, if `CLOUDFLARE_API_KEY` **is not** given   |
| `CLOUDFLARE_API_EMAIL`  | **Yes**, if `CLOUDFLARE_API_KEY` **is** given       |
| `CLOUDFLARE_API_KEY`    | **Yes**, if `CLOUDFLAER_API_TOKEN` **is not** given |
| `CLOUDFLARE_ZONE_NAMES` | **Always**                                          |
| `EXPORTER_LISTEN_ADDR`  | **No**, defaults to `:9299`                         |

For example, assuming `$CLOUDFLARE_API_TOKEN` is set in your shell:

```console
$ docker run -d -p 9299:9299 \
    -e CLOUDFLARE_API_TOKEN="$CLOUDFLARE_API_TOKEN" \
    -e CLOUDFLARE_ZONE_NAMES=example.org \
    cloudflare-logpull-exporter
```

# cloudflare-logpull-exporter

A Prometheus exporter to aggregate logs from Cloudflare's Logpull* API.

*NOTE: This API is only available to Enterprise customers*

## Building

```console
$ docker build -t cloudflare-logpull-exporter .
```

## Running

All configuration is done through the following environment variables:

| Name                            | Required                                            |
|---------------------------------|-----------------------------------------------------|
| `CLOUDFLARE_API_TOKEN`          | **Yes**, if `CLOUDFLARE_API_KEY` **is not** given   |
| `CLOUDFLARE_API_EMAIL`          | **Yes**, if `CLOUDFLARE_API_KEY` **is** given       |
| `CLOUDFLARE_API_KEY`            | **Yes**, if `CLOUDFLAER_API_TOKEN` **is not** given |
| `CLOUDFLARE_ZONE_NAMES`         | **Always**                                          |
| `EXPORTER_LISTEN_ADDR`          | **No**, defaults to `:9299`                         |
| `EXPORTER_MINIMAL_PERMISSIONS`  | **No**, default to `false`                          |

The exporter must have `Zone -> Logs -> Edit` permissions, unless `EXPORTER_MINIMAL_PERMISSIONS` is set to a non-empty value. If so, `Zone -> Logs -> Read` is sufficient. In _minimal permissions_ mode, the exporter is unable to verify and enable log retention for the given zones. This is useful if you wish to manage this configuration elsewhere and run more securely.

For example, assuming `$CLOUDFLARE_API_TOKEN` is set in your shell:

```console
$ docker run -d -p 9299:9299 \
    -e CLOUDFLARE_API_TOKEN="$CLOUDFLARE_API_TOKEN" \
    -e CLOUDFLARE_ZONE_NAMES=example.org \
    cloudflare-logpull-exporter
```

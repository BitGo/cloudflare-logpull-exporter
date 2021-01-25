# cloudflare-logpull-exporter

A Prometheus exporter to aggregate logs from Cloudflare's [Logpull* API][logpull-api].

*NOTE: This API is only available to Enterprise customers*

## Building

```console
$ docker build -t cloudflare-logpull-exporter .
```

## Running

In order for the exporter to work, [log retention][docs-enabling-log-retention] must be enabled for all of the zones to be targetted. One way to do this, if using Terraform, would be to define a [`cloudflare_logpull_retention`][terraform-cloudflare-logpull-retention] resource.

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

[logpull-api]: https://developers.cloudflare.com/logs/logpull-api
[docs-enabling-log-retention]: https://developers.cloudflare.com/logs/logpull-api/enabling-log-retention
[terraform-cloudflare-logpull-retention]: https://registry.terraform.io/providers/cloudflare/cloudflare/latest/docs/resources/logpull_retention

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

* `CLOUDFLARE_API_EMAIL`
* `CLOUDFLARE_API_KEY`
* `CLOUDFLARE_API_TOKEN`
* `CLOUDFLARE_API_USER_SERVICE_KEY`
* `CLOUDFLARE_ZONE_NAMES`
* `EXPORTER_LISTEN_ADDR`

There are three different ways to authenticate with Cloudflare's API. Exactly one of the following must be provided:

* API key and email via `CLOUDFLARE_API_KEY` and `CLOUDFLARE_API_EMAIL`
* API tokens via `CLOUDFLARE_API_TOKEN`
* User service keys via `CLOUDFLARE_API_USER_SERVICE_KEY`

`CLOUDFLARE_ZONE_NAMES` is a required parameter and should be a comma-separated list of zones from which to gather metrics.

`EXPORTER_LISTEN_ADDR` is optional and allows binding the exporter to a different IP/port. The default value is `:9299`.

### Example

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

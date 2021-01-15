FROM golang:1.15.6
WORKDIR /go/src/cloudflare-logpull-exporter
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /cloudflare-logpull-exporter

FROM scratch
COPY --from=0 /cloudflare-logpull-exporter .
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
CMD ["/cloudflare-logpull-exporter"]
EXPOSE 9299

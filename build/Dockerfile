FROM golang:1.24.13-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /access-monitor ./cmd/access-monitor

FROM alpine:3.22
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN addgroup -g 10001 monitor && adduser -D -H -u 10001 -G monitor monitor && mkdir -p /var/log/access-monitor && chown monitor:monitor /var/log/access-monitor
COPY --from=build /access-monitor /usr/local/bin/access-monitor
USER 10001:10001
WORKDIR /app
ENTRYPOINT ["/usr/local/bin/access-monitor"]

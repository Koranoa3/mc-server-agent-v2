FROM golang:1.24-alpine AS build
WORKDIR /src

# Download dependencies
COPY go.mod .
RUN apk add --no-cache git && \
    go env -w GOPROXY=https://proxy.golang.org,direct && \
    go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /mc-agent ./app

FROM alpine:3.18
RUN addgroup -S app && adduser -S -G app app
COPY --from=build /mc-agent /usr/local/bin/mc-agent
WORKDIR /data
RUN chown app:app /data || true
USER app
ENV SETTINGS_PATH=/data/settings.json
ENTRYPOINT ["/usr/local/bin/mc-agent"]

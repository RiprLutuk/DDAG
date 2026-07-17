# Multi-stage build for any DDAG Go service.
# Build a specific service:  docker build -f deploy/docker/Dockerfile --build-arg SERVICE=api-gateway -t ddag-api-gateway .
# All connectors/services share this file; drivers are pure-Go so CGO is off.
ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-alpine AS build
ARG SERVICE
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN test -n "$SERVICE" || (echo "SERVICE build-arg is required" && false)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/service ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
ARG SERVICE
COPY --from=build /out/service /usr/local/bin/ddag-service
# Document the conventional port (overridable via DDAG_HTTP_ADDR).
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/ddag-service"]

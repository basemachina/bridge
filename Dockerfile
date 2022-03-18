FROM golang:1.18-alpine3.15 AS builder
ARG VERSION
ENV CGO_ENABLED=0
WORKDIR /go/src/github.com/basemachina/bridge
COPY . .
RUN go build -mod=readonly -o bridge -buildvcs=false -trimpath -ldflags "-w -s -X main.version=$VERSION -X main.serviceName=bridge" ./cmd/bridge

# runtime image
FROM alpine:3.15
RUN addgroup -S nonroot && adduser -S nonroot -G nonroot
COPY --from=builder --chown=nonroot:nonroot /go/src/github.com/basemachina/bridge /bridge

# hadolint ignore=DL3018
RUN apk update && apk add --no-cache ca-certificates \
    'libretls>3.3.4-r2' # CVE-2022-0778
USER nonroot
EXPOSE 8080
CMD ["/bridge"]
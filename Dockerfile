# syntax=docker/dockerfile:1
# Multi-stage build → a ~10MB distroless, non-root image. The web dashboard is embedded
# in the binary (go:embed), so this one image is the whole control plane.

FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/dibakshya01/chaotic-shoebill-stork/internal/buildinfo.Version=${VERSION}" \
    -o /out/shoebill ./cmd/shoebill

# Distroless static + nonroot: no shell, no package manager, minimal attack surface.
FROM gcr.io/distroless/static:nonroot
WORKDIR /home/nonroot
COPY --from=build /out/shoebill /usr/local/bin/shoebill
COPY --from=build /src/catalog ./catalog
EXPOSE 8099
USER nonroot:nonroot

ENTRYPOINT ["shoebill"]
# Default: serve the dashboard. Binding 0.0.0.0 inside a container is expected, but the
# API is UNAUTHENTICATED — run this behind an authenticating proxy / ingress, and scope
# the network. Override the command for `demo`, `catalog validate`, etc.
CMD ["serve", "--demo", "--addr", "0.0.0.0:8099", "--insecure-remote", "--dir", "/home/nonroot/.shoebill"]

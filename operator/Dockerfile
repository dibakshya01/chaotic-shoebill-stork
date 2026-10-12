# syntax=docker/dockerfile:1
# The operator manager. Build from the REPO ROOT so the `replace => ../` to the core
# module resolves:
#     docker build -f operator/Dockerfile -t shoebill-operator .
FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY . .
ARG VERSION=docker
RUN cd operator \
    && CGO_ENABLED=0 go build -trimpath \
       -ldflags "-s -w -X github.com/dibakshya01/chaotic-shoebill-stork/internal/buildinfo.Version=${VERSION}" \
       -o /out/manager ./cmd/manager

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/manager /usr/local/bin/manager
USER nonroot:nonroot
ENTRYPOINT ["manager"]

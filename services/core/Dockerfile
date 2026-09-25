# syntax=docker/dockerfile:1.7
FROM golang:1.27-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY gen ./gen
COPY internal ./internal
COPY demo ./demo
COPY services/core ./services/core
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/core ./services/core/cmd/core

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /out/core /app/core
USER 10001
EXPOSE 8080 8081
ENTRYPOINT ["/app/core"]
CMD ["serve"]

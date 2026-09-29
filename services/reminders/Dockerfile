# syntax=docker/dockerfile:1
# Образ reminders-service. Сборка из корня репозитория:
#   docker build -f services/reminders/Dockerfile .
ARG GO_VERSION=1.27
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY gen ./gen
COPY internal ./internal
COPY services/reminders ./services/reminders
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/reminders ./services/reminders/cmd/reminders

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/reminders /app/reminders
USER app
EXPOSE 9091 8081
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=5 \
    CMD ["/app/reminders", "healthcheck"]
ENTRYPOINT ["/app/reminders"]
CMD ["serve"]

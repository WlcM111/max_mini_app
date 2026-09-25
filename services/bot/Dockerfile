# syntax=docker/dockerfile:1.7
FROM golang:1.27-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY gen ./gen
COPY internal ./internal
COPY services/bot ./services/bot
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/bot ./services/bot/cmd/bot
COPY deploy/ca /ca
# awk 1 завершает каждый файл переводом строки: при склейке через cat строки
# -----END CERTIFICATE----- и -----BEGIN CERTIFICATE----- слипаются, и Go
# не разбирает ни один сертификат. Проверка ниже обрывает сборку, если в бандле
# меньше двух корректных заголовков.
RUN awk '{sub(/\r$/,""); print}' /ca/russian_trusted_root_ca.pem /ca/russian_trusted_sub_ca.pem \
        > /out/russian_trusted_ca_bundle.pem \
    && [ "$(grep -c '^-----BEGIN CERTIFICATE-----$' /out/russian_trusted_ca_bundle.pem)" -ge 1 ]

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /out/bot /app/bot
COPY --from=build /out/russian_trusted_ca_bundle.pem /etc/vovremya/ca/russian_trusted_ca_bundle.pem
USER 10001
EXPOSE 8080 8081 9090
ENTRYPOINT ["/app/bot"]
CMD ["serve"]

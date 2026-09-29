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
COPY deploy/ca /ca
# Бандл НУЦ Минцифры нужен для TLS с доменами GigaChat API (ADR-032).
# awk снимает CR и завершает каждый файл переводом строки: при простой склейке
# строки -----END CERTIFICATE----- и -----BEGIN CERTIFICATE----- слипаются,
# и Go не разбирает ни один сертификат. Проверка ниже обрывает сборку, если в бандле
# меньше двух корректных заголовков (корневой и выпускающий сертификаты).
RUN awk '{sub(/\r$/,""); print}' /ca/russian_trusted_root_ca.pem /ca/russian_trusted_sub_ca.pem \
        > /out/russian_trusted_ca_bundle.pem \
    && [ "$(grep -c '^-----BEGIN CERTIFICATE-----$' /out/russian_trusted_ca_bundle.pem)" -ge 2 ]

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
COPY --from=build /out/core /app/core
COPY --from=build /out/russian_trusted_ca_bundle.pem /etc/vovremya/ca/russian_trusted_ca_bundle.pem
USER 10001
EXPOSE 8080 8081
ENTRYPOINT ["/app/core"]
CMD ["serve"]

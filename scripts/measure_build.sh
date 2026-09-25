#!/bin/sh
# Замер сборки образов без кэша слоёв. По условию кейса время загрузки базовых образов не учитывается,
# поэтому они загружаются заранее.
set -eu
for img in golang:1.27-alpine node:22-alpine caddy:2.10-alpine alpine:3.22 postgres:18-alpine; do docker pull -q "$img"; done
start=$(date +%s)
docker compose build --no-cache
elapsed=$(( $(date +%s) - start ))
echo "build_seconds=$elapsed"
[ "$elapsed" -le 300 ] || { echo "FAIL: сборка дольше 300 секунд"; exit 1; }
echo "OK"

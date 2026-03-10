#!/usr/bin/env bash
# Запуск полного стенда Algorhythm
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OPS_DIR="$(cd "$SCRIPT_DIR/../ops/full-stack" && pwd)"

cd "$OPS_DIR"
[ -f .env ] || cp .env.example .env
docker compose up -d
echo "Infrastructure started. Run 'make logs' in ops/full-stack for logs."

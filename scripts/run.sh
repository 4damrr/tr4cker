#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

mkdir -p bin data
go build -o bin/tr4cker ./cmd/server

export ADDR="${ADDR:-:8080}"
export DB_PATH="${DB_PATH:-data/tr4cker.db}"

exec ./bin/tr4cker

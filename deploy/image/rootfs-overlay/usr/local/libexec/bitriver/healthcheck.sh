#!/usr/bin/env bash
set -euo pipefail

curl --fail --silent http://127.0.0.1:8080/healthz >/dev/null
curl --fail --silent http://127.0.0.1:8080/api/v1/system >/dev/null

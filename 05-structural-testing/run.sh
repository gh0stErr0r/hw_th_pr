#!/bin/sh
set -eu

profile=$(mktemp)
trap 'rm -f "$profile"' EXIT

go test -overlay=05-structural-testing/overlay.json -count=1 \
    -covermode=atomic -coverprofile="$profile" ./04-coding/internal/...
go tool cover -func="$profile"

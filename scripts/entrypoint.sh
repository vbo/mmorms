#!/bin/sh
set -e
# Embedded mode: start overlord in background when OVERLORD_URL is not set
if [ -z "$OVERLORD_URL" ]; then
    PORT=7070 ./overlord &
    OVERLORD_PID=$!
    trap "kill $OVERLORD_PID 2>/dev/null || true" EXIT
fi
exec ./mmorms

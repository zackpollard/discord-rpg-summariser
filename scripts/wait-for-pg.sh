#!/usr/bin/env bash
# Wait for PostgreSQL to become ready. Used by Makefile targets.
set -e

DATABASE_URL="${1:-postgres://rpg:rpg@localhost:5432/rpg_summariser?sslmode=disable}"
MAX_ATTEMPTS=30
ATTEMPT=0

until pg_isready -h localhost -p 5432 -U rpg -q 2>/dev/null; do
    ATTEMPT=$((ATTEMPT + 1))
    if [ "$ATTEMPT" -ge "$MAX_ATTEMPTS" ]; then
        echo "ERROR: PostgreSQL not ready after ${MAX_ATTEMPTS} attempts" >&2
        exit 1
    fi
    sleep 1
done

echo "PostgreSQL is ready"

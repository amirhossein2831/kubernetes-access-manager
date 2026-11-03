#!/bin/sh
set -eu
export GRAFANA_READER_PASSWORD="$(cat /run/secrets/grafana_reader_password)"
exec /run.sh

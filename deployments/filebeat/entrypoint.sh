#!/bin/sh
set -eu
export FILEBEAT_PASSWORD="$(cat /run/secrets/filebeat_password)"
exec "$@"

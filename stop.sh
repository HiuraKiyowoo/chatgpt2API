#!/bin/bash
# stop chatgpt2API
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$DIR/start.sh" stop

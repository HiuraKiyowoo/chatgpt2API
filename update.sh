#!/bin/bash
# update + rebuild + restart
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$DIR/start.sh" update

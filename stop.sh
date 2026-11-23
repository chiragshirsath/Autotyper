#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

if [ ! -f "./stop" ]; then
    echo "⚠️  Binary 'stop' not found. Building first..."
    ./build.sh
fi

echo "🛑 Stopping Auto Typer..."
./stop

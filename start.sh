#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

if [ ! -f "./server" ]; then
    echo "⚠️  Binary 'server' not found. Building first..."
    ./build.sh
fi

echo "🚀 Starting Auto Typer server..."
./server

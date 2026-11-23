#!/bin/bash
set -e

echo "🔨 Building Auto Typer for macOS..."
go build -ldflags="-s -w" -o server ./cmd/server
go build -ldflags="-s -w" -o stop ./cmd/stop
chmod +x server stop
echo "✅ Build complete! Binaries 'server' and 'stop' are ready."

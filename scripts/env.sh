#!/usr/bin/env bash
# Source this file to load environment variables:
#   source scripts/env.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

export ANTHROPIC_API_KEY=$(cat "$PROJECT_DIR/secrets/ClaudeCodeAPIKey.txt")

echo "Environment loaded."

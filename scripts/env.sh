#!/usr/bin/env bash
# Source this file to load environment variables:
#   source scripts/env.sh

# Support both bash and zsh
SCRIPT_PATH="${BASH_SOURCE[0]:-${(%):-%x}}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

export ANTHROPIC_API_KEY=$(cat "$PROJECT_DIR/secrets/ClaudeCodeAPIKey.txt")

echo "Environment loaded."

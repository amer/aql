#!/usr/bin/env bash
# Test that env.sh correctly loads the API key in both bash and zsh.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

echo "Running env.sh tests..."

# Test 1: bash loads key
KEY=$(bash -c "source '$SCRIPT_DIR/env.sh' >/dev/null 2>&1 && echo \$ANTHROPIC_API_KEY")
if [ -n "$KEY" ]; then
    pass "bash loads ANTHROPIC_API_KEY"
else
    fail "bash did not load ANTHROPIC_API_KEY"
fi

# Test 2: zsh loads key
if command -v zsh &>/dev/null; then
    KEY=$(zsh -c "source '$SCRIPT_DIR/env.sh' >/dev/null 2>&1 && echo \$ANTHROPIC_API_KEY")
    if [ -n "$KEY" ]; then
        pass "zsh loads ANTHROPIC_API_KEY"
    else
        fail "zsh did not load ANTHROPIC_API_KEY"
    fi
else
    echo "  SKIP: zsh not available"
fi

# Test 3: key file exists
if [ -f "$PROJECT_DIR/secrets/ClaudeCodeAPIKey.txt" ]; then
    pass "secrets/ClaudeCodeAPIKey.txt exists"
else
    fail "secrets/ClaudeCodeAPIKey.txt missing"
fi

# Test 4: key starts with expected prefix
KEY=$(bash -c "source '$SCRIPT_DIR/env.sh' >/dev/null 2>&1 && echo \$ANTHROPIC_API_KEY")
if [[ "$KEY" == sk-ant-* ]]; then
    pass "key has valid sk-ant- prefix"
else
    fail "key does not start with sk-ant-"
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1

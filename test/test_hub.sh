#!/usr/bin/env bash
set -euo pipefail

HUB="http://127.0.0.1:9020"
echo "=== mcp-core Integration Test ==="

echo ""
echo "1. Health check"
curl -s "$HUB/health" | python3 -m json.tool

echo ""
echo "2. Opening SSE connection to discover endpoint..."
# Start SSE connection in background, capture output to temp file
TMPFILE=$(mktemp)
curl -s -N "$HUB/sse" > "$TMPFILE" &
SSE_PID=$!
sleep 1

# Extract endpoint from the SSE stream
ENDPOINT=$(grep "data:" "$TMPFILE" | head -1 | sed 's/data: //')
echo "   Discovered endpoint: $ENDPOINT"

echo ""
echo "3. Sending tools/list request..."
REQUEST='{"jsonrpc":"2.0","id":"test-1","method":"tools/list","params":{}}'
curl -s -X POST "$HUB$ENDPOINT" \
  -H "Content-Type: application/json" \
  -d "$REQUEST"
echo ""

sleep 1
echo ""
echo "4. SSE stream response:"
cat "$TMPFILE"
echo ""

echo ""
echo "5. Sending tools/call for echo..."
REQUEST2='{"jsonrpc":"2.0","id":"test-2","method":"tools/call","params":{"name":"echo","arguments":{"text":"Hello mcp-core!"}}}'
curl -s -X POST "$HUB$ENDPOINT" \
  -H "Content-Type: application/json" \
  -d "$REQUEST2"
echo ""

sleep 1
echo ""
echo "6. Full SSE stream output:"
cat "$TMPFILE"
echo ""

# Cleanup
kill $SSE_PID 2>/dev/null || true
rm -f "$TMPFILE"

echo ""
echo "=== Test Complete ==="

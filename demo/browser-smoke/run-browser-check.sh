#!/bin/sh
set -eu

state_file=".ai/browser-smoke-attempt.txt"
mkdir -p .ai frontend/playwright-report frontend/test-results

attempt=1
if [ -f "$state_file" ]; then
    prev=$(cat "$state_file" 2>/dev/null || echo "0")
    case "$prev" in
        ''|*[!0-9]*) prev=0 ;;
    esac
    attempt=$((prev + 1))
fi

printf '%s\n' "$attempt" > "$state_file"

if [ "$attempt" -eq 1 ]; then
    echo "Running 1 test using 1 worker" >&2
    echo "page.goto failed: net::ERR_INTERNET_DISCONNECTED"
    echo "<html>retry-needed</html>" > frontend/playwright-report/index.html
    echo "attempt=1" > frontend/test-results/result.txt
    exit 1
fi

echo "<html>ok</html>" > frontend/playwright-report/index.html
echo "attempt=$attempt" > frontend/test-results/result.txt
echo "browser smoke passed"

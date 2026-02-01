#!/bin/sh
#
# Verify journald-plus output after running the test container.
# Run on the host after:
#   docker run --log-driver journald-plus --log-opt tag=test-jp --rm test-jp-image
#
# Usage: ./verify-journal.sh [tag]
#   tag defaults to "test-jp"

TAG="${1:-test-jp}"

echo "=== All entries (verbose) ==="
journalctl -t "$TAG" --no-pager -o verbose --since "5 min ago"

echo ""
echo "=== Entry count by priority ==="
for p in emerg alert crit err warning notice info debug; do
    count=$(journalctl -t "$TAG" -p "$p..$p" --no-pager --since "5 min ago" -o cat 2>/dev/null | wc -l)
    printf "  %-8s %d\n" "$p" "$count"
done

echo ""
echo "=== Warnings and above ==="
journalctl -t "$TAG" -p warning --no-pager --since "5 min ago" -o cat

echo ""
echo "=== Check multiline (entries containing newlines) ==="
journalctl -t "$TAG" --no-pager --since "5 min ago" -o cat | grep -c $'\n' || echo "(check verbose output for embedded newlines)"

echo ""
echo "=== Spot checks ==="

check() {
    local desc="$1"
    local pattern="$2"
    local pri="$3"
    match=$(journalctl -t "$TAG" -p "$pri..$pri" --no-pager --since "5 min ago" -o cat | grep -c "$pattern" 2>/dev/null || true)
    if [ "$match" -gt 0 ]; then
        printf "  PASS  %s\n" "$desc"
    else
        printf "  FAIL  %s (expected pattern '%s' at priority %s)\n" "$desc" "$pattern" "$pri"
    fi
}

check "sd-daemon <3> -> err"           "Error via sd-daemon"         "err"
check "sd-daemon <4> -> warning"       "Warning via sd-daemon"       "warning"
check "sd-daemon <7> -> debug"         "Debug via sd-daemon"         "debug"
check "ERROR pattern -> err"           "something went wrong"        "err"
check "FATAL pattern -> err"           "process crashed"             "err"
check "WARNING pattern -> warning"     "disk usage high"             "warning"
check "[Note] pattern -> notice"       "Server socket created"       "notice"
check "DEBUG pattern -> debug"         "variable x = 42"             "debug"
check "stderr default -> err"          "Simple stderr message"       "err"
check "stdout default -> info"         "Simple stdout message"       "info"
check "Multiline java stack trace"     "NullPointerException"        "err"
check "Multiline python traceback"     "Traceback"                   "err"

echo ""
echo "Done. Review verbose output above for multiline merge correctness."

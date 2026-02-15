#!/bin/sh
#
# Verify baraverkstad/journald-plus output after running test-logging.sh.
# Run on the host after:
#   docker run --rm -i --log-driver baraverkstad/journald-plus --log-opt tag=test alpine:latest sh < test/test-logging.sh
#
# Usage: ./verify-journal.sh [tag]
#   tag defaults to "test"

TAG="${1:-test}"

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

check "Basic stdout gets info"              "Simple.*stdout"              "info"
check "Basic stderr gets err"               "Simple.*stderr"              "err"
check "sd-daemon <3> stdout -> err"         "Error.*sd-daemon.*stdout"    "err"
check "sd-daemon <3> stderr -> err"         "Error.*sd-daemon.*stderr"    "err"
check "sd-daemon <4> stdout -> warning"     "Warning.*sd-daemon.*stdout"  "warning"
check "sd-daemon <7> stdout -> debug"       "Debug.*sd-daemon.*stdout"    "debug"
check "ERROR pattern stdout -> err"         "ERROR.*wrong.*stdout"        "err"
check "ERROR pattern stderr -> err"         "ERROR.*wrong.*stderr"        "err"
check "FATAL pattern stdout -> err"         "FATAL.*crashed.*stdout"      "err"
check "WARNING pattern stdout -> warning"   "WARNING.*high.*stdout"       "warning"
check "WARNING pattern stderr -> warning"   "WARNING.*high.*stderr"       "warning"
check "[Note] pattern stdout -> notice"     "Note.*socket.*stdout"        "notice"
check "DEBUG pattern stdout -> debug"       "DEBUG.*42.*stdout"           "debug"
check "Multiline java stack trace"          "NullPointerException"        "err"
check "Multiline python traceback"          "Traceback"                   "err"

echo ""
echo "Done. Review verbose output above for multiline merge correctness."

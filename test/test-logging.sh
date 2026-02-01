#!/bin/sh
#
# Test script for journald-plus log driver.
# Run inside a container with:
#   docker run --log-driver journald-plus --log-opt tag=test-jp --rm test-image
#
# Then verify with:
#   journalctl -t test-jp --no-pager -o verbose
#   journalctl -t test-jp -p warning --no-pager    # should show warnings+above only
#
# Each test section sleeps 100ms to ensure multiline buffers flush between tests.

set -e

# --- 1. Basic stdout/stderr (default priorities: info/err) ---
echo "=== Test 1: Basic stdout (should be INFO) ==="
echo "Simple stdout message"
sleep 0.1

echo "=== Test 2: Basic stderr (should be ERR) ==="
echo "Simple stderr message" >&2
sleep 0.1

# --- 2. sd-daemon <N> prefix (should strip prefix, set priority) ---
echo "=== Test 3: sd-daemon prefixes ==="
echo "<7>Debug via sd-daemon prefix"
echo "<6>Info via sd-daemon prefix"
echo "<4>Warning via sd-daemon prefix"
echo "<3>Error via sd-daemon prefix"
echo "<2>Critical via sd-daemon prefix"
echo "<0>Emergency via sd-daemon prefix"
sleep 0.1

# --- 3. Priority regex pattern matching (defaults) ---
echo "=== Test 4: Priority patterns - uppercase ==="
echo "ERROR something went wrong"
echo "FATAL process crashed"
echo "WARNING disk usage high"
echo "WARN connection timeout"
echo "CRITICAL system overload"
echo "DEBUG variable x = 42"
sleep 0.1

echo "=== Test 5: Priority patterns - bracketed (MariaDB style) ==="
echo "[ERROR] InnoDB: unable to lock file"
echo "[Fatal] out of memory"
echo "[Warning] Aborted connection 12345"
echo "[Note] Server socket created on IP: 0.0.0.0"
echo "[Critical] Too many connections"
echo "[Debug] packet dump follows"
sleep 0.1

# --- 4. Multiline merging (continuation lines start with whitespace) ---
echo "=== Test 6: Multiline - Java-style stack trace ==="
echo "ERROR java.lang.NullPointerException: Something is null"
echo "    at com.example.MyClass.myMethod(MyClass.java:42)"
echo "    at com.example.Main.main(Main.java:10)"
echo "    at java.base/java.lang.Thread.run(Thread.java:829)"
sleep 0.1

echo "=== Test 7: Multiline - Python traceback ==="
echo "ERROR Traceback (most recent call last):"
echo "  File \"app.py\", line 10, in main"
echo "    result = process(data)"
echo "  File \"app.py\", line 25, in process"
echo "    return data[key]"
echo "  KeyError: 'missing_key'"
sleep 0.1

echo "=== Test 8: Multiline - indented config/data block ==="
echo "Configuration loaded:"
echo "  database:"
echo "    host: localhost"
echo "    port: 5432"
echo "  cache:"
echo "    ttl: 300"
sleep 0.1

echo "=== Test 9: Multiline with tab indentation ==="
printf "Go panic:\n"
sleep 0.05
printf "goroutine 1 [running]:\n"
printf "\tmain.handler(0xc0000b2000)\n"
printf "\t\t/app/main.go:42 +0x1a4\n"
printf "\tmain.main()\n"
printf "\t\t/app/main.go:15 +0x85\n"
sleep 0.1

# --- 5. sd-daemon prefix + multiline ---
echo "=== Test 10: sd-daemon prefix with multiline ==="
echo "<3>Database connection failed"
echo "  host: db.example.com"
echo "  port: 5432"
echo "  error: connection refused"
sleep 0.1

# --- 6. Messages that should NOT merge (no leading whitespace) ---
echo "=== Test 11: Separate messages (no merge) ==="
echo "First independent message"
sleep 0.15
echo "Second independent message"
sleep 0.15
echo "Third independent message"
sleep 0.1

# --- 7. Priority on stderr with pattern ---
echo "=== Test 12: Pattern match on stderr ==="
echo "WARNING this warning is on stderr" >&2
echo "DEBUG this debug is on stderr" >&2
sleep 0.1

# --- 8. No-match lines (should get source default) ---
echo "=== Test 13: Unmatched lines get default priority ==="
echo "Just a regular log line on stdout"
echo "Another normal line on stderr" >&2
sleep 0.1

echo "=== All tests complete ==="

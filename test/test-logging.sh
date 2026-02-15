#!/bin/sh
#
# Test script for baraverkstad/journald-plus log driver. Run with:
#   docker run --rm -i --log-driver baraverkstad/journald-plus --log-opt tag=test alpine:latest sh < test/test-logging.sh
#
# Then verify with:
#   journalctl -t test --no-pager -o verbose
#   journalctl -t test -p warning --no-pager
#
# Each test sleeps 100ms to ensure buffers flush between tests.

set -e

# Prints test header to stdout
header() {
    echo "=== $1 ==="
    sleep 0.1
}

# Prints to both stdout and stderr
dualprint() {
    echo "$1 (stdout)"
    echo "$1 (stderr)" >&2
    sleep 0.1
}

header "Test 1: Basic stdout/stderr"
dualprint "Simple message on both streams"

header "Test 2: sd-daemon prefixes"
dualprint "<7>Debug via sd-daemon prefix"
dualprint "<6>Info via sd-daemon prefix"
dualprint "<4>Warning via sd-daemon prefix"
dualprint "<3>Error via sd-daemon prefix"
dualprint "<2>Critical via sd-daemon prefix"
dualprint "<0>Emergency via sd-daemon prefix"

header "Test 3: Priority patterns - uppercase"
dualprint "ERROR something went wrong"
dualprint "FATAL process crashed"
dualprint "WARNING disk usage high"
dualprint "WARN connection timeout"
dualprint "CRITICAL system overload"
dualprint "DEBUG variable x = 42"

header "Test 4: Priority patterns - bracketed (MariaDB style)"
dualprint "[ERROR] InnoDB: unable to lock file"
dualprint "[Fatal] out of memory"
dualprint "[Warning] Aborted connection 12345"
dualprint "[Note] Server socket created on IP: 0.0.0.0"
dualprint "[Critical] Too many connections"
dualprint "[Debug] packet dump follows"

header "Test 5: Multiline - Java-style stack trace"
cat <<'EOF'
ERROR java.lang.NullPointerException: Something is null
    at com.example.MyClass.myMethod(MyClass.java:42)
    at com.example.Main.main(Main.java:10)
    at java.base/java.lang.Thread.run(Thread.java:829)
EOF
sleep 0.1

header "Test 6: Multiline - Python traceback"
cat <<'EOF'
ERROR Traceback (most recent call last):
  File "app.py", line 10, in main
    result = process(data)
  File "app.py", line 25, in process
    return data[key]
  KeyError: 'missing_key'
EOF
sleep 0.1

header "Test 7: Multiline - indented config/data block"
cat <<'EOF'
Configuration loaded:
  database:
    host: localhost
    port: 5432
  cache:
    ttl: 300
EOF
sleep 0.1

header "Test 8: Multiline with tab indentation (tests delay)"
printf "Go panic:\n"
sleep 0.05
printf "goroutine 1 [running]:\n"
printf "\tmain.handler(0xc0000b2000)\n"
printf "\t\t/app/main.go:42 +0x1a4\n"
printf "\tmain.main()\n"
printf "\t\t/app/main.go:15 +0x85\n"
sleep 0.1

header "Test 9: sd-daemon prefix with multiline"
cat <<'EOF'
<3>Database connection failed
  host: db.example.com
  port: 5432
  error: connection refused
EOF
sleep 0.1

header "Test 10: Separate messages (no merge)"
echo "First independent message"
echo "Second independent message"
echo "Third independent message"
sleep 0.1

header "All tests complete"

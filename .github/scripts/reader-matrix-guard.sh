#!/bin/sh
# Dependency-free guard: every reader-matrix expectation with status: pass must carry screenshot/log/conversion_log
awk '
/^  - reader:/ { if (rec != "" && st == "pass" && !ev) { print "pass without evidence: " rec " " cs; bad = 1 } rec = $3; cs = ""; st = ""; ev = 0; next }
/^[a-z_]+:/    { if (rec != "" && st == "pass" && !ev) { print "pass without evidence: " rec " " cs; bad = 1 } rec = ""; next }
/^    case:/   { cs = $2 }
/^    status:/ { st = $2 }
/^    (screenshot|log|conversion_log):/ { ev = 1 }
END { if (rec != "" && st == "pass" && !ev) { print "pass without evidence: " rec " " cs; bad = 1 } exit bad }
' "${1:-docs/final/reader-matrix.yaml}"

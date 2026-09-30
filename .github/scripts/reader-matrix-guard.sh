#!/bin/sh
# Dependency-free guard: every reader-matrix expectation with status: pass must carry screenshot/log/conversion_log
awk '
function trim(value) {
  sub(/^[[:space:]]+/, "", value)
  sub(/[[:space:]]+$/, "", value)
  return value
}
function scalar(line, value, first, last, single_quote) {
  sub(/^[[:space:]]*[^:]+:[[:space:]]*/, "", line)
  sub(/[[:space:]]+#.*$/, "", line)
  line = trim(line)
  first = substr(line, 1, 1)
  last = substr(line, length(line), 1)
  single_quote = sprintf("%c", 39)
  if ((first == "\"" && last == "\"") || (first == single_quote && last == single_quote)) {
    line = substr(line, 2, length(line) - 2)
  }
  return trim(line)
}
function field(line, key, value, lower_value) {
  key = line
  sub(/^[[:space:]]*/, "", key)
  sub(/:.*/, "", key)
  value = scalar(line)
  if (key == "reader") rec = value
  else if (key == "case") cs = value
  else if (key == "status") st = tolower(value)
  else if (key == "screenshot" || key == "log" || key == "conversion_log") {
    lower_value = tolower(value)
    if (lower_value != "" && lower_value != "null" && lower_value != "~") ev = 1
  }
}
function finish_record() {
  if (in_record && st == "pass" && !ev) {
    print "pass without evidence: " rec " " cs
    bad = 1
  }
}
/^  - / {
  finish_record()
  in_record = 1
  rec = "?"
  cs = "?"
  st = ""
  ev = 0
  field(substr($0, 5))
  next
}
/^[[:alpha:]_][[:alnum:]_-]*:/ {
  finish_record()
  in_record = 0
  next
}
/^    [[:alnum:]_-]+:/ && in_record { field($0) }
END { finish_record(); exit bad }
' "${1:-docs/final/reader-matrix.yaml}"

#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
clamp="$here/clamp_version.sh"

fail=0
check() {
  local desc="$1" prev="$2" computed="$3" want="$4" got
  got="$(bash "$clamp" "$prev" "$computed")"
  if [ "$got" = "$want" ]; then
    printf 'ok   - %s\n' "$desc"
  else
    printf 'FAIL - %s: clamp(%s, %s) = %s, want %s\n' "$desc" "$prev" "$computed" "$got" "$want"
    fail=1
  fi
}

check "breaking change stays in 0.x" 0.1.0 1.0.0 0.2.0
check "feat minor within 0.x passes through" 0.1.0 0.2.0 0.2.0
check "fix patch within 0.x passes through" 0.1.0 0.1.1 0.1.1
check "breaking at higher 0.x minor" 0.9.0 1.0.0 0.10.0
check "breaking ignores prev patch digit" 0.9.3 1.0.0 0.10.0
check "minor zero bumps to 0.1.0" 0.0.4 1.0.0 0.1.0
check "already 1.x passes through" 1.2.0 2.0.0 2.0.0
check "1.x minor passes through" 1.2.0 1.3.0 1.3.0

exit "$fail"

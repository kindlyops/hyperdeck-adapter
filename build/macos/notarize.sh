#!/usr/bin/env bash
# Submit an artifact (.dmg or .app) to Apple notarization and staple the ticket.
# No-op until notarization credentials are configured.
#
# Required secrets to activate notarization:
#   APPLE_ID       Apple ID email
#   APPLE_PASSWORD app-specific password for that Apple ID
#   APPLE_TEAM_ID  10-char team identifier
set -euo pipefail

TARGET="$1"

if [ -z "${APPLE_ID:-}" ] || [ -z "${APPLE_PASSWORD:-}" ] || [ -z "${APPLE_TEAM_ID:-}" ]; then
  echo "No Apple notarization credentials: skipping notarization for '$TARGET'."
  exit 0
fi

xcrun notarytool submit "$TARGET" \
  --apple-id "$APPLE_ID" \
  --password "$APPLE_PASSWORD" \
  --team-id "$APPLE_TEAM_ID" \
  --wait
xcrun stapler staple "$TARGET"
echo "Notarized and stapled '$TARGET'"

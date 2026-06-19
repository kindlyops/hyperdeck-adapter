#!/usr/bin/env bash
# Code-sign the macOS .app with a Developer ID identity. No-op (leaves the app
# ad-hoc/unsigned) until the signing secrets are configured, so release builds
# work without credentials.
#
# Required secrets to activate signing:
#   APPLE_CERTIFICATE          base64 of the Developer ID Application .p12
#   APPLE_CERTIFICATE_PASSWORD password for that .p12
#   APPLE_SIGNING_IDENTITY     e.g. "Developer ID Application: Acme (TEAMID)"
set -euo pipefail

APP="$1"

if [ -z "${APPLE_CERTIFICATE:-}" ] || [ -z "${APPLE_SIGNING_IDENTITY:-}" ]; then
  echo "No Apple signing identity configured: '$APP' will be unsigned (Gatekeeper will warn on first launch)."
  exit 0
fi

KEYCHAIN="$RUNNER_TEMP/build.keychain"
KEYCHAIN_PW="$(openssl rand -base64 24)"
echo "$APPLE_CERTIFICATE" | base64 --decode > "$RUNNER_TEMP/cert.p12"

security create-keychain -p "$KEYCHAIN_PW" "$KEYCHAIN"
security set-keychain-settings -lut 21600 "$KEYCHAIN"
security unlock-keychain -p "$KEYCHAIN_PW" "$KEYCHAIN"
security import "$RUNNER_TEMP/cert.p12" -P "${APPLE_CERTIFICATE_PASSWORD:-}" \
  -A -t cert -f pkcs12 -k "$KEYCHAIN"
security set-key-partition-list -S apple-tool:,apple:,codesign: \
  -s -k "$KEYCHAIN_PW" "$KEYCHAIN" >/dev/null
security list-keychain -d user -s "$KEYCHAIN"

# Sign nested binary first, then the bundle, with hardened runtime (required for
# notarization).
codesign --force --options runtime --timestamp \
  --sign "$APPLE_SIGNING_IDENTITY" "$APP/Contents/MacOS/injcheck"
codesign --force --options runtime --timestamp \
  --sign "$APPLE_SIGNING_IDENTITY" "$APP"
codesign --verify --deep --strict --verbose=2 "$APP"
echo "Signed '$APP' with $APPLE_SIGNING_IDENTITY"

# hyperdeck-adapter — task runner (https://github.com/casey/just)
# Run `just` with no arguments to list recipes.

# default address the adapter listens on (HyperDeck protocol port)
bind := "127.0.0.1:9993"
profiles := "examples/profiles.yaml"

# show available recipes
default:
    @just --list

# build both binaries into ./bin
build:
    go build -o bin/hyperdeck-adapter ./cmd/hyperdeck-adapter
    go build -o bin/injcheck ./cmd/injcheck

# run the full test suite with the race detector
test:
    go test ./... -race -count=1

# vet + gofmt check (no changes)
check:
    go vet ./...
    @test -z "$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)

# format the code in place
fmt:
    gofmt -w .

# cross-compile sanity for the other targets (Windows + Linux)
cross:
    GOOS=windows GOARCH=amd64 go build ./...
    GOOS=linux   GOARCH=amd64 go build ./...

# (macOS) trigger / check the Accessibility permission for bin/injcheck
trust: build
    ./bin/injcheck trust

# list on-screen windows (optionally filtered): `just list vlc`
list filter="": build
    ./bin/injcheck list {{filter}}

# run the adapter pipeline in the foreground (no tray), locking onto a running player
serve: build
    ./bin/injcheck serve {{profiles}} {{bind}}

# stop any background adapter (serve) process
stop:
    -pkill -f 'injcheck serve'

# run the real tray application (needs its own Accessibility grant on macOS)
run: build
    ./bin/hyperdeck-adapter -config {{profiles}} -bind {{bind}}

# end-to-end demo: start the adapter and send HyperDeck commands to the locked player
demo: build
    #!/usr/bin/env bash
    set -euo pipefail
    BIND="{{bind}}"; HOST="${BIND%:*}"; PORT="${BIND##*:}"
    if ! ./bin/injcheck trust >/dev/null 2>&1; then
      echo "⚠️  Accessibility not granted for $(pwd)/bin/injcheck"
      echo "    Enable it in System Settings → Privacy & Security → Accessibility,"
      echo "    then re-run 'just demo'. (Opening the prompt now…)"
      ./bin/injcheck trust || true
      exit 1
    fi
    echo "▶ starting adapter on ${BIND} (locks onto a running player)…"
    ./bin/injcheck serve {{profiles}} "${BIND}" >/tmp/hda-serve.log 2>&1 &
    SERVE=$!
    trap 'kill "${SERVE}" 2>/dev/null || true' EXIT
    for _ in $(seq 1 25); do nc -z "${HOST}" "${PORT}" 2>/dev/null && break; sleep 0.2; done
    sleep 0.4
    sed 's/^/  [adapter] /' /tmp/hda-serve.log
    echo
    python3 scripts/hyperdeck-demo.py "${BIND}"

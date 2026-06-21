# hyperdeck-adapter — task runner (https://github.com/casey/just)
# Run `just` with no arguments to list recipes.

# default address the adapter listens on (HyperDeck protocol port)
bind := "127.0.0.1:9993"

# show available recipes
default:
    @just --list

# check + lint + test the Rust library crates (the OS-independent core + adapters)
test:
    cargo fmt --all --check
    cargo clippy --workspace --all-targets -- -D warnings
    cargo test --workspace

# format the code in place
fmt:
    cargo fmt --all
    cd src-tauri && cargo fmt

# build the Tauri tray app (debug). Requires the Tauri prerequisites + Tauri CLI
# (`cargo install tauri-cli`): https://v2.tauri.app/start/prerequisites/
build:
    cd src-tauri && cargo tauri build --debug

# run the Tauri tray app in dev mode (needs its own Accessibility grant on macOS)
run:
    cd src-tauri && cargo tauri dev

# (macOS) trigger / check the input (Accessibility) permission for the adapter
trust:
    cd src-tauri && cargo run -- --check-accessibility

# run the adapter headless (no tray) in the foreground, locking onto a running player
serve:
    cd src-tauri && cargo run -- --headless

# end-to-end demo: drive the locked player with HyperDeck protocol commands
# (run `just serve` in another terminal first, or point at any running adapter)
demo:
    python3 scripts/hyperdeck-demo.py {{bind}}

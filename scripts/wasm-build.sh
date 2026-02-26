#!/usr/bin/env bash
# Build a wasm32-unknown-unknown Rust project using the rustup toolchain,
# bypassing any Homebrew rustc that may be on PATH.
#
# Usage: scripts/wasm-build.sh <project-dir>
#   project-dir: path to the Cargo project (relative to repo root)
#
# Expects RUSTUP_HOME to already be set (e.g. via artifact_policy).
set -euo pipefail

PROJECT_DIR="${1:?usage: wasm-build.sh <project-dir>}"

# Locate the stable toolchain from RUSTUP_HOME.
RUSTUP_HOME="${RUSTUP_HOME:-$HOME/.rustup}"
TOOLCHAIN_BIN=""
for tc in "$RUSTUP_HOME/toolchains"/stable-*/bin; do
    TOOLCHAIN_BIN="$tc"
    break
done
if [[ -z "$TOOLCHAIN_BIN" || ! -d "$TOOLCHAIN_BIN" ]]; then
    echo "ERROR: could not find stable rustup toolchain under $RUSTUP_HOME" >&2
    exit 1
fi

# Prepend the rustup toolchain bin so both cargo and wasm-pack use it.
export PATH="$TOOLCHAIN_BIN:$PATH"
export RUSTC="$TOOLCHAIN_BIN/rustc"

cd "$PROJECT_DIR"
cargo build --target wasm32-unknown-unknown --release
wasm-pack build --target web --out-dir www/pkg

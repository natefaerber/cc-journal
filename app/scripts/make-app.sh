#!/usr/bin/env bash
# Assemble CCJournal.app from the SwiftPM release binary + Info.plist.
# Usage: ./scripts/make-app.sh [output-dir]
# Output: <output-dir>/CCJournal.app (default: .build/bundle)

set -euo pipefail

APP_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG="${CONFIG:-release}"
BIN="$APP_DIR/.build/$CONFIG/CCJournal"
PLIST="$APP_DIR/Info.plist"
ICON="$APP_DIR/Resources/AppIcon.icns"
OUT_DIR="${1:-$APP_DIR/.build/bundle}"
BUNDLE="$OUT_DIR/CCJournal.app"

if [[ ! -x "$BIN" ]]; then
  echo "error: $BIN not found. Run 'swift build -c release' first." >&2
  exit 1
fi
if [[ ! -f "$PLIST" ]]; then
  echo "error: $PLIST not found." >&2
  exit 1
fi

rm -rf "$BUNDLE"
mkdir -p "$BUNDLE/Contents/MacOS" "$BUNDLE/Contents/Resources"
cp "$BIN" "$BUNDLE/Contents/MacOS/CCJournal"
cp "$PLIST" "$BUNDLE/Contents/Info.plist"
if [[ -f "$ICON" ]]; then
  cp "$ICON" "$BUNDLE/Contents/Resources/AppIcon.icns"
fi

echo "Built $BUNDLE"

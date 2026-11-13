#!/usr/bin/env bash
# Builds the G-CATION Android app (build/gcation.apk).
#
#   ./build.sh                          # release APK, talks to the live backend
#   API=http://10.0.2.2:8000 ./build.sh # emulator build against a backend on your computer
#
# Needs Go, gogio (go install gioui.org/cmd/gogio@latest), the Android SDK + NDK and Java 17+.
set -euo pipefail
cd "$(dirname "$0")"

export ANDROID_HOME="${ANDROID_HOME:-$HOME/Library/Android/sdk}"
if [[ -z "${JAVA_HOME:-}" && -d /opt/homebrew/opt/openjdk@21 ]]; then
  export JAVA_HOME=/opt/homebrew/opt/openjdk@21
fi

VERSION="${VERSION:-1.0.0.1}"
ARCH="${ARCH:-arm64,arm}"
LDFLAGS=""
if [[ -n "${API:-}" ]]; then
  LDFLAGS="-X main.defaultAPI=$API"
fi

args=(-target android -arch "$ARCH" -appid in.gcation.app -name "G-CATION"
  -version "$VERSION" -minsdk 24 -targetsdk 35 -icon appicon.png -o build/gcation.apk)
if [[ -n "$LDFLAGS" ]]; then
  args+=(-ldflags "$LDFLAGS")
fi
# Sign with your own key for the Play Store: SIGNKEY=path/to.keystore SIGNPASS=... ./build.sh
if [[ -n "${SIGNKEY:-}" ]]; then
  args+=(-signkey "$SIGNKEY" -signpass "${SIGNPASS:?set SIGNPASS}")
fi

mkdir -p build
gogio "${args[@]}" .
echo "Built build/gcation.apk"

# Release builds (live backend) also go on the website's "Download for Android" button.
if [[ -z "${API:-}" ]]; then
  cp build/gcation.apk ../frontend/downloads/gcation.apk
  echo "Copied to frontend/downloads/gcation.apk"
fi

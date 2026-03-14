#!/usr/bin/env sh
set -eu

OUT_DIR="${OUT_DIR:-dist}"
TARGETS="${TARGETS:-linux-amd64}"
NO_STRIP="${NO_STRIP:-0}"

mkdir -p "$OUT_DIR"

LDFLAGS=""
if [ "$NO_STRIP" != "1" ]; then
  LDFLAGS="-s -w"
fi

echo "OutDir: $OUT_DIR"
echo "Targets: $TARGETS"

for t in $TARGETS; do
  goos="$(echo "$t" | cut -d- -f1)"
  goarch="$(echo "$t" | cut -d- -f2)"

  echo ""
  echo "==> Building $goos/$goarch"

  export GOOS="$goos"
  export GOARCH="$goarch"
  export CGO_ENABLED=0

  ext=""
  if [ "$goos" = "windows" ]; then ext=".exe"; fi

  center_out="$OUT_DIR/tanzhen-center-$goos-$goarch$ext"
  probe_out="$OUT_DIR/tanzhen-probe-$goos-$goarch$ext"

  if [ -n "$LDFLAGS" ]; then
    go build -trimpath -ldflags "$LDFLAGS" -o "$center_out" ./cmd/center
    go build -trimpath -ldflags "$LDFLAGS" -o "$probe_out" ./cmd/agent
  else
    go build -trimpath -o "$center_out" ./cmd/center
    go build -trimpath -o "$probe_out" ./cmd/agent
  fi

  echo "  OK: $center_out"
  echo "  OK: $probe_out"
done

echo ""
echo "Done."

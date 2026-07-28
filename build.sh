#!/bin/sh

echo "Select build target:"
echo "  1) linux/amd64"
echo "  2) linux/arm64"
echo "  3) darwin/amd64"
echo "  4) darwin/arm64"
echo "  5) windows/amd64"
echo "  6) windows/arm64"
printf "Choice [1-6]: "
read choice

case "$choice" in
  1) GOOS=linux   GOARCH=amd64 ;;
  2) GOOS=linux   GOARCH=arm64 ;;
  3) GOOS=darwin  GOARCH=amd64 ;;
  4) GOOS=darwin  GOARCH=arm64 ;;
  5) GOOS=windows GOARCH=amd64 ;;
  6) GOOS=windows GOARCH=arm64 ;;
  *) echo "Invalid choice"; exit 1 ;;
esac

echo ""
echo "Select build type:"
echo "  1) debug   — full symbols, source paths exposed"
echo "  2) release — stripped, -trimpath, smaller binary"
printf "Choice [1-2]: "
read bt

case "$bt" in
  1) MODE=debug
     LDFLAGS="" ;;
  2) MODE=release
     LDFLAGS="-s -w"
     TRIM="-trimpath" ;;
  *) echo "Invalid choice"; exit 1 ;;
esac

ext=""
[ "$GOOS" = "windows" ] && ext=".exe"
if [ "$GOOS" = "linux" ] && [ "$GOARCH" = "arm64" ]; then
  out="ColumbinaHotfix${ext}"
else
  out="ColumbinaHotfix_${GOOS}_${GOARCH}_${MODE}${ext}"
fi

env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 go build $TRIM -ldflags="$LDFLAGS" -o "$out" .
echo "Built: $out"

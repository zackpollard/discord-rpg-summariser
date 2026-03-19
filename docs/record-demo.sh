#!/bin/bash
# Record a ~45s demo video walkthrough of the web panel.
# Requires: agent-browser, ffmpeg, app running on localhost:5173
set -e

AB=~/.volta/tools/image/packages/agent-browser/lib/node_modules/agent-browser/bin/agent-browser-darwin-arm64
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT="$SCRIPT_DIR/demo.webm"
FRAMES_DIR=$(mktemp -d)
trap "rm -rf '$FRAMES_DIR'" EXIT INT

go() { $AB open "$1"; sleep "${2:-1.5}"; }

scroll() {
  local px=${1:-1000}
  $AB eval --stdin <<JSEOF
new Promise(r=>{let y=0;const i=setInterval(()=>{y+=8;window.scrollTo(0,y);if(y>=$px){clearInterval(i);r()}},6)})
JSEOF
}

scroll_all() {
  $AB eval --stdin <<'JSEOF'
new Promise(r=>{const m=document.documentElement.scrollHeight-window.innerHeight;if(m<=0){r();return}let y=0;const i=setInterval(()=>{y+=12;window.scrollTo(0,y);if(y>=m){clearInterval(i);r()}},5)})
JSEOF
}

# Capture a screenshot frame with a sequential name
FRAME=0
snap() {
  FRAME=$((FRAME+1))
  $AB screenshot "$(printf '%s/frame_%05d.png' "$FRAMES_DIR" "$FRAME")" 2>/dev/null
}

# Capture multiple frames over a duration at ~5fps
hold() {
  local ms=${1:-1500}
  local frames=$((ms / 200))
  for _ in $(seq 1 "$frames"); do
    snap
    sleep 0.2
  done
}

# Navigate and hold
go_hold() {
  $AB open "$1"
  sleep 1
  hold "${2:-1500}"
}

scroll_capture() {
  local px=${1:-1000}
  local steps=$((px / 40))
  for _ in $(seq 1 "$steps"); do
    $AB eval "window.scrollBy(0, 40)" 2>/dev/null
    snap
    sleep 0.03
  done
}

scroll_all_capture() {
  $AB eval --stdin <<'JSEOF'
Math.max(0, document.documentElement.scrollHeight - window.innerHeight)
JSEOF
}

# Fresh browser
$AB close 2>/dev/null || true
sleep 1
$AB set viewport 1280 720
$AB open http://localhost:5173/campaigns/3
sleep 2

echo "Capturing frames..."

hold 3000                                                   # Dashboard

go_hold http://localhost:5173/campaigns/3/sessions 3000     # Sessions list

$AB open http://localhost:5173/sessions/3; sleep 1          # Session detail
hold 1500
scroll_capture 1500
hold 1000

go_hold http://localhost:5173/campaigns/3/characters 3000   # Characters

$AB open http://localhost:5173/campaigns/3/lore; sleep 1    # Lore
hold 1500
scroll_capture 1000
hold 1000

$AB eval 'document.querySelector("a[href*=\"/lore/\"]")?.click()'
sleep 1
hold 3000                                                   # Entity detail

$AB open http://localhost:5173/campaigns/3/quests; sleep 1  # Quests
hold 1500
scroll_capture 600
hold 1000

$AB open http://localhost:5173/campaigns/3/timeline; sleep 1 # Timeline
hold 1000
MAX=$($AB eval 'Math.max(0, document.documentElement.scrollHeight - window.innerHeight)')
scroll_capture "${MAX:-2000}"
hold 1000

go_hold http://localhost:5173/campaigns/3/recap 3000        # Recap

echo "Captured $FRAME frames"

# Encode frames to webm
ffmpeg -y -framerate 5 -i "$FRAMES_DIR/frame_%05d.png" \
  -c:v libvpx-vp9 -b:v 800k -pix_fmt yuv420p \
  "$OUT" 2>&1 | tail -5

DUR=$(ffprobe -v quiet -show_entries format=duration -of csv=p=0 "$OUT" 2>/dev/null || echo "?")
SIZE=$(du -h "$OUT" | cut -f1)
RES=$(ffprobe -v quiet -show_entries stream=width,height -of csv=p=0 "$OUT" 2>/dev/null || echo "?")
echo "Demo: $OUT ($SIZE, ${DUR}s, ${RES})"

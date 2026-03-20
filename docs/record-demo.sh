#!/bin/bash
# Record a demo video walkthrough of the web panel.
# Captures at ~5fps with 6x slower timings, then encodes at 30fps (6x speedup)
# for smooth 30fps playback.
# Requires: agent-browser, ffmpeg, app running on localhost:5173
set -e

AB=~/.volta/tools/image/packages/agent-browser/lib/node_modules/agent-browser/bin/agent-browser-darwin-arm64
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT="$SCRIPT_DIR/demo.webm"
FRAMES_DIR=$(mktemp -d)
trap "rm -rf '$FRAMES_DIR'" EXIT INT

# All timings are 6x real-time. Final video is sped up 6x via 30fps encoding.
# 6000ms capture = 1s playback, 3000ms = 0.5s, 12000ms = 2s, etc.

FRAME=0
snap() {
  FRAME=$((FRAME+1))
  $AB screenshot "$(printf '%s/frame_%05d.png' "$FRAMES_DIR" "$FRAME")" 2>/dev/null
}

# Hold on current page
hold() {
  local ms=${1:-9000}
  local frames=$((ms / 200))
  for _ in $(seq 1 "$frames"); do
    snap
    sleep 0.2
  done
}

# Navigate then hold — waits for page to load before capturing
nav() {
  $AB open "$1"
  sleep 2
  hold "${2:-9000}"
}

# Smooth scroll with ease-in-out using JS. Captures a frame every 200ms
# while the browser scrolls with CSS smooth behavior.
# $1 = pixels to scroll, captured in 6x slow-motion
scroll_smooth() {
  local px=${1:-1000}

  # Use JS to animate scroll with easeInOutCubic over a duration.
  # Duration in ms = px * 30 (at 6x slow-mo, so ~5px/ms → 30px/ms playback)
  local duration_ms=$((px * 30))
  if [ "$duration_ms" -lt 3000 ]; then duration_ms=3000; fi

  # Start the scroll animation in the browser (non-blocking eval)
  $AB eval --stdin <<JSEOF
(() => {
  const distance = $px;
  const duration = $duration_ms;
  const startY = window.scrollY;
  const startTime = performance.now();
  function easeInOutCubic(t) {
    return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
  }
  function step(now) {
    const elapsed = now - startTime;
    const progress = Math.min(elapsed / duration, 1);
    window.scrollTo(0, startY + distance * easeInOutCubic(progress));
    if (progress < 1) requestAnimationFrame(step);
  }
  requestAnimationFrame(step);
})()
JSEOF

  # Capture frames for the duration of the scroll
  local capture_frames=$((duration_ms / 200))
  for _ in $(seq 1 "$capture_frames"); do
    snap
    sleep 0.2
  done
}

# Scroll the full page height
scroll_smooth_full() {
  local max
  max=$($AB eval 'Math.max(0, document.documentElement.scrollHeight - window.innerHeight)')
  if [ -z "$max" ] || [ "$max" = "null" ] || [ "$max" -le 0 ] 2>/dev/null; then
    return
  fi
  scroll_smooth "$max"
}

# Fresh browser at 1280x720
$AB close 2>/dev/null || true
sleep 1
$AB set viewport 1280 720

# Pre-load first page
$AB open http://localhost:5173/campaigns/3
sleep 3

echo "Capturing frames (6x slow-motion)..."

# Dashboard (2s playback)
hold 12000

# Sessions list (2s playback)
nav http://localhost:5173/campaigns/3/sessions 12000

# Session detail: hold, scroll with ease, hold (total ~4s playback)
nav http://localhost:5173/sessions/3 6000
scroll_smooth 1500
hold 6000

# Characters (2s playback)
nav http://localhost:5173/campaigns/3/characters 12000

# Lore: hold, scroll, hold (~3s playback)
nav http://localhost:5173/campaigns/3/lore 6000
scroll_smooth 1000
hold 6000

# Click into entity detail (2.5s playback)
$AB eval 'document.querySelector("a[href*=\"/lore/\"]")?.click()'
sleep 2
hold 15000

# Quests: hold, scroll, hold (~3s playback)
nav http://localhost:5173/campaigns/3/quests 6000
scroll_smooth 600
hold 6000

# Timeline: hold, scroll full, hold
nav http://localhost:5173/campaigns/3/timeline 6000
scroll_smooth_full
hold 6000

# Recap (2.5s playback)
nav http://localhost:5173/campaigns/3/recap 15000

echo "Captured $FRAME frames"

# Encode at 30fps (= 5fps capture * 6x speedup)
ffmpeg -y -framerate 30 -i "$FRAMES_DIR/frame_%05d.png" \
  -c:v libvpx-vp9 -b:v 800k -pix_fmt yuv420p \
  "$OUT" 2>&1 | tail -3

DUR=$(ffprobe -v quiet -show_entries format=duration -of csv=p=0 "$OUT" 2>/dev/null || echo "?")
SIZE=$(du -h "$OUT" | cut -f1)
RES=$(ffprobe -v quiet -show_entries stream=width,height -of csv=p=0 "$OUT" 2>/dev/null || echo "?")
echo "Demo: $OUT ($SIZE, ${DUR}s, ${RES})"

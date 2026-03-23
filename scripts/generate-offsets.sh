#!/bin/bash
# Generate offsets.json for session audio directories that don't have one.
#
# Uses the creation time (or modification time) of each user's WAV file
# relative to the earliest WAV in the directory to compute join offsets.
#
# Usage:
#   ./scripts/generate-offsets.sh <audio_base_dir>
#
# Example:
#   ./scripts/generate-offsets.sh /data/audio
#   ./scripts/generate-offsets.sh data/audio

set -euo pipefail

AUDIO_BASE="${1:?Usage: $0 <audio_base_dir>}"

if [ ! -d "$AUDIO_BASE" ]; then
    echo "Error: $AUDIO_BASE is not a directory" >&2
    exit 1
fi

# Use birth time if available (macOS/btrfs), fall back to mtime.
get_timestamp() {
    local file="$1"
    # Try stat -c %W (birth time) on Linux
    if stat -c %W "$file" 2>/dev/null | grep -qv '^0$'; then
        stat -c %W "$file" 2>/dev/null
        return
    fi
    # Try stat -f %B (birth time) on macOS
    if stat -f %B "$file" 2>/dev/null | grep -qv '^0$'; then
        stat -f %B "$file" 2>/dev/null
        return
    fi
    # Fall back to mtime
    stat -c %Y "$file" 2>/dev/null || stat -f %m "$file" 2>/dev/null
}

count=0
skipped=0

# Find all session directories (contain WAV files).
find "$AUDIO_BASE" -name "*.wav" -print0 | xargs -0 -n1 dirname | sort -u | while read -r session_dir; do
    # Skip if offsets.json already exists.
    if [ -f "$session_dir/offsets.json" ]; then
        skipped=$((skipped + 1))
        continue
    fi

    # Collect WAV files and their timestamps.
    declare -A timestamps
    earliest=""

    for wav in "$session_dir"/*.wav; do
        [ -f "$wav" ] || continue
        ts=$(get_timestamp "$wav")
        user_id=$(basename "$wav" .wav)
        timestamps[$user_id]=$ts

        if [ -z "$earliest" ] || [ "$ts" -lt "$earliest" ]; then
            earliest=$ts
        fi
    done

    if [ -z "$earliest" ]; then
        unset timestamps
        continue
    fi

    # Build JSON with offsets in seconds relative to the earliest file.
    json="{"
    first=true
    for user_id in "${!timestamps[@]}"; do
        ts=${timestamps[$user_id]}
        offset=$((ts - earliest))
        if [ "$first" = true ]; then
            first=false
        else
            json+=","
        fi
        json+="\"$user_id\":$offset.0"
    done
    json+="}"

    echo "$json" > "$session_dir/offsets.json"
    echo "Created $session_dir/offsets.json: $json"
    count=$((count + 1))

    unset timestamps
done

echo "Done. Created $count offsets.json file(s)."

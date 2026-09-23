#!/usr/bin/env bash
# Upload and clone one generated video to the selected account's Saved Messages.
# The account database is copied to a private temporary directory before use.
set -euo pipefail

source_store=${1:?pass the path to an existing Bolt account file}
namespace=${2:?pass the account namespace}
driver=${3:-bolt}
project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
binary="$project_root/dist/tdl-linux-amd64"
test -x "$binary"
test -f "$source_store"

run_dir=$(mktemp -d /tmp/tdl-cover-e2e.XXXXXXXX)
mkdir -m 700 "$run_dir/data"
case "$driver" in
  bolt)
    cp -- "$source_store" "$run_dir/data/$namespace"
    storage="type=bolt,path=$run_dir/data"
    ;;
  legacy)
    cp -- "$source_store" "$run_dir/legacy.kv"
    storage="type=legacy,path=$run_dir/legacy.kv"
    ;;
  *)
    echo "storage driver must be bolt or legacy" >&2
    exit 2
    ;;
esac
cleanup() {
  rm -f -- "$run_dir/data/$namespace" "$run_dir/legacy.kv" "$run_dir/source.json" "$run_dir/recent.json" "$run_dir/command.log"
  if [[ -n ${video:-} ]]; then rm -f -- "$video"; fi
  rmdir -- "$run_dir/data" 2>/dev/null || true
  rmdir -- "$run_dir" 2>/dev/null || true
}
trap cleanup EXIT

video_name="TDL-COVER-E2E-$(date +%Y%m%d-%H%M%S)-$$.mp4"
video="$run_dir/$video_name"
ffmpeg -hide_banner -loglevel error -y -f lavfi \
  -i 'testsrc2=size=1920x1080:rate=24:duration=5' \
  -c:v libx264 -preset ultrafast -threads 1 -b:v 5M -pix_fmt yuv420p "$video"

common=(--storage "$storage" --ns "$namespace" --threads 4 --limit 1)
run_cli() {
  if ! "$binary" "${common[@]}" "$@" >"$run_dir/command.log" 2>&1; then
    tail -n 15 "$run_dir/command.log"
    return 1
  fi
}
run_cli up -p "$video" --cover-at 1s
printf 'UPLOAD=ok\n'
"$binary" "${common[@]}" chat export --type last --input 20 \
  --filter "Media.Name == \"$video_name\"" --output "$run_dir/source.json" >"$run_dir/command.log" 2>&1
run_cli forward --mode clone --from "$run_dir/source.json" --cover-at 1s
printf 'CLONE=ok\n'
"$binary" "${common[@]}" chat export --type last --input 20 \
  --filter "Media.Name == \"$video_name\"" --output "$run_dir/recent.json" --raw >"$run_dir/command.log" 2>&1

python3 - "$run_dir/recent.json" "$video_name" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    messages = json.load(stream)["messages"]
matching = [message for message in messages if message.get("file") == sys.argv[2]]
matching.sort(key=lambda message: message["id"])
if len(matching) < 2:
    raise SystemExit(f"expected uploaded and cloned video, found {len(matching)}")
for message in matching[-2:]:
    media = message["raw"]["Media"]
    cover = media.get("VideoCover")
    if not cover or media.get("VideoTimestamp", 0) != 0:
        raise SystemExit(f"message {message['id']} has no cover or starts after zero")
    dimensions = [(size.get("W", 0), size.get("H", 0)) for size in cover.get("Sizes", [])]
    print(f"MESSAGE_ID={message['id']} COVER_MAX={max(dimensions, default=(0, 0))} VIDEO_TIMESTAMP={media.get('VideoTimestamp', 0)}")
PY

printf 'TEST_VIDEO=%s\n' "$video_name"

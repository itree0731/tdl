#!/usr/bin/env bash
set -euo pipefail
source_store=${1:?pass the path to an existing Bolt account file}
namespace=${2:?pass the account namespace}
driver=${3:-bolt}
project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
run_dir=$(mktemp -d /tmp/tdl-cover-inspect.XXXXXXXX)
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
  *) exit 2 ;;
esac
cleanup() {
  rm -f -- "$run_dir/data/$namespace" "$run_dir/legacy.kv" "$run_dir/recent.json"
  rmdir -- "$run_dir/data" "$run_dir" 2>/dev/null || true
}
trap cleanup EXIT
"$project_root/dist/tdl-linux-amd64" --storage "$storage" --ns "$namespace" \
  chat export --type last --input 20 --output "$run_dir/recent.json" --raw
python3 - "$run_dir/recent.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as stream:
    messages = json.load(stream)["messages"]
for item in sorted(messages, key=lambda value: value["id"]):
    if not item.get("file", "").startswith("TDL-COVER-E2E-"):
        continue
    media = item["raw"]["Media"]
    cover = media.get("VideoCover")
    sizes = [] if not cover else [(size.get("W", 0), size.get("H", 0)) for size in cover.get("Sizes", [])]
    doc = media.get("Document") or {}
    thumbs = [(size.get("W", 0), size.get("H", 0)) for size in doc.get("Thumbs", [])]
    print(f"ID={item['id']} DOC_ID={doc.get('ID')} COVER={bool(cover)} COVER_SIZES={sizes} DOC_THUMBS={thumbs} MIME={doc.get('MimeType')} VIDEO_TIMESTAMP={media.get('VideoTimestamp')} VIDEO_START_TS={media.get('VideoStartTs')}")
PY

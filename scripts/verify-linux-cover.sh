#!/usr/bin/env bash
set -euo pipefail
export PATH="$HOME/.local/bin:$PATH"
project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$project_root"
mapfile -t packages < <(go list ./... | grep -v '^github.com/iyear/tdl/test$')
go test "${packages[@]}"
go vet "${packages[@]}"
go test -race ./app/dl ./app/up ./app/forward
cd "$project_root/core"
go test ./...
go vet ./...
go test -race ./transfer ./videocover ./forwarder ./uploader ./downloader
cd "$project_root/extension"
go test ./...
go vet ./...
cd "$project_root"
mkdir -p dist
commit=${TDL_BUILD_COMMIT:-$(git rev-parse --short=12 HEAD)}
build_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
CGO_ENABLED=0 go build -trimpath -ldflags "-X github.com/iyear/tdl/pkg/consts.Commit=$commit -X github.com/iyear/tdl/pkg/consts.CommitDate=$build_date" -o dist/tdl-linux-amd64 .
./dist/tdl-linux-amd64 version

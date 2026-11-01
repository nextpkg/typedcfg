#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
scratch="$(mktemp -d "$repo/.module-check.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
{
  echo 'go 1.26.0'
  echo 'use ('
  for module in . advanced validator adapters/yaml adapters/filewatch adapters/nacos; do
    if [[ "$module" == . ]]; then printf '"%s"\n' "$repo"; else printf '"%s/%s"\n' "$repo" "$module"; fi
  done
  echo ')'
} > "$scratch/go.work"
export GOWORK="$scratch/go.work"
"$@"

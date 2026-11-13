#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
scratch="$(mktemp -d "$repo/.module-check.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
modules=(. advanced validator adapters/yaml adapters/filewatch adapters/nacos)
{
  echo 'go 1.26.0'
  echo 'use ('
  for module in "${modules[@]}"; do
    if [[ "$module" == . ]]; then printf '"%s"\n' "$repo"; else printf '"%s/%s"\n' "$repo" "$module"; fi
  done
  echo ')'
} > "$scratch/go.work"
export GOWORK="$scratch/go.work"
if [[ "${1:-}" == --each ]]; then
  shift
  for module in "${modules[@]}"; do
    (cd "$repo/$module" && "$@")
  done
else
  "$@"
fi

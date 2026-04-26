#!/usr/bin/env bash
set -euo pipefail

HEADER="// Copyright (c) 2026 Mintlayer Institutional FZCO"
FAILED=0

if [ "$#" -gt 0 ]; then
  FILES=("$@")
else
  mapfile -t FILES < <(find . -name "*.go" -not -path "./.git/*")
fi

for f in "${FILES[@]}"; do
  first_line=$(head -1 "$f")
  if [ "$first_line" != "$HEADER" ]; then
    echo "MISSING license header: $f"
    FAILED=1
  fi
done

if [ "$FAILED" -ne 0 ]; then
  echo ""
  echo "Run 'bash scripts/add-license.sh' to add missing headers."
  exit 1
fi

echo "All .go files have the required license header."

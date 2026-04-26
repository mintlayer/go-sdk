#!/usr/bin/env bash
set -euo pipefail

HEADER='// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.'

while IFS= read -r -d '' f; do
  first=$(head -1 "$f")
  if [ "$first" != "// Copyright (c) 2026 Mintlayer Institutional FZCO" ]; then
    printf '%s\n\n' "$HEADER" | cat - "$f" > /tmp/_go_license_tmp && mv /tmp/_go_license_tmp "$f"
    echo "Added header: $f"
  fi
done < <(find . -name "*.go" -not -path "./.git/*" -print0)

// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"fmt"
	"strconv"
	"strings"
)

// Uint64 is a uint64 that accepts both a bare JSON number and a quoted-string
// number (e.g. both 42 and "42"). Several Mintlayer API fields are documented
// as integers but are serialised as strings by the server.
type Uint64 uint64

func (n *Uint64) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("Uint64: empty value")
	}
	if b[0] == '"' {
		if len(b) < 2 || b[len(b)-1] != '"' {
			return fmt.Errorf("Uint64: malformed string %s", b)
		}
		v, err := strconv.ParseUint(string(b[1:len(b)-1]), 10, 64)
		if err != nil {
			return fmt.Errorf("Uint64: %w", err)
		}
		*n = Uint64(v)
		return nil
	}
	v, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		return fmt.Errorf("Uint64: %w", err)
	}
	*n = Uint64(v)
	return nil
}

func (n Uint64) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatUint(uint64(n), 10)), nil
}

// PerThousand holds a margin_ratio_per_thousand value as a float64.
// The API may return the value as a bare number (35), a quoted integer ("35"),
// or a quoted decimal with a spurious percent sign ("3.5%"). In all cases the
// numeric part is stored as-is in per-thousand units.
type PerThousand float64

func (n *PerThousand) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("PerThousand: empty value")
	}
	var s string
	if b[0] == '"' {
		if len(b) < 2 || b[len(b)-1] != '"' {
			return fmt.Errorf("PerThousand: malformed string %s", b)
		}
		s = strings.TrimSuffix(string(b[1:len(b)-1]), "%")
	} else {
		s = string(b)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("PerThousand: %w", err)
	}
	*n = PerThousand(f)
	return nil
}

func (n PerThousand) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatFloat(float64(n), 'f', -1, 64)), nil
}

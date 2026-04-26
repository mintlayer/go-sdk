// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"fmt"
	"strconv"
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

// Uint32 is a uint32 that accepts both a bare JSON number and a quoted-string
// number. See Uint64 for rationale.
type Uint32 uint32

func (n *Uint32) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("Uint32: empty value")
	}
	if b[0] == '"' {
		if len(b) < 2 || b[len(b)-1] != '"' {
			return fmt.Errorf("Uint32: malformed string %s", b)
		}
		v, err := strconv.ParseUint(string(b[1:len(b)-1]), 10, 32)
		if err != nil {
			return fmt.Errorf("Uint32: %w", err)
		}
		*n = Uint32(v)
		return nil
	}
	v, err := strconv.ParseUint(string(b), 10, 32)
	if err != nil {
		return fmt.Errorf("Uint32: %w", err)
	}
	*n = Uint32(v)
	return nil
}

func (n Uint32) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatUint(uint64(n), 10)), nil
}

// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	mintlayer "github.com/mintlayer/go-sdk/wasm"
)

// selfTestDecodeRequested reports whether the hidden crash-probe flag was passed.
func selfTestDecodeRequested() bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "self-test-decode-crash" {
			found = true
		}
	})
	// the flag is not declared via the public FlagSet; also scan os.Args
	if !found {
		for _, a := range os.Args[1:] {
			if a == "--self-test-decode-crash" || a == "-self-test-decode-crash" {
				return true
			}
		}
	}
	return found
}

// runDecodeCrashSelfTest calls DecodeSignedTransactionToJS on a known-good
// signed transaction. If the wasm runtime faults, this process dies with a
// non-zero exit — which the parent harness interprets as the finding.
func runDecodeCrashSelfTest() {
	// a 1-in/2-out signed coin transfer captured live from the testnet mempool
	const txHex = "010004000066343f500ae07aae1f980160017f10e1e9333a38b890f1574b13ca85dc14b38c01000000080000821a060001d6f4e7f45046511618a50b1a435a1e3056c4421800000f099bfe3f76e905019b4113335e128eac1c26a39d3479df95b418e3440401018d0100034a9784bbd56ab79511fb7e4b12608b29d1caf9f5b981bb25a051dd428736b293001bf6d7aef2e4c4e8d1828f9a81c08de024dbdd3fe37a080ff0127560f19e80cb8de999e59371ed356f804e48b778fb7b26aa23e8cf041eeae797b5ef94787a25"
	w, err := mintlayer.New(context.Background())
	if err != nil {
		fmt.Println("wasm init:", err)
		os.Exit(3)
	}
	defer w.Close()
	b, err := hex.DecodeString(txHex)
	if err != nil {
		fmt.Println("hex:", err)
		os.Exit(3)
	}
	dec, err := w.DecodeSignedTransactionToJS(b, mintlayer.Testnet)
	if err != nil {
		// clean error — not a crash, still notable but survivable
		fmt.Println("clean error:", err)
		os.Exit(4)
	}
	fmt.Println("decoded bytes:", len(dec))
	os.Exit(0)
}

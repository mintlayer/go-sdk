// Package mintlayer provides Go bindings for the Mintlayer blockchain WASM library.
//
// It wraps the compiled WebAssembly module from the Mintlayer wasm-wrappers project,
// exposing all cryptographic and transaction-building primitives as idiomatic Go functions.
//
// # Usage
//
//	ctx := context.Background()
//	client, err := mintlayer.New(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	privKey, err := client.MakePrivateKey()
//	pubKey, err := client.PublicKeyFromPrivateKey(privKey)
//	addr, err := client.PubkeyToPubkeyHashAddress(pubKey, mintlayer.Mainnet)
//
// # Thread Safety
//
// A Client instance is safe for concurrent use by multiple goroutines.
// For high-throughput workloads, consider creating a pool of Client instances.
//
// # Networks
//
// Mintlayer supports four networks: Mainnet, Testnet, Regtest, and Signet.
// Most functions require a Network parameter that determines address prefixes
// and consensus parameters.
//
// # Amounts
//
// All coin and token amounts are represented as [Amount], which stores the
// value as a decimal string of "atoms" — the smallest indivisible unit.
// 1 ML = 100_000_000_000 atoms (11 decimal places).
package mintlayer

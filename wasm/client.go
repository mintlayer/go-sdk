// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

import (
	"context"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed wasm_wrappers_bg.wasm
var wasmBytes []byte

// Client provides access to all Mintlayer WASM functions.
// A Client instance is safe for concurrent use by multiple goroutines.
type Client struct {
	mu         sync.Mutex
	ctx        context.Context
	rt         wazero.Runtime
	mod        api.Module
	lastErrMsg string          // populated after each call; read by extractError
	lastJSON   json.RawMessage // populated when WASM calls JSON.parse
}

// New creates a new Client, compiling and instantiating the embedded WASM module.
// The provided context is stored as the runtime's base context.
// Call [Client.Close] when done to free WASM resources.
func New(ctx context.Context) (*Client, error) {
	rt := wazero.NewRuntime(ctx)

	if err := registerHostModule(ctx, rt); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("mintlayer: register host module: %w", err)
	}

	mod, err := rt.Instantiate(ctx, wasmBytes)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("mintlayer: instantiate wasm: %w", err)
	}

	return &Client{ctx: ctx, rt: rt, mod: mod}, nil
}

// Close releases all WASM resources associated with this client.
func (c *Client) Close() error {
	return c.rt.Close(c.ctx)
}

// ── low-level call helpers ────────────────────────────────────────────────────

// call executes a named WASM export function with the given parameters and
// returns the raw uint64 results.
func (c *Client) call(fn string, params ...uint64) ([]uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastErrMsg = ""
	c.lastJSON = nil

	ctx := ctxWithCall(c.ctx)
	f := c.mod.ExportedFunction(fn)
	if f == nil {
		return nil, fmt.Errorf("mintlayer: function %q not found", fn)
	}
	res, err := f.Call(ctx, params...)

	// Capture per-call state into Client fields before returning.
	if cc := getCallCtx(ctx); cc != nil {
		c.lastErrMsg = cc.errMsg
		c.lastJSON = cc.lastJSONResult
	}

	if err != nil {
		if c.lastErrMsg != "" {
			return nil, fmt.Errorf("mintlayer: %s", c.lastErrMsg)
		}
		return nil, fmt.Errorf("mintlayer: call %s: %w", fn, err)
	}
	return res, nil
}

// extractError returns an error using the last error message captured from the WASM call.
func (c *Client) extractError(errIdx uint32) error {
	if c.lastErrMsg != "" {
		return fmt.Errorf("mintlayer: %s", c.lastErrMsg)
	}
	return fmt.Errorf("mintlayer: wasm returned error (ref=%d)", errIdx)
}

// callReturnBytes calls fn expecting [ptr, len, errRef, errFlag] return.
func (c *Client) callReturnBytes(fn string, params ...uint64) ([]byte, error) {
	ret, err := c.call(fn, params...)
	if err != nil {
		return nil, err
	}
	if len(ret) >= 4 && ret[3] != 0 {
		return nil, c.extractError(uint32(ret[2]))
	}
	if len(ret) < 2 {
		return nil, fmt.Errorf("mintlayer: unexpected return count from %s", fn)
	}
	ptr, length := uint32(ret[0]), uint32(ret[1])
	if length == 0 {
		return []byte{}, nil
	}
	data, ok := c.mod.Memory().Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("mintlayer: memory read failed")
	}
	result := make([]byte, length)
	copy(result, data)
	c.mod.ExportedFunction("__wbindgen_free").Call(c.ctx, uint64(ptr), uint64(length), 1) //nolint:errcheck
	return result, nil
}

// callReturnBytesNoErr calls fn expecting [ptr, len] return (infallible).
func (c *Client) callReturnBytesNoErr(fn string, params ...uint64) ([]byte, error) {
	ret, err := c.call(fn, params...)
	if err != nil {
		return nil, err
	}
	if len(ret) < 2 {
		return nil, fmt.Errorf("mintlayer: unexpected return count from %s", fn)
	}
	ptr, length := uint32(ret[0]), uint32(ret[1])
	if length == 0 {
		return []byte{}, nil
	}
	data, ok := c.mod.Memory().Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("mintlayer: memory read failed")
	}
	result := make([]byte, length)
	copy(result, data)
	c.mod.ExportedFunction("__wbindgen_free").Call(c.ctx, uint64(ptr), uint64(length), 1) //nolint:errcheck
	return result, nil
}

// callReturnString calls fn expecting [ptr, len, errRef, errFlag] and decodes as UTF-8.
func (c *Client) callReturnString(fn string, params ...uint64) (string, error) {
	ret, err := c.call(fn, params...)
	if err != nil {
		return "", err
	}
	if len(ret) >= 4 && ret[3] != 0 {
		return "", c.extractError(uint32(ret[2]))
	}
	if len(ret) < 2 {
		return "", fmt.Errorf("mintlayer: unexpected return count from %s", fn)
	}
	ptr, length := uint32(ret[0]), uint32(ret[1])
	data, ok := c.mod.Memory().Read(ptr, length)
	if !ok {
		return "", fmt.Errorf("mintlayer: memory read failed")
	}
	s := string(data)
	c.mod.ExportedFunction("__wbindgen_free").Call(c.ctx, uint64(ptr), uint64(length), 1) //nolint:errcheck
	return s, nil
}

// callReturnBool calls fn expecting [bool, errRef, errFlag].
func (c *Client) callReturnBool(fn string, params ...uint64) (bool, error) {
	ret, err := c.call(fn, params...)
	if err != nil {
		return false, err
	}
	if len(ret) >= 3 && ret[2] != 0 {
		return false, c.extractError(uint32(ret[1]))
	}
	return ret[0] != 0, nil
}

// callReturnAmount calls fn that returns a single Amount pointer (infallible).
func (c *Client) callReturnAmount(fn string, params ...uint64) (Amount, error) {
	ret, err := c.call(fn, params...)
	if err != nil {
		return Amount{}, err
	}
	if len(ret) == 0 {
		return Amount{}, fmt.Errorf("mintlayer: no return value from %s", fn)
	}
	return c.readAmount(uint32(ret[0]))
}

// callReturnU32 calls fn expecting [u32, errRef, errFlag].
func (c *Client) callReturnU32(fn string, params ...uint64) (uint32, error) {
	ret, err := c.call(fn, params...)
	if err != nil {
		return 0, err
	}
	if len(ret) >= 3 && ret[2] != 0 {
		return 0, c.extractError(uint32(ret[1]))
	}
	return uint32(ret[0]), nil
}

// callReturnU64 calls fn expecting [u64, errRef, errFlag].
func (c *Client) callReturnU64(fn string, params ...uint64) (uint64, error) {
	ret, err := c.call(fn, params...)
	if err != nil {
		return 0, err
	}
	if len(ret) >= 3 && ret[2] != 0 {
		return 0, c.extractError(uint32(ret[1]))
	}
	return ret[0], nil
}

// callReturnAmountFallible calls fn expecting [amtPtr, errRef, errFlag].
func (c *Client) callReturnAmountFallible(fn string, params ...uint64) (Amount, error) {
	ret, err := c.call(fn, params...)
	if err != nil {
		return Amount{}, err
	}
	if len(ret) >= 3 && ret[2] != 0 {
		return Amount{}, c.extractError(uint32(ret[1]))
	}
	if len(ret) == 0 {
		return Amount{}, fmt.Errorf("mintlayer: no return value from %s", fn)
	}
	return c.readAmount(uint32(ret[0]))
}

// callVoidFallible calls fn expecting [errRef, errFlag] (void on success).
func (c *Client) callVoidFallible(fn string, params ...uint64) error {
	ret, err := c.call(fn, params...)
	if err != nil {
		return err
	}
	if len(ret) >= 2 && ret[1] != 0 {
		return c.extractError(uint32(ret[0]))
	}
	return nil
}

// callReturnJSON calls fn and returns the JSON captured by the JSON.parse host function.
func (c *Client) callReturnJSON(fn string, params ...uint64) (json.RawMessage, error) {
	ret, err := c.call(fn, params...)
	if err != nil {
		return nil, err
	}
	if len(ret) >= 3 && ret[2] != 0 {
		return nil, c.extractError(uint32(ret[1]))
	}
	if c.lastJSON != nil {
		return c.lastJSON, nil
	}
	// Fallback: try to get the value from our Go refs table (set by __wbg_parse_*).
	key := uintptr(ret[0])
	if v, ok := refs.get(key); ok {
		refs.free(key)
		data, err2 := json.Marshal(v)
		if err2 != nil {
			return nil, fmt.Errorf("mintlayer: marshal externref: %w", err2)
		}
		return json.RawMessage(data), nil
	}
	return nil, fmt.Errorf("mintlayer: no JSON result from %s", fn)
}

// ── memory helpers ────────────────────────────────────────────────────────────

// writeBytes copies data into WASM heap memory. Caller must call freeWASM when done.
func (c *Client) writeBytes(data []byte) (ptr, length uint32, err error) {
	if len(data) == 0 {
		return 0, 0, nil
	}
	result, callErr := c.mod.ExportedFunction("__wbindgen_malloc").Call(c.ctx, uint64(len(data)), 1)
	if callErr != nil || len(result) == 0 {
		return 0, 0, fmt.Errorf("mintlayer: malloc failed: %w", callErr)
	}
	ptr = uint32(result[0])
	if !c.mod.Memory().Write(ptr, data) {
		return 0, 0, fmt.Errorf("mintlayer: memory write failed")
	}
	return ptr, uint32(len(data)), nil
}

// writeString copies a UTF-8 string into WASM heap memory.
func (c *Client) writeString(s string) (ptr, length uint32, err error) {
	return c.writeBytes([]byte(s))
}

// freeWASM frees a WASM heap allocation.
func (c *Client) freeWASM(ptr, size uint32) {
	if ptr == 0 {
		return
	}
	c.mod.ExportedFunction("__wbindgen_free").Call(c.ctx, uint64(ptr), uint64(size), 1) //nolint:errcheck
}

// writeOptionalString writes a string (or nil as ptr=0/len=0).
func (c *Client) writeOptionalString(s *string) (ptr, length uint32, err error) {
	if s == nil {
		return 0, 0, nil
	}
	return c.writeString(*s)
}

// writeOptionalBytes writes a []byte (or nil as ptr=0/len=0).
func (c *Client) writeOptionalBytes(b []byte) (ptr, length uint32, err error) {
	if b == nil {
		return 0, 0, nil
	}
	return c.writeBytes(b)
}

// newWASMAmount allocates an Amount struct in the WASM heap.
// WASM functions that accept Amount take ownership; do NOT free separately.
// amount_from_atoms takes ownership of the string allocation (Rust stores it
// inside the Amount), so we must NOT free strPtr after this call.
func (c *Client) newWASMAmount(amount Amount) (uint32, error) {
	strPtr, strLen, err := c.writeString(amount.atoms)
	if err != nil {
		return 0, err
	}
	// Do NOT defer freeWASM here: amount_from_atoms takes ownership of strPtr.

	result, err := c.mod.ExportedFunction("amount_from_atoms").Call(c.ctx, uint64(strPtr), uint64(strLen))
	if err != nil || len(result) == 0 {
		// Free strPtr only on error, since ownership wasn't transferred.
		c.freeWASM(strPtr, strLen)
		return 0, fmt.Errorf("mintlayer: amount_from_atoms: %w", err)
	}
	return uint32(result[0]), nil
}

// readAmount reads the atom string from a WASM Amount pointer, returning an Amount.
// amount_atoms consumes the Amount pointer.
func (c *Client) readAmount(wasmPtr uint32) (Amount, error) {
	ret, err := c.mod.ExportedFunction("amount_atoms").Call(c.ctx, uint64(wasmPtr))
	if err != nil || len(ret) < 2 {
		return Amount{}, fmt.Errorf("mintlayer: amount_atoms: %w", err)
	}
	ptr, length := uint32(ret[0]), uint32(ret[1])
	data, ok := c.mod.Memory().Read(ptr, length)
	if !ok {
		return Amount{}, fmt.Errorf("mintlayer: memory read for amount failed")
	}
	atoms := string(data)
	c.freeWASM(ptr, length)
	return NewAmount(atoms), nil
}

// writeStringArray writes a []string as an array of WASM externref table indices in
// WASM linear memory, matching the passArrayJsValueToWasm0 pattern.
// Returns (ptr, count). Call freeStringArray to release.
func (c *Client) writeStringArray(strs []string) (ptr, count uint32, err error) {
	if len(strs) == 0 {
		return 0, 0, nil
	}
	n := uint32(len(strs))

	mallocResult, callErr := c.mod.ExportedFunction("__wbindgen_malloc").Call(c.ctx, uint64(n*4), 4)
	if callErr != nil || len(mallocResult) == 0 {
		return 0, 0, fmt.Errorf("mintlayer: malloc for string array: %w", callErr)
	}
	arrPtr := uint32(mallocResult[0])

	allocFn := c.mod.ExportedFunction("__externref_table_alloc")
	if allocFn == nil {
		c.freeWASM(arrPtr, n*4)
		return 0, 0, fmt.Errorf("mintlayer: __externref_table_alloc not found")
	}

	tableIndices := make([]uint32, 0, n)
	for _, s := range strs {
		idxResult, err2 := allocFn.Call(c.ctx)
		if err2 != nil || len(idxResult) == 0 {
			// Cleanup already-allocated slots
			for _, idx := range tableIndices {
				key := getWASMTableRef(c.mod, 0, idx)
				refs.free(key)
			}
			c.freeWASM(arrPtr, n*4)
			return 0, 0, fmt.Errorf("mintlayer: __externref_table_alloc: %w", err2)
		}
		tableIdx := uint32(idxResult[0])
		tableIndices = append(tableIndices, tableIdx)

		// Store string in Go refs and wire it into the WASM table.
		key := refs.alloc(s)
		setWASMTableRef(c.mod, 0, tableIdx, key)

		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], tableIdx)
		c.mod.Memory().Write(arrPtr+uint32(len(tableIndices)-1)*4, buf[:])
	}
	return arrPtr, n, nil
}

// writeUint8ArrayArray writes a [][]byte as an array of WASM externref table indices.
// Each byte slice is first copied into WASM heap and wrapped as a uint8ArrayRef.
func (c *Client) writeUint8ArrayArray(slices [][]byte) (ptr, count uint32, err error) {
	if len(slices) == 0 {
		return 0, 0, nil
	}
	n := uint32(len(slices))

	mallocResult, callErr := c.mod.ExportedFunction("__wbindgen_malloc").Call(c.ctx, uint64(n*4), 4)
	if callErr != nil || len(mallocResult) == 0 {
		return 0, 0, fmt.Errorf("mintlayer: malloc for byte-array array: %w", callErr)
	}
	arrPtr := uint32(mallocResult[0])

	allocFn := c.mod.ExportedFunction("__externref_table_alloc")
	if allocFn == nil {
		c.freeWASM(arrPtr, n*4)
		return 0, 0, fmt.Errorf("mintlayer: __externref_table_alloc not found")
	}

	type allocation struct {
		tableIdx uint32
		wasmPtr  uint32
		wasmLen  uint32
	}
	allocs := make([]allocation, 0, n)

	for _, b := range slices {
		wasmPtr, wasmLen, err2 := c.writeBytes(b)
		if err2 != nil {
			for _, a := range allocs {
				key := getWASMTableRef(c.mod, 0, a.tableIdx)
				refs.free(key)
				c.freeWASM(a.wasmPtr, a.wasmLen)
			}
			c.freeWASM(arrPtr, n*4)
			return 0, 0, fmt.Errorf("mintlayer: write bytes for array: %w", err2)
		}

		idxResult, err2 := allocFn.Call(c.ctx)
		if err2 != nil || len(idxResult) == 0 {
			c.freeWASM(wasmPtr, wasmLen)
			for _, a := range allocs {
				key := getWASMTableRef(c.mod, 0, a.tableIdx)
				refs.free(key)
				c.freeWASM(a.wasmPtr, a.wasmLen)
			}
			c.freeWASM(arrPtr, n*4)
			return 0, 0, fmt.Errorf("mintlayer: __externref_table_alloc: %w", err2)
		}
		tableIdx := uint32(idxResult[0])

		key := refs.alloc(uint8ArrayRef{ptr: wasmPtr, len: wasmLen})
		setWASMTableRef(c.mod, 0, tableIdx, key)
		allocs = append(allocs, allocation{tableIdx, wasmPtr, wasmLen})

		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], tableIdx)
		c.mod.Memory().Write(arrPtr+uint32(len(allocs)-1)*4, buf[:])
	}
	return arrPtr, n, nil
}

// freeStringArray releases resources from writeStringArray.
func (c *Client) freeStringArray(arrPtr, count uint32) {
	if arrPtr == 0 {
		return
	}
	deallocFn := c.mod.ExportedFunction("__externref_table_dealloc")
	for i := uint32(0); i < count; i++ {
		data, ok := c.mod.Memory().Read(arrPtr+i*4, 4)
		if !ok {
			continue
		}
		tableIdx := binary.LittleEndian.Uint32(data)
		key := getWASMTableRef(c.mod, 0, tableIdx)
		refs.free(key)
		if deallocFn != nil {
			deallocFn.Call(c.ctx, uint64(tableIdx)) //nolint:errcheck
		}
	}
	c.freeWASM(arrPtr, count*4)
}

// freeUint8ArrayArray releases resources from writeUint8ArrayArray.
func (c *Client) freeUint8ArrayArray(arrPtr, count uint32) {
	if arrPtr == 0 {
		return
	}
	deallocFn := c.mod.ExportedFunction("__externref_table_dealloc")
	for i := uint32(0); i < count; i++ {
		data, ok := c.mod.Memory().Read(arrPtr+i*4, 4)
		if !ok {
			continue
		}
		tableIdx := binary.LittleEndian.Uint32(data)
		key := getWASMTableRef(c.mod, 0, tableIdx)
		if v, ok2 := refs.get(key); ok2 {
			if arr, ok3 := v.(uint8ArrayRef); ok3 {
				c.freeWASM(arr.ptr, arr.len)
			}
		}
		refs.free(key)
		if deallocFn != nil {
			deallocFn.Call(c.ctx, uint64(tableIdx)) //nolint:errcheck
		}
	}
	c.freeWASM(arrPtr, count*4)
}

// allocExternRef stores val in the Go refs registry and returns the key.
// The key can be passed directly to WASM functions that accept an externref parameter.
func (c *Client) allocExternRef(val interface{}) uintptr {
	return refs.alloc(val)
}

// freeExternRef releases a Go refs registry entry.
func (c *Client) freeExternRef(key uintptr) {
	refs.free(key)
}

// ── WASM externref table access (reflect-based workaround) ───────────────────
//
// wazero v1.8.0 has a "// TODO: Table" in its public API.
// We reach into the internal wasm.ModuleInstance.Tables[idx].References slice
// using reflect + unsafe so we can store and retrieve externref values.

// setWASMTableRef writes val into the WASM externref table at the given indices.
func setWASMTableRef(mod api.Module, tableIdx, refIdx uint32, val uintptr) {
	refsSlice, ok := wasmTableRefsSlice(mod, tableIdx)
	if !ok || int(refIdx) >= refsSlice.Len() {
		return
	}
	ptr := unsafe.Pointer(refsSlice.Pointer() + uintptr(refIdx)*unsafe.Sizeof(uintptr(0)))
	*(*uintptr)(ptr) = val
}

// getWASMTableRef reads a value from the WASM externref table.
func getWASMTableRef(mod api.Module, tableIdx, refIdx uint32) uintptr {
	refsSlice, ok := wasmTableRefsSlice(mod, tableIdx)
	if !ok || int(refIdx) >= refsSlice.Len() {
		return 0
	}
	ptr := unsafe.Pointer(refsSlice.Pointer() + uintptr(refIdx)*unsafe.Sizeof(uintptr(0)))
	return *(*uintptr)(ptr)
}

// wasmTableRefsSlice returns the reflect.Value of the References slice for a table.
func wasmTableRefsSlice(mod api.Module, tableIdx uint32) (reflect.Value, bool) {
	mv := reflect.ValueOf(mod)
	if mv.Kind() != reflect.Ptr || mv.IsNil() {
		return reflect.Value{}, false
	}
	tables := mv.Elem().FieldByName("Tables")
	if !tables.IsValid() || tables.Kind() != reflect.Slice {
		return reflect.Value{}, false
	}
	if int(tableIdx) >= tables.Len() {
		return reflect.Value{}, false
	}
	tablePtr := tables.Index(int(tableIdx))
	if tablePtr.Kind() != reflect.Ptr || tablePtr.IsNil() {
		return reflect.Value{}, false
	}
	refsField := tablePtr.Elem().FieldByName("References")
	if !refsField.IsValid() || refsField.Kind() != reflect.Slice {
		return reflect.Value{}, false
	}
	return refsField, true
}

// ── utility ───────────────────────────────────────────────────────────────────

func encodeBool(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

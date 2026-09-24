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
	"errors"
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

// errExportNotInvoked marks call errors that happened before the WASM export
// started executing (unknown name, arity mismatch). The guest cannot have
// taken ownership of any wrapper-allocated arguments, so callers may still
// discard (fully free) externRefArrays passed to the failed call. Any other
// error (domain error, trap) means the export ran — possibly partially — so
// arrays must be released (leak-safe) instead of discarded.
var errExportNotInvoked = errors.New("export was not invoked")

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
		return nil, fmt.Errorf("mintlayer: function %q not found: %w", fn, errExportNotInvoked)
	}
	if len(params) != len(f.Definition().ParamTypes()) {
		return nil, fmt.Errorf("mintlayer: function %q: got %d params, want %d: %w",
			fn, len(params), len(f.Definition().ParamTypes()), errExportNotInvoked)
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
		c.freeWASM(ptr, uint32(len(data))) // nothing handed out, wrapper still owns it
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

// externrefTableIndex is the module table holding externref values (strings
// and Uint8Array wrappers). The embedded module declares two tables: table 0
// is funcrefs (indirect calls), table 1 is the externref table used by
// wasm-bindgen's passArrayJsValueToWasm0 pattern.
const externrefTableIndex = 1

// externRefElement is one element of an externRefArray: a Go refs registry
// entry, the externref table slot it occupies, and — for byte-array elements
// only — the wrapper-owned backing buffer the guest copies from.
type externRefElement struct {
	key uintptr
	idx uint32
	buf uint8ArrayRef // zero value for string elements
}

// externRefArray is the Go-side bookkeeping for an array of externref values
// passed to a WASM export (wasm-bindgen reference-types passArrayJsValueToWasm0
// pattern).
//
// OWNERSHIP: the array buffer (ptr) is transferred to the guest when the
// export is invoked. During the call the guest converts every element,
// recycles the externref table slots, and frees the buffer itself — exactly
// like wasm-bindgen's own JS glue for owned Vec params, which never frees the
// buffer post-call. The wrapper must therefore NEVER free ptr or touch the
// table slots after the export ran (doing so is a double free that corrupts
// the dlmalloc heap). The per-element Go refs entries — and, for byte-array
// elements, the wrapper-allocated backing buffers — remain owned by the
// wrapper and are released by release.
type externRefArray struct {
	ptr   uint32 // array buffer; owned by the guest once the export is called
	count uint32
	elems []externRefElement
}

// release frees everything the wrapper still owns after an export consumed
// the array. It must be called after the export returned (success or domain
// error). It never touches the array buffer or the externref table: the guest
// consumed and freed those during the call. Idempotent.
func (a *externRefArray) release(c *Client) {
	if a.ptr == 0 && len(a.elems) == 0 {
		return // empty or already released
	}
	for _, el := range a.elems {
		c.freeWASM(el.buf.ptr, el.buf.len)
		refs.free(el.key)
	}
	a.elems = nil
	a.ptr = 0
	a.count = 0
}

// discard frees everything, including the array buffer and the externref
// table slots. Only valid on error paths where the export was never invoked:
// the guest has not recycled the slots or freed the buffer yet, so the
// wrapper still owns all of it. Idempotent.
func (a *externRefArray) discard(c *Client) {
	if a.ptr == 0 && len(a.elems) == 0 {
		return // empty or already discarded
	}
	ptr, count := a.ptr, a.count
	deallocFn := c.mod.ExportedFunction("__externref_table_dealloc")
	for _, el := range a.elems {
		if deallocFn != nil {
			deallocFn.Call(c.ctx, uint64(el.idx)) //nolint:errcheck
		}
	}
	a.release(c)
	c.freeWASM(ptr, count*4)
}

// finish cleans up arr after an export attempt returned invokeErr. If the
// export was never invoked (unknown name, arity mismatch) the wrapper still
// owns everything: the array is discarded and finish reports true so the
// caller can free any sibling wrapper-owned buffers handed to the same
// export. Otherwise (success, domain error, or trap — the guest ran, possibly
// partially) the guest owns the buffer and slots and only the wrapper-owned
// records are released.
func (a *externRefArray) finish(c *Client, invokeErr error) bool {
	if invokeErr != nil && errors.Is(invokeErr, errExportNotInvoked) {
		a.discard(c)
		return true
	}
	a.release(c)
	return false
}

// writeStringArray writes a []string as an array of WASM externref table indices in
// WASM linear memory, matching the passArrayJsValueToWasm0 pattern.
// The returned array must be released after the consuming export runs
// (release); discard it (discard) only if the export is never called.
func (c *Client) writeStringArray(strs []string) (externRefArray, error) {
	var arr externRefArray
	if len(strs) == 0 {
		return arr, nil
	}
	n := uint32(len(strs))

	mallocResult, callErr := c.mod.ExportedFunction("__wbindgen_malloc").Call(c.ctx, uint64(n*4), 4)
	if callErr != nil || len(mallocResult) == 0 {
		return arr, fmt.Errorf("mintlayer: malloc for string array: %w", callErr)
	}
	arr.ptr = uint32(mallocResult[0])
	arr.count = n

	allocFn := c.mod.ExportedFunction("__externref_table_alloc")
	if allocFn == nil {
		arr.discard(c)
		return externRefArray{}, fmt.Errorf("mintlayer: __externref_table_alloc not found")
	}

	for i, s := range strs {
		idxResult, err2 := allocFn.Call(c.ctx)
		if err2 != nil || len(idxResult) == 0 {
			arr.discard(c) // export never called: wrapper still owns everything
			return externRefArray{}, fmt.Errorf("mintlayer: __externref_table_alloc: %w", err2)
		}
		tableIdx := uint32(idxResult[0])

		// Store string in Go refs and wire it into the WASM table.
		key := refs.alloc(s)
		setWASMTableRef(c.mod, externrefTableIndex, tableIdx, key)
		arr.elems = append(arr.elems, externRefElement{key: key, idx: tableIdx})

		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], tableIdx)
		if !c.mod.Memory().Write(arr.ptr+uint32(i)*4, buf[:]) {
			arr.discard(c) // export never called: wrapper still owns everything
			return externRefArray{}, fmt.Errorf("mintlayer: write table index %d for string array: out of bounds", i)
		}
	}
	return arr, nil
}

// writeUint8ArrayArray writes a [][]byte as an array of WASM externref table indices.
// Each byte slice is first copied into WASM heap and wrapped as a uint8ArrayRef.
// The element backing buffers stay wrapper-owned (the guest only copies from
// them); see externRefArray for the ownership rules.
func (c *Client) writeUint8ArrayArray(slices [][]byte) (externRefArray, error) {
	var arr externRefArray
	if len(slices) == 0 {
		return arr, nil
	}
	n := uint32(len(slices))

	mallocResult, callErr := c.mod.ExportedFunction("__wbindgen_malloc").Call(c.ctx, uint64(n*4), 4)
	if callErr != nil || len(mallocResult) == 0 {
		return arr, fmt.Errorf("mintlayer: malloc for byte-array array: %w", callErr)
	}
	arr.ptr = uint32(mallocResult[0])
	arr.count = n

	allocFn := c.mod.ExportedFunction("__externref_table_alloc")
	if allocFn == nil {
		arr.discard(c)
		return externRefArray{}, fmt.Errorf("mintlayer: __externref_table_alloc not found")
	}

	for i, b := range slices {
		wasmPtr, wasmLen, err2 := c.writeBytes(b)
		if err2 != nil {
			arr.discard(c) // export never called: wrapper still owns everything
			return externRefArray{}, fmt.Errorf("mintlayer: write bytes for array: %w", err2)
		}

		idxResult, err2 := allocFn.Call(c.ctx)
		if err2 != nil || len(idxResult) == 0 {
			c.freeWASM(wasmPtr, wasmLen)
			arr.discard(c)
			return externRefArray{}, fmt.Errorf("mintlayer: __externref_table_alloc: %w", err2)
		}
		tableIdx := uint32(idxResult[0])

		key := refs.alloc(uint8ArrayRef{ptr: wasmPtr, len: wasmLen})
		setWASMTableRef(c.mod, externrefTableIndex, tableIdx, key)
		arr.elems = append(arr.elems, externRefElement{key: key, idx: tableIdx, buf: uint8ArrayRef{ptr: wasmPtr, len: wasmLen}})

		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], tableIdx)
		if !c.mod.Memory().Write(arr.ptr+uint32(i)*4, buf[:]) {
			arr.discard(c) // export never called: wrapper still owns everything
			return externRefArray{}, fmt.Errorf("mintlayer: write table index %d for byte-array array: out of bounds", i)
		}
	}
	return arr, nil
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

// setWASMTableRef writes val into the WASM table at the given index.
func setWASMTableRef(mod api.Module, tableIdx, refIdx uint32, val uintptr) {
	refsSlice, ok := wasmTableRefsSlice(mod, tableIdx)
	if !ok || int(refIdx) >= refsSlice.Len() {
		return
	}
	refs := unsafe.Slice((*uintptr)(refsSlice.UnsafePointer()), refsSlice.Len())
	refs[refIdx] = val
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

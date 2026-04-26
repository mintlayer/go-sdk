// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// hostModule is the module name the WASM imports all host functions from.
const hostModule = "./wasm_wrappers_bg.js"

// Sentinel types used as externref values in the global ref registry.
type (
	globalSentinel   struct{} // represents "globalThis"
	cryptoSentinel   struct{} // represents "crypto"
	uint8ArrayRef    struct{ ptr, len uint32 }
	functionSentinel struct{ code string }
)

// ── per-call context ──────────────────────────────────────────────────────────

type callCtxKey struct{}

type callCtx struct {
	errMsg         string          // set by __wbindgen_throw
	lastJSONResult json.RawMessage // set by __wbg_parse when WASM returns a JS object
}

func ctxWithCall(parent context.Context) context.Context {
	return context.WithValue(parent, callCtxKey{}, &callCtx{})
}

func getCallCtx(ctx context.Context) *callCtx {
	v, _ := ctx.Value(callCtxKey{}).(*callCtx)
	return v
}

// ── value type shorthands ─────────────────────────────────────────────────────

var (
	i32 = api.ValueTypeI32
	ext = api.ValueTypeExternref
)

func vt(types ...api.ValueType) []api.ValueType { return types }

// ── host module registration ──────────────────────────────────────────────────

// registerHostModule creates and instantiates the host module for WASM imports.
func registerHostModule(ctx context.Context, rt wazero.Runtime) error {
	b := rt.NewHostModuleBuilder(hostModule)

	// ── global accessors (no params → i32 table index) ───────────────────────
	//
	// These functions return a WASM externref table index (i32), not an externref
	// directly. The table index is later retrieved via table.get to obtain the
	// actual externref value.

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module) uint32 {
		idx, err := allocTableSlot(ctx, m, refGlobal)
		if err != nil {
			return 0
		}
		return idx
	}).Export("__wbg_static_accessor_GLOBAL_12837167ad935116")

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module) uint32 {
		idx, err := allocTableSlot(ctx, m, refGlobal)
		if err != nil {
			return 0
		}
		return idx
	}).Export("__wbg_static_accessor_GLOBAL_THIS_e628e89ab3b1c95f")

	b.NewFunctionBuilder().WithFunc(func(_ context.Context, _ api.Module) uint32 {
		return 0 // no "self" in Go → null
	}).Export("__wbg_static_accessor_SELF_a621d3dfbb60d0ce")

	b.NewFunctionBuilder().WithFunc(func(_ context.Context, _ api.Module) uint32 {
		return 0 // no "window" in Go → null
	}).Export("__wbg_static_accessor_WINDOW_f8727f0cf888e0bd")

	// ── crypto: (externref) → externref ──────────────────────────────────────

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		if v, ok := refs.get(uintptr(stack[0])); ok {
			if _, ok2 := v.(globalSentinel); ok2 {
				stack[0] = uint64(refCrypto)
				return
			}
		}
		stack[0] = 0
	}), vt(ext), vt(ext)).Export("__wbg_crypto_86f2631e91b51511")

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		stack[0] = 0 // no msCrypto
	}), vt(ext), vt(ext)).Export("__wbg_msCrypto_d562bbe83e0d4b91")

	// ── getRandomValues / randomFillSync: (externref, externref) → void ──────

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		fillRandomFromRef(ctx, m, uintptr(stack[1]))
	}), vt(ext, ext), vt()).Export("__wbg_getRandomValues_b3f15fcbfabb0f8b")

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		fillRandomFromRef(ctx, m, uintptr(stack[1]))
	}), vt(ext, ext), vt()).Export("__wbg_randomFillSync_f8c153b79f285817")

	// ── Node.js stubs: (externref) → externref ────────────────────────────────

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		stack[0] = 0 // no .process
	}), vt(ext), vt(ext)).Export("__wbg_process_3975fd6c72f520aa")

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		stack[0] = 0 // no .node
	}), vt(ext), vt(ext)).Export("__wbg_node_e1f24f89a7336c2e")

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		stack[0] = 0 // no .versions
	}), vt(ext), vt(ext)).Export("__wbg_versions_4e31226f5e8dc909")

	// ── require: () → externref ───────────────────────────────────────────────

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		fmt.Println("[DEBUG] require called")
		stack[0] = 0 // return null; is_function(null)==false so WASM skips Node.js path
	}), vt(), vt(ext)).Export("__wbg_require_b74f47fc2d022fd6")

	// ── function call stubs: (externref, externref[, externref]) → externref ─

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		fnKey := uintptr(stack[0])
		if v, ok := refs.get(fnKey); ok {
			if fn, ok2 := v.(functionSentinel); ok2 {
				switch fn.code {
				case "return this":
					// new Function("return this").call(null) → returns globalThis
					stack[0] = uint64(refGlobal)
					return
				}
			}
		}
		signalException(ctx, m, "unsupported call/2")
		stack[0] = 0
	}), vt(ext, ext), vt(ext)).Export("__wbg_call_389efe28435a9388")

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		signalException(ctx, m, "unsupported call (one arg)")
		stack[0] = 0
	}), vt(ext, ext, ext), vt(ext)).Export("__wbg_call_4708e0c13bdc8e95")

	// ── Uint8Array operations ─────────────────────────────────────────────────

	// length(externref) → i32
	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		if u, ok := refs.get(uintptr(stack[0])); ok {
			if arr, ok2 := u.(uint8ArrayRef); ok2 {
				stack[0] = uint64(arr.len)
				return
			}
		}
		stack[0] = 0
	}), vt(ext), vt(i32)).Export("__wbg_length_32ed9a279acd054c")

	// new Uint8Array(n) → externref
	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		size := uint32(stack[0])
		result, err := m.ExportedFunction("__wbindgen_malloc").Call(ctx, uint64(size), 1)
		if err != nil || len(result) == 0 {
			stack[0] = 0
			return
		}
		ptr := uint32(result[0])
		m.Memory().Write(ptr, make([]byte, size))
		stack[0] = uint64(refs.alloc(uint8ArrayRef{ptr: ptr, len: size}))
	}), vt(i32), vt(ext)).Export("__wbg_new_with_length_a2c39cbe88fd8ff1")

	// Uint8Array.prototype.set.call(dst[ptr,len], src[externref]) → void
	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		dstPtr := uint32(stack[0])
		dstLen := uint32(stack[1])
		src := uintptr(stack[2])
		if u, ok := refs.get(src); ok {
			if arr, ok2 := u.(uint8ArrayRef); ok2 {
				n := arr.len
				if n > dstLen {
					n = dstLen
				}
				if data, ok3 := m.Memory().Read(arr.ptr, n); ok3 {
					m.Memory().Write(dstPtr, data)
				}
			}
		}
	}), vt(i32, i32, ext), vt()).Export("__wbg_prototypesetcall_bdcdcc5842e4d77d")

	// buf.subarray(start, end) → externref
	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		src := uintptr(stack[0])
		start := uint32(stack[1])
		end := uint32(stack[2])
		if u, ok := refs.get(src); ok {
			if arr, ok2 := u.(uint8ArrayRef); ok2 {
				stack[0] = uint64(refs.alloc(uint8ArrayRef{ptr: arr.ptr + start, len: end - start}))
				return
			}
		}
		stack[0] = 0
	}), vt(ext, i32, i32), vt(ext)).Export("__wbg_subarray_a96e1fef17ed23cb")

	// ── JSON: (i32, i32) → externref ─────────────────────────────────────────

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		ptr := uint32(stack[0])
		length := uint32(stack[1])
		data, _ := m.Memory().Read(ptr, length)
		var v interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			signalException(ctx, m, "JSON.parse: "+err.Error())
			stack[0] = 0
			return
		}
		if cc := getCallCtx(ctx); cc != nil {
			cc.lastJSONResult = json.RawMessage(data)
		}
		stack[0] = uint64(refs.alloc(v))
	}), vt(i32, i32), vt(ext)).Export("__wbg_parse_708461a1feddfb38")

	// JSON.stringify: (externref) → externref
	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		v, ok := refs.get(uintptr(stack[0]))
		if !ok {
			signalException(ctx, m, "JSON.stringify: unknown reference")
			stack[0] = 0
			return
		}
		data, err := json.Marshal(v)
		if err != nil {
			signalException(ctx, m, "JSON.stringify: "+err.Error())
			stack[0] = 0
			return
		}
		stack[0] = uint64(refs.alloc(string(data)))
	}), vt(ext), vt(ext)).Export("__wbg_stringify_8d1cc6ff383e8bae")

	// ── new Function(code): (i32, i32) → externref ───────────────────────────

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		ptr := uint32(stack[0])
		length := uint32(stack[1])
		data, _ := m.Memory().Read(ptr, length)
		key := refs.alloc(functionSentinel{code: string(data)})
		stack[0] = uint64(key)
	}), vt(i32, i32), vt(ext)).Export("__wbg_new_no_args_1c7c842f08d00ebb")

	// ── cast intrinsics: (i32, i32) → externref ──────────────────────────────

	// (ptr, len) → Uint8Array externref
	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		stack[0] = uint64(refs.alloc(uint8ArrayRef{ptr: uint32(stack[0]), len: uint32(stack[1])}))
	}), vt(i32, i32), vt(ext)).Export("__wbindgen_cast_0000000000000001")

	// (ptr, len) → string externref — also captures last error string
	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		ptr := uint32(stack[0])
		length := uint32(stack[1])
		data, _ := m.Memory().Read(ptr, length)
		s := string(data)
		if cc := getCallCtx(ctx); cc != nil {
			cc.errMsg = s
		}
		stack[0] = uint64(refs.alloc(s))
	}), vt(i32, i32), vt(ext)).Export("__wbindgen_cast_0000000000000002")

	// ── debug / type predicates: (externref) → i32 ───────────────────────────

	// debug_string: (i32, externref) → void
	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		outPtr := uint32(stack[0])
		s := refDebugString(uintptr(stack[1]))
		writeStringToMem(ctx, m, outPtr, s)
	}), vt(i32, ext), vt()).Export("__wbg___wbindgen_debug_string_0bc8482c6e3508ae")

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		val := uintptr(stack[0])
		if v, ok := refs.get(val); ok {
			if _, ok2 := v.(functionSentinel); ok2 {
				stack[0] = 1
				return
			}
		}
		stack[0] = 0
	}), vt(ext), vt(i32)).Export("__wbg___wbindgen_is_function_0095a73b8b156f76")

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		val := uintptr(stack[0])
		if val == 0 {
			stack[0] = 0
			return
		}
		if v, ok := refs.get(val); ok {
			switch v.(type) {
			case globalSentinel, cryptoSentinel, uint8ArrayRef:
				stack[0] = 1
				return
			}
		}
		stack[0] = 0
	}), vt(ext), vt(i32)).Export("__wbg___wbindgen_is_object_5ae8e5880f2c1fbd")

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		val := uintptr(stack[0])
		if v, ok := refs.get(val); ok {
			if _, ok2 := v.(string); ok2 {
				stack[0] = 1
				return
			}
		}
		stack[0] = 0
	}), vt(ext), vt(i32)).Export("__wbg___wbindgen_is_string_cd444516edc5b180")

	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		if stack[0] == 0 {
			stack[0] = 1
		} else {
			stack[0] = 0
		}
	}), vt(ext), vt(i32)).Export("__wbg___wbindgen_is_undefined_9e4d92534c42d778")

	// string_get: (i32, externref) → void
	b.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
		outPtr := uint32(stack[0])
		val := uintptr(stack[1])
		if v, ok := refs.get(val); ok {
			if s, ok2 := v.(string); ok2 {
				writeStringToMem(ctx, m, outPtr, s)
				return
			}
		}
		m.Memory().WriteUint32Le(outPtr, 0)
		m.Memory().WriteUint32Le(outPtr+4, 0)
	}), vt(i32, ext), vt()).Export("__wbg___wbindgen_string_get_72fb696202c56729")

	// ── throw (pure i32) ─────────────────────────────────────────────────────

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, length uint32) {
		data, _ := m.Memory().Read(ptr, length)
		msg := string(data)
		if cc := getCallCtx(ctx); cc != nil {
			cc.errMsg = msg
		}
		panic(fmt.Sprintf("wasm throw: %s", msg))
	}).Export("__wbg___wbindgen_throw_be289d5034ed271b")

	// ── externref table init (pure void) ────────────────────────────────────

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module) {
		allocFn := m.ExportedFunction("__externref_table_alloc")
		if allocFn == nil {
			return
		}
		for i := 0; i < 4; i++ {
			allocFn.Call(ctx) //nolint:errcheck
		}
	}).Export("__wbindgen_init_externref_table")

	_, err := b.Instantiate(ctx)
	return err
}

// ── internal helpers ──────────────────────────────────────────────────────────

// allocTableSlot allocates a slot in the WASM externref table, stores key in
// the slot, and returns the slot index.  Used by host functions that must
// return an i32 table index (e.g. GLOBAL_THIS, GLOBAL accessors).
func allocTableSlot(ctx context.Context, m api.Module, key uintptr) (uint32, error) {
	allocFn := m.ExportedFunction("__externref_table_alloc")
	if allocFn == nil {
		return 0, fmt.Errorf("mintlayer: __externref_table_alloc not found (mod=%T)", m)
	}
	result, err := allocFn.Call(ctx)
	if err != nil || len(result) == 0 {
		return 0, fmt.Errorf("mintlayer: __externref_table_alloc failed: %w", err)
	}
	idx := uint32(result[0])
	setWASMTableRef(m, 0, idx, key)
	return idx, nil
}

// fillRandomFromRef fills a Uint8Array externref with random bytes.
func fillRandomFromRef(ctx context.Context, m api.Module, buf uintptr) {
	v, ok := refs.get(buf)
	if !ok {
		signalException(ctx, m, "fillRandom: unknown buffer reference")
		return
	}
	u, ok := v.(uint8ArrayRef)
	if !ok {
		signalException(ctx, m, "fillRandom: expected Uint8Array")
		return
	}
	data := make([]byte, u.len)
	if _, err := rand.Read(data); err != nil {
		signalException(ctx, m, "fillRandom: "+err.Error())
		return
	}
	m.Memory().Write(u.ptr, data)
}

// signalException allocates an externref table slot and calls __wbindgen_exn_store
// so the WASM runtime knows a host function threw.
func signalException(ctx context.Context, m api.Module, msg string) {
	allocFn := m.ExportedFunction("__externref_table_alloc")
	storeFn := m.ExportedFunction("__wbindgen_exn_store")
	if allocFn == nil || storeFn == nil {
		return
	}
	idxResult, err := allocFn.Call(ctx)
	if err != nil || len(idxResult) == 0 {
		return
	}
	storeFn.Call(ctx, idxResult[0]) //nolint:errcheck
}

// writeStringToMem allocates a string in WASM heap and writes (ptr, len) as
// two little-endian i32s at WASM address outPtr.
func writeStringToMem(ctx context.Context, m api.Module, outPtr uint32, s string) {
	data := []byte(s)
	mallocFn := m.ExportedFunction("__wbindgen_malloc")
	if mallocFn == nil || len(data) == 0 {
		m.Memory().WriteUint32Le(outPtr, 0)
		m.Memory().WriteUint32Le(outPtr+4, 0)
		return
	}
	result, err := mallocFn.Call(ctx, uint64(len(data)), 1)
	if err != nil || len(result) == 0 {
		m.Memory().WriteUint32Le(outPtr, 0)
		m.Memory().WriteUint32Le(outPtr+4, 0)
		return
	}
	ptr := uint32(result[0])
	m.Memory().Write(ptr, data)
	m.Memory().WriteUint32Le(outPtr, ptr)
	m.Memory().WriteUint32Le(outPtr+4, uint32(len(data)))
}

// refDebugString returns a human-readable description for a ref key.
func refDebugString(key uintptr) string {
	if key == 0 {
		return "undefined"
	}
	v, ok := refs.get(key)
	if !ok {
		return "unknown"
	}
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case uint8ArrayRef:
		return fmt.Sprintf("Uint8Array(%d)", val.len)
	case globalSentinel:
		return "[object global]"
	case cryptoSentinel:
		return "[object Crypto]"
	case functionSentinel:
		return fmt.Sprintf("function %s", val.code)
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

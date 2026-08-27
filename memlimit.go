package gojq

import (
	"encoding/json"
	"math/big"
	"reflect"
)

// MaxAlloc bounds the total number of bytes a single query execution may
// allocate for its values. Zero, the default, disables the limit and keeps
// the original behaviour. Set it once before running queries.
var MaxAlloc int64

type allocLimitError struct{}

func (*allocLimitError) Error() string {
	return "value allocation exceeds the configured memory limit"
}

// maxRecursionDepth caps the Go-recursive builtins ( compare, contains, encode,
// flatten, deleteEmpty, update, deepmerge, and the JSON decoder ) independently
// of MaxAlloc. Their per-level depth bounds scale with MaxAlloc, so a very large
// MaxAlloc plus a deeply nested input would let the recursion overflow the
// goroutine stack ( a fatal, uncatchable crash ). This hard cap converts that
// into a clean allocation error at any MaxAlloc. At MaxAlloc <= ~4 MiB the
// per-level bound ( MaxAlloc/16 ) is the binding one, so this never changes
// behavior there; it only fires far past any depth a real input reaches.
const maxRecursionDepth = 1 << 19

// allocSize is a cheap, non-recursive estimate of a value's byte size,
// used only by the allocation meter.
func allocSize(v any) int64 {
	switch t := v.(type) {
	case string:
		return int64(len(t)) + 16 // string header (ptr + len)
	case []any:
		return int64(len(t))*16 + 16
	case map[string]any:
		// A Go map has a ~320-byte minimum ( hmap header + a full first
		// bucket ) that len*24 missed entirely, so a chain or collection of
		// tiny 1-key maps ran ~6x over the limit. Charge that base plus ~40
		// per entry ( bucket slot + copied key header ); key string content is
		// still charged separately where the key is built.
		return int64(len(t))*40 + 320
	case *big.Int:
		return int64(len(t.Bits()))*8 + 16
	case json.Number:
		return int64(len(t)) + 16 // a numeric string, like string
	default:
		// scalars (int, float64, bool, nil) are tiny and transient - they
		// cannot grow a run out of memory, so the gas meter does not count
		// them. this keeps a streaming query that produces many scalars, such
		// as `reduce range(N) as $x (0; . + 1)`, from tripping the limit while
		// holding almost nothing live. retained scalars in an array still cost
		// their 16-byte slot, charged at opappend.
		return 0
	}
}

// chargeFreshResult charges all storage a native callback freshly allocated for
// result. Containers reachable from input or args are borrowed by identity and
// pruned, so copy-on-write spines are charged while their shared branches remain
// free. The iterative walk handles arbitrary depth and shared-reference DAGs;
// every native callback, including future and custom callbacks, inherits it at
// the single opcall return boundary.
func (env *env) chargeFreshResult(result, input any, args []any) bool {
	if MaxAlloc <= 0 {
		return false
	}
	switch result.(type) {
	case []any, map[string]any:
		// Compound results need the ownership walk below.
	default:
		// Scalars cost zero; strings, numbers, and big integers are shallow
		// leaves. Keep this common path allocation-free.
		return env.charge(result)
	}
	// An allocator-backed update can return the same container it received and
	// mutate below that root. Avoid an O(size(input)) seed on every such update,
	// but retain the old shallow charge rather than treating the alias as free.
	if addr, _ := containerAddr(result); addr != 0 {
		if inputAddr, _ := containerAddr(input); addr == inputAddr {
			return env.charge(result)
		}
		for _, arg := range args {
			if argAddr, _ := containerAddr(arg); addr == argAddr {
				return env.charge(result)
			}
		}
	}
	seen := map[uintptr]struct{}{}
	markContainers(input, seen)
	for _, arg := range args {
		markContainers(arg, seen)
	}
	stack := []any{result}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch x := x.(type) {
		case []any:
			addr := reflect.ValueOf(x).Pointer()
			if _, ok := seen[addr]; ok {
				continue
			}
			seen[addr] = struct{}{}
			if env.chargeBytes(allocSize(x)) {
				return true
			}
			stack = append(stack, x...)
		case map[string]any:
			addr := reflect.ValueOf(x).Pointer()
			if _, ok := seen[addr]; ok {
				continue
			}
			seen[addr] = struct{}{}
			if env.chargeBytes(allocSize(x)) {
				return true
			}
			for key, value := range x {
				if env.chargeBytes(int64(len(key)) + 16) {
					return true
				}
				stack = append(stack, value)
			}
		default:
			if env.charge(x) {
				return true
			}
		}
	}
	return false
}

func containerAddr(v any) (uintptr, bool) {
	switch v := v.(type) {
	case []any:
		return reflect.ValueOf(v).Pointer(), true
	case map[string]any:
		return reflect.ValueOf(v).Pointer(), true
	default:
		return 0, false
	}
}

// markContainers records all container identities reachable from v. A result
// node with one of these identities existed before the callback, so its whole
// subtree is shared input rather than newly allocated output.
func markContainers(v any, seen map[uintptr]struct{}) {
	stack := []any{v}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch x := x.(type) {
		case []any:
			addr := reflect.ValueOf(x).Pointer()
			if _, ok := seen[addr]; ok {
				continue
			}
			seen[addr] = struct{}{}
			stack = append(stack, x...)
		case map[string]any:
			addr := reflect.ValueOf(x).Pointer()
			if _, ok := seen[addr]; ok {
				continue
			}
			seen[addr] = struct{}{}
			for _, value := range x {
				stack = append(stack, value)
			}
		}
	}
}

// chargeBytes adds n to the running total and reports whether the limit has
// now been exceeded.
func (env *env) chargeBytes(n int64) bool {
	if MaxAlloc <= 0 {
		return false
	}
	env.alloc += n
	return env.alloc > MaxAlloc
}

// charge adds v's size to the running total and reports whether the limit
// has now been exceeded.
func (env *env) charge(v any) bool {
	return env.chargeBytes(allocSize(v))
}

// arrayTooLarge reports whether a []any of n elements would exceed MaxAlloc.
// Used to pre-check builtins that allocate an input-proportional array in one make().
func arrayTooLarge(n int) bool {
	return MaxAlloc > 0 && int64(n)*16 > MaxAlloc
}

// arrayAppendTooLarge reports whether growing a []any to n elements can exceed
// MaxAlloc while append temporarily retains both its old and new backing arrays.
// For large slices Go grows capacity by about 25%, so the old n-slot backing
// array and new ~1.25n-slot array need about 36 bytes per eventual element at
// the growth point. Single-shot makes use arrayTooLarge because they do not have
// an old backing array.
func arrayAppendTooLarge(n int) bool {
	return MaxAlloc > 0 && int64(n) > MaxAlloc/36
}

// implodeWorkingSetTooLarge bounds the live input array plus the old and new
// UTF-8 builder buffers that can coexist while encoding up to four bytes per
// rune. The same conservative 36-byte factor also includes slice/buffer growth
// slack that is not visible in the retained result size.
func implodeWorkingSetTooLarge(n int) bool {
	return MaxAlloc > 0 && int64(n) > MaxAlloc/36
}

// overStackLimit reports whether the interpreter's own live stacks exceed
// MaxAlloc. Deep or infinite recursion, such as `def f: [f]; f`, grows these
// stacks without ever completing a value, so the value meter never charges it.
// The per-block sizes match the interpreter structs (stack/paths block, scope
// block, fork).
func (env *env) overStackLimit() bool {
	return int64(len(env.stack.data))*24+
		int64(len(env.paths.data))*24+
		int64(len(env.scopes.data))*56+
		int64(len(env.values))*16+
		int64(len(env.forks))*80 > MaxAlloc
}

// decodeJSONLimited decodes one JSON value from dec, tracking the cumulative
// shallow size of the structure it builds and erroring once that size passes
// MaxAlloc. It mirrors json.Decoder.Decode under UseNumber, so a huge document
// cannot be materialized in one shot before the value meter would see it.
// Sizes match allocSize: 16 bytes per array slot, 40 per object entry (24 map
// bucket + a 16-byte string header for the key), plus each value's own footprint.
func decodeJSONLimited(dec *json.Decoder, size *int64, depth int) (any, error) {
	if MaxAlloc > 0 && depth > maxRecursionDepth {
		return nil, &allocLimitError{}
	}
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := t.(type) {
	case json.Delim:
		switch t {
		case '[':
			arr := []any{}
			for dec.More() {
				if *size += 16; *size > MaxAlloc {
					return nil, &allocLimitError{}
				}
				v, err := decodeJSONLimited(dec, size, depth+1)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			_, err := dec.Token() // consume ]
			return arr, err
		case '{':
			obj := map[string]any{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k := key.(string)
				if *size += int64(len(k)) + 40; *size > MaxAlloc {
					return nil, &allocLimitError{}
				}
				v, err := decodeJSONLimited(dec, size, depth+1)
				if err != nil {
					return nil, err
				}
				obj[k] = v
			}
			_, err := dec.Token() // consume }
			return obj, err
		default:
			return nil, &json.SyntaxError{}
		}
	case string:
		*size += int64(len(t)) + 16
	case json.Number:
		*size += int64(len(t)) + 16
	default: // bool, nil
		*size += 16
	}
	if *size > MaxAlloc {
		return nil, &allocLimitError{}
	}
	return t, nil
}

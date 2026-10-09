package gojq

import (
	"strings"
	"sync"
)

var (
	formatRegistryMu sync.RWMutex
	formatRegistry   = make(map[string]func(any) any)
)

// RegisterFormat registers a custom formatting or escape filter function globally.
// The name can be specified with or without a leading "@".
func RegisterFormat(name string, f func(any) any) {
	if strings.HasPrefix(name, "@") {
		name = name[1:]
	}
	if name == "" {
		panic("empty format name")
	}
	if f == nil {
		panic("nil format function")
	}
	formatRegistryMu.Lock()
	defer formatRegistryMu.Unlock()
	formatRegistry[name] = f
}

// RegisterEscaper is an alias of [RegisterFormat] for registering an escape filter function.
func RegisterEscaper(name string, f func(any) any) {
	RegisterFormat(name, f)
}

// UnregisterFormat removes a custom formatting or escape filter function from the global registry.
func UnregisterFormat(name string) {
	if strings.HasPrefix(name, "@") {
		name = name[1:]
	}
	formatRegistryMu.Lock()
	defer formatRegistryMu.Unlock()
	delete(formatRegistry, name)
}

// UnregisterEscaper is an alias of [UnregisterFormat].
func UnregisterEscaper(name string) {
	UnregisterFormat(name)
}

func getFormat(name string) func(any) any {
	if strings.HasPrefix(name, "@") {
		name = name[1:]
	}
	formatRegistryMu.RLock()
	defer formatRegistryMu.RUnlock()
	return formatRegistry[name]
}

// WithFormat is a compiler option for adding a custom format filter function.
// The name can be specified with or without a leading "@".
func WithFormat(name string, f func(any) any) CompilerOption {
	if strings.HasPrefix(name, "@") {
		name = name[1:]
	}
	if name == "" {
		panic("empty format name")
	}
	if f == nil {
		panic("nil format function")
	}
	return func(c *compiler) {
		if c.customFormats == nil {
			c.customFormats = make(map[string]func(any) any)
		}
		c.customFormats[name] = f
		withFunction(name, 0, 0, false, func(v any, _ []any) any {
			return f(v)
		})(c)
	}
}

// WithEscaper is an alias of [WithFormat] for adding a custom escape filter function.
func WithEscaper(name string, f func(any) any) CompilerOption {
	return WithFormat(name, f)
}

package gojq

// defaultMaxStackDepth bounds each live interpreter stack to 4096 entries.
// On a 64-bit build, the operand/path blocks, scope blocks, forks, and value
// slots are 24, 24, 56, 80, and 16 bytes respectively. Even allowing nearly
// 2x slice-capacity growth, their combined backing storage stays below 2 MiB.
const defaultMaxStackDepth = 1 << 12

// MaxStackDepth bounds the interpreter stacks and logical user-function call
// depth of each metered query execution. The value is captured when a run
// starts, so stack state and enforcement remain per-run. Set it once before
// running queries; values below one reject every query. For compatibility with
// gojq's historically unlimited mode, the default is enforced when MaxAlloc is
// enabled; explicitly changing MaxStackDepth also enables it without MaxAlloc.
var MaxStackDepth = defaultMaxStackDepth

func configuredStackDepth() int {
	if MaxAlloc <= 0 && MaxStackDepth == defaultMaxStackDepth {
		return int(^uint(0) >> 1)
	}
	return MaxStackDepth
}

type stackLimitError struct{}

func (*stackLimitError) Error() string {
	return "interpreter stack exceeds the configured depth limit"
}

// checkStackDepth is called before growing an interpreter stack. Keeping the
// configured limit on env makes the hot path one per-run integer comparison.
func (env *env) checkStackDepth(depth int) {
	if depth > env.maxStackDepth {
		panic(&stackLimitError{})
	}
}

func nextScopeDepth(s *scopeStack) int {
	return max(s.index, s.limit) + 2
}

package gojq

type code struct {
	op opcode
	v  interface{}
}

type opcode int

const (
	opnop opcode = iota
	oppush
	oppop
	opdup
	opswap
	opconst
	opload
	opstore
	opappend
	opfork
	opforkopt
	opbacktrack
	opjump
	opjumppop
	opjumpifnot
	opret
	opcall
	opscope
	opeach
)

func (op opcode) String() string {
	switch op {
	case opnop:
		return "nop"
	case oppush:
		return "push"
	case oppop:
		return "pop"
	case opdup:
		return "dup"
	case opswap:
		return "swap"
	case opconst:
		return "const"
	case opload:
		return "load"
	case opstore:
		return "store"
	case opappend:
		return "append"
	case opfork:
		return "fork"
	case opforkopt:
		return "forkopt"
	case opbacktrack:
		return "backtrack"
	case opjump:
		return "jump"
	case opjumppop:
		return "jumppop"
	case opjumpifnot:
		return "jumpifnot"
	case opret:
		return "ret"
	case opcall:
		return "call"
	case opscope:
		return "scope"
	case opeach:
		return "each"
	default:
		panic(op)
	}
}

package gojq

type code struct {
	op opcode
	v  interface{}
}

type opcode int

const (
	oppush opcode = iota
	oppop
	opdup
	opswap
	opconst
	opload
	opstore
	opfork
	opbacktrack
	opjump
	opjumpifnot
	opret
	oparray
)

func (op opcode) String() string {
	switch op {
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
	case opfork:
		return "fork"
	case opbacktrack:
		return "backtrack"
	case opjump:
		return "jump"
	case opjumpifnot:
		return "jumpifnot"
	case opret:
		return "ret"
	case oparray:
		return "array"
	default:
		panic(op)
	}
}

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
	oplt
	opincr
	opload
	opstore
	opfork
	opbacktrack
	opjump
	opjumppop
	opjumpifnot
	opret
	opcall
	oparray
	opindex
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
	case oplt:
		return "lt"
	case opincr:
		return "incr"
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
	case opjumppop:
		return "jumppop"
	case opjumpifnot:
		return "jumpifnot"
	case opret:
		return "ret"
	case opcall:
		return "call"
	case oparray:
		return "array"
	case opindex:
		return "index"
	default:
		panic(op)
	}
}

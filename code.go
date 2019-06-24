package gojq

type code struct {
	op opcode
	v  interface{}
}

type opcode int

const (
	opload opcode = iota
	oppop
	opswap
	opconst
	opfork
	opjump
	opret
	oparray
)

func (op opcode) String() string {
	switch op {
	case opload:
		return "load"
	case oppop:
		return "pop"
	case opswap:
		return "swap"
	case opconst:
		return "const"
	case opfork:
		return "fork"
	case opjump:
		return "jump"
	case opret:
		return "ret"
	case oparray:
		return "array"
	default:
		panic(op)
	}
}

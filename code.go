package gojq

type code struct {
	op opcode
	v  interface{}
}

type opcode int

const (
	opload opcode = iota
	opconst
	opfork
	opjump
	opret
)

func (op opcode) String() string {
	switch op {
	case opload:
		return "load"
	case opconst:
		return "const"
	case opfork:
		return "fork"
	case opjump:
		return "jump"
	case opret:
		return "ret"
	default:
		panic(op)
	}
}

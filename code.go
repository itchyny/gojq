package gojq

type code struct {
	op opcode
	v  interface{}
}

type opcode int

const (
	opload opcode = iota
	opconst
	opret
)

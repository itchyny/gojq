package gojq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	debug    bool
	debugOut io.Writer
)

func init() {
	if out := os.Getenv("GOJQ_DEBUG"); out != "" {
		debug = true
		if out == "stdout" {
			debugOut = os.Stdout
		} else {
			debugOut = os.Stderr
		}
	}
}

func (env *env) debugCodes() {
	if !debug {
		return
	}
	for i, c := range env.codes {
		fmt.Fprintf(debugOut, "\t%d\t%s\t%s\n", i, formatOp(c.op), debugJSON(c.v))
	}
	fmt.Fprintln(debugOut, "\t"+strings.Repeat("-", 20))
}

func (env *env) debugState(pc int) {
	if !debug {
		return
	}
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "\t%d\t%s\t", pc, formatOp(env.codes[pc].op))
	buf.WriteString(debugJSON(env.codes[pc].v))
	buf.WriteString("\t|")
	for _, v := range env.stack {
		buf.WriteString("\t")
		buf.WriteString(debugJSON(v))
	}
	fmt.Fprintln(debugOut, buf.String())
}

func formatOp(c opcode) string {
	return c.String() + strings.Repeat(" ", 15-len(c.String()))
}

func (env *env) debugForks(pc int, op string) {
	buf := new(bytes.Buffer)
	for i, v := range env.forks {
		if i > 0 {
			buf.WriteByte('\t')
		}
		if i == len(env.forks)-1 {
			buf.WriteByte('<')
		}
		fmt.Fprintf(buf, "%d, %s", v.pc, debugJSON(v.v))
		if i == len(env.forks)-1 {
			buf.WriteByte('>')
		}
	}
	fmt.Fprintf(debugOut, "\t-\t%s            \t%d\t|\t%s\n", op, pc, buf.String())
}

func debugJSON(v interface{}) string {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(v)
	return strings.TrimSpace(b.String())
}

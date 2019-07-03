// +build debug

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

func (c *compiler) appendCodeInfo(name string) {
	if !debug {
		return
	}
	var prefix string
	var diff int
	if c.codes[len(c.codes)-1].op == opret {
		prefix = "end of "
		diff = -1
	}
	c.codeinfos = append(c.codeinfos, &codeinfo{prefix + name, c.pc() + diff})
}

func (env *env) lookupFuncName(pc int) string {
	for _, ci := range env.codeinfos {
		if ci.pc == pc {
			return ci.name
		}
	}
	return ""
}

func (env *env) debugCodes() {
	if !debug {
		return
	}
	for i, c := range env.codes {
		pc := i
		if c.op == opcall {
			xs := c.v.([2]interface{})
			if x, ok := xs[0].(int); ok {
				pc = x + 1
			}
		}
		var s string
		if name := env.lookupFuncName(pc); name != "" {
			if c.op == opcall {
				s = "\t## call " + name
			} else {
				s = "\t## " + name
			}
		}
		fmt.Fprintf(debugOut, "\t%d\t%s\t%s%s\n", i, formatOp(c.op), debugJSON(c.v), s)
	}
	fmt.Fprintln(debugOut, "\t"+strings.Repeat("-", 20))
}

func (env *env) debugState(pc int) {
	if !debug {
		return
	}
	buf := new(bytes.Buffer)
	c := env.codes[pc]
	fmt.Fprintf(buf, "\t%d\t%s\t", pc, formatOp(c.op))
	buf.WriteString(debugJSON(c.v))
	buf.WriteString("\t|")
	for _, v := range env.stack {
		buf.WriteString("\t")
		buf.WriteString(debugJSON(v))
	}
	if c.op == opcall {
		xs := c.v.([2]interface{})
		if x, ok := xs[0].(int); ok {
			pc = x + 1
		}
	}
	if name := env.lookupFuncName(pc); name != "" {
		if c.op == opcall {
			buf.WriteString("\t\t\t## call " + name)
		} else {
			buf.WriteString("\t\t\t## " + name)
		}
	}
	fmt.Fprintln(debugOut, buf.String())
}

func formatOp(c opcode) string {
	return c.String() + strings.Repeat(" ", 15-len(c.String()))
}

func (env *env) debugForks(pc int, op string) {
	if !debug {
		return
	}
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

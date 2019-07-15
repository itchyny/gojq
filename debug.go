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

func (c *compiler) appendCodeInfo(x interface{}) {
	var name string
	switch x := x.(type) {
	case string:
		name = x
	default:
		name = fmt.Sprint(x)
	}
	if !debug {
		return
	}
	var prefix string
	var diff int
	if c.codes[len(c.codes)-1].op == opret {
		prefix = "end of "
		diff = -1
	}
	c.codeinfos = append(c.codeinfos, codeinfo{prefix + name, c.pc() + diff})
}

func (c *compiler) deleteCodeInfo(name string) {
	for i := 0; i < len(c.codeinfos); i++ {
		if strings.HasSuffix(c.codeinfos[i].name, name) {
			copy(c.codeinfos[i:], c.codeinfos[i+1:])
			c.codeinfos = c.codeinfos[:len(c.codeinfos)-1]
			i--
		}
	}
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
		switch c.op {
		case opcall:
			if x, ok := c.v.(int); ok {
				pc = x
			}
		case opjump:
			x := c.v.(int)
			if x > 0 && env.codes[x-1].op == opscope {
				pc = x - 1
			}
		}
		var s string
		if name := env.lookupFuncName(pc); name != "" {
			if c.op == opcall || c.op == opjump {
				s = "\t## call " + name
			} else {
				s = "\t## " + name
			}
		}
		fmt.Fprintf(debugOut, "\t%d\t%s%s%s\n", i, formatOp(c.op, false), debugOperand(c), s)
	}
	fmt.Fprintln(debugOut, "\t"+strings.Repeat("-", 40)+"+")
}

func (env *env) debugState(pc int, backtrack bool) {
	if !debug {
		return
	}
	buf := new(bytes.Buffer)
	c := env.codes[pc]
	fmt.Fprintf(buf, "\t%d\t%s%s\t|", pc, formatOp(c.op, backtrack), debugOperand(c))
	var xs []int
	for i := env.stack.index; i >= 0; i = env.stack.data[i].next {
		xs = append(xs, i)
	}
	for i := len(xs) - 1; i >= 0; i-- {
		buf.WriteString("\t")
		buf.WriteString(debugJSON(env.stack.data[xs[i]].value))
	}
	switch c.op {
	case opcall:
		if x, ok := c.v.(int); ok {
			pc = x
		}
	case opjump:
		x := c.v.(int)
		if x > 0 && env.codes[x-1].op == opscope {
			pc = x - 1
		}
	}
	if name := env.lookupFuncName(pc); name != "" {
		if c.op == opcall || c.op == opjump {
			buf.WriteString("\t\t\t## call " + name)
		} else {
			buf.WriteString("\t\t\t## " + name)
		}
	}
	fmt.Fprintln(debugOut, buf.String())
}

func formatOp(c opcode, backtrack bool) string {
	if backtrack {
		return c.String() + " <backtrack>" + strings.Repeat(" ", 10-len(c.String()))
	}
	return c.String() + strings.Repeat(" ", 22-len(c.String()))
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
		fmt.Fprintf(buf, "%d, %s", v.pc, debugJSON(env.stack.data[v.stackindex].value))
		if i == len(env.forks)-1 {
			buf.WriteByte('>')
		}
	}
	fmt.Fprintf(debugOut, "\t-\t%s%s%d\t|\t%s\n", op, strings.Repeat(" ", 19), pc, buf.String())
}

func debugOperand(c *code) string {
	if c.op == opcall {
		switch v := c.v.(type) {
		case int:
			return debugJSON(v)
		case [3]interface{}:
			return fmt.Sprintf("%s/%d", v[2], v[1])
		default:
			panic(c)
		}
	} else {
		return debugJSON(c.v)
	}
}

func debugJSON(v interface{}) string {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(v)
	return strings.TrimSpace(b.String())
}

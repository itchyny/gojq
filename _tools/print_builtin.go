package main

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/itchyny/gojq"
)

func main() {
	cnt, err := os.ReadFile("builtin.jq")
	if err != nil {
		panic(err)
	}
	fds := make(map[string][]*gojq.FuncDef)
	q, err := gojq.Parse(string(cnt))
	if err != nil {
		panic(err)
	}
	for _, fd := range q.FuncDefs {
		name := fd.Name
		if name[0] == '_' {
			name = name[1:]
		}
		fd.Minify()
		fds[name] = append(fds[fd.Name], fd)
	}
	count := len(fds)
	names, i := make([]string, count), 0
	for n := range fds {
		names[i] = n
		i++
	}
	sort.Strings(names)
	for _, n := range names {
		var s strings.Builder
		for _, fd := range fds[n] {
			fmt.Fprintf(&s, "%s ", fd)
		}
		q, err := gojq.Parse(s.String())
		if err != nil {
			panic(fmt.Sprintf("%s: %s", err, s.String()))
		}
		for _, fd := range q.FuncDefs {
			fd.Minify()
		}
		if !reflect.DeepEqual(q.FuncDefs, fds[n]) {
			fmt.Printf("failed: %s: %s %s\n", n, q.FuncDefs, fds[n])
			continue
		}
		fmt.Printf("ok: %s: %s\n", n, s.String())
		count--
	}
	if count > 0 {
		os.Exit(1)
	}
}

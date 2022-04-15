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
	q, err := gojq.Parse(string(cnt))
	if err != nil {
		panic(err)
	}
	fds := make(map[string][]*gojq.FuncDef)
	for _, fd := range q.FuncDefs {
		fd.Minify()
		fds[fd.Name] = append(fds[fd.Name], fd)
	}
	count := len(fds)
	names, i := make([]string, count), 0
	for n := range fds {
		names[i] = n
		i++
	}
	sort.Strings(names)
	for _, n := range names {
		var sb strings.Builder
		for _, fd := range fds[n] {
			fmt.Fprintf(&sb, "%s ", fd)
		}
		q, err := gojq.Parse(sb.String())
		if err != nil {
			panic(fmt.Sprintf("%s: %s", err, sb.String()))
		}
		for _, fd := range q.FuncDefs {
			fd.Minify()
		}
		if !reflect.DeepEqual(q.FuncDefs, fds[n]) {
			fmt.Printf("failed: %s: %s %s\n", n, q.FuncDefs, fds[n])
			continue
		}
		fmt.Printf("ok: %s: %s\n", n, sb.String())
		count--
	}
	if count > 0 {
		os.Exit(1)
	}
}

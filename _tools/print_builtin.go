package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/itchyny/gojq"
)

func main() {
	cnt, err := ioutil.ReadFile("builtin.jq")
	if err != nil {
		panic(err)
	}
	fds := make(map[string][]*gojq.FuncDef)
	m, err := gojq.ParseModule(string(cnt))
	if err != nil {
		panic(err)
	}
	for _, fd := range m.FuncDefs {
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
		m, err := gojq.ParseModule(s.String())
		if err != nil {
			panic(err)
		}
		for _, fd := range m.FuncDefs {
			fd.Minify()
		}
		if !reflect.DeepEqual(m.FuncDefs, fds[n]) {
			fmt.Printf("failed: %s: %s %s\n", n, m.FuncDefs, fds[n])
			continue
		}
		fmt.Printf("ok: %s: %s\n", n, s.String())
		count--
	}
	if count > 0 {
		os.Exit(1)
	}
}

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
	qs := make(map[string][]*gojq.FuncDef)
	q, err := gojq.Parse(string(cnt) + ".")
	if err != nil {
		panic(err)
	}
	for _, fd := range q.Commas[0].Filters[0].FuncDefs {
		name := fd.Name
		if name[0] == '_' {
			name = name[1:]
		}
		qs[name] = append(qs[fd.Name], fd)
	}
	count := len(qs)
	names, i := make([]string, count), 0
	for n := range qs {
		names[i] = n
		i++
	}
	sort.Strings(names)
	for _, n := range names {
		var s strings.Builder
		for _, q := range qs[n] {
			fmt.Fprintf(&s, "%s ", q)
		}
		q, err := gojq.Parse(s.String() + ".")
		if err != nil {
			panic(err)
		}
		if !reflect.DeepEqual(q.Commas[0].Filters[0].FuncDefs, qs[n]) {
			fmt.Printf("failed: %s: %s %s\n", n, q.Commas[0].Filters[0].FuncDefs, qs[n])
			continue
		}
		fmt.Printf("ok: %s: %s\n", n, s.String())
		count--
	}
	if count > 0 {
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"
	"reflect"
	"sort"

	"github.com/itchyny/gojq"
)

func main() {
	count := len(gojq.BuiltinFuncDefinitions)
	names, i := make([]string, count), 0
	for n := range gojq.BuiltinFuncDefinitions {
		names[i] = n
		i++
	}
	sort.Strings(names)
	for _, n := range names {
		q, err := gojq.Parse(gojq.BuiltinFuncDefinitions[n])
		if err != nil {
			panic(err)
		}
		s := q.String()
		qq, err := gojq.Parse(s)
		if err != nil {
			fmt.Printf("failed: %s: %s %s\n", n, s, err)
			continue
		}
		if !reflect.DeepEqual(q, qq) {
			fmt.Printf("failed: %s: %s %s\n", n, s, qq)
			continue
		}
		fmt.Printf("ok: %s: %s\n", n, s)
		count--
	}
	if count > 0 {
		os.Exit(1)
	}
}

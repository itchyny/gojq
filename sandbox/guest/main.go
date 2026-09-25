// Command guest is the sandbox interpreter: gojq compiled to a wasm module
// (GOOS=wasip1 GOARCH=wasm). It evaluates the query given as its argument on
// the JSON input read from stdin, and prints each output as a JSON line.
// Regenerate the committed module with make build-sandbox.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/itchyny/gojq"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: guest query (JSON input on stdin)")
		os.Exit(2)
	}
	query, err := gojq.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var input any
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	iter := code.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		bs, err := gojq.Marshal(v)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", bs)
	}
}

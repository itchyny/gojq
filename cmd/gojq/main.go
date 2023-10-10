// gojq - Go implementation of jq
package main

import (
	"os"

	"github.com/Sosivio/gojq/cli"
)

func main() {
	os.Exit(cli.Run())
}

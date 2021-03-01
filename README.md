# gojq
[![CI Status](https://github.com/itchyny/gojq/workflows/CI/badge.svg)](https://github.com/itchyny/gojq/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/itchyny/gojq)](https://goreportcard.com/report/github.com/itchyny/gojq)
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/itchyny/gojq/blob/main/LICENSE)
[![release](https://img.shields.io/github/release/itchyny/gojq/all.svg)](https://github.com/itchyny/gojq/releases)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/itchyny/gojq)](https://pkg.go.dev/github.com/itchyny/gojq)

### Pure Go implementation of [jq](https://github.com/stedolan/jq)
This is an implementation of jq command written in Go language.
You can also embed gojq as a library to your Go products.

## Usage
```sh
 $ echo '{"foo": 128}' | gojq '.foo'
128
 $ echo '{"a": {"b": 42}}' | gojq '.a.b'
42
 $ echo '{"id": "sample", "10": {"b": 42}}' | gojq '{(.id): .["10"].b}'
{
  "sample": 42
}
 $ echo '[{"id":1},{"id":2},{"id":3}]' | gojq '.[] | .id'
1
2
3
 $ echo '{"a":1,"b":2}' | gojq '.a += 1 | .b *= 2'
{
  "a": 2,
  "b": 4
}
 $ echo '{"a":1} [2] 3' | gojq '. as {$a} ?// [$a] ?// $a | $a'
1
2
3
 $ echo '{"foo": 4722366482869645213696}' | gojq .foo
4722366482869645213696  # keeps the precision of large numbers
 $ gojq -n 'def fact($n): if $n < 1 then 1 else $n * fact($n - 1) end; fact(50)'
30414093201713378043612608166064768844377641568960512000000000000 # arbitrary-precision integer calculation
```

Nice error messages.
```sh
 $ echo '[1,2,3]' | gojq '.foo & .bar'
gojq: invalid query: .foo & .bar
    .foo & .bar
         ^  unexpected token "&"
 $ echo '{"foo": { bar: [] } }' | gojq '.'
gojq: invalid json: <stdin>
    {"foo": { bar: [] } }
              ^  invalid character 'b' looking for beginning of object key string
```

## Installation
### Homebrew
```sh
brew install gojq
```

### Build from source
```sh
go get github.com/itchyny/gojq/cmd/gojq
```

### Docker
```sh
docker run -i --rm itchyny/gojq
```

## Difference to jq
- gojq is purely implemented with Go language and is completely portable. jq depends on the C standard library so the availability of math functions depends on the library. jq also depends on the regular expression library and it makes build scripts complex.
- gojq implements nice error messages for invalid query and JSON input. The error message of jq is sometimes difficult to tell where to fix the query.
- gojq does not keep the order of object keys. I understand this might cause problems for some scripts but basically, we should not rely on the order of object keys. Due to this limitation, gojq does not have `keys_unsorted` function and `--sort-keys` (`-S`) option. I would implement when ordered map is implemented in the standard library of Go but I'm less motivated. Also, gojq assumes only valid JSON input while jq deals with some JSON extensions; `NaN` and `Infinity`.
- gojq supports arbitrary-precision integer calculation while jq does not. This is important to keep the precision of numeric IDs or nanosecond values. You can also use gojq to solve some mathematical problems which require big integers. Note that mathematical functions convert integers to floating-point numbers; only addition, subtraction, multiplication, modulo operation, and division (when divisible) keep integer precisions. When you want to calculate floor division of big integers, use `def intdiv($x; $y): ($x - $x % $y) / $y;`, instead of `$x / $y`.
- gojq fixes some bugs of jq. gojq correctly deletes elements of arrays by `|= empty` ([jq#2051](https://github.com/stedolan/jq/issues/2051)). gojq fixes update assignments including `try` or `//` operator ([jq#1885](https://github.com/stedolan/jq/issues/1885), [jq#2140](https://github.com/stedolan/jq/issues/2140)). gojq consistently counts by characters (not by bytes) in `index`, `rindex`, and `indices` functions; `"１２３４５" | .[index("３"):]` results in `"３４５"` ([jq#1430](https://github.com/stedolan/jq/issues/1430), [jq#1624](https://github.com/stedolan/jq/issues/1624)). gojq can deal with `%f` in `strftime` and `strptime` ([jq#1409](https://github.com/stedolan/jq/issues/1409)).
- gojq supports reading from YAML input while jq does not. gojq also supports YAML output.

### Color configuration
The gojq command automatically disables coloring output when the output is not a tty.
To force coloring output, specify `--color-output` (`-C`) option.
When [`NO_COLOR` environment variable](https://no-color.org/) is present or `--monochrome-output` (`-M`) option is specified, gojq disables coloring output.

Use `GOJQ_COLORS` environment variable to configure individual colors.
The variable is a colon-separated list of ANSI escape sequences of `null`, `false`, `true`, numbers, strings, object keys, arrays, and objects.
The default configuration is `90:33:33:36:32:34;1`.

## Usage as a library
You can use the gojq parser and interpreter from your Go products.

```go
package main

import (
	"fmt"
	"log"

	"github.com/itchyny/gojq"
)

func main() {
	query, err := gojq.Parse(".foo | ..")
	if err != nil {
		log.Fatalln(err)
	}
	input := map[string]interface{}{"foo": []interface{}{1, 2, 3}}
	iter := query.Run(input) // or query.RunWithContext
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			log.Fatalln(err)
		}
		fmt.Printf("%#v\n", v)
	}
}
```

- Firstly, use [`gojq.Parse(string) (*Query, error)`](https://pkg.go.dev/github.com/itchyny/gojq#Parse) to get the query from a string.
- Secondly, get the result iterator
  - using [`query.Run`](https://pkg.go.dev/github.com/itchyny/gojq#Query.Run) or [`query.RunWithContext`](https://pkg.go.dev/github.com/itchyny/gojq#Query.RunWithContext)
  - or alternatively, compile the query using [`gojq.Compile`](https://pkg.go.dev/github.com/itchyny/gojq#Compile) and then [`code.Run`](https://pkg.go.dev/github.com/itchyny/gojq#Code.Run) or [`code.RunWithContext`](https://pkg.go.dev/github.com/itchyny/gojq#Code.RunWithContext). You can reuse the `*Code` against multiple inputs to avoid compilation of the same query.
  - In either case, you cannot use custom type values as the query input. The type should be `[]interface{}` for an array and `map[string]interface{}` for a map (just like decoded to an `interface{}` using the [encoding/json](https://golang.org/pkg/encoding/json/) package). You can't use `[]int` or `map[string]string`, for example. If you want to query your custom struct, marshal to JSON, unmarshal to `interface{}` and use it as the query input.
- Thirdly, iterate through the results using [`iter.Next() (interface{}, bool)`](https://pkg.go.dev/github.com/itchyny/gojq#Iter). The iterator can emit an error so make sure to handle it. Termination is notified by the second returned value of `Next()`. The reason why the return type is not `(interface{}, error)` is that the iterator can emit multiple errors and you can continue after an error.

[`gojq.Compile`](https://pkg.go.dev/github.com/itchyny/gojq#Compile) allows to configure the following compiler options.

- [`gojq.WithModuleLoader`](https://pkg.go.dev/github.com/itchyny/gojq#WithModuleLoader) allows to load modules. By default, the module feature is disabled. If you want to load modules from the file system, use [`gojq.NewModuleLoader`](https://pkg.go.dev/github.com/itchyny/gojq#NewModuleLoader).
- [`gojq.WithEnvironLoader`](https://pkg.go.dev/github.com/itchyny/gojq#WithEnvironLoader) allows to configure the environment variables referenced by `env` and `$ENV`. By default, OS environment variables are not accessible due to security reasons. You can use `gojq.WithEnvironLoader(os.Environ)` if you want.
- [`gojq.WithVariables`](https://pkg.go.dev/github.com/itchyny/gojq#WithVariables) allows to configure the variables which can be used in the query. Pass the values of the variables to [`code.Run`](https://pkg.go.dev/github.com/itchyny/gojq#Code.Run) in the same order.
- [`gojq.WithFunction`](https://pkg.go.dev/github.com/itchyny/gojq#WithFunction) allows to add a custom internal function. An internal function can return a single value (which can be an error) each invocation. To add a jq function (which may include a comma operator to emit multiple values, `empty` function, accept a filter for its argument, or call another built-in function), prepend the function to each query on parsing.
- [`gojq.WithInputIter`](https://pkg.go.dev/github.com/itchyny/gojq#WithInputIter) allows to use `input` and `inputs` functions. By default, these functions are disabled.

## Bug Tracker
Report bug at [Issues・itchyny/gojq - GitHub](https://github.com/itchyny/gojq/issues).

## Author
itchyny (https://github.com/itchyny)

## License
This software is released under the MIT License, see LICENSE.

# gojq [![CI Status](https://github.com/itchyny/gojq/workflows/CI/badge.svg)](https://github.com/itchyny/gojq/actions)
Pure Go implementation of [jq](https://github.com/stedolan/jq).

## Usage
### from command line
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
4722366482869645213696  # keeps the precision of number while jq does not
 $ gojq -n 'def fact($n): if $n < 1 then 1 else $n * fact($n - 1) end; fact(50)'
30414093201713378043612608166064768844377641568960512000000000000 # arbitrary-precision integer calculation
```

Nice error messages.
```sh
 $ echo '[1,2,3]' | gojq  '.foo & .bar'
gojq: invalid query: .foo & .bar
    .foo & .bar
         ^  unexpected token "&"
 $ echo '{"foo": { bar: [] } }' | gojq '.'
gojq: invalid json: <stdin>
    {"foo": { bar: [] } }
              ^  invalid character 'b' looking for beginning of object key string
```

### as a library
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
		log.Fatalf("%s\n", err)
	}
	input := map[string]interface{}{"foo": []interface{}{1, 2, 3}}
	iter := query.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			log.Fatalf("%s\n", err)
		}
		fmt.Printf("%#v\n", v)
	}
}
```

## Installation
### Homebrew
```sh
brew install itchyny/tap/gojq
```

### Build from source
```bash
go get -u github.com/itchyny/gojq/cmd/gojq
```

## Difference to jq
- gojq is purely implemented with Go language and is completely portable. jq depends on the C standard library so the availability of math functions depends on the library. jq also depends on the regular expression library and it makes build scripts complex.
- gojq implements nice error messages for invalid query and JSON input. The error message of jq is sometimes difficult to tell where to fix the query.
- gojq does not keep the order of object keys. I understand this might cause problems for some scripts but basically we should not rely on the order of object keys. I would implement when ordered map is implemented in the standard library of Go but I'm less motivated.
- gojq supports arbitrary-precision integer calculation while jq does not. This is important to keeping the precision of numeric IDs or nanosecond values. You can use gojq to solve some mathematical problems which require big integers.
- gojq supports reading from YAML input while jq does not. gojq also supports YAML output.

## Bug Tracker
Report bug at [Issues・itchyny/gojq - GitHub](https://github.com/itchyny/gojq/issues).

## Author
itchyny (https://github.com/itchyny)

## License
This software is released under the MIT License, see LICENSE.

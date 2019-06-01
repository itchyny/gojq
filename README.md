# gojq [![Travis Build Status](https://travis-ci.org/itchyny/gojq.svg?branch=master)](https://travis-ci.org/itchyny/gojq)
Yet another Go implementation of [jq](https://github.com/stedolan/jq).

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

## Installation
### Homebrew
```sh
brew install itchyny/tap/gojq
```

### Build from source
```bash
go get -u github.com/itchyny/gojq/cmd/gojq
```

## Bug Tracker
Report bug at [Issues・itchyny/gojq - GitHub](https://github.com/itchyny/gojq/issues).

## Author
itchyny (https://github.com/itchyny)

## License
This software is released under the MIT License, see LICENSE.

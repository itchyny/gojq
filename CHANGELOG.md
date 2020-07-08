# Changelog
## [v0.11.0](https://github.com/itchyny/gojq/compare/v0.10.4..v0.11.0) (2020-07-08)
* improve parsing performance significantly
* rewrite the parser from `participle` library to `goyacc` generated parser
* release to [itchyny/gojq - Docker Hub](https://hub.docker.com/r/itchyny/gojq)
* support string interpolation for object pattern key

## [v0.10.4](https://github.com/itchyny/gojq/compare/v0.10.3..v0.10.4) (2020-06-30)
* implement variable in object key (`. as $x | { $x: 1 }`)
* fix modify operator (`|=`) with `try` `catch` expression
* fix optional operator (`?`) with alternative operator (`//`) in `map_values` function
* fix normalizing numeric types for library users
* export `gojq.NewModuleLoader` function for library users

## [v0.10.3](https://github.com/itchyny/gojq/compare/v0.10.2..v0.10.3) (2020-06-06)
* implement `add`, `unique_by`, `max_by`, `min_by`, `reverse` by internal
  functions for performance and reducing the binary size
* improve performance of `setpath`, `delpaths` functions
* fix assignment against nested slicing (`[1,2,3] | .[1:][:1] = [5]`)
* limit the array index of assignment operator
* optimize constant arrays and objects

## [v0.10.2](https://github.com/itchyny/gojq/compare/v0.10.1..v0.10.2) (2020-05-24)
* implement `sort_by`, `group_by`, `bsearch` by internal functions for performance
  and reducing the binary size
* fix object construction and constant object to allow trailing commas
* fix `tonumber` function to allow leading zeros
* minify the builtin functions to reduce the binary size

## [v0.10.1](https://github.com/itchyny/gojq/compare/v0.10.0..v0.10.1) (2020-04-24)
* fix array addition not to modify the left hand side

## [v0.10.0](https://github.com/itchyny/gojq/compare/v0.9.0..v0.10.0) (2020-04-02)
* implement various functions (`format`, `significand`, `modulemeta`, `halt_error`)
* implement `input`, `inputs` functions
* implement stream option (`--stream`)
* implement slicing with object (`.[{"start": 1, "end": 2}]`)
* implement `NO_COLOR` environment variable support
* implement `nul` output option (`-0`, `--nul-output`)
* implement exit status option (`-e`, `--exit-status`)
* implement `search` field of module meta object
* implement combination of `--yaml-input` and `--slurp`
* improve string token lexer and support nested string interpolation
* improve the exit code for jq compatibility
* improve default module search paths for jq compatibility
* improve documentation for the usage as a library
* change methods of `ModuleLoader` optional, implement `LoadModuleWithMeta` and `LoadJSONWithMeta`
* fix number normalization for JSON arguments (`--argjson`, `--slurpfile`)
* fix `0/0` and `infinite/infinite`
* fix `error` function against `null`

## [v0.9.0](https://github.com/itchyny/gojq/compare/v0.8.0..v0.9.0) (2020-03-15)
* implement various functions (`infinite`, `isfinite`, `isinfinite`, `finites`, `isnormal`, `normals`)
* implement environment variables loader as a compiler option
* implement `$NAME::NAME` syntax for imported JSON variable
* fix modify operator with empty against array (`[range(9)] | (.[] | select(. % 2 > 0)) |= empty`)
* fix variable and function scopes (`{ x: 1 } | . as $x | (.x as $x | $x) | ., $x`)
* fix path analyzer
* fix type check in `startswith` and `endswith`
* ignore type error of `ltrimstr` and `rtrimstr`
* remove nano seconds from `mktime` output
* trim newline at the end of error messages
* improve documents and examples

## [v0.8.0](https://github.com/itchyny/gojq/compare/v0.7.0..v0.8.0) (2020-03-02)
* implement format strings (`@text`, `@json`, `@html`, `@uri`, `@csv`, `@tsv`,
  `@sh`, `@base64`, `@base64d`)
* implement modules feature (`-L` option for directory to search modules from)
* implement options for binding variables from arguments (`--arg`, `--argjson`)
* implement options for binding variables from files (`--slurpfile`, `--rawfile`)
* implement an option for indentation count (`--indent`)
* fix `isnan` for `null`
* fix path analyzer
* fix error after optional operator (`1? | .x`)
* add `$ENV` variable
* add zsh completion file

## [v0.7.0](https://github.com/itchyny/gojq/compare/v0.6.0..v0.7.0) (2019-12-22)
* implement YAML input (`--yaml-input`) and output (`--yaml-output`)
* fix pipe in object value
* fix precedence of if, try, reduce and foreach expressions
* release from GitHub Actions

## [v0.6.0](https://github.com/itchyny/gojq/compare/v0.5.0..v0.6.0) (2019-08-26)
* implement arbitrary-precision integer calculation
* implement various functions (`repeat`, `pow10`, `nan`, `isnan`, `nearbyint`,
  `halt`, `INDEX`, `JOIN`, `IN`)
* implement long options (`--compact-output`, `--raw-output`, `--join-output`,
  `--color-output`, `--monochrome-output`, `--null-input`, `--raw-input`,
  `--slurp`, `--from-file`, `--version`)
* implement join output options (`-j`, `--join-output`)
* implement color/monochrome output options (`-C`, `--color-output`,
  `-M`, `--monochrome-output`)
* refactor builtin functions

## [v0.5.0](https://github.com/itchyny/gojq/compare/v0.4.0..v0.5.0) (2019-08-03)
* implement various functions (`with_entries`, `from_entries`, `leaf_paths`,
  `contains`, `inside`, `split`, `stream`, `fromstream`, `truncate_stream`,
  `bsearch`, `path`, `paths`, `map_values`, `del`, `delpaths`, `getpath`,
  `gmtime`, `localtime`, `mktime`, `strftime`, `strflocaltime`, `strptime`,
  `todate`, `fromdate`, `now`, `match`, `test`, `capture`, `scan`, `splits`,
  `sub`, `gsub`, `debug`, `stderr`)
* implement assignment operator (`=`)
* implement modify operator (`|=`)
* implement update operators (`+=`, `-=`, `*=`, `/=`, `%=`, `//=`)
* implement destructuring alternative operator (`?//`)
* allow function declaration inside query
* implement `-f` flag for loading query from file
* improve error message for parsing multiple line query

## [v0.4.0](https://github.com/itchyny/gojq/compare/v0.3.0..v0.4.0) (2019-07-20)
* improve performance significantly
* rewrite from recursive interpreter to stack machine based interpreter
* allow debugging with `make install-debug` and `export GOJQ_DEBUG=1`
* parse built-in functions and generate syntax trees before compilation
* optimize tail recursion
* fix behavior of optional operator
* fix scopes of arguments of recursive function call
* fix duplicate function argument names
* implement `setpath` function

## [v0.3.0](https://github.com/itchyny/gojq/compare/v0.2.0..v0.3.0) (2019-06-05)

* implement `reduce`, `foreach`, `label`, `break` syntax
* improve binding variable syntax to bind to an object or an array
* implement string interpolation
* implement object index by string (`."example"`)
* implement various functions (`add`, `flatten`, `min`, `min_by`, `max`,
  `max_by`, `sort`, `sort_by`, `group_by`, `unique`, `unique_by`, `tostring`,
  `indices`, `index`, `rindex`, `walk`, `transpose`, `first`, `last`, `nth`,
  `limit`, `all`, `any`, `isempty`, `error`, `builtins`, `env`)
* implement math functions (`sin`, `cos`, `tan`, `asin`, `acos`, `atan`,
  `sinh`, `cosh`, `tanh`, `asinh`, `acosh`, `atanh`, `floor`, `round`,
  `rint`, `ceil`, `trunc`, `fabs`, `sqrt`, `cbrt`, `exp`, `exp10`, `exp2`,
  `expm1`, `frexp`, `modf`, `log`, `log10`, `log1p`, `log2`, `logb`,
  `gamma`, `tgamma`, `lgamma`, `erf`, `erfc`, `j0`, `j1`, `y0`, `y1`,
  `atan2/2`, `copysign/2`, `drem/2`, `fdim/2`, `fmax/2`, `fmin/2`, `fmod/2`,
  `hypot/2`, `jn/2`, `ldexp/2`, `nextafter/2`, `nexttoward/2`, `remainder/2`,
  `scalb/2`, `scalbln/2`, `pow/2`, `yn/2`, `fma/3`)
* support object construction with variables
* support indexing against strings
* fix function evaluation for recursive call
* fix error handling of `//` operator
* fix string representation of NaN and Inf
* implement `-R` flag for reading input as raw strings
* implement `-c` flag for compact output
* implement `-n` flag for using null as input value
* implement `-r` flag for outputting raw string
* implement `-s` flag for reading all inputs into an array

## [v0.2.0](https://github.com/itchyny/gojq/compare/v0.1.0..v0.2.0) (2019-05-06)

* implement binding variable syntax (`... as $var`)
* implement `try` `catch` syntax
* implement alternative operator (`//`)
* implement various functions (`in`, `to_entries`, `startswith`, `endswith`,
  `ltrimstr`, `rtrimstr`, `combinations`, `ascii_downcase`, `ascii_upcase`,
  `tojson`, `fromjson`)
* support query for object indexing
* support object construction with variables
* support indexing against strings

## [v0.1.0](https://github.com/itchyny/gojq/compare/v0.0.1..v0.1.0) (2019-05-02)

* implement binary operators (`+`, `-`, `*`, `/`, `%`, `==`, `!=`, `>`, `<`,
  `>=`, `<=`, `and`, `or`)
* implement unary operators (`+`, `-`)
* implement booleans (`false`, `true`), `null`, number and string constant
  values
* implement `empty` value
* implement conditional syntax (`if` `then` `elif` `else` `end`)
* implement various functions (`length`, `utf8bytelength`, `not`, `keys`,
  `has`, `map`, `select`, `recurse`, `while`, `until`, `range`, `tonumber`,
  `type`, `arrays`, `objects`, `iterables`, `booleans`, `numbers`, `strings`,
  `nulls`, `values`, `scalars`, `reverse`, `explode`, `implode`, `join`)
* support function declaration
* support iterators in object keys
* support object construction shortcut
* support query in array indices
* support negative number indexing against arrays
* support json file name arguments

## [v0.0.1](https://github.com/itchyny/gojq/compare/0fa3241..v0.0.1) (2019-04-14)

* initial implementation

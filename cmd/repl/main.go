package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/itchyny/gojq"
)

func read(c interface{}, a []interface{}) interface{} {
	prompt, ok := a[0].(string)
	if !ok {
		return fmt.Errorf("%v: src is not a string", a[0])
	}
	fmt.Fprint(os.Stdout, prompt)
	s := bufio.NewScanner(os.Stdin)
	if ok = s.Scan(); !ok {
		fmt.Fprintln(os.Stdout)
		return io.EOF
	}

	return s.Text()
}

func eval(c interface{}, a []interface{}) interface{} {
	src, ok := a[0].(string)
	if !ok {
		return fmt.Errorf("%v: src is not a string", a[0])
	}
	iter, err := replRun(c, src)
	if err != nil {
		return err
	}

	return iter
}

func print(c interface{}, a []interface{}) interface{} {
	if _, err := fmt.Fprintln(os.Stdout, c); err != nil {
		return err
	}

	return gojq.EmptyIter{}
}

func itertest(c interface{}, a []interface{}) interface{} {
	return &gojq.SliceIter{Slice: []interface{}{1, 2, 3}}
}

func itererr(c interface{}, a []interface{}) interface{} {
	return &gojq.SliceIter{Slice: []interface{}{1, fmt.Errorf("itervaluerr")}}
}

type preludeLoader struct{}

func (preludeLoader) LoadInitModules() ([]*gojq.Query, error) {
	replSrc := `
def repl:
	def _wrap: if (. | type) != "array" then [.] end;
	def _repl:
		try read("> ") as $e |
		(try (.[] | eval($e)) catch . | print),
		_repl;
	_wrap | _repl;
`
	gq, err := gojq.Parse(replSrc)
	if err != nil {
		return nil, err
	}

	return []*gojq.Query{gq}, nil
}

func replRun(c interface{}, src string) (gojq.Iter, error) {
	gq, err := gojq.Parse(src)
	if err != nil {
		return nil, err
	}
	gc, err := gojq.Compile(gq,
		gojq.WithModuleLoader(preludeLoader{}),
		gojq.WithFunction("read", 1, 1, read),
		gojq.WithIterator("eval", 1, 1, eval),
		gojq.WithIterator("print", 0, 0, print),

		gojq.WithIterator("itertest", 0, 0, itertest),
		gojq.WithIterator("itererr", 0, 0, itererr),
	)
	if err != nil {
		return nil, err
	}

	return gc.Run(c), nil
}

func main() {
	expr := "repl"
	if len(os.Args) > 1 {
		expr = os.Args[1]
	}
	iter, err := replRun(nil, expr)
	if err != nil {
		panic(err)
	}
	for {
		v, ok := iter.Next()
		if !ok {
			break
		} else if err, ok := v.(error); ok {
			fmt.Fprintf(os.Stderr, "err: %v\n", err)
			break
		} else if d, ok := v.([2]interface{}); ok {
			fmt.Fprintf(os.Stdout, "%s: %v\n", d[0], d[1])
		}
	}
}

package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/itchyny/gojq"
)

type inputIter interface {
	gojq.Iter
	io.Closer
}

type jsonInputIter struct {
	dec   *json.Decoder
	buf   *bytes.Buffer
	fname string
	err   error
}

func newJSONInputIter(r io.Reader, fname string) inputIter {
	buf := new(bytes.Buffer)
	dec := json.NewDecoder(io.TeeReader(r, buf))
	dec.UseNumber()
	return &jsonInputIter{dec: dec, buf: buf, fname: fname}
}

func (i *jsonInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	var v interface{}
	if err := i.dec.Decode(&v); err != nil {
		if err == io.EOF {
			i.err = err
			return nil, false
		}
		i.err = &jsonParseError{i.fname, i.buf.String(), err}
		return i.err, true
	}
	if i.buf.Len() >= 256*1024 {
		i.buf.Reset()
	}
	return v, true
}

func (i *jsonInputIter) Close() error {
	i.err = io.EOF
	return nil
}

type nullInputIter struct {
	err error
}

func newNullInputIter() inputIter {
	return &nullInputIter{}
}

func (i *nullInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	i.err = io.EOF
	return nil, true
}

func (i *nullInputIter) Close() error {
	i.err = io.EOF
	return nil
}

type filesInputIter struct {
	newIter func(io.Reader, string) inputIter
	fnames  []string
	iter    inputIter
	file    *os.File
	err     error
}

func newFilesInputIter(newIter func(io.Reader, string) inputIter, fnames []string) inputIter {
	return &filesInputIter{newIter: newIter, fnames: fnames}
}

func (i *filesInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	for {
		if i.file == nil {
			if len(i.fnames) == 0 {
				i.err = io.EOF
				return nil, false
			}
			fname := i.fnames[0]
			i.fnames = i.fnames[1:]
			file, err := os.Open(fname)
			if err != nil {
				return err, true
			}
			i.file = file
			if i.iter != nil {
				i.iter.Close()
			}
			i.iter = i.newIter(i.file, fname)
		}
		if v, ok := i.iter.Next(); ok {
			return v, ok
		}
		i.file.Close()
		i.file = nil
	}
}

func (i *filesInputIter) Close() error {
	if i.file != nil {
		if err := i.file.Close(); err != nil {
			return err
		}
		i.file = nil
		i.err = io.EOF
	}
	return nil
}

type rawInputIter struct {
	scanner *bufio.Scanner
	err     error
}

func newRawInputIter(r io.Reader, _ string) inputIter {
	return &rawInputIter{scanner: bufio.NewScanner(r)}
}

func (i *rawInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	if i.scanner.Scan() {
		return i.scanner.Text(), true
	}
	if i.err = i.scanner.Err(); i.err != nil {
		return i.err, true
	}
	i.err = io.EOF
	return nil, false
}

func (i *rawInputIter) Close() error {
	i.err = io.EOF
	return nil
}

type readAllInputIter struct {
	r   io.Reader
	err error
}

func newReadAllInputIter(r io.Reader, _ string) inputIter {
	return &readAllInputIter{r: r}
}

func (i *readAllInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	bs, err := ioutil.ReadAll(i.r)
	if err != nil {
		i.err = err
		return err, true
	}
	i.err = io.EOF
	return string(bs), true
}

func (i *readAllInputIter) Close() error {
	i.err = io.EOF
	return nil
}

type streamInputIter struct {
	stream *jsonStream
	buf    *bytes.Buffer
	fname  string
	err    error
}

func newStreamInputIter(r io.Reader, fname string) inputIter {
	buf := new(bytes.Buffer)
	dec := json.NewDecoder(io.TeeReader(r, buf))
	dec.UseNumber()
	return &streamInputIter{stream: newJSONStream(dec), buf: buf, fname: fname}
}

func (i *streamInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	v, err := i.stream.next()
	if err != nil {
		if err == io.EOF {
			i.err = err
			return nil, false
		}
		i.err = &jsonParseError{i.fname, i.buf.String(), err}
		return i.err, true
	}
	if i.buf.Len() >= 256*1024 {
		i.buf.Reset()
	}
	return v, true
}

func (i *streamInputIter) Close() error {
	i.err = io.EOF
	return nil
}

type yamlInputIter struct {
	dec   *yaml.Decoder
	buf   *bytes.Buffer
	fname string
	err   error
}

func newYAMLInputIter(r io.Reader, fname string) inputIter {
	buf := new(bytes.Buffer)
	dec := yaml.NewDecoder(io.TeeReader(r, buf))
	return &yamlInputIter{dec: dec, buf: buf, fname: fname}
}

func (i *yamlInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	var v interface{}
	if err := i.dec.Decode(&v); err != nil {
		if err == io.EOF {
			i.err = err
			return nil, false
		}
		i.err = &yamlParseError{i.fname, i.buf.String(), err}
		return i.err, true
	}
	return fixMapKeyToString(v), true
}

func (i *yamlInputIter) Close() error {
	i.err = io.EOF
	return nil
}

type slurpInputIter struct {
	iter inputIter
	err  error
}

func newSlurpInputIter(newIter func(io.Reader, string) inputIter) func(io.Reader, string) inputIter {
	return func(r io.Reader, fname string) inputIter {
		return &slurpInputIter{iter: newIter(r, fname)}
	}
}

func (i *slurpInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	var vs []interface{}
	var v interface{}
	var ok bool
	for {
		v, ok = i.iter.Next()
		if !ok {
			i.err = io.EOF
			return vs, true
		}
		if i.err, ok = v.(error); ok {
			return i.err, true
		}
		vs = append(vs, v)
	}
}

func (i *slurpInputIter) Close() error {
	if i.iter != nil {
		i.iter.Close()
		i.iter = nil
		i.err = io.EOF
	}
	return nil
}

package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/itchyny/gojq"
)

type inputReader struct {
	io.Reader
	file *os.File
	buf  *bytes.Buffer
}

func newInputReader(r io.Reader) *inputReader {
	if r, ok := r.(*os.File); ok {
		if _, err := r.Seek(0, io.SeekCurrent); err == nil {
			return &inputReader{r, r, nil}
		}
	}
	var buf bytes.Buffer // do not use strings.Builder because we need to Reset
	return &inputReader{io.TeeReader(r, &buf), nil, &buf}
}

func (ir *inputReader) getContents(offset *int64, line *int) string {
	if buf := ir.buf; buf != nil {
		return buf.String()
	}
	if current, err := ir.file.Seek(0, io.SeekCurrent); err == nil {
		defer func() { ir.file.Seek(current, io.SeekStart) }()
	}
	ir.file.Seek(0, io.SeekStart)
	const bufSize = 16 * 1024
	var buf bytes.Buffer // do not use strings.Builder because we need to Reset
	if offset != nil && *offset > bufSize {
		buf.Grow(bufSize)
		for *offset > bufSize {
			n, err := io.Copy(&buf, io.LimitReader(ir.file, bufSize))
			*offset -= int64(n)
			*line += bytes.Count(buf.Bytes(), []byte{'\n'})
			buf.Reset()
			if err != nil || n == 0 {
				break
			}
		}
	}
	var r io.Reader
	if offset == nil {
		r = ir.file
	} else {
		r = io.LimitReader(ir.file, bufSize*2)
	}
	io.Copy(&buf, r)
	return buf.String()
}

type inputIter interface {
	gojq.Iter
	io.Closer
}

type jsonInputIter struct {
	dec    *json.Decoder
	ir     *inputReader
	fname  string
	offset int64
	line   int
	err    error
}

func newJSONInputIter(r io.Reader, fname string) inputIter {
	ir := newInputReader(r)
	dec := json.NewDecoder(ir)
	dec.UseNumber()
	return &jsonInputIter{dec: dec, ir: ir, fname: fname}
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
		var offset *int64
		var line *int
		if err, ok := err.(*json.SyntaxError); ok {
			err.Offset -= i.offset
			offset, line = &err.Offset, &i.line
		}
		i.err = &jsonParseError{i.fname, i.ir.getContents(offset, line), i.line, err}
		return i.err, true
	}
	if buf := i.ir.buf; buf != nil && buf.Len() >= 16*1024 {
		i.offset += int64(buf.Len())
		i.line += bytes.Count(buf.Bytes(), []byte{'\n'})
		buf.Reset()
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
		i.file.Close()
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

type streamInputIter struct {
	stream *jsonStream
	ir     *inputReader
	fname  string
	offset int64
	line   int
	err    error
}

func newStreamInputIter(r io.Reader, fname string) inputIter {
	ir := newInputReader(r)
	dec := json.NewDecoder(ir)
	dec.UseNumber()
	return &streamInputIter{stream: newJSONStream(dec), ir: ir, fname: fname}
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
		var offset *int64
		var line *int
		if err, ok := err.(*json.SyntaxError); ok {
			err.Offset -= i.offset
			offset, line = &err.Offset, &i.line
		}
		i.err = &jsonParseError{i.fname, i.ir.getContents(offset, line), i.line, err}
		return i.err, true
	}
	if buf := i.ir.buf; buf != nil && buf.Len() >= 16*1024 {
		i.offset += int64(buf.Len())
		i.line += bytes.Count(buf.Bytes(), []byte{'\n'})
		buf.Reset()
	}
	return v, true
}

func (i *streamInputIter) Close() error {
	i.err = io.EOF
	return nil
}

type yamlInputIter struct {
	dec   *yaml.Decoder
	ir    *inputReader
	fname string
	err   error
}

func newYAMLInputIter(r io.Reader, fname string) inputIter {
	ir := newInputReader(r)
	dec := yaml.NewDecoder(ir)
	return &yamlInputIter{dec: dec, ir: ir, fname: fname}
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
		i.err = &yamlParseError{i.fname, i.ir.getContents(nil, nil), err}
		return i.err, true
	}
	return normalizeYAML(v), true
}

func (i *yamlInputIter) Close() error {
	i.err = io.EOF
	return nil
}

type slurpInputIter struct {
	iter inputIter
	err  error
}

func newSlurpInputIter(iter inputIter) inputIter {
	return &slurpInputIter{iter: iter}
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

type slurpRawInputIter struct {
	iter inputIter
	err  error
}

func newSlurpRawInputIter(iter inputIter) inputIter {
	return &slurpRawInputIter{iter: iter}
}

func (i *slurpRawInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	var vs []string
	var v interface{}
	var ok bool
	for {
		v, ok = i.iter.Next()
		if !ok {
			i.err = io.EOF
			return strings.Join(vs, ""), true
		}
		if i.err, ok = v.(error); ok {
			return i.err, true
		}
		vs = append(vs, v.(string), "\n")
	}
}

func (i *slurpRawInputIter) Close() error {
	if i.iter != nil {
		i.iter.Close()
		i.iter = nil
		i.err = io.EOF
	}
	return nil
}

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
	Name() string
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

func (i *jsonInputIter) Next() (any, bool) {
	if i.err != nil {
		return nil, false
	}
	var v any
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

func (i *jsonInputIter) Name() string {
	return i.fname
}

type nullInputIter struct {
	err error
}

func newNullInputIter() inputIter {
	return &nullInputIter{}
}

func (i *nullInputIter) Next() (any, bool) {
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

func (i *nullInputIter) Name() string {
	return ""
}

type filesInputIter struct {
	newIter func(io.Reader, string) inputIter
	fnames  []string
	stdin   io.Reader
	iter    inputIter
	file    io.Reader
	err     error
}

func newFilesInputIter(
	newIter func(io.Reader, string) inputIter, fnames []string, stdin io.Reader,
) inputIter {
	return &filesInputIter{newIter: newIter, fnames: fnames, stdin: stdin}
}

func (i *filesInputIter) Next() (any, bool) {
	if i.err != nil {
		return nil, false
	}
	for {
		if i.file == nil {
			if len(i.fnames) == 0 {
				i.err = io.EOF
				if i.iter != nil {
					i.iter.Close()
					i.iter = nil
				}
				return nil, false
			}
			fname := i.fnames[0]
			i.fnames = i.fnames[1:]
			if fname == "-" && i.stdin != nil {
				i.file, fname = i.stdin, "<stdin>"
			} else {
				file, err := os.Open(fname)
				if err != nil {
					return err, true
				}
				i.file = file
			}
			if i.iter != nil {
				i.iter.Close()
			}
			i.iter = i.newIter(i.file, fname)
		}
		if v, ok := i.iter.Next(); ok {
			return v, ok
		}
		if r, ok := i.file.(io.Closer); ok && i.file != i.stdin {
			r.Close()
		}
		i.file = nil
	}
}

func (i *filesInputIter) Close() error {
	if i.file != nil {
		if r, ok := i.file.(io.Closer); ok && i.file != i.stdin {
			r.Close()
		}
		i.file = nil
		i.err = io.EOF
	}
	return nil
}

func (i *filesInputIter) Name() string {
	if i.iter != nil {
		return i.iter.Name()
	}
	return ""
}

type rawInputIter struct {
	r     *bufio.Reader
	fname string
	err   error
}

func newRawInputIter(r io.Reader, fname string) inputIter {
	return &rawInputIter{r: bufio.NewReader(r), fname: fname}
}

func (i *rawInputIter) Next() (any, bool) {
	if i.err != nil {
		return nil, false
	}
	line, err := i.r.ReadString('\n')
	if err != nil {
		i.err = err
		if err != io.EOF {
			return err, true
		}
		if line == "" {
			return nil, false
		}
	}
	return strings.TrimSuffix(line, "\n"), true
}

func (i *rawInputIter) Close() error {
	i.err = io.EOF
	return nil
}

func (i *rawInputIter) Name() string {
	return i.fname
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

func (i *streamInputIter) Next() (any, bool) {
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

func (i *streamInputIter) Name() string {
	return i.fname
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

func (i *yamlInputIter) Next() (any, bool) {
	if i.err != nil {
		return nil, false
	}
	var v any
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

func (i *yamlInputIter) Name() string {
	return i.fname
}

type slurpInputIter struct {
	iter inputIter
	err  error
}

func newSlurpInputIter(iter inputIter) inputIter {
	return &slurpInputIter{iter: iter}
}

func (i *slurpInputIter) Next() (any, bool) {
	if i.err != nil {
		return nil, false
	}
	var vs []any
	var v any
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

func (i *slurpInputIter) Name() string {
	return i.iter.Name()
}

type readAllIter struct {
	r     io.Reader
	fname string
	err   error
}

func newReadAllIter(r io.Reader, fname string) inputIter {
	return &readAllIter{r: r, fname: fname}
}

func (i *readAllIter) Next() (any, bool) {
	if i.err != nil {
		return nil, false
	}
	i.err = io.EOF
	cnt, err := io.ReadAll(i.r)
	if err != nil {
		return err, true
	}
	return string(cnt), true
}

func (i *readAllIter) Close() error {
	i.err = io.EOF
	return nil
}

func (i *readAllIter) Name() string {
	return i.fname
}

type slurpRawInputIter struct {
	iter inputIter
	err  error
}

func newSlurpRawInputIter(iter inputIter) inputIter {
	return &slurpRawInputIter{iter: iter}
}

func (i *slurpRawInputIter) Next() (any, bool) {
	if i.err != nil {
		return nil, false
	}
	var vs []string
	var v any
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
		vs = append(vs, v.(string))
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

func (i *slurpRawInputIter) Name() string {
	return i.iter.Name()
}

package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/itchyny/go-yaml"

	"github.com/itchyny/gojq"
)

type inputReader struct {
	io.Reader
	rs  io.ReadSeeker
	buf *bytes.Buffer
}

func newInputReader(r io.Reader) *inputReader {
	if r, ok := r.(io.ReadSeeker); ok {
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
	if current, err := ir.rs.Seek(0, io.SeekCurrent); err == nil {
		defer ir.rs.Seek(current, io.SeekStart)
	}
	_, _ = ir.rs.Seek(0, io.SeekStart)
	const bufSize = 16 * 1024
	var buf bytes.Buffer // do not use strings.Builder because we need to Reset
	for offset != nil && *offset > bufSize*3/4 {
		n, err := io.Copy(&buf,
			io.LimitReader(ir.rs, min(bufSize, *offset-bufSize/4)))
		*offset -= n
		*line += bytes.Count(buf.Bytes(), []byte{'\n'})
		buf.Reset()
		if err != nil || n == 0 {
			break
		}
	}
	var r io.Reader
	if offset == nil {
		r = ir.rs
	} else {
		r = io.LimitReader(ir.rs, bufSize)
	}
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

type inputIter interface {
	gojq.Iter
	io.Closer
	Name() string
}

type jsonInputIter struct {
	newNext    func(*json.Decoder) func() (any, error)
	next       func() (any, error)
	dec        *json.Decoder
	ir         *inputReader
	fname      string
	baseOffset int64
	offset     int64
	line       int
	err        error
}

func newJSONInputIter(r io.Reader, fname string) inputIter {
	ir := newInputReader(r)
	dec := json.NewDecoder(ir)
	dec.UseNumber()
	newNext := func(dec *json.Decoder) func() (any, error) {
		return func() (v any, err error) { err = dec.Decode(&v); return }
	}
	return &jsonInputIter{newNext: newNext, next: newNext(dec), dec: dec, ir: ir, fname: fname}
}

func (i *jsonInputIter) Next() (any, bool) {
	if i.err != nil {
		return nil, false
	}
	v, err := i.next()
	if err != nil {
		if err == io.EOF {
			i.err = err
			return nil, false
		}
		var offset *int64
		var line *int
		rawOffset := int64(0)
		isSyntaxErr := false
		if e, ok := err.(*json.SyntaxError); ok {
			rawOffset = e.Offset
			isSyntaxErr = true
			e.Offset += i.baseOffset - i.offset
			offset, line = &e.Offset, &i.line
		} else if err == io.ErrUnexpectedEOF && i.ir.rs != nil {
			if pos, err := i.ir.rs.Seek(0, io.SeekEnd); err == nil {
				offset, line = &pos, &i.line
			}
		}
		parseErr := &jsonParseError{i.fname, i.ir.getContents(offset, line), i.line, err}
		if !isSyntaxErr {
			i.err = parseErr
			return parseErr, true
		}
		i.recover(rawOffset)
		return parseErr, true
	}
	if buf := i.ir.buf; buf != nil && buf.Len() >= 16*1024 {
		consumed := i.dec.InputOffset() - i.offset
		i.line += bytes.Count(buf.Next(int(consumed)), []byte{'\n'})
		i.offset += consumed
	}
	return v, true
}

func (i *jsonInputIter) recover(rawOffset int64) {
	if i.ir.rs != nil {
		startPos := i.baseOffset + rawOffset
		if _, err := i.ir.rs.Seek(startPos, io.SeekStart); err == nil {
			var buf [4096]byte
			curPos := startPos
			nlPos := int64(-1)
			for {
				n, rerr := i.ir.rs.Read(buf[:])
				if n > 0 {
					if idx := bytes.IndexByte(buf[:n], '\n'); idx >= 0 {
						nlPos = curPos + int64(idx) + 1
						break
					}
					curPos += int64(n)
				}
				if rerr != nil {
					break
				}
			}
			if nlPos < 0 {
				nlPos = curPos
			}
			_, _ = i.ir.rs.Seek(nlPos, io.SeekStart)
			i.baseOffset = nlPos
			i.dec = json.NewDecoder(i.ir)
			i.dec.UseNumber()
			i.next = i.newNext(i.dec)
			i.offset = 0
			i.line = 0
		}
		return
	}
	if i.ir.buf != nil {
		bufErrOffset := int(rawOffset - i.offset)
		nlIdx := -1
		start := max(0, bufErrOffset)
		if start < i.ir.buf.Len() {
			if idx := bytes.IndexByte(i.ir.buf.Bytes()[start:], '\n'); idx >= 0 {
				nlIdx = start + idx
			}
		}
		if nlIdx < 0 {
			var b [1024]byte
			for {
				n, rerr := i.ir.Read(b[:])
				if n > 0 {
					if idx := bytes.IndexByte(b[:n], '\n'); idx >= 0 {
						nlIdx = i.ir.buf.Len() - n + idx
						break
					}
				}
				if rerr != nil {
					break
				}
			}
		}
		discardLen := i.ir.buf.Len()
		if nlIdx >= 0 {
			discardLen = nlIdx + 1
		}
		discarded := i.ir.buf.Next(discardLen)
		i.line += bytes.Count(discarded, []byte{'\n'})
		i.offset = 0
		rem := i.ir.buf.Bytes()
		var reader io.Reader = i.ir
		if len(rem) > 0 {
			reader = io.MultiReader(bytes.NewReader(rem), i.ir)
		}
		i.dec = json.NewDecoder(reader)
		i.dec.UseNumber()
		i.next = i.newNext(i.dec)
	}
}

func (i *jsonInputIter) Close() error {
	i.err = io.EOF
	return nil
}

func (i *jsonInputIter) Name() string {
	return i.fname
}

func newStreamInputIter(r io.Reader, fname string) inputIter {
	ir := newInputReader(r)
	dec := json.NewDecoder(ir)
	dec.UseNumber()
	newNext := func(dec *json.Decoder) func() (any, error) {
		return newJSONStream(dec).next
	}
	return &jsonInputIter{newNext: newNext, next: newNext(dec), dec: dec, ir: ir, fname: fname}
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

func (*nullInputIter) Name() string {
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
	return v, true
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

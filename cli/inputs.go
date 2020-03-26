package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"os"

	"github.com/itchyny/gojq"
)

type inputIter interface {
	gojq.Iter
	io.Closer
}

type singleInputIter struct {
	dec   *json.Decoder
	fname string
	err   error
}

func newSingleInputIter(in io.Reader, fname string) inputIter {
	dec := json.NewDecoder(in)
	dec.UseNumber()
	return &singleInputIter{dec: dec, fname: fname}
}

func (i *singleInputIter) Next() (interface{}, bool) {
	if i.err != nil {
		return nil, false
	}
	var v interface{}
	if err := i.dec.Decode(&v); err != nil {
		if err == io.EOF {
			i.err = err
			return nil, false
		}
		i.err = &jsonParseError{i.fname, "", err}
		return i.err, true
	}
	return v, true
}

func (i *singleInputIter) Close() error {
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

func newFilesInputIter(newIter func(io.Reader, string) inputIter, fnames []string) *filesInputIter {
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
			i.file, i.err = os.Open(fname)
			if i.err != nil {
				return i.err, true
			}
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

func newRawInputIter(in io.Reader, _ string) inputIter {
	return &rawInputIter{scanner: bufio.NewScanner(in)}
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
	fname  string
	err    error
}

func newStreamInputIter(in io.Reader, fname string) inputIter {
	dec := json.NewDecoder(in)
	dec.UseNumber()
	return &streamInputIter{stream: newJSONStream(dec), fname: fname}
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
		i.err = &jsonParseError{i.fname, "", err}
		return i.err, true
	}
	return v, true
}

func (i *streamInputIter) Close() error {
	i.err = io.EOF
	return nil
}

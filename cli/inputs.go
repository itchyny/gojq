package cli

import (
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

func newSingleInputIter(in io.Reader, fname string) *singleInputIter {
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
	fnames []string
	stream bool
	iter   inputIter
	file   *os.File
	err    error
}

func newFilesInputIter(fnames []string, stream bool) *filesInputIter {
	return &filesInputIter{fnames: fnames, stream: stream}
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
			if i.stream {
				i.iter = newStreamInputIter(i.file, fname)
			} else {
				i.iter = newSingleInputIter(i.file, fname)
			}
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

type streamInputIter struct {
	stream *jsonStream
	fname  string
	err    error
}

func newStreamInputIter(in io.Reader, fname string) *streamInputIter {
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

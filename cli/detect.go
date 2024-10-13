package cli

import (
	"bytes"
	"io"
)

type DetectedFormat int

func (d DetectedFormat) String() string {
	switch d {
	case JsonFormat:
		return "json"
	case YamlFormat:
		return "yaml"
	case XmlFormat:
		return "xml"
	}
	return ""
}

const (
	JsonFormat DetectedFormat = iota
	YamlFormat
	XmlFormat
)

func detectInputType(r io.Reader, bufSize int) (io.Reader, DetectedFormat) {
	readers := make([]io.Reader, 0)
	var buf []byte
	index := 0
	length := 0
	var err error
	result := func(t DetectedFormat) (io.Reader, DetectedFormat) {
		readers = append(readers, r)
		return io.MultiReader(readers...), t
	}
	readByte := func() (byte, error) {
		if index == length {
			if err != nil {
				return 0, err
			}
			buf = make([]byte, bufSize)
			length, err = r.Read(buf)
			if length == 0 && err != nil {
				return 0, err
			}
			readers = append(readers, bytes.NewReader(buf[0:length]))
			index = 0
		}
		i := index
		index = index + 1
		return buf[i], nil
	}

	// state machine
	state := "loop"
	var b, c byte
loop:
	for {
		switch state {
		// main loop
		case "loop":
			for {
				b, err = readByte()
				if err != nil {
					return result(JsonFormat)
				}
				switch b {
				case ' ', '\t', '\r', '\n':
				case '{', '[', '/':
					return result(JsonFormat)
				case '#':
					return result(YamlFormat)
				case '<':
					return result(XmlFormat)
				case '-':
					// yaml if "- " or "---"
					c, err = readByte()
					if err != nil {
						return result(JsonFormat)
					}
					if c == ' ' {
						return result(YamlFormat)
					}
					if c != '-' {
						return result(JsonFormat)
					}
					c, err = readByte()
					if err != nil || c != '-' {
						return result(JsonFormat)
					}
					return result(YamlFormat)
				case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
					return result(JsonFormat)
				case '"':
					// string can be either a json/yaml text
					state = "string"
					c = b
					continue loop
				case 't':
					// json if true
					for _, c = range []byte("rue") {
						b, err = readByte()
						if err != nil || b != c {
							return result(YamlFormat)
						}
					}
					state = "after"
					continue loop
				case 'f':
					// json if false
					for _, c = range []byte("alse") {
						b, err = readByte()
						if err != nil || b != c {
							return result(YamlFormat)
						}
					}
					state = "after"
					continue loop
				case 'n':
					// json if false
					for _, c = range []byte("ull") {
						b, err = readByte()
						if err != nil || b != c {
							return result(YamlFormat)
						}
					}
					state = "after"
					continue loop
				default:
					// neither a number or string with "
					return result(YamlFormat)
				}
			}
		// string, started by "
		case "string":
			escape := false
			for {
				b, err = readByte()
				if err != nil {
					return result(JsonFormat)
				}
				if escape {
					continue
				}
				switch b {
				case ' ', '\t':
				case '\r', '\n':
					// new line not allowed in yaml tags
					result(JsonFormat)
				case '\\':
					// escape next character
					escape = true
				case c:
					// close string, look for next char to identify if it is yaml tag
					state = "after"
					continue loop
				}
			}
		// close string, look for next char to identify if it is yaml tag
		case "after":
			for {
				b, err = readByte()
				if err != nil {
					return result(JsonFormat)
				}
				switch b {
				case ' ', '\t':
				case '\r', '\n':
					// new line not allowed in yaml tags
					return result(JsonFormat)
				case ':':
					// it is a yaml tag
					return result(YamlFormat)
				default:
					// it is not a yaml tag
					return result(JsonFormat)
				}
			}
		}

	}
}

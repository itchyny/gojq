package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestDetectInputType(t *testing.T) {
	for _, s := range []string{"", "\t", "\r", "\n", " ", " \t", " \r", " \n", " \t ", " \r ", " \n "} {
		testDetectInputType(t, s+"", JsonFormat)
		testDetectInputType(t, s+"{", JsonFormat)
		testDetectInputType(t, s+"#", YamlFormat)
		testDetectInputType(t, s+"<", XmlFormat)
		testDetectInputType(t, s+"a", YamlFormat)
		testDetectInputType(t, s+"a:", YamlFormat)
		testDetectInputType(t, s+"a: 1", YamlFormat)
		testDetectInputType(t, s+"true", JsonFormat)
		testDetectInputType(t, s+"true true", JsonFormat)
		testDetectInputType(t, s+"true:", YamlFormat)
		testDetectInputType(t, s+"null", JsonFormat)
		testDetectInputType(t, s+"null null", JsonFormat)
		testDetectInputType(t, s+"null:", YamlFormat)
		testDetectInputType(t, s+"false", JsonFormat)
		testDetectInputType(t, s+"false false", JsonFormat)
		testDetectInputType(t, s+"false:", YamlFormat)
		testDetectInputType(t, s+"1", JsonFormat)
		testDetectInputType(t, s+"-1", JsonFormat)
		testDetectInputType(t, s+"-1e3", JsonFormat)
		testDetectInputType(t, s+"--", JsonFormat)
		testDetectInputType(t, s+"---", YamlFormat)
		testDetectInputType(t, s+`"hello"`, JsonFormat)
		testDetectInputType(t, s+`"hello":1`, YamlFormat)
		testDetectInputType(t, s+`"hello": 1`, YamlFormat)
		testDetectInputType(t, s+`'hello'`, YamlFormat)
		testDetectInputType(t, s+`'hello':1`, YamlFormat)
		testDetectInputType(t, s+`'hello': 1`, YamlFormat)
	}
}

func testDetectInputType(t *testing.T, s string, format DetectedFormat) {
	r, f := detectInputType(strings.NewReader(s), 1)
	if f != format {
		t.Fatalf("failed: invalid format '%s' expected '%s' for string '%s'", f, format, s)
	}
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, r)
	if err != nil {
		t.Fatalf("failed: copy error for string '%s'", s)
	}
	if buf.String() != s {
		t.Fatalf("failed: invalid reader content '%s'' for string '%s'", buf.String(), s)
	}
}

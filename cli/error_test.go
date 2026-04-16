package cli

import (
	"fmt"
	"strings"
	"testing"
)

func generateString(size int) string {
	var sb strings.Builder
	sb.Grow(size)
	for i, j := 0, 0; i < size; i, j = i+1, (i+j)%256 {
		sb.WriteByte(byte(j%10 | '0'))
	}
	return sb.String()
}

func TestGetLineByOffset(t *testing.T) {
	numbers := generateString(500)
	testCases := []struct {
		str          string
		offset       int
		linestr      string
		line, column int
	}{
		{
			"", 0,
			"", 0, 0,
		},
		{
			"abc", -1,
			"abc", 1, 0,
		},
		{
			"abc", 0,
			"abc", 1, 0,
		},
		{
			"abc", 1,
			"abc", 1, 0,
		},
		{
			"abc", 2,
			"abc", 1, 1,
		},
		{
			"abc", 3,
			"abc", 1, 2,
		},
		{
			"abc", 4,
			"abc", 1, 3,
		},
		{
			"abc\ndef\nghi", 4,
			"abc", 1, 3,
		},
		{
			"abc\rdef\rghi", 4,
			"abc", 1, 3,
		},
		{
			"abc\r\ndef\r\nghi", 4,
			"abc", 1, 3,
		},
		{
			"abc\ndef\nghi", 5,
			"def", 2, 0,
		},
		{
			"abc\rdef\rghi", 5,
			"def", 2, 0,
		},
		{
			"abc\r\ndef\r\nghi", 6,
			"def", 2, 0,
		},
		{
			"abc\ndef\nghi", 7,
			"def", 2, 2,
		},
		{
			"abc\ndef\nghi", 8,
			"def", 2, 3,
		},
		{
			"abc\ndef\nghi", 9,
			"ghi", 3, 0,
		},
		{
			"abc\ndef\nghi", 12,
			"ghi", 3, 3,
		},
		{
			"abc\ndef\nghi", 13,
			"ghi", 3, 3,
		},
		{
			"abc\n０１２\nghi", 5,
			"０１２", 2, 0,
		},
		{
			"abc\n０１２\nghi", 6,
			"０１２", 2, 0,
		},
		{
			"abc\n０１２\nghi", 7,
			"０１２", 2, 0,
		},
		{
			"abc\n０１２\nghi", 8,
			"０１２", 2, 2,
		},
		{
			"abc\n０１２\nghi", 9,
			"０１２", 2, 2,
		},
		{
			"abc\n０１２\nghi", 10,
			"０１２", 2, 2,
		},
		{
			"abc\n０１２\nghi", 11,
			"０１２", 2, 4,
		},
		{
			"abc\ndef\xef\xbc\nghi", 10,
			"def", 2, 3,
		},
		{
			numbers, 0,
			numbers[:64], 1, 0,
		},
		{
			numbers, 30,
			numbers[:64], 1, 29,
		},
		{
			numbers, 100,
			numbers[51:115], 1, 48,
		},
		{
			numbers, 400,
			numbers[351:415], 1, 48,
		},
		{
			numbers, 450,
			numbers[401:465], 1, 48,
		},
		{
			numbers, 500,
			numbers[451:], 1, 48,
		},
	}
	for _, tc := range testCases {
		var name string
		if len(tc.str) > 20 {
			name = tc.str[:20] + "..."
		} else {
			name = tc.str
		}
		t.Run(fmt.Sprintf("%q,%d", name, tc.offset), func(t *testing.T) {
			linestr, line, column := getLineByOffset(tc.str, tc.offset)
			if linestr != tc.linestr || line != tc.line || column != tc.column {
				t.Errorf("getLineByOffset(%q, %d):\n"+
					"     got: %q, %d, %d\n"+
					"expected: %q, %d, %d", tc.str, tc.offset,
					linestr, line, column, tc.linestr, tc.line, tc.column)
			}
		})
	}
}

func benchJQQuery(size int) string {
	lines := []string{
		"def map_values(f): [.[] | f];",
		".[] | select(.age > 21) | {name, age}",
		"if .status == \"active\" then .name else empty end",
		"reduce .[] as $x (0; . + $x)",
		"[.items[] | {key: .id, value: .count}] | from_entries",
		"def walk(f): . as $in | if type == \"object\" then",
		"  reduce keys_unsorted[] as $k ({}; . + {($k): ($in[$k] | walk(f))})",
		"elif type == \"array\" then map(walk(f)) else f end;",
	}
	var sb strings.Builder
	for sb.Len() < size {
		sb.WriteString(lines[sb.Len()%len(lines)])
		sb.WriteByte('\n')
	}
	return sb.String()[:size]
}

func benchJSONLines(size int) string {
	line := `{"id":12345,"name":"Alice Johnson","active":true,"score":98.6,"tags":["admin","user"]}` + "\n"
	var sb strings.Builder
	for sb.Len() < size {
		sb.WriteString(line)
	}
	return sb.String()[:size]
}

func benchYAML(size int) string {
	entry := "- id: 42\n  name: Alice Johnson\n  active: true\n  address:\n    street: 123 Main St\n    city: Springfield\n"
	var sb strings.Builder
	for sb.Len() < size {
		sb.WriteString(entry)
	}
	return sb.String()[:size]
}

func BenchmarkGetLineByOffset(b *testing.B) {
	// queryParseError: jq source — inline arg or -f file, typically short.
	// jsonParseError: ~16 KiB window (seekable) or full buffered stdin.
	// yamlParseError: full file contents (no windowing).
	query := benchJQQuery(2048)
	jsonWindow := benchJSONLines(16 * 1024)
	yamlFull := benchYAML(64 * 1024)
	unicodeLine := "{\n  \"k\": \"" + strings.Repeat("０１２", 120) + "\",\n  \"x\": 1\n}"
	cases := []struct {
		name   string
		str    string
		offset int
	}{
		{"query_inline", ".foo | select(.x > 1)", 15},
		{"query_2KiB_mid", query, len(query) / 2},
		{"json_16KiB_mid", jsonWindow, len(jsonWindow) / 2},
		{"json_16KiB_end", jsonWindow, len(jsonWindow)},
		{"json_unicode", unicodeLine, strings.Index(unicodeLine, "０")},
		{"yaml_64KiB_mid", yamlFull, len(yamlFull) / 2},
		{"yaml_64KiB_end", yamlFull, len(yamlFull)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.str)))
			for b.Loop() {
				getLineByOffset(tc.str, tc.offset)
			}
		})
	}
}

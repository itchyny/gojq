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

func TestGetLineByLine(t *testing.T) {
	testCases := []struct {
		str     string
		line    int
		linestr string
	}{
		{
			"", 0,
			"",
		},
		{
			"abc", -1,
			"",
		},
		{
			"abc", 0,
			"",
		},
		{
			"abc", 1,
			"abc",
		},
		{
			"abc\n", 1,
			"abc",
		},
		{
			"abc", 2,
			"",
		},
		{
			"abc\n", 2,
			"",
		},
		{
			"abc\ndef\nghi", 1,
			"abc",
		},
		{
			"abc\ndef\nghi", 2,
			"def",
		},
		{
			"abc\rdef\rghi", 2,
			"def",
		},
		{
			"abc\r\ndef\r\nghi", 2,
			"def",
		},
		{
			"abc\ndef\nghi", 3,
			"ghi",
		},
		{
			"abc\ndef\nghi", 4,
			"",
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q,%d", tc.str, tc.line), func(t *testing.T) {
			linestr := getLineByLine(tc.str, tc.line)
			if linestr != tc.linestr {
				t.Errorf("getLineByLine(%q, %d):\n"+
					"     got: %q\n"+
					"expected: %q", tc.str, tc.line, linestr, tc.linestr)
			}
		})
	}
}

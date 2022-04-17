package cli

import (
	"bytes"
	"fmt"
	"strings"
)

var noColor bool

func newColor(c string) []byte {
	return []byte("\x1b[" + c + "m")
}

func setColor(buf *bytes.Buffer, color []byte) {
	if !noColor {
		buf.Write(color)
	}
}

var (
	resetColor     = newColor("0")    // Reset
	nullColor      = newColor("90")   // Bright black
	falseColor     = newColor("33")   // Yellow
	trueColor      = newColor("33")   // Yellow
	numberColor    = newColor("36")   // Cyan
	stringColor    = newColor("32")   // Green
	objectKeyColor = newColor("34;1") // Bold Blue
	arrayColor     = []byte(nil)      // No color
	objectColor    = []byte(nil)      // No color
)

func validColor(x string) bool {
	var num bool
	for _, c := range x {
		if '0' <= c && c <= '9' {
			num = true
		} else if c == ';' && num {
			num = false
		} else {
			return false
		}
	}
	return num || x == ""
}

func setColors(colors string) error {
	var i int
	var color string
	for _, target := range []*[]byte{
		&nullColor, &falseColor, &trueColor, &numberColor,
		&stringColor, &objectKeyColor, &arrayColor, &objectColor,
	} {
		if i < len(colors) {
			if j := strings.IndexByte(colors[i:], ':'); j >= 0 {
				color = colors[i : i+j]
				i += j + 1
			} else {
				color = colors[i:]
				i = len(colors)
			}
			if !validColor(color) {
				return fmt.Errorf("invalid color: %q", color)
			}
			if color == "" {
				*target = nil
			} else {
				*target = newColor(color)
			}
		} else {
			*target = nil
		}
	}
	return nil
}

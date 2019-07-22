package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestCliRun(t *testing.T) {
	f, err := os.Open("test.yaml")
	require.NoError(t, err)
	defer f.Close()

	var testCases []struct {
		Name     string
		Args     []string
		Input    string
		Expected string
		Error    string
	}
	require.NoError(t, yaml.NewDecoder(f).Decode(&testCases))

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			outStream := new(bytes.Buffer)
			errStream := new(bytes.Buffer)
			cli := cli{
				inStream:  strings.NewReader(tc.Input),
				outStream: outStream,
				errStream: errStream,
			}
			code := cli.run(tc.Args)
			if tc.Error == "" {
				assert.Equal(t, exitCodeOK, code)
				assert.Equal(t, tc.Expected, outStream.String())
				assert.Equal(t, "", errStream.String())
			} else {
				assert.Equal(t, exitCodeErr, code)
				assert.Equal(t, tc.Expected, outStream.String())
				assert.Contains(t, errStream.String(), strings.TrimSpace(tc.Error))
			}
		})
	}
}

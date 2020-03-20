package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func init() {
	addDefaultModulePath = false
	os.Setenv("NO_COLOR", "")
}

func setLocation(loc *time.Location) func() {
	orig := time.Local
	time.Local = loc
	return func() { time.Local = orig }
}

func TestCliRun(t *testing.T) {
	f, err := os.Open("test.yaml")
	require.NoError(t, err)
	defer f.Close()
	defer setLocation(time.FixedZone("UTC-7", -7*60*60))()

	var testCases []struct {
		Name     string
		Args     []string
		Input    string
		Env      []string
		Expected string
		Error    string
		ExitCode int `yaml:"exit_code"`
	}
	require.NoError(t, yaml.NewDecoder(f).Decode(&testCases))

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			defer func() { assert.Nil(t, recover()) }()
			outStream := new(bytes.Buffer)
			errStream := new(bytes.Buffer)
			cli := cli{
				inStream:  strings.NewReader(tc.Input),
				outStream: outStream,
				errStream: errStream,
			}
			for _, env := range tc.Env {
				xs := strings.SplitN(env, "=", 2)
				k, v := xs[0], xs[1]
				defer func(v string) { os.Setenv(k, v) }(os.Getenv(k))
				os.Setenv(k, v)
			}
			code := cli.run(tc.Args)
			if tc.Error == "" {
				assert.Equal(t, tc.ExitCode, code)
				assert.Equal(t, tc.Expected, outStream.String())
				assert.Equal(t, "", errStream.String())
			} else {
				errStr := errStream.String()
				if strings.Contains(errStr, "DEBUG:") {
					assert.Equal(t, exitCodeOK, code)
				} else if tc.ExitCode != 0 {
					assert.Equal(t, tc.ExitCode, code)
				} else {
					assert.Equal(t, exitCodeErr, code)
				}
				assert.Equal(t, tc.Expected, outStream.String())
				assert.Contains(t,
					strings.ReplaceAll(errStr, name+": ", ""),
					strings.TrimSpace(tc.Error))
				assert.Equal(t, '\n', rune(errStr[len(errStr)-1]), errStr)
				assert.NotEqual(t, '\n', rune(errStr[len(errStr)-2]), errStr)
			}
		})
	}
}

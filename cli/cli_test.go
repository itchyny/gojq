package cli

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func init() {
	addDefaultModulePaths = false
	os.Setenv("NO_COLOR", "")
	os.Setenv("GOJQ_COLORS", "")
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
	errorReplacer := strings.NewReplacer(
		name+": ", "",
		"testdata\\", "testdata/",
		"flag `/", "flag `--",
	)

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
			var outStream, errStream strings.Builder
			cli := cli{
				inStream:  strings.NewReader(tc.Input),
				outStream: &outStream,
				errStream: &errStream,
			}
			for _, env := range tc.Env {
				xs := strings.SplitN(env, "=", 2)
				k, v := xs[0], xs[1]
				defer func(v string) { os.Setenv(k, v) }(os.Getenv(k))
				if k == "GOJQ_COLORS" {
					defer func(colors [][]byte) {
						nullColor, falseColor, trueColor, numberColor,
							stringColor, objectKeyColor, arrayColor, objectColor =
							colors[0], colors[1], colors[2], colors[3],
							colors[4], colors[5], colors[6], colors[7]
					}([][]byte{
						nullColor, falseColor, trueColor, numberColor,
						stringColor, objectKeyColor, arrayColor, objectColor,
					})
				}
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
					assert.Equal(t, exitCodeDefaultErr, code)
				}
				assert.Equal(t, tc.Expected, outStream.String())
				assert.Contains(t, errorReplacer.Replace(errStr), strings.TrimSpace(tc.Error))
				assert.Equal(t, true, strings.HasSuffix(errStr, "\n"), errStr)
				assert.Equal(t, false, strings.HasSuffix(errStr, "\n\n"), errStr)
			}
		})
	}
}

package cli

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v3"
)

func init() {
	addDefaultModulePaths = false
}

func setLocation(loc *time.Location) func() {
	orig := time.Local
	time.Local = loc
	return func() { time.Local = orig }
}

func TestCliRun(t *testing.T) {
	if err := os.Setenv("NO_COLOR", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GOJQ_COLORS", ""); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("test.yaml")
	if err != nil {
		t.Fatal(err)
	}
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
	if err = yaml.NewDecoder(f).Decode(&testCases); err != nil {
		t.Fatal(err)
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			defer func() {
				if err := recover(); err != nil {
					t.Fatal(err)
				}
			}()
			var outStream, errStream strings.Builder
			cli := cli{
				inStream:  strings.NewReader(tc.Input),
				outStream: &outStream,
				errStream: &errStream,
			}
			for _, env := range tc.Env {
				xs := strings.SplitN(env, "=", 2)
				k, v := xs[0], xs[1]
				defer func(v string) {
					if err := os.Setenv(k, v); err != nil {
						t.Fatal(err)
					}
				}(os.Getenv(k))
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
				if err := os.Setenv(k, v); err != nil {
					t.Fatal(err)
				}
			}
			code := cli.run(tc.Args)
			if tc.Error == "" {
				if code != tc.ExitCode {
					t.Errorf("exit code: got: %v, expected: %v", code, tc.ExitCode)
				}
				if diff := cmp.Diff(tc.Expected, outStream.String()); diff != "" {
					t.Error("standard output:\n" + diff)
				}
				if diff := cmp.Diff("", errStream.String()); diff != "" {
					t.Error("standard error output:\n" + diff)
				}
			} else {
				errStr := errStream.String()
				if strings.Contains(errStr, "DEBUG:") {
					if code != exitCodeOK {
						t.Errorf("exit code: got: %v, expected: %v", code, exitCodeOK)
					}
				} else if tc.ExitCode != 0 {
					if code != tc.ExitCode {
						t.Errorf("exit code: got: %v, expected: %v", code, tc.ExitCode)
					}
				} else {
					if code != exitCodeDefaultErr {
						t.Errorf("exit code: got: %v, expected: %v", code, exitCodeDefaultErr)
					}
				}
				if diff := cmp.Diff(tc.Expected, outStream.String()); diff != "" {
					t.Error("standard output:\n" + diff)
				}
				if got := errorReplacer.Replace(errStr); !strings.HasPrefix(got, tc.Error) && !strings.HasSuffix(got, tc.Error) {
					t.Error("standard error output:\n" + cmp.Diff(tc.Error, got))
				}
				if !strings.HasSuffix(errStr, "\n") && !strings.Contains(tc.Name, "stderr") && !strings.Contains(tc.Name, "halt_error") {
					t.Error(`standard error output should end with "\n"`)
				}
				if strings.HasSuffix(errStr, "\n\n") {
					t.Error(`standard error output should not end with "\n\n"`)
				}
			}
		})
	}
}

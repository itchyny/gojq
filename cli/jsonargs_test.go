package cli

import (
	"strings"
	"testing"
)

func TestJSONArgumentsRequireSingleValue(t *testing.T) {
	for _, flag := range []string{"--argjson", "--jsonargs"} {
		for _, tc := range []struct {
			name, value, want string
		}{
			{"empty", "", ""},
			{"whitespace", " \t\r\n", ""},
			{"two numbers", "1 2", ""},
			{"two containers", "{} []", ""},
			{"adjacent containers", "{}[]", ""},
			{"two booleans", "true false", ""},
			{"trailing text", "1 trailing", ""},
			{"trailing incomplete JSON", "1 {", ""},
			{"malformed JSON", "[1,]", ""},
			{"incomplete JSON", "{", ""},
			{"null", "null", "null"},
			{"false", "false", "false"},
			{"number with whitespace", " \t42\r\n", "42"},
			{"empty string", `""`, `""`},
			{"unicode string", `"世界"`, `"世界"`},
			{"array", "[1, 2]", "[1,2]"},
			{"object", `{"x": 1}`, `{"x":1}`},
			{"big integer", "10000000000000000000000000000001", "10000000000000000000000000000001"},
		} {
			t.Run(flag+"/"+tc.name, func(t *testing.T) {
				args := []string{"-nc"}
				want := tc.want + "\n"
				if flag == "--argjson" {
					args = append(args, flag, "x", tc.value, "$x")
				} else {
					args = append(args, "$ARGS.positional", flag, tc.value)
					want = "[" + tc.want + "]\n"
				}
				var out, errOut strings.Builder
				cli := cli{inStream: strings.NewReader(""), outStream: &out, errStream: &errOut}
				code := cli.run(args)
				if tc.want == "" {
					if code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "invalid json:") {
						t.Fatalf("invalid JSON argument: code %d, stdout %q, stderr %q", code, &out, &errOut)
					}
				} else if code != 0 || out.String() != want || errOut.Len() != 0 {
					t.Fatalf("code %d, stdout %q, stderr %q; want stdout %q", code, &out, &errOut, want)
				}
			})
		}
	}
}

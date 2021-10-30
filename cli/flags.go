package cli

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func parseFlags(args []string, opts interface{}) ([]string, error) {
	rest := make([]string, 0, len(args))
	val := reflect.ValueOf(opts).Elem()
	typ := val.Type()
	longToValue := map[string]reflect.Value{}
	longToPositional := map[string]struct{}{}
	shortToValue := map[string]reflect.Value{}
	for i, l := 0, val.NumField(); i < l; i++ {
		if flag, ok := typ.Field(i).Tag.Lookup("long"); ok {
			longToValue[flag] = val.Field(i)
			if _, ok := typ.Field(i).Tag.Lookup("positional"); ok {
				longToPositional[flag] = struct{}{}
			}
		}
		if flag, ok := typ.Field(i).Tag.Lookup("short"); ok {
			shortToValue[flag] = val.Field(i)
		}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var (
			val        reflect.Value
			ok         bool
			positional bool
			shortopts  string
		)
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			if val, ok = longToValue[arg[2:]]; ok {
				_, positional = longToPositional[arg[2:]]
			} else {
				if j := strings.IndexByte(arg, '='); j >= 0 {
					if val, ok = longToValue[arg[2:j]]; ok {
						if val.Kind() == reflect.Bool {
							return nil, fmt.Errorf("boolean flag `%s' cannot have an argument", arg[:j])
						}
						args[i] = arg[j+1:]
						arg = arg[:j]
						i--
					}
				}
				if !ok {
					return nil, fmt.Errorf("unknown flag `%s'", arg)
				}
			}
		} else if arg > "-" && arg[0] == '-' {
			if val, ok = shortToValue[arg[1:]]; !ok {
				var skip bool
				for i := 1; i < len(arg); i++ {
					opt := arg[i : i+1]
					if val, ok = shortToValue[opt]; ok {
						if val.Kind() != reflect.Bool {
							break
						}
					} else if !("A" <= opt && opt <= "Z" || "a" <= opt && opt <= "z") {
						skip = true
						break
					}
				}
				if !skip {
					shortopts = arg[1:]
					goto L
				}
			}
		}
		if !ok {
			rest = append(rest, arg)
			continue
		}
	S:
		switch val.Kind() {
		case reflect.Bool:
			val.SetBool(true)
		case reflect.String:
			if i++; i >= len(args) {
				return nil, fmt.Errorf("expected argument for flag `%s'", arg)
			}
			val.SetString(args[i])
		case reflect.Ptr:
			if val.Type().Elem().Kind() == reflect.Int {
				if i++; i >= len(args) {
					return nil, fmt.Errorf("expected argument for flag `%s'", arg)
				}
				v, err := strconv.Atoi(args[i])
				if err != nil {
					return nil, fmt.Errorf("invalid argument for flag `%s': %w", arg, err)
				}
				val.Set(reflect.New(val.Type().Elem()))
				val.Elem().SetInt(int64(v))
			}
		case reflect.Slice:
			if i++; i >= len(args) {
				return nil, fmt.Errorf("expected argument for flag `%s'", arg)
			}
			for ; i < len(args); i++ {
				val.Set(reflect.Append(val, reflect.ValueOf(args[i])))
				if !positional {
					break
				}
			}
		case reflect.Map:
			if i += 2; i >= len(args) {
				return nil, fmt.Errorf("expected 2 arguments for flag `%s'", arg)
			}
			if val.IsNil() {
				val.Set(reflect.MakeMap(val.Type()))
			}
			val.SetMapIndex(reflect.ValueOf(args[i-1]), reflect.ValueOf(args[i]))
		}
	L:
		if shortopts != "" {
			opt := shortopts[:1]
			if val, ok = shortToValue[opt]; !ok {
				return nil, fmt.Errorf("unknown flag `%s'", opt)
			}
			if val.Kind() != reflect.Bool {
				args[i] = shortopts[1:]
				i--
				shortopts = ""
			} else {
				shortopts = shortopts[1:]
			}
			arg = "-" + opt
			goto S
		}
	}
	return rest, nil
}

func formatFlags(opts interface{}) string {
	val := reflect.ValueOf(opts).Elem()
	typ := val.Type()
	var sb strings.Builder
	sb.WriteString("Command Options:\n")
	for i, l := 0, typ.NumField(); i < l; i++ {
		tag := typ.Field(i).Tag
		if i == l-1 {
			sb.WriteString("\nHelp Option:\n")
		}
		sb.WriteString("  ")
		var short bool
		if flag, ok := tag.Lookup("short"); ok {
			sb.WriteString("-")
			sb.WriteString(flag)
			short = true
		} else {
			sb.WriteString("  ")
		}
		m := sb.Len()
		if flag, ok := tag.Lookup("long"); ok {
			if short {
				sb.WriteString(", ")
			} else {
				sb.WriteString("  ")
			}
			sb.WriteString("--")
			sb.WriteString(flag)
			switch val.Field(i).Kind() {
			case reflect.Bool:
				sb.WriteString(" ")
			case reflect.Map:
				if strings.HasSuffix(flag, "file") {
					sb.WriteString(" name file")
				} else {
					sb.WriteString(" name value")
				}
			default:
				sb.WriteString("=")
			}
		} else {
			sb.WriteString("=")
		}
		sb.WriteString("                       "[:24-sb.Len()+m])
		sb.WriteString(tag.Get("description"))
		sb.WriteString("\n")
	}
	return sb.String()
}

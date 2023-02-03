package cli

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func parseFlags(args []string, opts any) ([]string, error) {
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
	mapKeys := map[string]struct{}{}
	var positionalVal reflect.Value
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var (
			val       reflect.Value
			ok        bool
			shortopts string
		)
		if arg == "--" {
			if positionalVal.IsValid() {
				for _, arg := range args[i+1:] {
					positionalVal.Set(reflect.Append(positionalVal, reflect.ValueOf(arg)))
				}
			} else {
				rest = append(rest, args[i+1:]...)
			}
			break
		}
		if strings.HasPrefix(arg, "--") {
			if val, ok = longToValue[arg[2:]]; !ok {
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
		} else if len(arg) > 1 && arg[0] == '-' {
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
			if !skip && (len(arg) > 2 || !ok) {
				shortopts = arg[1:]
				goto L
			}
		}
		if !ok {
			if positionalVal.IsValid() && len(rest) > 0 {
				positionalVal.Set(reflect.Append(positionalVal, reflect.ValueOf(arg)))
			} else {
				rest = append(rest, arg)
			}
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
			if _, ok := longToPositional[arg[2:]]; ok {
				if positionalVal.IsValid() {
					for positionalVal.Len() > val.Len() {
						val.Set(reflect.Append(val, reflect.Zero(val.Type().Elem())))
					}
				}
				positionalVal = val
			} else {
				if i++; i >= len(args) {
					return nil, fmt.Errorf("expected argument for flag `%s'", arg)
				}
				val.Set(reflect.Append(val, reflect.ValueOf(args[i])))
			}
		case reflect.Map:
			if i += 2; i >= len(args) {
				return nil, fmt.Errorf("expected 2 arguments for flag `%s'", arg)
			}
			if val.IsNil() {
				val.Set(reflect.MakeMap(val.Type()))
			}
			name := args[i-1]
			if _, ok := mapKeys[name]; !ok {
				mapKeys[name] = struct{}{}
				val.SetMapIndex(reflect.ValueOf(name), reflect.ValueOf(args[i]))
			}
		}
	L:
		if shortopts != "" {
			opt := shortopts[:1]
			if val, ok = shortToValue[opt]; !ok {
				return nil, fmt.Errorf("unknown flag `%s'", opt)
			}
			if val.Kind() != reflect.Bool && len(shortopts) > 1 {
				if shortopts[1] == '=' {
					args[i] = shortopts[2:]
				} else {
					args[i] = shortopts[1:]
				}
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

func formatFlags(opts any) string {
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
				if _, ok = tag.Lookup("positional"); !ok {
					sb.WriteString("=")
				}
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

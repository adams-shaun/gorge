package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// help writes the usage of the command at path.
func (r *runner) help(w io.Writer, path []*Command) {
	cmd := path[len(path)-1]
	fs := r.flagSet(path)
	p := pathString(path)
	var b strings.Builder
	text := cmd.Long
	if text == "" {
		text = cmd.Short
	}
	if text != "" {
		b.WriteString(strings.TrimRight(text, "\n") + "\n\n")
	}
	b.WriteString("Usage:\n")
	if cmd.Run != nil {
		line := "  " + p + " [flags]"
		if cmd.Args != "" {
			line += " " + cmd.Args
		}
		b.WriteString(line + "\n")
	}
	subs := cmd.visibleSubs()
	if len(cmd.Sub) > 0 {
		b.WriteString("  " + p + " <command> [flags]\n")
	}
	if len(cmd.Sub) > 0 {
		b.WriteString("\nCommands:\n")
		rows := [][2]string{}
		for _, s := range subs {
			rows = append(rows, [2]string{s.Name, s.Short})
		}
		if cmd.subNamed("help") == nil {
			rows = append(rows, [2]string{"help", "show a command's help (" + p + " help <command>)"})
			sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
		}
		writeRows(&b, rows)
	}
	var own, global []*Flag
	for _, f := range fs.order {
		if f.global {
			global = append(global, f)
		} else {
			own = append(own, f)
		}
	}
	byName := func(fl []*Flag) {
		sort.Slice(fl, func(i, j int) bool { return fl[i].Name < fl[j].Name })
	}
	byName(own)
	byName(global)
	if len(own) > 0 {
		b.WriteString("\nFlags:\n")
		writeRows(&b, flagRows(own))
	}
	b.WriteString("\nGlobal flags:\n")
	writeRows(&b, flagRows(global))
	var here, inherited []*Flag
	for _, f := range own {
		if f.level == len(path)-1 {
			here = append(here, f)
		} else {
			inherited = append(inherited, f)
		}
	}
	if len(own) > 0 {
		b.WriteString("\nEnvironment and config:\n")
		if len(here) > 0 {
			section := `the "` + sectionKey(path) + `" section`
			if len(path) == 1 {
				section = "the top level"
			}
			fmt.Fprintf(&b, "  Each flag is also read from the environment variable %s_<FLAG>\n"+
				"  (upper case, '-' as '_'; e.g. %s) and from %s of the\n"+
				"  JSON file --config names.\n",
				envUpper("GORGE_"+strings.Join(pathWords(path), "_")), here[0].Env, section)
		}
		if len(inherited) > 0 {
			var names []string
			for _, f := range inherited {
				names = append(names, "--"+f.Name)
			}
			fmt.Fprintf(&b, "  Inherited flags (%s) are read from their own command's variable\n"+
				"  and config section (e.g. %s).\n", strings.Join(names, ", "), inherited[0].Env)
		}
		b.WriteString("  Precedence: flag > environment > config > default; --print-config prints\n" +
			"  the result. A long flag may also be spelled with one dash (-name value),\n" +
			"  as Go's flag package spells it.\n")
	}
	if len(cmd.Sub) > 0 {
		fmt.Fprintf(&b, "\nRun '%s <command> --help' for a command's flags.\n", p)
	}
	io.WriteString(w, b.String())
}

// flagRows are flags' usage rows: spelling and type, then usage and default.
func flagRows(fl []*Flag) [][2]string {
	rows := make([][2]string, 0, len(fl))
	for _, f := range fl {
		var left string
		switch {
		case len(f.Name) == 1:
			left = "-" + f.Name
		case f.Short != 0:
			left = "-" + string(f.Short) + ", --" + f.Name
		default:
			left = "    --" + f.Name
		}
		if _, ok := f.Value.(boolFlag); !ok {
			left += " " + f.Value.Type()
		}
		right := f.Usage
		if d := defaultText(f); d != "" {
			right += " (default " + d + ")"
		}
		rows = append(rows, [2]string{left, right})
	}
	return rows
}

// defaultText is the default for usage, or "" for a zero default.
func defaultText(f *Flag) string {
	d := f.DefValue
	switch f.Value.(type) {
	case *stringValue:
		if d == "" {
			return ""
		}
		return strconv.Quote(d)
	case *boolValue:
		if d == "false" {
			return ""
		}
	case *intValue, *int64Value, *uint64Value, *float64Value:
		if d == "0" {
			return ""
		}
	case *sliceValue:
		if d == "" {
			return ""
		}
		return "[" + d + "]"
	default:
		if d == "" {
			return ""
		}
	}
	return d
}

// writeRows writes two-column rows, the second column aligned.
func writeRows(b *strings.Builder, rows [][2]string) {
	w := 0
	for _, r := range rows {
		w = max(w, len(r[0]))
	}
	for _, r := range rows {
		if r[1] == "" {
			b.WriteString("  " + r[0] + "\n")
			continue
		}
		b.WriteString("  " + r[0] + strings.Repeat(" ", w-len(r[0])+3) + r[1] + "\n")
	}
}

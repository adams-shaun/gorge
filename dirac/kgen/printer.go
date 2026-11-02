package kgen

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// lit writes v as a Go composite-literal expression in package kop terms.
// Zero struct fields are omitted; enum and mask types print symbolically.
func lit(v reflect.Value) string {
	var b strings.Builder
	writeLit(&b, v)
	return b.String()
}

func typeName(t reflect.Type) string {
	if t.PkgPath() != "" && strings.HasSuffix(t.PkgPath(), "/kop") {
		return "kop." + t.Name()
	}
	return t.String()
}

var valueRefType = reflect.TypeOf(kop.ValueRef{})

// multiline are the struct types printed one field per line.
var multiline = map[reflect.Type]bool{
	reflect.TypeOf(kop.CardDef{}): true,
	reflect.TypeOf(kop.Ability{}): true,
	reflect.TypeOf(kop.Emblem{}):  true,
}

func writeLit(b *strings.Builder, v reflect.Value) {
	t := v.Type()
	if t == valueRefType {
		if vr := v.Interface().(kop.ValueRef); vr.K == kop.VConst {
			fmt.Fprintf(b, "kop.Const(%d)", vr.N)
			return
		}
	}
	if names, ok := enumNames[t]; ok {
		u := toU64(v)
		for _, n := range names {
			if n.v == u {
				b.WriteString(n.name)
				return
			}
		}
		panic(fmt.Sprintf("kgen: %s(%d) has no name in dirac/kgen/names.go", typeName(t), u))
	}
	if names, ok := maskNames[t]; ok {
		u := toU64(v)
		if u == 0 {
			b.WriteString("0")
			return
		}
		var parts []string
		for _, n := range names {
			if u&n.v != 0 && n.v != 0 {
				parts = append(parts, n.name)
				u &^= n.v
			}
		}
		if u != 0 {
			parts = append(parts, fmt.Sprintf("%s(%#x)", typeName(t), u))
		}
		b.WriteString(strings.Join(parts, " | "))
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		multi := multiline[t]
		b.WriteString(typeName(t) + "{")
		first := true
		for i := 0; i < t.NumField(); i++ {
			f := v.Field(i)
			if !t.Field(i).IsExported() || f.IsZero() {
				continue
			}
			switch {
			case multi:
				b.WriteString("\n")
			case !first:
				b.WriteString(", ")
			}
			first = false
			b.WriteString(t.Field(i).Name + ": ")
			if n := t.Field(i).Name; (n == "Targets" || n == "TgtModes") && f.Kind() == reflect.Array && f.Len() == kop.MaxTargets {
				writeArray(b, f, "kop.MaxTargets") // the next bump needs no regen
			} else {
				writeLit(b, f)
			}
			if multi {
				b.WriteString(",")
			}
		}
		if multi && !first {
			b.WriteString("\n")
		}
		b.WriteString("}")
	case reflect.Slice:
		b.WriteString("[]" + typeName(t.Elem()) + "{")
		multi := t.Elem().Kind() == reflect.Struct && t.Elem() != valueRefType
		for i := 0; i < v.Len(); i++ {
			switch {
			case multi:
				b.WriteString("\n")
			case i > 0:
				b.WriteString(", ")
			}
			writeElem(b, v.Index(i))
			if multi {
				b.WriteString(",")
			}
		}
		if multi && v.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("}")
	case reflect.Array:
		writeArray(b, v, strconv.Itoa(v.Len()))
	case reflect.Bool:
		fmt.Fprintf(b, "%v", v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if t.PkgPath() != "" {
			fmt.Fprintf(b, "%s(%d)", typeName(t), v.Int())
		} else {
			fmt.Fprintf(b, "%d", v.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if t.PkgPath() != "" {
			fmt.Fprintf(b, "%s(%d)", typeName(t), v.Uint())
		} else if v.Kind() == reflect.Uint64 && v.Uint() > 255 {
			fmt.Fprintf(b, "%#x", v.Uint())
		} else {
			fmt.Fprintf(b, "%d", v.Uint())
		}
	default:
		panic(fmt.Sprintf("lit: unsupported kind %s", v.Kind()))
	}
}

// writeArray prints array v as a literal whose length is written as n.
func writeArray(b *strings.Builder, v reflect.Value, n string) {
	fmt.Fprintf(b, "[%s]%s{", n, typeName(v.Type().Elem()))
	last := -1
	for i := 0; i < v.Len(); i++ {
		if !v.Index(i).IsZero() {
			last = i
		}
	}
	for i := 0; i <= last; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		writeElem(b, v.Index(i))
	}
	b.WriteString("}")
}

// writeElem writes a slice/array element, eliding the element type of a
// struct literal (Go allows it inside a typed composite literal).
func writeElem(b *strings.Builder, v reflect.Value) {
	if v.Kind() == reflect.Struct && v.Type() != valueRefType {
		if _, ok := enumNames[v.Type()]; !ok {
			s := lit(v)
			b.WriteString(strings.TrimPrefix(s, typeName(v.Type())))
			return
		}
	}
	writeLit(b, v)
}

func toU64(v reflect.Value) uint64 {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(v.Int())
	}
	return v.Uint()
}

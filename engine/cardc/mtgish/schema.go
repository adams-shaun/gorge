package mtgish

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Variant is one `Enum.Variant` of mtgish's ts-rs output. Args is the tuple
// payload type ("" for a unit or struct variant); Fields are a struct
// variant's "Name?: Type" fields.
type Variant struct {
	Enum, Name, Args string
	Fields           []string
}

// Key is "Enum.Variant", the spelling the registry and census use.
func (v Variant) Key() string { return v.Enum + "." + v.Name }

// Schema is the variant catalogue of typescript_types/oracle_cards.d.ts.
type Schema struct {
	Variants []Variant // sorted by key
	index    map[string]int
}

var variantLine = regexp.MustCompile(`^\| \{ "_(\w+)": "(\w+)"(.*)$`)

// ReadSchema parses the union lines of the d.ts file. A variant repeated under
// two TS types (ManaSymbol and ManaSymbolX) is one entry, the first line seen.
func ReadSchema(path string) (Schema, error) {
	f, err := os.Open(path)
	if err != nil {
		return Schema{}, err
	}
	defer f.Close()
	seen := map[string]Variant{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		m := variantLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		v := Variant{Enum: m[1], Name: m[2]}
		rest := strings.TrimSpace(m[3])
		rest = strings.TrimSuffix(rest, ";")
		rest = strings.TrimSpace(strings.TrimSuffix(rest, "}"))
		rest = strings.TrimSpace(strings.TrimSuffix(rest, ","))
		rest = strings.TrimPrefix(rest, ",")
		rest = strings.TrimSpace(rest)
		if a, ok := strings.CutPrefix(rest, `"args":`); ok {
			v.Args = strings.TrimSpace(a)
		} else if rest != "" {
			v.Fields = splitTop(rest)
		}
		if _, dup := seen[v.Key()]; !dup {
			seen[v.Key()] = v
		}
	}
	if err := sc.Err(); err != nil {
		return Schema{}, err
	}
	if len(seen) == 0 {
		return Schema{}, fmt.Errorf("mtgish: %s: no variants found", path)
	}
	var s Schema
	for _, v := range seen {
		s.Variants = append(s.Variants, v)
	}
	sort.Slice(s.Variants, func(i, j int) bool { return s.Variants[i].Key() < s.Variants[j].Key() })
	s.index = make(map[string]int, len(s.Variants))
	for i, v := range s.Variants {
		s.index[v.Key()] = i
	}
	return s, nil
}

// splitTop splits "A: X, B?: Array<Y, Z>" at top-level commas.
func splitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<', '(', '[', '{':
			depth++
		case '>', ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if t := strings.TrimSpace(s[start:]); t != "" {
		out = append(out, t)
	}
	return out
}

// Has reports whether key ("Enum.Variant") is in the schema.
func (s Schema) Has(key string) bool {
	_, ok := s.index[key]
	return ok
}

// FaceKeys derives the struct fields under which a layout nests a face: every
// field of an _OracleCard variant typed Card, FlipInfo or DoorInfo, sorted.
// (Cards: Array<Card> is the loader's separate list case.)
func (s Schema) FaceKeys() []string {
	set := map[string]bool{}
	for _, v := range s.Variants {
		if v.Enum != "OracleCard" {
			continue
		}
		for _, f := range v.Fields {
			name, typ, ok := strings.Cut(f, ":")
			if !ok {
				continue
			}
			switch strings.TrimSpace(typ) {
			case "Card", "FlipInfo", "DoorInfo":
				set[strings.TrimSuffix(strings.TrimSpace(name), "?")] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

package cards

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

// Parse reads one cardsfolder script from disk.
func Parse(path string) (*Card, []Diag) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, []Diag{{path, err.Error()}}
	}
	return ParseBytes(path, src)
}

// ParseBytes parses a script already in memory. Kept separate from Parse so
// tests never need fixture files on disk.
func ParseBytes(path string, src []byte) (*Card, []Diag) {
	c := &Card{Path: path}
	cur := newFace()
	c.Faces = append(c.Faces, cur)
	var diags []Diag

	sc := bufio.NewScanner(bytes.NewReader(src))
	// Initial buffer is the stdlib default (4096): the pinned corpus's longest
	// line is ~911 bytes, so a 64 KiB initial buffer was sixteen times too
	// large and, allocated once per script file, was ParseBytes' single
	// largest allocation (4.25 GB across the whole corpus). A tighter initial
	// buffer never grows here.
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "ALTERNATE" {
			cur = newFace()
			c.Faces = append(c.Faces, cur)
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			diags = append(diags, Diag{path, "unkeyed line: " + line})
			continue
		}
		switch key {
		case "Name":
			cur.Name = val
		case "ManaCost":
			cur.ManaCost = val
		case "Types":
			cur.Types = strings.Fields(val)
		case "PT":
			cur.PT = val
		case "Loyalty":
			cur.Loyalty = val
		case "Defense":
			cur.Defense = val
		case "Colors":
			cur.Colors = val
		case "Oracle":
			cur.Oracle = val
		case "K":
			cur.Keywords = append(cur.Keywords, val)
		case "A":
			sa, d := parseSA(path, val)
			diags = append(diags, d...)
			if sa != nil {
				cur.Abilities = append(cur.Abilities, sa)
			}
		case "T":
			p := parseParams(val)
			cur.Triggers = append(cur.Triggers, Trigger{Mode: p["Mode"], Params: p})
		case "S":
			p := parseParams(val)
			cur.Statics = append(cur.Statics, Static{Mode: p["Mode"], Params: p})
		case "R":
			p := parseParams(val)
			cur.Repls = append(cur.Repls, Repl{Event: p["Event"], Params: p})
		case "SVar":
			name, body, ok := strings.Cut(val, ":")
			if !ok {
				diags = append(diags, Diag{path, "malformed SVar: " + val})
				continue
			}
			cur.SVars[name] = body
		}
		// Unrecognised keys (DeckHas, AI, Draft, ...) are deck-builder and AI
		// hints Forge's own tooling consumes. Ignoring them is correct, not a
		// gap: they carry no rules meaning.
	}
	if err := sc.Err(); err != nil {
		diags = append(diags, Diag{path, err.Error()})
	}
	// derive every face now, once the printed fields are final: ParseBytes is
	// one of the two construction routes into a *Face (the other is the gob
	// decode in LoadRegistry), and both must end with identical derived
	// values.
	for _, f := range c.Faces {
		f.derive()
	}
	return c, diags
}

// parseSA turns "SP$ DealDamage | NumDmg$ 3" into an SA. The leading token is
// the ability kind and its value is the API name.
func parseSA(path, val string) (*SA, []Diag) {
	p := parseParams(val)
	for _, kind := range [...]string{"SP", "AB", "DB", "ST"} {
		if api, ok := p[kind]; ok {
			delete(p, kind)
			return &SA{Kind: kind, API: api, Params: p, Line: val}, nil
		}
	}
	return nil, []Diag{{path, "ability with no SP$/AB$/DB$/ST$ head: " + val}}
}

// parseParams splits a "| Key$ value" chain. Values routinely contain "$" and
// occasionally "|" inside description text, so split on "|" first and then on
// the first "$" only.
func parseParams(val string) map[string]string {
	out := make(map[string]string, 8)
	for _, seg := range strings.Split(val, "|") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		k, v, ok := strings.Cut(seg, "$")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

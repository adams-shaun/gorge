package recstream

import (
	"bytes"
	"errors"
)

// A node is a parsed JSON value that keeps the source text of every scalar
// and key, so writing it back reproduces the source exactly. Nodes are never
// changed once built: applying a patch copies the path it changes and shares
// the rest.
type node struct {
	kind byte     // 'o' object, 'a' array, 's' scalar
	raw  []byte   // scalar: its source text
	keys [][]byte // object: each key's source text, quotes included
	kids []*node  // object values or array elements
}

// style is how a record's text is spaced.
type style byte

const (
	compact style = '+' // {"a":1,"b":[1,2]}
	spaced  style = '~' // {"a": 1, "b": [1, 2]}  (Python's json.dumps default)
)

var errSyntax = errors.New("recstream: invalid JSON")

// parse reads exactly one JSON value from src, allowing surrounding
// whitespace. Scalars and keys alias src.
func parse(src []byte) (*node, error) {
	p := parser{src: src}
	p.ws()
	n, err := p.value(0)
	if err != nil {
		return nil, err
	}
	p.ws()
	if p.i != len(p.src) {
		return nil, errSyntax
	}
	return n, nil
}

type parser struct {
	src []byte
	i   int
}

const maxDepth = 512

func (p *parser) ws() {
	for p.i < len(p.src) {
		switch p.src[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *parser) value(depth int) (*node, error) {
	if p.i >= len(p.src) || depth > maxDepth {
		return nil, errSyntax
	}
	switch c := p.src[p.i]; {
	case c == '{':
		return p.object(depth)
	case c == '[':
		return p.array(depth)
	case c == '"':
		s, err := p.str()
		if err != nil {
			return nil, err
		}
		return &node{kind: 's', raw: s}, nil
	default:
		start := p.i
		for p.i < len(p.src) {
			c := p.src[p.i]
			if c == ',' || c == '}' || c == ']' || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ':' {
				break
			}
			p.i++
		}
		lit := p.src[start:p.i]
		if !validLiteral(lit) {
			return nil, errSyntax
		}
		return &node{kind: 's', raw: lit}, nil
	}
}

func validLiteral(b []byte) bool {
	switch string(b) {
	case "true", "false", "null":
		return true
	}
	// JSON number: -?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?
	i := 0
	if i < len(b) && b[i] == '-' {
		i++
	}
	if i >= len(b) {
		return false
	}
	if b[i] == '0' {
		i++
	} else if b[i] >= '1' && b[i] <= '9' {
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			i++
		}
	} else {
		return false
	}
	if i < len(b) && b[i] == '.' {
		i++
		d := i
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			i++
		}
		if i == d {
			return false
		}
	}
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		d := i
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			i++
		}
		if i == d {
			return false
		}
	}
	return i == len(b)
}

// str returns a string literal's source text, quotes included.
func (p *parser) str() ([]byte, error) {
	start := p.i
	p.i++
	for p.i < len(p.src) {
		switch c := p.src[p.i]; {
		case c == '"':
			p.i++
			return p.src[start:p.i], nil
		case c == '\\':
			if p.i+1 >= len(p.src) {
				return nil, errSyntax
			}
			p.i += 2
		case c < 0x20:
			return nil, errSyntax
		default:
			p.i++
		}
	}
	return nil, errSyntax
}

func (p *parser) object(depth int) (*node, error) {
	n := &node{kind: 'o'}
	p.i++
	p.ws()
	if p.i < len(p.src) && p.src[p.i] == '}' {
		p.i++
		return n, nil
	}
	for {
		p.ws()
		if p.i >= len(p.src) || p.src[p.i] != '"' {
			return nil, errSyntax
		}
		k, err := p.str()
		if err != nil {
			return nil, err
		}
		p.ws()
		if p.i >= len(p.src) || p.src[p.i] != ':' {
			return nil, errSyntax
		}
		p.i++
		p.ws()
		v, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		n.keys = append(n.keys, k)
		n.kids = append(n.kids, v)
		p.ws()
		if p.i >= len(p.src) {
			return nil, errSyntax
		}
		switch p.src[p.i] {
		case ',':
			p.i++
		case '}':
			p.i++
			return n, nil
		default:
			return nil, errSyntax
		}
	}
}

func (p *parser) array(depth int) (*node, error) {
	n := &node{kind: 'a'}
	p.i++
	p.ws()
	if p.i < len(p.src) && p.src[p.i] == ']' {
		p.i++
		return n, nil
	}
	for {
		p.ws()
		v, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		n.kids = append(n.kids, v)
		p.ws()
		if p.i >= len(p.src) {
			return nil, errSyntax
		}
		switch p.src[p.i] {
		case ',':
			p.i++
		case ']':
			p.i++
			return n, nil
		default:
			return nil, errSyntax
		}
	}
}

// write appends n's text in style st.
func (n *node) write(b []byte, st style) []byte {
	switch n.kind {
	case 's':
		return append(b, n.raw...)
	case 'a':
		b = append(b, '[')
		for i, k := range n.kids {
			if i > 0 {
				b = sep(b, st)
			}
			b = k.write(b, st)
		}
		return append(b, ']')
	default:
		b = append(b, '{')
		for i, k := range n.kids {
			if i > 0 {
				b = sep(b, st)
			}
			b = append(b, n.keys[i]...)
			b = append(b, ':')
			if st == spaced {
				b = append(b, ' ')
			}
			b = k.write(b, st)
		}
		return append(b, '}')
	}
}

func sep(b []byte, st style) []byte {
	if st == spaced {
		return append(b, ',', ' ')
	}
	return append(b, ',')
}

// equal reports whether a and b have the same text in any one style.
func equal(a, b *node) bool {
	if a == b {
		return true
	}
	if a.kind != b.kind || len(a.kids) != len(b.kids) {
		return false
	}
	if a.kind == 's' {
		return bytes.Equal(a.raw, b.raw)
	}
	for i := range a.kids {
		if a.kind == 'o' && !bytes.Equal(a.keys[i], b.keys[i]) {
			return false
		}
		if !equal(a.kids[i], b.kids[i]) {
			return false
		}
	}
	return true
}

// deltaable reports whether a record parsed from text may be stored as a
// delta, and in which style: it must be an object, reproduce text exactly,
// and have no duplicate or escaped keys at any depth.
func deltaable(n *node, text []byte) (style, bool) {
	if n.kind != 'o' || !cleanKeys(n) {
		return 0, false
	}
	var buf []byte
	for _, st := range []style{compact, spaced} {
		buf = n.write(buf[:0], st)
		if bytes.Equal(buf, text) {
			return st, true
		}
	}
	return 0, false
}

func cleanKeys(n *node) bool {
	if n.kind == 'o' {
		if len(n.keys) > 1 {
			seen := make(map[string]struct{}, len(n.keys))
			for _, k := range n.keys {
				if _, dup := seen[string(k)]; dup {
					return false
				}
				seen[string(k)] = struct{}{}
			}
		}
		for _, k := range n.keys {
			if bytes.IndexByte(k, '\\') >= 0 {
				return false
			}
		}
	}
	for _, k := range n.kids {
		if k.kind != 's' && !cleanKeys(k) {
			return false
		}
	}
	return true
}

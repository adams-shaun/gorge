package recstream

import (
	"bytes"
	"errors"
	"strconv"
)

// maxEdits bounds the array edit script diffArray looks for; past it the
// array is spliced or replaced whole, which costs bytes, never correctness.
const maxEdits = 64

// diff returns the PATCH that turns record prev into record cur. Both are
// objects that passed deltaable. ok is false when cur cannot be expressed as
// a patch of prev (its keys moved), and the caller writes a keyframe.
func diff(prev, cur *node) (patch []byte, ok bool) {
	if prev == nil {
		prev = &node{kind: 'o'}
	}
	return diffObject(nil, prev, cur)
}

// diffValue appends the op that turns prev into cur. same reports that
// they are equal, and nothing was appended.
func diffValue(b []byte, prev, cur *node) (out []byte, same bool) {
	if equal(prev, cur) {
		return b, true
	}
	if cur.kind == 's' {
		return append(b, cur.raw...), false
	}
	start := len(b)
	b = append(b, "[0,"...)
	b = cur.write(b, compact)
	b = append(b, ']')
	best := append([]byte(nil), b[start:]...)
	try := func(cand []byte) {
		if len(cand) < len(best) {
			best = cand
		}
	}
	switch {
	case prev.kind == 'o' && cur.kind == 'o':
		if p, ok := diffObject([]byte("[1,"), prev, cur); ok {
			try(append(p, ']'))
		}
	case prev.kind == 'a' && cur.kind == 'a':
		for _, cand := range diffArray(prev.kids, cur.kids) {
			try(cand)
		}
	}
	if c, ok := chain(cur); ok {
		try(c)
	}
	return append(b[:start], best...), false
}

// newValue appends the op that sets a value with no predecessor: the
// scalar itself, [0,V], or a chain when that is shorter.
func newValue(b []byte, v *node) []byte {
	if v.kind == 's' {
		return append(b, v.raw...)
	}
	start := len(b)
	b = append(b, "[0,"...)
	b = v.write(b, compact)
	b = append(b, ']')
	if c, ok := chain(v); ok && len(c) < len(b)-start {
		return append(b[:start], c...)
	}
	return b
}

// chain encodes an array whose elements are objects or arrays as its first
// element followed by each element's op against the one before it ([] for
// a repeat): [5,V0,OP1,OP2,...]. ok is false for arrays it cannot help.
func chain(v *node) ([]byte, bool) {
	if v.kind != 'a' || len(v.kids) < 2 {
		return nil, false
	}
	structured := false
	for _, k := range v.kids {
		if k.kind != 's' {
			structured = true
			break
		}
	}
	if !structured {
		return nil, false
	}
	b := []byte("[5,")
	b = newValue(b, v.kids[0])
	for i := 1; i < len(v.kids); i++ {
		b = append(b, ',')
		var same bool
		if b, same = diffValue(b, v.kids[i-1], v.kids[i]); same {
			b = append(b, "[]"...)
		}
	}
	return append(b, ']'), true
}

// diffObject appends an object PATCH, or reports !ok when cur's keys are
// not prev's keys in prev's order followed by new keys.
func diffObject(b []byte, prev, cur *node) ([]byte, bool) {
	idx := make(map[string]int, len(prev.keys))
	for i, k := range prev.keys {
		idx[string(k)] = i
	}
	last, fresh := -1, false
	present := make([]bool, len(prev.keys))
	for _, k := range cur.keys {
		i, ok := idx[string(k)]
		if !ok {
			fresh = true
			continue
		}
		if fresh || i < last {
			return b, false
		}
		last = i
		present[i] = true
	}
	b = append(b, '{')
	n := 0
	for j, k := range cur.keys {
		mark := len(b)
		if n > 0 {
			b = append(b, ',')
		}
		b = append(b, k...)
		b = append(b, ':')
		if i, ok := idx[string(k)]; ok {
			var same bool
			if b, same = diffValue(b, prev.kids[i], cur.kids[j]); same {
				b = b[:mark]
				continue
			}
		} else {
			b = newValue(b, cur.kids[j])
		}
		n++
	}
	for i, k := range prev.keys {
		if present[i] {
			continue
		}
		if n > 0 {
			b = append(b, ',')
		}
		b = append(b, k...)
		b = append(b, ":[]"...)
		n++
	}
	return append(b, '}'), true
}

// diffArray returns candidate array ops: a splice around the common prefix
// and suffix, an element-wise patch when the lengths match, and an edit
// script when one of at most maxEdits edits exists.
func diffArray(a, c []*node) [][]byte {
	var out [][]byte
	p := 0
	for p < len(a) && p < len(c) && equal(a[p], c[p]) {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(c)-p && equal(a[len(a)-1-s], c[len(c)-1-s]) {
		s++
	}
	sp := append([]byte("[2,"), strconv.Itoa(p)...)
	sp = append(sp, ',')
	sp = strconv.AppendInt(sp, int64(s), 10)
	sp = append(sp, ",["...)
	sp = writeList(sp, c[p:len(c)-s])
	out = append(out, append(sp, "]]"...))

	if len(a) == len(c) {
		ew := []byte("[3,{")
		n := 0
		for i := p; i < len(a)-s; i++ {
			mark := len(ew)
			if n > 0 {
				ew = append(ew, ',')
			}
			ew = append(ew, '"')
			ew = strconv.AppendInt(ew, int64(i), 10)
			ew = append(ew, '"', ':')
			var same bool
			if ew, same = diffValue(ew, a[i], c[i]); same {
				ew = ew[:mark]
				continue
			}
			n++
		}
		out = append(out, append(ew, "}]"...))
	}

	if hunks, ok := editScript(a[p:len(a)-s], c[p:len(c)-s]); ok && len(hunks) > 1 {
		es := []byte("[4")
		for _, h := range hunks {
			es = append(es, ',')
			es = strconv.AppendInt(es, int64(p+h.at), 10)
			es = append(es, ',')
			es = strconv.AppendInt(es, int64(h.del), 10)
			es = append(es, ",["...)
			es = writeList(es, c[p+h.from:p+h.from+h.ins])
			es = append(es, ']')
		}
		out = append(out, append(es, ']'))
	}
	return out
}

func writeList(b []byte, xs []*node) []byte {
	for i, x := range xs {
		if i > 0 {
			b = append(b, ',')
		}
		b = x.write(b, compact)
	}
	return b
}

// A hunk replaces old[at:at+del] with new[from:from+ins].
type hunk struct{ at, del, from, ins int }

// editScript is Myers' O(ND) diff, giving up past maxEdits edits.
func editScript(a, c []*node) ([]hunk, bool) {
	n, m := len(a), len(c)
	maxD := min(maxEdits, n+m)
	off := maxD + 1
	v := make([]int, 2*maxD+3)
	var trace [][]int
	for d := 0; d <= maxD; d++ {
		trace = append(trace, append([]int(nil), v...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[k-1+off] < v[k+1+off]) {
				x = v[k+1+off]
			} else {
				x = v[k-1+off] + 1
			}
			y := x - k
			for x < n && y < m && equal(a[x], c[y]) {
				x++
				y++
			}
			v[k+off] = x
			if x >= n && y >= m {
				return backtrack(trace, n, m, off), true
			}
		}
	}
	return nil, false
}

func backtrack(trace [][]int, x, y, off int) []hunk {
	type edit struct{ old, new int } // new < 0: delete a[old]; else insert c[new] before a[old]
	var edits []edit
	for d := len(trace) - 1; d > 0; d-- {
		v := trace[d]
		k := x - y
		var pk int
		if k == -d || (k != d && v[k-1+off] < v[k+1+off]) {
			pk = k + 1
		} else {
			pk = k - 1
		}
		px := v[pk+off]
		py := px - pk
		for x > px && y > py {
			x--
			y--
		}
		if x == px {
			edits = append(edits, edit{old: x, new: py})
		} else {
			edits = append(edits, edit{old: px, new: -1})
		}
		x, y = px, py
	}
	var hs []hunk
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		var h *hunk
		if len(hs) > 0 && hs[len(hs)-1].at+hs[len(hs)-1].del == e.old {
			h = &hs[len(hs)-1]
		} else {
			// from: the new-array index where this hunk's insertions start,
			// which is e.old shifted by the net edits so far.
			shift := 0
			for _, q := range hs {
				shift += q.ins - q.del
			}
			hs = append(hs, hunk{at: e.old, from: e.old + shift})
			h = &hs[len(hs)-1]
		}
		if e.new < 0 {
			h.del++
		} else {
			h.ins++
		}
	}
	return hs
}

var errPatch = errors.New("recstream: malformed patch")

// apply returns base with the object PATCH p applied. base is not changed.
func apply(base, p *node) (*node, error) {
	if base == nil || base.kind != 'o' || p.kind != 'o' {
		return nil, errPatch
	}
	out := &node{kind: 'o', keys: append([][]byte(nil), base.keys...), kids: append([]*node(nil), base.kids...)}
	var gone map[int]bool
	for j, k := range p.keys {
		op := p.kids[j]
		i := -1
		for x := range out.keys {
			if bytes.Equal(out.keys[x], k) {
				i = x
				break
			}
		}
		if op.kind == 'a' && len(op.kids) == 0 {
			if i < 0 {
				return nil, errPatch
			}
			if gone == nil {
				gone = map[int]bool{}
			}
			gone[i] = true
			continue
		}
		var old *node
		if i >= 0 {
			old = out.kids[i]
		}
		v, err := applyOp(old, op)
		if err != nil {
			return nil, err
		}
		if i >= 0 {
			out.kids[i] = v
		} else {
			out.keys = append(out.keys, k)
			out.kids = append(out.kids, v)
		}
	}
	if gone != nil {
		keys, kids := out.keys[:0:0], out.kids[:0:0]
		for i := range out.keys {
			if !gone[i] {
				keys = append(keys, out.keys[i])
				kids = append(kids, out.kids[i])
			}
		}
		out.keys, out.kids = keys, kids
	}
	return out, nil
}

func applyOp(old, op *node) (*node, error) {
	if op.kind != 'a' {
		return op, nil
	}
	if len(op.kids) < 2 || op.kids[0].kind != 's' {
		return nil, errPatch
	}
	args := op.kids[1:]
	switch string(op.kids[0].raw) {
	case "0":
		return args[0], nil
	case "1":
		return apply(old, args[0])
	case "2":
		if old == nil || old.kind != 'a' || len(args) != 3 || args[2].kind != 'a' {
			return nil, errPatch
		}
		p, err1 := strconv.Atoi(string(args[0].raw))
		s, err2 := strconv.Atoi(string(args[1].raw))
		if err1 != nil || err2 != nil || p < 0 || s < 0 || p+s > len(old.kids) {
			return nil, errPatch
		}
		kids := make([]*node, 0, p+s+len(args[2].kids))
		kids = append(kids, old.kids[:p]...)
		kids = append(kids, args[2].kids...)
		kids = append(kids, old.kids[len(old.kids)-s:]...)
		return &node{kind: 'a', kids: kids}, nil
	case "3":
		if old == nil || old.kind != 'a' || args[0].kind != 'o' {
			return nil, errPatch
		}
		kids := append([]*node(nil), old.kids...)
		for j, k := range args[0].keys {
			i, err := strconv.Atoi(string(k[1 : len(k)-1]))
			if err != nil || i < 0 || i >= len(kids) {
				return nil, errPatch
			}
			if kids[i], err = applyOp(kids[i], args[0].kids[j]); err != nil {
				return nil, err
			}
		}
		return &node{kind: 'a', kids: kids}, nil
	case "5":
		kids := make([]*node, len(args))
		for i, a := range args {
			switch {
			case i == 0:
				v, err := applyOp(nil, a)
				if err != nil {
					return nil, err
				}
				kids[0] = v
			case a.kind == 'a' && len(a.kids) == 0:
				kids[i] = kids[i-1]
			default:
				v, err := applyOp(kids[i-1], a)
				if err != nil {
					return nil, err
				}
				kids[i] = v
			}
		}
		return &node{kind: 'a', kids: kids}, nil
	case "4":
		if old == nil || old.kind != 'a' || len(args)%3 != 0 {
			return nil, errPatch
		}
		var kids []*node
		at := 0
		for h := 0; h < len(args); h += 3 {
			p, err1 := strconv.Atoi(string(args[h].raw))
			d, err2 := strconv.Atoi(string(args[h+1].raw))
			if err1 != nil || err2 != nil || p < at || d < 0 || p+d > len(old.kids) || args[h+2].kind != 'a' {
				return nil, errPatch
			}
			kids = append(kids, old.kids[at:p]...)
			kids = append(kids, args[h+2].kids...)
			at = p + d
		}
		kids = append(kids, old.kids[at:]...)
		return &node{kind: 'a', kids: kids}, nil
	}
	return nil, errPatch
}

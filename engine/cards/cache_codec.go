package cards

import (
	"encoding/binary"
	"errors"
	"sort"
)

// The face codec: Face's canonical cache encoding (cache_canon.go's
// Face.GobEncode/GobDecode) written as one flat, deterministic byte string
// per face instead of a nested gob stream per map-bearing object.
//
// The canonical gob pairs gave every SA, Trigger, Static, Repl and Face its
// own gob.Encoder/Decoder, so each nested object re-sent its type
// descriptors and every nesting level copied its children's bytes again on
// decode: the uncompressed cache grew 36.7 MB -> 67 MB, a corpus open
// allocated ~540 MB decoding it (113 MB before), and the millions of
// short-lived decoder objects interleaved with the registry's long-lived
// ones left the heap ~5x more fragmented, which raised every later GC's mark
// cost (enginebench clone/step rows, 2026-10-13). This codec keeps the
// determinism -- every map is written in sorted key order -- with one
// append-only buffer per face and no type tables.
//
// Layout: uvarint lengths and counts, strings as length + bytes, a map as
// its sorted (key, value) pairs, an *SA as a presence byte then its fields.
// A zero-length slice or map decodes to nil, as gob decoded it. Adding an
// exported field to Face, SA, Trigger, Static or Repl means adding it to
// both halves here (TestFaceCodecRoundTrips walks every exported field).

type faceEncoder struct{ b []byte }

func (e *faceEncoder) n(v int) { e.b = binary.AppendUvarint(e.b, uint64(v)) }

func (e *faceEncoder) str(s string) {
	e.n(len(s))
	e.b = append(e.b, s...)
}

func (e *faceEncoder) strs(ss []string) {
	e.n(len(ss))
	for _, s := range ss {
		e.str(s)
	}
}

func (e *faceEncoder) kvs(m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	e.n(len(keys))
	for _, k := range keys {
		e.str(k)
		e.str(m[k])
	}
}

func (e *faceEncoder) sa(s *SA) {
	if s == nil {
		e.b = append(e.b, 0)
		return
	}
	e.b = append(e.b, 1)
	e.str(s.Kind)
	e.str(s.API)
	e.kvs(s.Params)
	e.sa(s.Sub)
	e.str(s.Line)
}

func (e *faceEncoder) face(f *Face) {
	e.str(f.SpecializeColor)
	e.str(f.CopyFaceFrom)
	e.str(f.Name)
	e.str(f.ManaCost)
	e.strs(f.Types)
	e.str(f.PT)
	e.str(f.Loyalty)
	e.str(f.Defense)
	e.str(f.Colors)
	e.str(f.Oracle)
	e.strs(f.Keywords)
	e.strs(f.Aliases)
	e.str(f.RemoveDeck)
	e.n(len(f.Abilities))
	for _, s := range f.Abilities {
		e.sa(s)
	}
	e.n(len(f.Triggers))
	for i := range f.Triggers {
		t := &f.Triggers[i]
		e.str(t.Mode)
		e.kvs(t.Params)
		e.sa(t.Effect)
	}
	e.n(len(f.Statics))
	for i := range f.Statics {
		s := &f.Statics[i]
		e.str(s.Mode)
		e.kvs(s.Params)
	}
	e.n(len(f.Repls))
	for i := range f.Repls {
		r := &f.Repls[i]
		e.str(r.Event)
		e.kvs(r.Params)
		e.sa(r.With)
	}
	e.kvs(f.SVars)
}

var errFaceCodec = errors.New("cards: malformed face encoding")

// faceDecoder reads a faceEncoder's bytes. The first malformed read sets err
// and every later read returns zero values, so callers check err once.
type faceDecoder struct {
	b   []byte
	err error
}

func (d *faceDecoder) fail() { d.err, d.b = errFaceCodec, nil }

// n reads a count or length; it can never exceed the bytes left, since
// every element takes at least one byte.
func (d *faceDecoder) n() int {
	v, k := binary.Uvarint(d.b)
	if k <= 0 || v > uint64(len(d.b)-k) {
		d.fail()
		return 0
	}
	d.b = d.b[k:]
	return int(v)
}

func (d *faceDecoder) str() string {
	l := d.n()
	if l == 0 {
		return ""
	}
	s := string(d.b[:l])
	d.b = d.b[l:]
	return s
}

func (d *faceDecoder) strs() []string {
	n := d.n()
	if n == 0 {
		return nil
	}
	out := make([]string, n)
	for i := range out {
		out[i] = d.str()
	}
	return out
}

func (d *faceDecoder) kvs() map[string]string {
	n := d.n()
	if n == 0 {
		return nil
	}
	m := make(map[string]string, n)
	for i := 0; i < n && d.err == nil; i++ {
		k := d.str()
		m[k] = d.str()
	}
	return m
}

func (d *faceDecoder) sa() *SA {
	if len(d.b) == 0 {
		d.fail()
		return nil
	}
	tag := d.b[0]
	d.b = d.b[1:]
	switch tag {
	case 0:
		return nil
	case 1:
	default:
		d.fail()
		return nil
	}
	s := &SA{}
	s.Kind = d.str()
	s.API = d.str()
	s.Params = d.kvs()
	s.Sub = d.sa()
	s.Line = d.str()
	return s
}

func (d *faceDecoder) face(f *Face) {
	f.SpecializeColor = d.str()
	f.CopyFaceFrom = d.str()
	f.Name = d.str()
	f.ManaCost = d.str()
	f.Types = d.strs()
	f.PT = d.str()
	f.Loyalty = d.str()
	f.Defense = d.str()
	f.Colors = d.str()
	f.Oracle = d.str()
	f.Keywords = d.strs()
	f.Aliases = d.strs()
	f.RemoveDeck = d.str()
	if n := d.n(); n > 0 {
		f.Abilities = make([]*SA, n)
		for i := range f.Abilities {
			f.Abilities[i] = d.sa()
		}
	}
	if n := d.n(); n > 0 {
		f.Triggers = make([]Trigger, n)
		for i := range f.Triggers {
			t := &f.Triggers[i]
			t.Mode = d.str()
			t.Params = d.kvs()
			t.Effect = d.sa()
		}
	}
	if n := d.n(); n > 0 {
		f.Statics = make([]Static, n)
		for i := range f.Statics {
			s := &f.Statics[i]
			s.Mode = d.str()
			s.Params = d.kvs()
		}
	}
	if n := d.n(); n > 0 {
		f.Repls = make([]Repl, n)
		for i := range f.Repls {
			r := &f.Repls[i]
			r.Event = d.str()
			r.Params = d.kvs()
			r.With = d.sa()
		}
	}
	f.SVars = d.kvs()
	if d.err == nil && len(d.b) != 0 {
		d.fail()
	}
}

package recstream

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// roundTrip packs recs and reads them back, failing on any difference.
func roundTrip(t testing.TB, recs [][]byte, opt Options) Stats {
	t.Helper()
	var buf bytes.Buffer
	w, err := NewWriter(&buf, opt)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if err := w.WriteRecord(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := NewScanner(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Condensed() {
		t.Fatal("output not detected as condensed")
	}
	for i, want := range recs {
		got, err := s.Next()
		if err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("record %d:\n got %s\nwant %s", i, got, want)
		}
	}
	if _, err := s.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF after %d records, got %v", len(recs), err)
	}
	return w.Stats()
}

func lines(s ...string) [][]byte {
	out := make([][]byte, len(s))
	for i := range s {
		out[i] = []byte(s[i])
	}
	return out
}

func TestRoundTripCases(t *testing.T) {
	cases := map[string][]string{
		"scalars":   {`{"a":1,"b":"x","c":true}`, `{"a":2,"b":"x","c":false}`, `{"a":2,"b":null,"c":false}`},
		"add-del":   {`{"a":1}`, `{"a":1,"b":2}`, `{"b":2}`, `{"b":2,"a":[]}`, `{"b":2,"a":{}}`},
		"reorder":   {`{"a":1,"b":2}`, `{"b":2,"a":1}`, `{"c":0,"b":2,"a":1}`},
		"nested":    {`{"s":{"x":1,"y":{"z":[1,2]}}}`, `{"s":{"x":1,"y":{"z":[1,3]}}}`, `{"s":{"y":{"z":[]},"x":1}}`},
		"arrays":    {`{"r":[1,2,3,4,5,6,7,8,9]}`, `{"r":[1,2,30,4,5,6,7,80,9]}`, `{"r":[0,1,2,30,4,6,7,80,9,10]}`, `{"r":[]}`, `{"r":[[1],[2]]}`, `{"r":[[1],[2,3]]}`},
		"objarrays": {`{"p":[{"s":0,"l":20},{"s":1,"l":20}]}`, `{"p":[{"s":0,"l":17},{"s":1,"l":20}]}`, `{"p":[{"s":0,"l":17},{"s":1,"l":20},{"s":2,"l":5}]}`},
		"spaced":    {`{"a": 1, "b": [1, 2]}`, `{"a": 2, "b": [1, 2, 3]}`, `{"a":3,"b":[1]}`, `{"a": 4, "b": [1]}`},
		"verbatim":  {`[1,2]`, `{"a":1}`, ``, `not json`, `{"a":2}`, ` {"a":2}`, "{\"a\":2}\r", `{"a":1,"a":2}`, `{"a\u0062":1}`, `{"a":3}`},
		"numbers":   {`{"n":1e400,"m":-0.0,"k":18446744073709551615}`, `{"n":1E5,"m":0.075,"k":12345678901234567890123}`},
		"strings":   {`{"s":"\u003cb\u003e","t":"é\n\"q\"","u":"\ud83d\ude00"}`, `{"s":"<b>","t":"é","u":""}`},
		"deep":      {`{"a":{"b":{"c":{"d":{"e":[{"f":[1,{"g":2}]}]}}}}}`, `{"a":{"b":{"c":{"d":{"e":[{"f":[1,{"g":3}]}]}}}}}`},
	}
	for name, recs := range cases {
		t.Run(name, func(t *testing.T) {
			for _, k := range []int{1, 2, 256} {
				roundTrip(t, lines(recs...), Options{Keyframe: k})
			}
		})
	}
}

// rng is a fixed xorshift generator: the tests stay deterministic without
// math/rand.
type rng uint64

func (r *rng) next(n int) int {
	*r ^= *r << 13
	*r ^= *r >> 7
	*r ^= *r << 17
	return int(uint64(*r) % uint64(n))
}

func randValue(r *rng, depth int) string {
	switch k := r.next(9); {
	case depth > 3 || k < 4:
		return []string{"0", "1", "-2.5", "true", "null", `"x"`, `"yy"`, "1e3", `""`}[r.next(9)]
	case k < 6:
		n := r.next(6)
		parts := make([]string, n)
		for i := range parts {
			parts[i] = randValue(r, depth+1)
		}
		return "[" + strings.Join(parts, ",") + "]"
	default:
		n := r.next(5)
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fmt.Sprintf(`"k%d":%s`, i, randValue(r, depth+1))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
}

// mutate changes a record a little, as consecutive log records do.
func mutate(r *rng, rec string) string {
	n, err := parse([]byte(rec))
	if err != nil {
		panic(err)
	}
	var walk func(n *node, depth int) *node
	walk = func(n *node, depth int) *node {
		if r.next(4) == 0 || n.kind == 's' {
			v, _ := parse([]byte(randValue(r, depth)))
			return v
		}
		out := &node{kind: n.kind, keys: append([][]byte(nil), n.keys...), kids: append([]*node(nil), n.kids...)}
		if len(out.kids) > 0 {
			i := r.next(len(out.kids))
			switch r.next(4) {
			case 0:
				out.kids = append(out.kids[:i], out.kids[i+1:]...)
				if n.kind == 'o' {
					out.keys = append(out.keys[:i], out.keys[i+1:]...)
				}
			default:
				out.kids[i] = walk(out.kids[i], depth+1)
			}
		}
		if r.next(3) == 0 {
			v, _ := parse([]byte(randValue(r, depth+1)))
			out.kids = append(out.kids, v)
			if n.kind == 'o' {
				out.keys = append(out.keys, []byte(strconv.Quote("n"+strconv.Itoa(r.next(1000)))))
			}
		}
		return out
	}
	m := walk(n, 0)
	if m.kind != 'o' || !cleanKeys(m) {
		return rec
	}
	return string(m.write(nil, compact))
}

func TestRoundTripRandomWalk(t *testing.T) {
	r := rng(0x9e3779b97f4a7c15)
	for trial := 0; trial < 50; trial++ {
		rec := `{"base":` + randValue(&r, 0) + `,"seq":0}`
		var recs [][]byte
		for i := 0; i < 200; i++ {
			recs = append(recs, []byte(rec))
			rec = mutate(&r, rec)
		}
		st := roundTrip(t, recs, Options{Keyframe: 1 + r.next(64)})
		if st.Records != len(recs) {
			t.Fatalf("stats %+v", st)
		}
	}
}

func TestDeltasActuallyHappen(t *testing.T) {
	var recs [][]byte
	for i := 0; i < 100; i++ {
		recs = append(recs, []byte(fmt.Sprintf(`{"game":"g1","deck":"%s","seq":%d,"rows":[1,2,3,%d,5,6,7,8,9,10,11,12]}`, strings.Repeat("Plains|", 30), i, i)))
	}
	st := roundTrip(t, recs, Options{Keyframe: 10})
	if st.Keyframes != 10 || st.Deltas != 90 {
		t.Fatalf("want 10 keyframes and 90 deltas, got %+v", st)
	}
	if st.FrameBytes*4 > st.RecordBytes {
		t.Fatalf("frames %d bytes for %d record bytes: deltas are not condensing", st.FrameBytes, st.RecordBytes)
	}
}

func TestMyersHunks(t *testing.T) {
	recs := lines(
		`{"r":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]}`,
		`{"r":[1,2,3,99,4,5,6,7,8,9,10,11,12,14,15,16,17,18,19,20,21]}`,
		`{"r":[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]}`,
	)
	roundTrip(t, recs, Options{})
	a, _ := parse(recs[0])
	c, _ := parse(recs[1])
	p, _ := diff(a, c)
	if !bytes.Contains(p, []byte("[4,")) {
		t.Fatalf("expected an edit-script op, got %s", p)
	}
}

// Arrays of similar objects (a game's snapshots) chain each element off the
// one before it, inside one record as well as across records.
func TestChainedSnapshots(t *testing.T) {
	var snaps []string
	for i := 0; i < 40; i++ {
		snaps = append(snaps, fmt.Sprintf(`{"turn":%d,"step":"main1","players":[{"seat":0,"life":%d,"hand":["Plains","Swamp","Darkstar Augur"]},{"seat":1,"life":20,"hand":["Island"]}]}`, i/4, 20-i/8))
	}
	rec := []byte(`{"id":"g1","snapshots":[` + strings.Join(snaps, ",") + `]}`)
	rec2 := bytes.Replace(rec, []byte(`"id":"g1"`), []byte(`"id":"g2"`), 1)
	st := roundTrip(t, [][]byte{rec, rec2, []byte(`{"id":"g3","snapshots":[1,{"a":[]},{"a":[]},[],[]]}`)}, Options{NoAdaptive: true})
	if st.FrameBytes*5 > st.RecordBytes {
		t.Fatalf("frames %d bytes for %d record bytes: snapshots are not chaining", st.FrameBytes, st.RecordBytes)
	}
	n, _ := parse(rec)
	p, _ := diff(nil, n)
	if !bytes.Contains(p, []byte(`"snapshots":[5,`)) {
		t.Fatalf("keyframe patch does not chain: %.200s", p)
	}
}

func TestWriterAsIOWriterAndPlainReaders(t *testing.T) {
	src := "{\"a\":1}\n{\"a\":2}\n{\"a\":3}"
	var buf bytes.Buffer
	w, _ := NewWriter(&buf, Options{})
	io.WriteString(w, src[:5])
	io.WriteString(w, src[5:])
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	if string(got) != src+"\n" {
		t.Fatalf("got %q", got)
	}
	// Plain and gzipped JSON Lines pass through byte for byte.
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	io.WriteString(zw, src)
	zw.Close()
	for name, in := range map[string][]byte{"plain": []byte(src), "gzip": gz.Bytes()} {
		r, err := NewReader(bytes.NewReader(in))
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(r)
		if string(got) != src {
			t.Fatalf("%s: got %q, want it byte for byte", name, got)
		}
	}
}

// A damaged tail loses only the records of the damaged member.
func TestTruncatedTailKeepsEarlierMembers(t *testing.T) {
	var recs [][]byte
	for i := 0; i < 1000; i++ {
		recs = append(recs, []byte(fmt.Sprintf(`{"i":%d,"pad":"%s"}`, i, strings.Repeat("z", i%50))))
	}
	var buf bytes.Buffer
	w, _ := NewWriter(&buf, Options{Keyframe: 100})
	for _, r := range recs {
		w.WriteRecord(r)
	}
	w.Close()
	cut := buf.Bytes()[:buf.Len()-200]
	s, _ := NewScanner(bytes.NewReader(cut))
	n := 0
	for {
		rec, err := s.Next()
		if err != nil {
			break
		}
		if !bytes.Equal(rec, recs[n]) {
			t.Fatalf("record %d differs", n)
		}
		n++
	}
	if n < 900 {
		t.Fatalf("only %d records survived a 200-byte truncation", n)
	}
}

// TestRoundTripRealSamples round-trips every *.jsonl under
// $RECSTREAM_SAMPLES (the fleet's measured samples); it skips without it.
func TestRoundTripRealSamples(t *testing.T) {
	dir := os.Getenv("RECSTREAM_SAMPLES")
	if dir == "" {
		t.Skip("RECSTREAM_SAMPLES not set")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if len(files) == 0 {
		t.Fatalf("no *.jsonl under %s", dir)
	}
	for _, f := range files {
		in, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		src, err := NewScanner(in)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		w, _ := NewWriter(&buf, Options{})
		var want [][]byte
		for len(want) < 20000 {
			rec, err := src.Next()
			if err != nil {
				break
			}
			want = append(want, append([]byte(nil), rec...))
			w.WriteRecord(rec)
		}
		in.Close()
		w.Close()
		s, _ := NewScanner(&buf)
		for i := range want {
			got, err := s.Next()
			if err != nil || !bytes.Equal(got, want[i]) {
				t.Fatalf("%s record %d: %v", f, i, err)
			}
		}
		st := w.Stats()
		t.Logf("%s: %d records, %d deltas, %d -> %d bytes", filepath.Base(f), st.Records, st.Deltas, st.RecordBytes, buf.Len())
	}
}

func FuzzPair(f *testing.F) {
	f.Add(`{"a":[1,2,3]}`, `{"a":[1,3,4],"b":{}}`)
	f.Add(`{"a": {"b": 1}}`, `{"a": {"b": 2, "c": null}}`)
	f.Fuzz(func(t *testing.T, a, b string) {
		if strings.ContainsRune(a, '\n') || strings.ContainsRune(b, '\n') {
			return
		}
		roundTrip(t, lines(a, b, a), Options{})
	})
}

package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/contracts"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// The contract goldens dzgorge owns (contracts/registry.json): run
// `go test ./cmd/dzgorge -run Contract -update` to rewrite them, then update
// their hashes in contracts/registry.json (docs/contracts.md says when that
// is an additive change and when it needs a new major).
var update = flag.Bool("update", false, "rewrite cmd/dzgorge/testdata/contracts goldens")

const goldenDir = "testdata/contracts"

// pinBuild fixes buildGorge for a golden and restores it after.
func pinBuild(t *testing.T) {
	t.Helper()
	old := buildGorge
	buildGorge = gorgeBuild{Version: "v0.0.0-golden", Revision: "0123456789abcdef0123456789abcdef01234567"}
	t.Cleanup(func() { buildGorge = old })
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join(goldenDir, name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (go test ./cmd/dzgorge -run Contract -update creates it)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: contract output changed:\n got: %s\nwant: %s\n(see docs/contracts.md: additive = -update, rehash in contracts/registry.json and a CHANGELOG line; otherwise a new major)",
			path, got, want)
	}
}

// TestContractGamesRecord pins dzg-games: a game record's JSON line.
func TestContractGamesRecord(t *testing.T) {
	pinBuild(t)
	rec := gameRec{Format: gamesFormat, Gorge: buildGorge, Game: 3, Pair: 1, Leg: 1, Seed: 42,
		Decks: [2]string{"red", "green"}, Side: [2]string{"B", "A"}, Pols: [2]string{"az:sims=10", "bot"},
		Winner: 1, Result: "A", Turns: 9, Intents: 210, Starter: 0, MS: 12.5,
		InHand: [2][]string{{"Mountain", "Shock"}, {"Forest"}}, Mull: [2]int{0, 1}, Visits: 4}
	stall := gameRec{Format: gamesFormat, Gorge: buildGorge, Game: 4, Pair: 2, Seed: 7, Decks: [2]string{"a", "b"},
		Side: [2]string{"A", "B"}, Pols: [2]string{"bot", "bot"}, Winner: -1, Result: "stall", Stall: "turn cap", Turns: 100}
	if gamesFormat != "dzg-games-1" {
		t.Fatalf("gamesFormat %q", gamesFormat)
	}
	var b bytes.Buffer
	for _, r := range []gameRec{rec, stall} {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	checkGolden(t, "games.jsonl", b.Bytes())
}

// packInto packs recs (one corpus file) with args into a fresh directory and
// returns it. The corpus is named relative to the working directory, so
// meta.json's source field is stable.
func packInto(t *testing.T, recs []policynet.VisitRecord, args ...string) (string, int) {
	t.Helper()
	out, err := filepath.Abs(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	t.Chdir(t.TempDir())
	writeVisitCorpus(t, "visits.jsonl.gz", recs)
	rc := runPack(append([]string{"-in", "visits.jsonl.gz", "-out", out}, args...))
	t.Chdir(wd)
	return out, rc
}

// sumsOf is a sha256sum-style manifest of files (path order).
func sumsOf(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var b bytes.Buffer
	for _, n := range names {
		s := sha256.Sum256(files[n])
		b.WriteString(hex.EncodeToString(s[:]) + "  " + n + "\n")
	}
	return b.Bytes()
}

// checkPackGolden compares a packed directory with testdata/contracts/<name>/
// file by file, and the golden directory with its SHA256SUMS (the file
// contracts/registry.json pins).
func checkPackGolden(t *testing.T, got string, name string) {
	t.Helper()
	dir := filepath.Join(goldenDir, name)
	files := readDir(t, got)
	if *update {
		os.RemoveAll(dir)
		for n, b := range files {
			p := filepath.Join(dir, filepath.FromSlash(n))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), sumsOf(files), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want := readDir(t, dir)
	sums := want["SHA256SUMS"]
	delete(want, "SHA256SUMS")
	if !bytes.Equal(sumsOf(want), sums) {
		t.Fatalf("%s: SHA256SUMS does not match the golden files", dir)
	}
	if !bytes.Equal(sumsOf(files), sums) {
		t.Fatalf("%s: pack output changed:\n got:\n%s\nwant:\n%s", dir, sumsOf(files), sums)
	}
}

// TestContractPack2 packs the synthetic corpus as dzg-pack-2 and checks the
// golden shard, meta.json's identity fields and the canonical kind column.
func TestContractPack2(t *testing.T) {
	pinBuild(t)
	out, rc := packInto(t, syntheticVisits())
	if rc != 0 {
		t.Fatalf("pack exit %d", rc)
	}
	checkPackGolden(t, out, "pack2")

	var meta struct {
		Format        string     `json:"format"`
		N             int        `json:"n"`
		Kinds         []string   `json:"kinds"`
		EncoderHash   string     `json:"encoder_hash"`
		RecordType    string     `json:"record_type"`
		SchemaVersion int        `json:"schema_version"`
		Gorge         gorgeBuild `json:"gorge"`
	}
	raw, err := os.ReadFile(filepath.Join(out, "00000", "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Format != "dzg-pack-2" || meta.N != 3 || meta.RecordType != policynet.VisitRecordType ||
		meta.SchemaVersion != policynet.VisitSchemaVersion || meta.Gorge != buildGorge ||
		meta.EncoderHash != syntheticHash {
		t.Fatalf("meta: %s", raw)
	}
	if strings.Join(meta.Kinds, ",") != strings.Join(contracts.PackKinds, ",") {
		t.Fatalf("meta kinds %v, want the canonical table %v", meta.Kinds, contracts.PackKinds)
	}
	col, err := os.ReadFile(filepath.Join(out, "00000", "kind.npy"))
	if err != nil {
		t.Fatal(err)
	}
	got := col[len(col)-3:] // three uint8 rows after the .npy header
	for i, k := range []decision.Kind{decision.KTarget, decision.KPriority, decision.KAttackers} {
		if int(got[i]) != contracts.PackKindIndex(string(k)) {
			t.Fatalf("kind column %v: row %d (%s) want %d", got, i, k, contracts.PackKindIndex(string(k)))
		}
	}
}

// TestContractPack1 holds the deprecated -format 1 to the dzg-pack-1 layout
// the pre-contract pack wrote (the golden was generated before the change).
func TestContractPack1(t *testing.T) {
	out, rc := packInto(t, syntheticVisits(), "-format", "1")
	if rc != 0 {
		t.Fatalf("pack exit %d", rc)
	}
	if *update {
		t.Skip("the dzg-pack-1 golden is frozen: it was written by the pre-contract pack")
	}
	checkPackGolden(t, out, "pack1")
}

// TestPackRefusesMixedCorpora: one encoder hash and record type per pack.
func TestPackRefusesMixedCorpora(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*policynet.VisitRecord)
	}{
		{"encoder_hash", func(r *policynet.VisitRecord) { r.EncoderHash = "00000000deadbeef" }},
		{"record_type", func(r *policynet.VisitRecord) { r.RecordType = "azvisits-v2" }},
		{"schema_version", func(r *policynet.VisitRecord) { r.SchemaVersion = 2 }},
	} {
		recs := syntheticVisits()
		tc.mutate(&recs[2])
		var stderr string
		_, rc := withStderr(t, &stderr, func() (string, int) { return packInto(t, recs) })
		if rc == 0 {
			t.Fatalf("%s: a mixed corpus packed", tc.name)
		}
		if !strings.Contains(stderr, "visits.jsonl.gz:3") {
			t.Fatalf("%s: error does not name the record's line: %s", tc.name, stderr)
		}
		if tc.name == "encoder_hash" && !(strings.Contains(stderr, "00000000deadbeef") && strings.Contains(stderr, recs[0].EncoderHash)) {
			t.Fatalf("encoder_hash: error does not name both hashes: %s", stderr)
		}
	}
}

// TestPackDecodeErrorNamesLine: a bad line is a hard error at file:line.
func TestPackDecodeErrorNamesLine(t *testing.T) {
	wd, _ := os.Getwd()
	t.Chdir(t.TempDir())
	defer t.Chdir(wd)
	var buf bytes.Buffer
	{
		var raw bytes.Buffer
		enc := json.NewEncoder(&raw)
		recs := syntheticVisits()
		enc.Encode(&recs[0])
		raw.WriteString("{not json\n")
		enc.Encode(&recs[1])
		writeGzip(t, &buf, raw.Bytes())
	}
	if err := os.WriteFile("bad.jsonl.gz", buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr string
	_, rc := withStderr(t, &stderr, func() (string, int) {
		return "", runPack([]string{"-in", "bad.jsonl.gz", "-out", "out"})
	})
	if rc == 0 || !strings.Contains(stderr, "bad.jsonl.gz:2: bad record") {
		t.Fatalf("exit %d, stderr %q", rc, stderr)
	}
	// A truncated tail is an error too, unless -allow-truncated.
	writeVisitCorpus(t, "good.jsonl.gz", syntheticVisits())
	b, err := os.ReadFile("good.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("cut.jsonl.gz", b[:len(b)-12], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, rc := withStderr(t, &stderr, func() (string, int) {
		return "", runPack([]string{"-in", "cut.jsonl.gz", "-out", "out2"})
	}); rc == 0 || !strings.Contains(stderr, "cut.jsonl.gz:") {
		t.Fatalf("a truncated corpus without -allow-truncated: exit %d, stderr %q", rc, stderr)
	}
	if _, rc := withStderr(t, &stderr, func() (string, int) {
		return "", runPack([]string{"-in", "cut.jsonl.gz", "-out", "out3", "-allow-truncated"})
	}); rc != 0 {
		t.Fatalf("-allow-truncated: exit %d, stderr %q", rc, stderr)
	}
}

func writeGzip(t *testing.T, w *bytes.Buffer, raw []byte) {
	t.Helper()
	zw := gzip.NewWriter(w)
	if _, err := zw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// withStderr runs f with os.Stderr captured into *out.
func withStderr(t *testing.T, out *string, f func() (string, int)) (string, int) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		b.ReadFrom(r)
		done <- b.String()
	}()
	s, rc := f()
	os.Stderr = old
	w.Close()
	*out = <-done
	return s, rc
}

// TestContractSpecGrammar pins dzgorge-spec: every accepted spec's parsed
// form, and the spellings the grammar refuses.
func TestContractSpecGrammar(t *testing.T) {
	accepted := []string{
		"random", "bot", "bot:autopay", "bot:mull", "bot:keep7", "sbtactical", "sbtactical:mull",
		"az", "az:sims=50",
		"az:sims=200:autopay:oppnodes:oppfull:morekinds:topk=6",
		"az:sims=10:explore:nonoise:exploreturns=8:temp=0.5:second=0.15:aseed=9",
		"az:sims=10:leaf=heuristic:prior=uniform:worlds=4:cpuct=1.25:fpu=0.2:cands=8",
		"az:sims=10:kinds=priority,target,modes", "az:sims=10:skippass", "az:sims=10:skippass=2",
		"az:sims=10:closedlist", "az:sims=10:mull", "az:sims=10:keep7",
	}
	rejected := []string{
		"", "bogus", "bot:bogus", "az:sims=x", "prior", "bot:temp=1", "bot:closedlist",
		"az:autopay=0", "az:autopay=1", "az:autopay=", "bot:mull=1", "az:explore=0", "az:nonoise=1",
		"az:keep7=1", "az:oppnodes=0", "az:oppfull=1", "az:closedlist=1", "az:morekinds=0",
		"az:leaf=none", "az:prior=x", "az:kinds=nope", "az:net=a.gpol:gonet=b.dzgw",
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, s := range accepted {
		p, err := parsePolicy(s)
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if err := enc.Encode(map[string]any{"spec": s, "parsed": p}); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range rejected {
		if _, err := parsePolicy(s); err == nil {
			t.Fatalf("%q parsed", s)
		}
		enc.Encode(map[string]any{"spec": s, "rejected": true})
	}
	checkGolden(t, "spec_grammar.jsonl", b.Bytes())
}

// TestFlagKeysRefuseValue: a flag key given a value is an error, never a
// silent "on" (autopay=0 used to enable autopay).
func TestFlagKeysRefuseValue(t *testing.T) {
	for k := range flagKeys {
		for _, v := range []string{"0", "1", ""} {
			spec := "az:sims=5:" + k + "=" + v
			_, err := parsePolicy(spec)
			if err == nil || !strings.Contains(err.Error(), "is a flag and takes no value") {
				t.Errorf("%s: err %v", spec, err)
			}
		}
		if _, err := parsePolicy("az:sims=5:" + k); err != nil && k != "oppfull" {
			t.Errorf("az:sims=5:%s: %v", k, err)
		}
	}
}

// TestContractVersionKeys pins dzgorge-version's key set (values vary by build).
func TestContractVersionKeys(t *testing.T) {
	v := buildVersionInfo(t.TempDir()) // no cards.lock there
	if v.CardsLock != nil {
		t.Fatal("a directory without cards.lock reported one")
	}
	raw, _ := json.Marshal(v)
	if !strings.Contains(string(raw), `"cards_lock":null`) {
		t.Fatalf("cards_lock not null: %s", raw)
	}
	v.CardsLock = &versionLock{}
	if v.Format != "dzgorge-version-1" || len(v.Contracts) != len(contracts.All()) || v.IRCacheVersion <= 0 ||
		len(v.EntityEncoderHash) != 16 || v.CardsCompiler == "" {
		t.Fatalf("version info: %+v", v)
	}
	raw, _ = json.Marshal(v)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var keys []string
	var walk func(prefix string, x map[string]any)
	walk = func(prefix string, x map[string]any) {
		for k, val := range x {
			keys = append(keys, prefix+k)
			if sub, ok := val.(map[string]any); ok && k != "contracts" {
				walk(prefix+k+".", sub)
			}
		}
	}
	walk("", m)
	sort.Strings(keys)
	checkGolden(t, "version_keys.txt", []byte(strings.Join(keys, "\n")+"\n"))
}

// TestContractRemoteFrames pins dzg-remote: the GZB1 request bytes the packer
// writes for a fixed batch, and a GZR1 response frame the client parses.
func TestContractRemoteFrames(t *testing.T) {
	st0 := policynet.State{Dense: []float32{1, 0.5}, Sparse: []policynet.Feature{{Row: 3, Value: 1}},
		Cards: []policynet.EntityCard{{Group: 1, Raw: []float32{0.25}, Rows: []policynet.Feature{{Row: 9, Value: 1}}}}}
	st1 := policynet.State{Dense: []float32{0, 1}}
	opts := []policynet.Option{
		{Dense: []float32{1}, BotPick: true, Slots: []policynet.Feature{{Row: 2, Value: 1}}, Hashed: []policynet.Feature{{Row: 5, Value: 0.5}}},
		{Dense: []float32{0, 1}, EntA: 1, Slots: []policynet.Feature{{Row: 4, Value: 1}}},
	}
	batch := []*rreq{{st: &st0, opts: opts, want: true}, {st: &st1, want: false}}

	// The response: values for both states, scores for state 0's two options.
	vals, scores := []float32{0.75, 0.25}, []float32{1.5, -0.5}
	body := make([]byte, 0, 32)
	for _, v := range vals {
		body = binary.LittleEndian.AppendUint32(body, math.Float32bits(v))
	}
	for len(body) < pad8(4*len(vals)) {
		body = append(body, 0)
	}
	for _, v := range scores {
		body = binary.LittleEndian.AppendUint32(body, math.Float32bits(v))
	}
	resp := binary.LittleEndian.AppendUint32(nil, uint32(16+len(body)))
	for _, x := range []uint32{respMagic, uint32(len(batch)), uint32(len(opts)), 0} {
		resp = binary.LittleEndian.AppendUint32(resp, x)
	}
	resp = append(resp, body...)

	var sent bytes.Buffer
	var enc packer
	r := &remoteNet{}
	if err := r.roundTrip(&enc, batch, bufio.NewWriter(&sent), bufio.NewReader(bytes.NewReader(resp))); err != nil {
		t.Fatal(err)
	}
	if batch[0].value != 0.75 || batch[1].value != 0.25 || len(batch[0].scores) != 2 ||
		batch[0].scores[0] != 1.5 || batch[0].scores[1] != -0.5 || batch[1].scores != nil {
		t.Fatalf("parsed response: %+v %+v", *batch[0], *batch[1])
	}
	if binary.LittleEndian.Uint32(sent.Bytes()[4:]) != reqMagic {
		t.Fatal("request magic is not GZB1")
	}
	checkGolden(t, "remote_request.bin", sent.Bytes())
	checkGolden(t, "remote_response.bin", resp)
}

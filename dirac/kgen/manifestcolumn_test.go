package kgen

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/adams-shaun/gorge/dirac/manifest"
)

// TestManifestColumn: a refusal's manifest column is the entry its
// vocabulary item has (an API, keyword, trigger, static or replacement mode,
// Count$ head, or the counter type a counter refusal quotes), and "-" for a
// class that names no code or an item with no entry.
func TestManifestColumn(t *testing.T) {
	entry := func(fam, name, forge string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("family: " + fam + "\nname: " + name + "\nvalue: next\nforge: " + forge + "\nstatus: reserved\n")}
	}
	man, err := manifest.Read(fstest.MapFS{
		manifest.OrdinalsPath:                     {Data: []byte("CtrP1P1\tCounterKind\t0\n")},
		"dirac/manifest/CounterKind/CtrBounty":    entry("CounterKind", "CtrBounty", "BOUNTY"),
		"dirac/manifest/ExtOp/ExtClash":           entry("ExtOp", "ExtClash", "Clash"),
		"dirac/manifest/KwID/KwCumulativeUpkeep2": entry("KwID", "KwCumulativeUpkeep2", "Cumulative upkeep"),
		"dirac/manifest/ReplKind/ReplBeginPhase":  entry("ReplKind", "ReplBeginPhase", "BeginPhase"),
		"dirac/manifest/ExprKind/ExIntensity":     entry("ExprKind", "ExIntensity", "Intensity"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ class, reason, want string }{
		{"api:Clash", "api Clash", "ExtOp/ExtClash"},
		{"kw:Cumulative upkeep", `keyword "Cumulative upkeep:2"`, "KwID/KwCumulativeUpkeep2"},
		{"repl:BeginPhase shape", "repl-hard: BeginPhase shape", "ReplKind/ReplBeginPhase"},
		{"count:Intensity", "Count$Intensity", "ExprKind/ExIntensity"},
		{`other:instr N PutCounter: counter kind "_"`, `instr 3 PutCounter: counter kind "BOUNTY"`, "CounterKind/CtrBounty"},
		{`other:MoveCounter CounterType "_"`, `MoveCounter CounterType "bounty"`, "CounterKind/CtrBounty"},
		{"api:Vote", "api Vote", "-"},
		{"param:PutCounter.UpTo", "unread params [UpTo]", "-"},
		{"prop:Foo", `unknown property "Foo"`, "-"},
	} {
		if got := manifestColumn(man, c.class, c.reason); got != c.want {
			t.Errorf("manifestColumn(%q, %q) = %q, want %q", c.class, c.reason, got, c.want)
		}
	}
	if got := manifestColumn(nil, "api:Clash", "api Clash"); got != "-" {
		t.Errorf("with no manifest: %q, want -", got)
	}

	var sb strings.Builder
	res := &stdCensusResult{tsv: []stdTSVRow{
		{set: "ZZZ", card: "Ok"},
		{set: "ZZZ", card: "Clasher", node: "A:1", class: "api:Clash", reason: "api Clash", label: "api:Clash"},
	}}
	if err := writeStdTSV(&sb, res, man); err != nil {
		t.Fatal(err)
	}
	want := "ZZZ\tOk\t\t\t\t\t\t\nZZZ\tClasher\t\tA:1\tapi:Clash\tapi Clash\tapi:Clash\tExtOp/ExtClash\n"
	if sb.String() != want {
		t.Errorf("TSV:\n%q\nwant:\n%q", sb.String(), want)
	}
}

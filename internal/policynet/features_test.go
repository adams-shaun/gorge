package policynet

import (
	"bytes"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// FeaturesV1 is the pinned encoder byte for byte: EncodeStateWith and
// EncodeOptionWith under v1 must be EncodeState / EncodeOption exactly, and
// EncoderHashFor(v1) must be EncoderHash (every existing checkpoint keeps
// loading).
func TestFeaturesV1IsThePinnedEncoder(t *testing.T) {
	v := goldenFixture()
	if !reflect.DeepEqual(EncodeStateWith(FeaturesV1, v, 0, &Diag{OppHand: []view.CardView{{Name: "X"}}}), EncodeState(v, 0)) {
		t.Fatal("EncodeStateWith(v1) differs from EncodeState")
	}
	opts := []decision.Option{
		{Index: 0, Kind: "cast", Obj: 10},
		{Index: 1, Kind: "block", Obj: 6, Attacker: 9},
		{Index: 2, Kind: "player", Obj: state.PlayerRef(1), Player: 1},
	}
	for i, o := range opts {
		for _, d := range []decision.Kind{decision.KPriority, "not-a-kind"} {
			if !reflect.DeepEqual(EncodeOptionWith(FeaturesV1, v, 0, d, o, i, len(opts)), EncodeOption(v, 0, d, o, i, len(opts))) {
				t.Fatalf("option %d kind %s: EncodeOptionWith(v1) differs from EncodeOption", i, d)
			}
		}
	}
	if EncoderHashFor(FeaturesV1) != EncoderHash() {
		t.Fatal("EncoderHashFor(v1) != EncoderHash()")
	}
}

// The MZ encodings are additive (the v1 bag is a sub-multiset of the MZ
// bag, dense untouched), deterministic, and the diagnostic tokens appear only
// under a diagnostic set given a Diag.
func TestMZStateIsAdditiveAndGated(t *testing.T) {
	v := goldenFixture()
	base := EncodeState(v, 0)
	diag := &Diag{OppHand: []view.CardView{{Name: "Counterspell", Types: "Instant", ManaCost: "U U"}}, OppLibraryTop: []string{"Island"}, OwnLibraryTop: []string{"Plains"}}
	count := func(fs []Feature) map[uint16]int {
		m := map[uint16]int{}
		for _, f := range fs {
			m[f.Row]++
		}
		return m
	}
	has := func(st State, s string) bool {
		r := hashID(s)
		for _, f := range st.Sparse {
			if f.Row == r {
				return true
			}
		}
		return false
	}
	var prev int
	for _, fs := range []FeatureSet{FeaturesMZ, FeaturesMZOppHand, FeaturesMZOracle} {
		st := EncodeStateWith(fs, v, 0, diag)
		if !reflect.DeepEqual(st, EncodeStateWith(fs, v, 0, diag)) {
			t.Fatalf("%s: two encodes differ", fs)
		}
		if !reflect.DeepEqual(st.Dense, base.Dense) {
			t.Fatalf("%s: dense changed", fs)
		}
		got := count(st.Sparse)
		for r, n := range count(base.Sparse) {
			if got[r] < n {
				t.Fatalf("%s: v1 row %d appears %d times, want >= %d", fs, r, got[r], n)
			}
		}
		for i := 1; i < len(st.Sparse); i++ {
			if st.Sparse[i].Row < st.Sparse[i-1].Row {
				t.Fatalf("%s: sparse bag not sorted", fs)
			}
		}
		if len(st.Sparse) <= prev {
			t.Fatalf("%s: %d rows, want more than the previous set's %d", fs, len(st.Sparse), prev)
		}
		prev = len(st.Sparse)
		// Opponent open mana and the opponent's attacking bears are read.
		if !has(st, "mz|bf-opp|attacking") || !has(st, "mz|bf-me|cr|kw|first strike") || !has(st, "mz|me|avail=7") {
			t.Fatalf("%s: expected MZ tokens missing", fs)
		}
		if has(st, "mz|opphand|Counterspell") != (fs >= FeaturesMZOppHand) {
			t.Fatalf("%s: opponent-hand token presence wrong", fs)
		}
		if has(st, "mz|opplib1|Island") != (fs == FeaturesMZOracle) {
			t.Fatalf("%s: library token presence wrong", fs)
		}
	}
	// A nil Diag under a diagnostic set encodes as plain MZ.
	if !reflect.DeepEqual(EncodeStateWith(FeaturesMZOracle, v, 0, nil), EncodeStateWith(FeaturesMZ, v, 0, nil)) {
		t.Fatal("diagnostic set with no Diag differs from mz")
	}
}

// The pn03 finding, pinned: v1 sets the decision-kind "other" slot at
// 24+len(decision.Kinds) == typeOffset, the Land bit. MZ drops that bit
// (keeping a real Land's own bit) and hashes the unknown kind instead.
func TestMZDecisionKindOtherDoesNotSetLandBit(t *testing.T) {
	if decKindOffset+len(decision.Kinds) != typeOffset {
		t.Skipf("the v1 layout no longer collides (decision.Kinds has %d kinds); the fix is moot", len(decision.Kinds))
	}
	v := goldenFixture()
	hasSlot := func(o Option, r int) int {
		n := 0
		for _, f := range o.Slots {
			if int(f.Row) == r {
				n++
			}
		}
		return n
	}
	nonLand := decision.Option{Index: 0, Kind: "cast", Obj: 11} // Swords to Plowshares
	land := decision.Option{Index: 0, Kind: "play_land", Obj: 14}
	if hasSlot(EncodeOption(v, 0, "not-a-kind", nonLand, 0, 1), typeOffset) != 1 {
		t.Fatal("v1 no longer sets the colliding bit (the golden layout moved)")
	}
	mz := EncodeOptionWith(FeaturesMZ, v, 0, "not-a-kind", nonLand, 0, 1)
	if hasSlot(mz, typeOffset) != 0 {
		t.Fatal("mz: a non-land option under an unknown decision kind still sets the Land bit")
	}
	if hasSlot(EncodeOptionWith(FeaturesMZ, v, 0, "not-a-kind", land, 0, 1), typeOffset) != 1 {
		t.Fatal("mz: a real Land lost its Land bit")
	}
	found := false
	for _, f := range mz.Hashed {
		if f.Row == hashID("mz|deckind|other") {
			found = true
		}
	}
	if !found {
		t.Fatal("mz: the unknown decision kind is not hashed")
	}
	// A known kind is unchanged apart from the appended hashed rows.
	k := EncodeOptionWith(FeaturesMZ, v, 0, decision.KPriority, nonLand, 0, 1)
	b := EncodeOption(v, 0, decision.KPriority, nonLand, 0, 1)
	if !reflect.DeepEqual(k.Slots, b.Slots) || !reflect.DeepEqual(k.Dense, b.Dense) || !reflect.DeepEqual(k.Hashed[:len(b.Hashed)], b.Hashed) {
		t.Fatal("mz changed a known-kind option's v1 encoding")
	}
}

// The MZ hash is pinned (a drift in its token format must fail loudly), a MZ
// checkpoint round-trips carrying its feature set, and a diagnostic model is
// never written.
func TestFeatureSetCheckpoints(t *testing.T) {
	const pinnedMZ = uint64(0xd1f1d3f0cef67bb2)
	if got := EncoderHashFor(FeaturesMZ); got != pinnedMZ {
		t.Fatalf("EncoderHashFor(mz) = %#016x, want pinned %#016x", got, pinnedMZ)
	}
	m := NewModel(TableRows, 4, 3, rand.New(rand.NewPCG(1, 2)))
	m.Features = FeaturesMZ
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Features != FeaturesMZ || NewScorer(got).Features() != FeaturesMZ {
		t.Fatalf("round-tripped features %s, want mz", got.Features)
	}
	m.Features = FeaturesV1
	buf.Reset()
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	if v1, err := LoadCheckpoint(bytes.NewReader(buf.Bytes())); err != nil || v1.Features != FeaturesV1 {
		t.Fatalf("v1 round trip: %v %v", v1, err)
	}
	for _, fs := range []FeatureSet{FeaturesMZOppHand, FeaturesMZOracle} {
		m.Features = fs
		if err := WriteCheckpoint(m, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "diagnostic") {
			t.Fatalf("%s: WriteCheckpoint = %v, want a diagnostic refusal", fs, err)
		}
		if _, ok := FeaturesForHash(EncoderHashFor(fs)); ok {
			t.Fatalf("%s: a diagnostic hash maps to a loadable feature set", fs)
		}
	}
}

func TestParseFeatureSet(t *testing.T) {
	for i, n := range featureSetNames {
		fs, err := ParseFeatureSet(n)
		if err != nil || fs != FeatureSet(i) || fs.String() != n {
			t.Fatalf("%s: %v %v", n, fs, err)
		}
	}
	if fs, err := ParseFeatureSet(""); err != nil || fs != FeaturesV1 {
		t.Fatalf("empty: %v %v", fs, err)
	}
	if _, err := ParseFeatureSet("bogus"); err == nil {
		t.Fatal("bogus feature set accepted")
	}
}

// LoadWith with the zero options reproduces Load exactly (JointCard nil).
func TestLoadWithZeroOptionsIsLoad(t *testing.T) {
	path := writeCorpus(t, fixtureRecords(), true)
	a, sa, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b, sb, err := LoadWith(path, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if sa != sb || !reflect.DeepEqual(a, b) {
		t.Fatalf("LoadWith(zero) differs from Load: stats %+v vs %+v", sa, sb)
	}
	kept, _, err := LoadWith(path, LoadOptions{Keep: func(_ string, g int) bool { return g < 2 }})
	if err != nil || len(kept) != 1 || kept[0].GameIndex != 0 {
		t.Fatalf("Keep filter: %d examples, %v", len(kept), err)
	}
}

// jointRecord is one schema 3 priority record: the bot casts Thalia (option
// 0, no target), the teacher overrides with Swords to Plowshares (option 1),
// whose follow-up target decision offers the opponent's bears or my Thalia;
// the bot's clone pick is Thalia but the game's actual answer was the bears.
func jointRecord(t *testing.T) string {
	t.Helper()
	vb, _ := json.Marshal(goldenFixture())
	targets := []decision.Option{
		{Index: 0, Kind: "permanent", Label: "Grizzly Bears", Obj: 9, Player: 1},
		{Index: 1, Kind: "permanent", Label: "Thalia", Obj: 6, Player: 0},
	}
	rec := leanRecord{
		RecordType: LabelRecordType, SchemaVersion: 3, Pair: "p", Seed: 1, Sequence: 3, Kind: decision.KPriority, Turn: 6, View: vb,
		Options: []decision.Option{
			{Index: 0, Kind: "cast", Obj: 10},
			{Index: 1, Kind: "cast", Obj: 11},
			{Index: 2, Kind: "pass"},
		},
		Candidates:    []labelCandidate{{Choices: []int{0}, Bot: true, Value: 0}, {Choices: []int{1}, Value: 1}, {Choices: []int{2}, Value: 0}},
		TeacherChoice: 1,
		Extras: &LabelExtras{
			DiagOppHand:   []view.CardView{{Name: "Counterspell", Types: "Instant"}},
			FollowTargets: []FollowTarget{{Option: 1, Targets: targets, Min: 1, Max: 1, Choices: []int{1}}},
			ChosenTarget:  &FollowTarget{Option: 1, Targets: targets, Min: 1, Max: 1, Choices: []int{0}},
		},
	}
	path := filepath.Join(t.TempDir(), "joint.jsonl")
	b, _ := json.Marshal(rec)
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestJointExpansion(t *testing.T) {
	path := jointRecord(t)
	split, _, err := LoadWith(path, LoadOptions{})
	if err != nil || len(split) != 1 || len(split[0].Options) != 3 || split[0].JointCard != nil {
		t.Fatalf("split load: %v", err)
	}
	ex, _, err := LoadWith(path, LoadOptions{Joint: true})
	if err != nil || len(ex) != 1 {
		t.Fatalf("joint load: %v", err)
	}
	j := ex[0]
	if !reflect.DeepEqual(j.JointCard, []int{0, 1, 1, 2}) {
		t.Fatalf("JointCard = %v, want [0 1 1 2]", j.JointCard)
	}
	// The played card's joint option follows the IN-GAME target (the bears),
	// not the clone's bot pick (Thalia).
	if !j.Options[1].Target.Preferred || j.Options[2].Target.Preferred || j.Options[0].Target.Preferred {
		t.Fatalf("preferred marks: %v %v %v", j.Options[0].Target.Preferred, j.Options[1].Target.Preferred, j.Options[2].Target.Preferred)
	}
	if !j.Options[0].BotPick || j.Options[1].BotPick || j.Options[2].BotPick {
		t.Fatal("bot pick should stay on the bot's untargeted card only")
	}
	if !j.Options[1].Target.Labelled || j.Options[1].Target.Value != 1 || j.Options[2].Target.Value != 1 {
		t.Fatal("joint options must share their card's labelled value")
	}
	// Every joint option keeps its card's v1 encoding and adds target rows;
	// the two expansions differ only in the target rows.
	if !reflect.DeepEqual(j.Options[1].Slots, split[0].Options[1].Slots) || reflect.DeepEqual(j.Options[1].Hashed, j.Options[2].Hashed) {
		t.Fatal("joint options do not extend their card's encoding with distinct target rows")
	}
	// The diagnostic set reads the recorded opponent hand; mz does not.
	d, _, _ := LoadWith(path, LoadOptions{Features: FeaturesMZOppHand})
	m, _, _ := LoadWith(path, LoadOptions{Features: FeaturesMZ})
	if len(d[0].State.Sparse) <= len(m[0].State.Sparse) {
		t.Fatal("the diagnostic opponent hand was not encoded")
	}
}

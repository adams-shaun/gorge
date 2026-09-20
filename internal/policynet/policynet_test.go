package policynet

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/traceboard"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// ---------------------------------------------------------------------------
// Golden encoding tests. The fixture is built literally (never from the
// engine), and the expected dense vectors, sparse rows, slot rows, hashed
// rows and hash literals are PINNED — a change to the hash, a string format
// or the layout fails here loudly, because a trained checkpoint is worthless
// if the encoding drifts.
// ---------------------------------------------------------------------------

// goldenFixture is the fixed two-seat view the goldens pin.
func goldenFixture() view.View {
	oppAtk := state.PlayerID(0)
	return view.View{
		Viewer:     0,
		Visibility: "seat",
		Turn:       6,
		Round:      3,
		Step:       "main1",
		Phase:      "main1",
		Active:     0,
		Priority:   0,
		Players: []view.PlayerView{
			{
				ID:            0,
				Name:          "me",
				Life:          17,
				LibrarySize:   40,
				HandSize:      2,
				GraveyardSize: 1,
				Hand: []view.CardView{
					{ID: 10, Name: "Thalia, Heretic Cathar", Types: "Creature Human", ManaCost: "1 W", Power: 2, Toughness: 1, Controller: 0, Owner: 0},
					{ID: 11, Name: "Swords to Plowshares", Types: "Instant", ManaCost: "W", Controller: 0, Owner: 0},
				},
				Battlefield: []view.CardView{
					{ID: 5, Name: "Plains", Types: "Land Basic Plains", Tapped: true, Controller: 0, Owner: 0},
					{ID: 6, Name: "Thalia, Heretic Cathar", Types: "Creature Human", ManaCost: "1 W", Power: 2, Toughness: 2, Damage: 1, Controller: 0, Owner: 0, Counters: map[string]int32{"P1P1": 1, "AGE": 2}, Keywords: []string{"First Strike", "Vigilance"}},
					{ID: 7, Name: "Island", Types: "Land Basic Island", Controller: 0, Owner: 0},
				},
				Graveyard: []view.CardView{
					{ID: 8, Name: "Grizzly Bears", Types: "Creature", Power: 2, Toughness: 2, Controller: 0, Owner: 0},
				},
				Exile:     []view.CardView{},
				Pool:      map[string]int32{"W": 2, "C": 1},
				Available: map[string]int32{"W": 3, "G": 1},
			},
			{
				ID:            1,
				Name:          "opp",
				Life:          20,
				LibrarySize:   35,
				HandSize:      0,
				GraveyardSize: 1,
				Battlefield: []view.CardView{
					{ID: 9, Name: "Grizzly Bears", Types: "Creature", ManaCost: "1 G", Power: 2, Toughness: 2, Attacking: true, AttackingPlayer: &oppAtk, Controller: 1, Owner: 1},
				},
				Graveyard: []view.CardView{
					{ID: 12, Name: "Shock", Types: "Instant", ManaCost: "R", Controller: 1, Owner: 1},
				},
				Exile: []view.CardView{},
				Pool:  map[string]int32{},
			},
		},
		Stack: []view.StackView{
			{ID: 13, Kind: "spell", Name: "Lightning Bolt", Controller: 1, Targets: []view.TargetView{}},
		},
	}
}

// goldenRows is the PINNED sparse bag for goldenFixture: one sorted row list,
// every row a pinned literal. Each entry is documented with its source
// string so a failure names what moved.
var goldenRows = []struct {
	row  uint16
	from string
}{
	{510, "Island|battlefield"},
	{607, "Swords to Plowshares|hand"},
	{3950, "Plains|battlefield"},
	{6452, "Lightning Bolt|stack"},
	{7364, "Island|t0|a0|s0|c-"},
	{8208, "Thalia, Heretic Cathar|hand"},
	{9753, "Thalia, Heretic Cathar|battlefield"},
	{11520, "Grizzly Bears|t0|a1|s0|c-"},
	{11589, "Grizzly Bears|battlefield-opp"},
	{12262, "Thalia, Heretic Cathar|t0|a0|s0|cAGE=2,P1P1=1"},
	{14185, "Plains|t1|a0|s0|c-"},
	{14762, "Grizzly Bears|graveyard"},
	{15476, "Shock|graveyard-opp"},
}

// goldenDense is the PINNED dense vector for goldenFixture, seat 0.
func goldenDense() []float32 {
	d := make([]float32, DenseWidth)
	d[denseTurn] = 6
	d[denseTurnBucket] = 0.15
	d[denseStep0+3] = 1    // main1
	d[densePhase0+1] = 1   // main1
	d[denseIsActive] = 1   // seat 0 active
	d[denseIsPriority] = 1 // seat 0 priority
	d[denseStackDepth] = 1 // one stack entry
	d[densePool0+0] = 2    // W
	d[densePool0+5] = 1    // C
	// seat block 0 (viewer)
	d[denseSeat0+0] = 17 // life
	d[denseSeat0+1] = 2  // hand
	d[denseSeat0+2] = 40 // library
	d[denseSeat0+3] = 1  // graveyard
	d[denseSeat0+4] = 3  // battlefield count
	d[denseSeat0+5] = 1  // creature count
	d[denseSeat0+6] = 2  // total power
	d[denseSeat0+7] = 2  // total toughness
	d[denseSeat0+8] = 2  // untapped permanents (Thalia, Island)
	d[denseSeat0+9] = 1  // untapped lands (Island)
	// seat block 1 (opponent)
	d[denseSeat0+10+0] = 20 // life
	d[denseSeat0+10+2] = 35 // library
	d[denseSeat0+10+3] = 1  // graveyard
	d[denseSeat0+10+4] = 1  // battlefield count
	d[denseSeat0+10+5] = 1  // creature count
	d[denseSeat0+10+6] = 2  // power
	d[denseSeat0+10+7] = 2  // toughness
	d[denseSeat0+10+8] = 1  // untapped permanents (the attacking bears are not marked tapped)
	return d
}

func nearF(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-6
}

func nearVec(t *testing.T, name string, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len %d, want %d", name, len(got), len(want))
	}
	for i := range want {
		if !nearF(got[i], want[i]) {
			t.Fatalf("%s[%d] = %g, want %g", name, i, got[i], want[i])
		}
	}
}

// TestGoldenHashPinsFNV1a pins the hash against an independent inline
// reference implementation of FNV-1a 64, so a wrong constant or loop is
// caught even though the golden rows below were computed from the package.
func TestGoldenHashPinsFNV1a(t *testing.T) {
	ref := func(s string) uint16 {
		h := uint64(14695981039346656037) // FNV offset basis
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= 1099511628211 // FNV prime
		}
		return uint16(h & (1<<14 - 1))
	}
	for _, s := range []string{
		"", "Thalia, Heretic Cathar|hand", "Plains|t1|a0|s0|c-",
		"card|Grizzly Bears\x1fatk|Grizzly Bears", "optkind|cast",
		"Thalia, Heretic Cathar|t0|a0|s0|cAGE=2,P1P1=1",
	} {
		if got := hashID(s); got != ref(s) {
			t.Fatalf("hashID(%q) = %d, reference FNV-1a 64 masked = %d", s, got, ref(s))
		}
	}
}

func TestGoldenStateEncoding(t *testing.T) {
	v := goldenFixture()
	st := EncodeState(v, 0)
	nearVec(t, "dense", st.Dense, goldenDense())
	if len(st.Sparse) != len(goldenRows) {
		t.Fatalf("sparse bag: %d rows, want %d", len(st.Sparse), len(goldenRows))
	}
	for i, g := range goldenRows {
		if st.Sparse[i].Row != g.row {
			t.Fatalf("sparse[%d] = row %d, want %d (pinned from %q) — the hash, a string format or the zone order moved", i, st.Sparse[i].Row, g.row, g.from)
		}
		if st.Sparse[i].Value != 1 {
			t.Fatalf("sparse[%d].Value = %g, want 1", i, st.Sparse[i].Value)
		}
	}
	// The bag is sorted ascending by row.
	for i := 1; i < len(st.Sparse); i++ {
		if st.Sparse[i-1].Row > st.Sparse[i].Row {
			t.Fatalf("sparse not sorted at %d: %d > %d", i, st.Sparse[i-1].Row, st.Sparse[i].Row)
		}
	}
}

// TestGoldenOptionEncoding pins the full option block: slots, hashed ids and
// dense scalars, for a cast, a play_land, a pass, and two block options.
func TestGoldenOptionEncoding(t *testing.T) {
	v := goldenFixture()

	type want struct {
		slots  []uint16
		hashed []uint16
		dense  []float32
	}

	opts := []decision.Option{
		{Index: 0, Kind: "cast", Label: "Thalia, Heretic Cathar", Obj: 10, Player: 0},
		{Index: 1, Kind: "play_land", Label: "Swords to Plowshares", Obj: 11, Player: 0},
		{Index: 2, Kind: "pass", Label: "Pass", Player: 0},
	}
	// cast Thalia (a 2/1 creature in hand): kind one-hot "cast" (index 5),
	// decision kind priority (index 0), type Creature, colour W, MV bucket 2,
	// mode "", mine+hand; dense power/toughness from the hand card.
	castDense := make([]float32, OptionDenseWidth)
	castDense[odPower] = 2
	castDense[odToughness] = 1
	castDense[odRemaining] = 1
	castDense[odCostTotal] = 2
	castDense[odCostDelta] = -5 // 2 − (pool 3 + available 4)
	castDense[odManaValue] = 2
	landDense := make([]float32, OptionDenseWidth)
	landDense[odCostTotal] = 1
	landDense[odCostDelta] = -6
	landDense[odIndex] = 0.5
	landDense[odManaValue] = 1
	passDense := make([]float32, OptionDenseWidth)
	passDense[odIndex] = 1
	wants := []want{
		{slots: []uint16{optKindOffset + 5, decKindOffset + 0, typeOffset + 1, colourOffset + 0, mvOffset + 2, modeOffset + 0, zoneOffset + 4 + zoneHand},
			hashed: []uint16{7095, 8602, 3247}, dense: castDense},
		{slots: []uint16{optKindOffset + 16, decKindOffset + 0, typeOffset + 4, colourOffset + 0, mvOffset + 1, modeOffset + 0, zoneOffset + 4 + zoneHand},
			hashed: []uint16{8782, 11413, 154}, dense: landDense},
		{slots: []uint16{optKindOffset + 14, decKindOffset + 0, typeOffset + 11, mvOffset + 0, modeOffset + 0},
			hashed: []uint16{15385}, dense: passDense},
	}
	for i, w := range wants {
		got := EncodeOption(v, 0, decision.KPriority, opts[i], i, len(opts))
		if len(got.Slots) != len(w.slots) {
			t.Fatalf("option %d: %d slots, want %d", i, len(got.Slots), len(w.slots))
		}
		for j, s := range w.slots {
			if got.Slots[j].Row != s {
				t.Fatalf("option %d slot[%d] = %d, want %d", i, j, got.Slots[j].Row, s)
			}
		}
		if len(got.Hashed) != len(w.hashed) {
			t.Fatalf("option %d: %d hashed, want %d", i, len(got.Hashed), len(w.hashed))
		}
		for j, h := range w.hashed {
			if got.Hashed[j].Row != h {
				t.Fatalf("option %d hashed[%d] = %d, want %d — the hash or a string format moved", i, j, got.Hashed[j].Row, h)
			}
		}
		nearVec(t, "option dense", got.Dense, w.dense)
	}

	// Block options: the joint hashed id is blocker × attacker; the dense
	// block carries the attacker's card and the survive-delta.
	blockOpts := []decision.Option{
		{Index: 0, Kind: "block", Label: "block with Thalia", Obj: 6, Attacker: 9, Player: 0},
		{Index: 1, Kind: "block", Label: "block with Island", Obj: 7, Attacker: 9, Player: 0},
	}
	blockThalia := EncodeOption(v, 0, decision.KBlockers, blockOpts[0], 0, len(blockOpts))
	// hashed: [0] card|Thalia…, [1] card|Thalia…␟atk|Grizzly Bears, [2] optkind|block
	if got := []uint16{blockThalia.Hashed[0].Row, blockThalia.Hashed[1].Row, blockThalia.Hashed[2].Row}; !reflect.DeepEqual(got, []uint16{7095, 8050, 7377}) {
		t.Fatalf("block hashed rows = %v, want [7095 8050 7377]", got)
	}
	blockDense := make([]float32, OptionDenseWidth)
	blockDense[odPower] = 2
	blockDense[odToughness] = 2
	blockDense[odRemaining] = 1 // 2 − damage 1
	blockDense[odDamage] = 1
	blockDense[odAtkPower] = 2
	blockDense[odAtkTough] = 2
	blockDense[odAtkDelta] = 0 // my toughness 2 − attacker power 2
	blockDense[odCostTotal] = 2
	blockDense[odCostDelta] = -5
	blockDense[odManaValue] = 2
	blockDense[odCounters] = 3 // P1P1:1 + AGE:2
	nearVec(t, "block dense", blockThalia.Dense, blockDense)
	// keyword bits: First Strike (index 7) and Vigilance (index 3) set.
	if len(blockThalia.Slots) < 2 {
		t.Fatalf("block option: %d slots", len(blockThalia.Slots))
	}
	found := map[uint16]bool{}
	for _, f := range blockThalia.Slots {
		found[f.Row] = true
	}
	if !found[kwOffset+7] || !found[kwOffset+3] {
		t.Fatalf("block option slots missing First Strike/Vigilance bits: %v", blockThalia.Slots)
	}
}

// TestGoldenPlayerOption pins the player-ref shape: a player-target option
// hashes "card|player" and sets the is-player dense scalar.
func TestGoldenPlayerOption(t *testing.T) {
	v := goldenFixture()
	o := decision.Option{Index: 0, Kind: "player", Label: "opponent", Obj: state.PlayerRef(1), Player: 1}
	got := EncodeOption(v, 0, decision.KTarget, o, 0, 1)
	if got.Dense[odIsPlayer] != 1 {
		t.Fatalf("player option is-player scalar = %g, want 1", got.Dense[odIsPlayer])
	}
	if len(got.Hashed) != 3 || got.Hashed[0].Row != 1508 {
		t.Fatalf("player option hashed = %v, want [1508 … …] (card|player first)", got.Hashed)
	}
}

func TestEncodeDeterminism(t *testing.T) {
	v := goldenFixture()
	a := EncodeState(v, 0)
	b := EncodeState(v, 0)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two EncodeState calls differ")
	}
	opts := []decision.Option{
		{Index: 0, Kind: "cast", Label: "Thalia, Heretic Cathar", Obj: 10, Player: 0},
		{Index: 1, Kind: "block", Label: "block", Obj: 6, Attacker: 9, Player: 0},
		{Index: 2, Kind: "player", Label: "opp", Obj: state.PlayerRef(1), Player: 1},
	}
	for i, o := range opts {
		x := EncodeOption(v, 0, decision.KPriority, o, i, len(opts))
		y := EncodeOption(v, 0, decision.KPriority, o, i, len(opts))
		if !reflect.DeepEqual(x, y) {
			t.Fatalf("option %d: two EncodeOption calls differ", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Loader tests. The corpus fixture is GENERATED in the test (never checked
// in) with the same JSON shape cmd/searchteacher -labels writes.
// ---------------------------------------------------------------------------

func fixtureRecords() []labelRecord {
	v := goldenFixture()
	vb, _ := json.Marshal(v)
	return []labelRecord{
		{
			RecordType: LabelRecordType, SchemaVersion: LabelSchemaVersion,
			Pair: "a-vs-b", GameIndex: 0, Seed: 1, Sequence: 7,
			Seat: 0, Kind: decision.KPriority, Turn: 6,
			Board: traceboardShim(),
			View:  vb,
			Options: []decision.Option{
				{Index: 0, Kind: "cast", Label: "Thalia, Heretic Cathar", Obj: 10, Player: 0},
				{Index: 1, Kind: "play_land", Label: "Swords to Plowshares", Obj: 11, Player: 0},
				{Index: 2, Kind: "pass", Label: "Pass", Player: 0},
			},
			Candidates: []labelCandidate{
				{Choices: []int{2}, Bot: true, Value: -0.5, Worlds: 16},
				{Choices: []int{0}, Value: -0.2, Worlds: 16},
			},
			TeacherChoice: 1, BotIndex: 0, Margin: 0.3,
			Worlds: 16, Attempts: 128, Accepted: 9, Horizon: 0,
		},
		{
			// no candidates at all — skipped and counted
			RecordType: LabelRecordType, SchemaVersion: LabelSchemaVersion,
			Pair: "a-vs-b", GameIndex: 1, Seed: 1, Sequence: 9,
			Seat: 0, Kind: decision.KPriority, Turn: 7,
			Board: traceboardShim(), View: vb,
			Options: []decision.Option{{Index: 0, Kind: "pass", Player: 0}},
			Worlds:  16,
		},
		{
			// the teacher kept the bot's answer: preferred falls on the bot's
			// own option set
			RecordType: LabelRecordType, SchemaVersion: LabelSchemaVersion,
			Pair: "a-vs-b", GameIndex: 2, Seed: 1, Sequence: 11,
			Seat: 0, Kind: decision.KPriority, Turn: 8,
			Board: traceboardShim(), View: vb,
			Options: []decision.Option{
				{Index: 0, Kind: "cast", Label: "Thalia, Heretic Cathar", Obj: 10, Player: 0},
				{Index: 1, Kind: "pass", Label: "Pass", Player: 0},
			},
			Candidates: []labelCandidate{
				{Choices: []int{0}, Bot: true, Value: -0.4, Worlds: 16},
			},
			TeacherChoice: 0, BotIndex: 0, Margin: 0,
			Worlds: 16,
		},
		{
			// an out-of-range teacher_choice marks nobody preferred
			RecordType: LabelRecordType, SchemaVersion: LabelSchemaVersion,
			Pair: "a-vs-b", GameIndex: 3, Seed: 1, Sequence: 13,
			Seat: 0, Kind: decision.KPriority, Turn: 9,
			Board: traceboardShim(), View: vb,
			Options: []decision.Option{
				{Index: 0, Kind: "cast", Label: "Thalia, Heretic Cathar", Obj: 10, Player: 0},
				{Index: 1, Kind: "pass", Label: "Pass", Player: 0},
			},
			Candidates: []labelCandidate{
				{Choices: []int{1}, Bot: true, Value: -0.4, Worlds: 16},
			},
			TeacherChoice: 5, BotIndex: 0, Margin: 0,
			Worlds: 16,
		},
	}
}

func traceboardShim() traceboard.Board {
	return traceboard.Board{SchemaVersion: traceboard.SchemaVersion}
}

func writeCorpus(t *testing.T, recs []labelRecord, gzipIt bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "labels.jsonl")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range recs {
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	data := buf.Bytes()
	if gzipIt {
		var zb bytes.Buffer
		zw := gzip.NewWriter(&zb)
		if _, err := zw.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		data = zb.Bytes()
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoaderRoundTrip(t *testing.T) {
	for _, gz := range []bool{false, true} {
		path := writeCorpus(t, fixtureRecords(), gz)
		examples, stats, err := Load(path)
		if err != nil {
			t.Fatalf("gzip=%v: %v", gz, err)
		}
		if stats.Records != 4 || stats.Skipped != 1 {
			t.Fatalf("gzip=%v stats: %d records, %d skipped, want 4/1", gz, stats.Records, stats.Skipped)
		}
		if len(examples) != 3 {
			t.Fatalf("gzip=%v: %d examples, want 3", gz, len(examples))
		}
		first := examples[0]
		if first.Pair != "a-vs-b" || first.Seed != 1 || first.Sequence != 7 || first.Turn != 6 {
			t.Fatalf("metadata lost: %+v", first)
		}
		if len(first.Options) != 3 {
			t.Fatalf("options: %d, want 3", len(first.Options))
		}
		// targets: option 0 = candidate 1's value −0.2, preferred (teacher's
		// choice); option 1 unlabelled; option 2 = candidate 0's −0.5, not
		// preferred.
		if !first.Options[0].Target.Labelled || first.Options[0].Target.Value != -0.2 || !first.Options[0].Target.Preferred {
			t.Fatalf("option 0 target = %+v", first.Options[0].Target)
		}
		if first.Options[1].Target.Labelled {
			t.Fatalf("option 1 target = %+v, want unlabelled", first.Options[1].Target)
		}
		if !first.Options[2].Target.Labelled || first.Options[2].Target.Value != -0.5 || first.Options[2].Target.Preferred {
			t.Fatalf("option 2 target = %+v", first.Options[2].Target)
		}
		if first.Margin != 0.3 || first.TeacherChoice != 1 || first.BotIndex != 0 {
			t.Fatalf("label metadata lost: %+v", first)
		}
		// the encoded state matches a direct encode of the fixture view
		var v view.View
		if err := json.Unmarshal(fixtureRecords()[0].View, &v); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first.State, EncodeState(v, 0)) {
			t.Fatal("loader state differs from a direct EncodeState")
		}
		// teacher-kept record: the bot's own option is preferred
		kept := examples[1]
		if !kept.Options[0].Target.Preferred || kept.Margin != 0 {
			t.Fatalf("teacher-kept targets: %+v margin %g", kept.Options[0].Target, kept.Margin)
		}
		// out-of-range teacher_choice: nobody preferred; the candidate's
		// answer (option 1) is still labelled
		oor := examples[2]
		if oor.Options[0].Target.Labelled || oor.Options[0].Target.Preferred {
			t.Fatalf("out-of-range teacher_choice, option 0: %+v, want unlabelled", oor.Options[0].Target)
		}
		if !oor.Options[1].Target.Labelled || oor.Options[1].Target.Preferred {
			t.Fatalf("out-of-range teacher_choice, option 1: %+v, want labelled, not preferred", oor.Options[1].Target)
		}
	}
}

func TestLoaderRejectsWrongSchema(t *testing.T) {
	recs := fixtureRecords()
	bad := recs[:1]
	bad[0].SchemaVersion = 2
	path := writeCorpus(t, bad, false)
	if _, _, err := Load(path); err == nil {
		t.Fatal("schema_version 2 accepted")
	}
	bad[0].SchemaVersion = LabelSchemaVersion
	bad[0].RecordType = "label-v0"
	path = writeCorpus(t, bad, false)
	if _, _, err := Load(path); err == nil {
		t.Fatal("record_type label-v0 accepted")
	}
}

func TestLoaderMissingFile(t *testing.T) {
	if _, _, err := Load(filepath.Join(t.TempDir(), "absent.jsonl")); err == nil {
		t.Fatal("missing corpus accepted")
	}
}

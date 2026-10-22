package policynet

import (
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

func relFixture() view.View {
	return view.View{
		Viewer:     0,
		Visibility: "seat",
		Step:       "main1",
		Phase:      "main1",
		Players: []view.PlayerView{
			{
				ID:   0,
				Name: "me",
				Battlefield: []view.CardView{
					{ID: 10, Name: "Grizzly Bears", Types: "Creature — Bear", Power: 2, Toughness: 2, Controller: 0, Owner: 0},
				},
				Hand: []view.CardView{
					{ID: 50, Name: "Shock", Types: "Instant", ManaCost: "{R}", Controller: 0, Owner: 0},
				},
			},
			{
				ID:   1,
				Name: "opp",
				Battlefield: []view.CardView{
					{ID: 20, Name: "Serra Angel", Types: "Creature — Angel", Power: 4, Toughness: 4, Keywords: []string{"Flying"}, Controller: 1, Owner: 1},
					{ID: 21, Name: "Mountain", Types: "Basic Land — Mountain", Controller: 1, Owner: 1},
				},
				Graveyard: []view.CardView{
					{ID: 22, Name: "Hill Giant", Types: "Creature — Giant", Controller: 1, Owner: 1},
				},
			},
		},
		Stack: []view.StackView{
			{ID: 40, Kind: "spell", Controller: 0, Card: &view.CardView{ID: 40, Name: "Lightning Bolt", Types: "Instant", Controller: 0}, Targets: []view.TargetView{{Obj: 20}}},
			{ID: 41, Kind: "spell", Controller: 1, Card: &view.CardView{ID: 41, Name: "Counterspell", Types: "Instant", Controller: 1}, Targets: []view.TargetView{{Obj: 40}}},
			{ID: 42, Kind: "ability", Controller: 1, Source: 20, Card: &view.CardView{ID: 20, Name: "Serra Angel", Types: "Creature — Angel", Controller: 1}, Targets: []view.TargetView{{Obj: 10}}},
			{ID: 43, Kind: "spell", Controller: 0, Card: &view.CardView{ID: 43, Name: "X Burn", Types: "Sorcery", Controller: 0}, Targets: []view.TargetView{{Player: 1, IsPlayer: true}}, X: 3, Kicks: 1},
			{ID: 44, Kind: "spell", Controller: 1, Card: &view.CardView{ID: 44, Name: "Raise Dead", Types: "Sorcery", Controller: 1}, Targets: []view.TargetView{{Obj: 22}}},
		},
	}
}

func relCard(t *testing.T, st State, idx int) []float32 {
	t.Helper()
	if idx >= len(st.Cards) {
		t.Fatalf("card %d missing (list has %d)", idx, len(st.Cards))
	}
	return st.Cards[idx].Raw
}

func wantRel(t *testing.T, raw []float32, want map[int]float32) {
	t.Helper()
	for i := EntityRawWidthV1; i < EntityRawWidth; i++ {
		w := want[i]
		if raw[i] != w {
			t.Errorf("slot %d (%s) = %v, want %v", i, entRelNames[i-EntityRawWidthV1], raw[i], w)
		}
	}
}

func TestEntityTargetSlots(t *testing.T) {
	v := relFixture()
	st := EncodeStateWith(FeaturesEntity, v, 0, nil)
	if len(st.Cards) != 8 {
		t.Fatalf("card list has %d cards, want 8", len(st.Cards))
	}
	wantRel(t, relCard(t, st, 0), map[int]float32{entTgtByOpp: 1}) // 10 Grizzly Bears
	wantRel(t, relCard(t, st, 1), map[int]float32{entTgtByMe: 1})  // 20 Serra Angel
	wantRel(t, relCard(t, st, 2), nil)                             // 21 Mountain
	wantRel(t, relCard(t, st, 3), nil)                             // 50 Shock
	wantRel(t, relCard(t, st, 4), map[int]float32{entTgtByOpp: 1, entTgtPermOpp: 1})
	wantRel(t, relCard(t, st, 5), map[int]float32{entTgtStack: 1})
	wantRel(t, relCard(t, st, 6), map[int]float32{entTgtPlayerOpp: 1, entStackX: float32(3) / 5, entStackKicked: 1})
	wantRel(t, relCard(t, st, 7), map[int]float32{entTgtGrave: 1})
	for k := range st.Cards {
		if got := relCard(t, st, k)[entReserved83]; got != 0 {
			t.Errorf("card %d: reserved slot = %v", k, got)
		}
	}

	st1 := EncodeStateWith(FeaturesEntity, v, 1, nil)
	if len(st1.Cards) != 7 {
		t.Fatalf("seat 1 card list has %d cards, want 7", len(st1.Cards))
	}
	wantRel(t, relCard(t, st1, 0), map[int]float32{entTgtByOpp: 1})                                                  // 20 Serra Angel
	wantRel(t, relCard(t, st1, 1), nil)                                                                              // 21 Mountain
	wantRel(t, relCard(t, st1, 2), map[int]float32{entTgtByMe: 1})                                                   // 10 Grizzly Bears
	wantRel(t, relCard(t, st1, 3), map[int]float32{entTgtByMe: 1, entTgtPermMe: 1})                                  // 40 Bolt
	wantRel(t, relCard(t, st1, 4), map[int]float32{entTgtStack: 1})                                                  // 41 Counterspell
	wantRel(t, relCard(t, st1, 5), map[int]float32{entTgtPlayerMe: 1, entStackX: float32(3) / 5, entStackKicked: 1}) // 43 X Burn
	wantRel(t, relCard(t, st1, 6), map[int]float32{entTgtGrave: 1})                                                  // 44 Raise Dead

	v0 := v
	v0.Stack = make([]view.StackView, len(v.Stack))
	copy(v0.Stack, v.Stack)
	for i := range v0.Stack {
		v0.Stack[i].Targets = nil
		v0.Stack[i].X = 0
		v0.Stack[i].Kicks = 0
	}
	base := EncodeStateWith(FeaturesEntity, v0, 0, nil)
	if len(base.Cards) != len(st.Cards) {
		t.Fatalf("prefix fixture changed the card list: %d vs %d", len(base.Cards), len(st.Cards))
	}
	for k := range st.Cards {
		a, b := relCard(t, st, k), relCard(t, base, k)
		for i := 0; i < EntityRawWidthV1; i++ {
			if a[i] != b[i] {
				t.Fatalf("card %d slot %d moved when targets were removed: %v vs %v", k, i, a[i], b[i])
			}
		}
		for i := EntityRawWidthV1; i < EntityRawWidth; i++ {
			if b[i] != 0 {
				t.Fatalf("card %d: relational slot %d = %v without targets", k, i, b[i])
			}
		}
	}
}

func TestEntityCombatSlots(t *testing.T) {
	v := view.View{
		Viewer:     0,
		Visibility: "seat",
		Step:       "declare-blockers",
		Phase:      "combat",
		Players: []view.PlayerView{
			{
				ID:   0,
				Name: "me",
				Battlefield: []view.CardView{
					{ID: 10, Name: "Hill Giant", Types: "Creature — Giant", Power: 3, Toughness: 3, Attacking: true, BlockedBy: []state.ObjID{20, 21}, Controller: 0, Owner: 0},
					{ID: 11, Name: "Grizzly Bears", Types: "Creature — Bear", Power: 2, Toughness: 2, Attacking: true, Controller: 0, Owner: 0},
					{ID: 12, Name: "Savannah Lions", Types: "Creature — Cat", Power: 1, Toughness: 1, Controller: 0, Owner: 0},
					{ID: 13, Name: "Darksteel Myr", Types: "Artifact Creature — Myr", Power: 0, Toughness: 1, Keywords: []string{"Indestructible"}, Controller: 0, Owner: 0},
				},
			},
			{
				ID:   1,
				Name: "opp",
				Battlefield: []view.CardView{
					{ID: 20, Name: "Runeclaw Bear", Types: "Creature — Bear", Power: 2, Toughness: 2, Controller: 1, Owner: 1},
					{ID: 21, Name: "Typhoid Rats", Types: "Creature — Rat", Power: 1, Toughness: 1, Keywords: []string{"Deathtouch"}, Controller: 1, Owner: 1},
					{ID: 22, Name: "Craw Wurm", Types: "Creature — Wurm", Power: 4, Toughness: 4, Attacking: true, BlockedBy: []state.ObjID{13}, Controller: 1, Owner: 1},
				},
			},
		},
	}
	st := EncodeStateWith(FeaturesEntity, v, 0, nil)
	if len(st.Cards) != 7 {
		t.Fatalf("card list has %d cards, want 7", len(st.Cards))
	}
	wantRel(t, relCard(t, st, 0), map[int]float32{
		entCombatFoes: 1, entCombatFoePow: 0.75, entCombatFoeTgh: 0.75,
		entCombatDies: 1, entCombatKills: 1, entBlocked: 1,
	}) // 10 Hill Giant, blocked by 20 and 21
	wantRel(t, relCard(t, st, 1), nil) // 11 unblocked attacker
	wantRel(t, relCard(t, st, 2), nil) // 12
	wantRel(t, relCard(t, st, 3), map[int]float32{
		entCombatFoes: 0.5, entCombatFoePow: 1, entCombatFoeTgh: 1,
		entBlocking: 1,
	}) // 13 Darksteel Myr blocks 22: cannot die, cannot kill
	wantRel(t, relCard(t, st, 4), map[int]float32{
		entCombatFoes: 0.5, entCombatFoePow: 0.75, entCombatFoeTgh: 0.75,
		entCombatDies: 1, entBlocking: 1,
	}) // 20 blocks 10
	wantRel(t, relCard(t, st, 5), map[int]float32{
		entCombatFoes: 0.5, entCombatFoePow: 0.75, entCombatFoeTgh: 0.75,
		entCombatDies: 1, entCombatKills: 1, entBlocking: 1,
	}) // 21 Typhoid Rats: deathtouch kills
	wantRel(t, relCard(t, st, 6), map[int]float32{
		entCombatFoes: 0.5, entCombatFoePow: 0, entCombatFoeTgh: 0.25,
		entCombatKills: 1,
	}) // 22 Craw Wurm blocked by 13

	v0 := v
	v0.Players[0].Battlefield[0].BlockedBy = nil
	v0.Players[1].Battlefield[2].BlockedBy = nil
	none := EncodeStateWith(FeaturesEntity, v0, 0, nil)
	for k := range none.Cards {
		for i := entCombatFoes; i <= entCombatKills; i++ {
			if got := none.Cards[k].Raw[i]; got != 0 {
				t.Errorf("no-block view: card %d slot %d (%s) = %v", k, i, entRelNames[i-EntityRawWidthV1], got)
			}
		}
	}
}

func TestEntityRelationalLayoutAndHashes(t *testing.T) {
	if EntityRawWidth != 84 || EntityRawWidthV1 != 68 {
		t.Fatalf("widths %d/%d, want 84/68", EntityRawWidth, EntityRawWidthV1)
	}
	if len(entRelNames) != EntityRawWidth-EntityRawWidthV1 {
		t.Fatalf("%d relational names for %d slots", len(entRelNames), EntityRawWidth-EntityRawWidthV1)
	}
	if entReserved83 != 83 {
		t.Fatalf("entReserved83 = %d", entReserved83)
	}
	if got := EntityRelNames()[entTgtByOpp-EntityRawWidthV1]; got != "tgt-by-opp" {
		t.Fatalf("slot %d named %q", entTgtByOpp, got)
	}
	if got := EncoderHashEntityV1(); got != 0x59a4a066ee4b0a7e {
		t.Fatalf("EncoderHashEntityV1 = %#016x, want 0x59a4a066ee4b0a7e", got)
	}
	if EncoderHashFor(FeaturesEntity) == EncoderHashEntityV1() {
		t.Fatal("live entity hash still equals the format-1 hash")
	}
	if _, ok := FeaturesForHash(EncoderHashEntityV1()); ok {
		t.Fatal("FeaturesForHash accepted the format-1 entity hash")
	}
	if _, ok := onPolicyFeatures(fmt.Sprintf("%016x", EncoderHashEntityV1())); ok {
		t.Fatal("onPolicyFeatures accepted the format-1 entity hash")
	}
}

func TestLegacyEntityCheckpointZeroPads(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	m, ex, _ := valueFixture(rng)
	m.Features = FeaturesMZ
	m.Rows = TableRows
	m.Table = make([]float32, TableRows*m.H)
	fillUniform(m.Table, 0.05, rng)
	if err := UpgradeEntity(m, 4, rand.New(rand.NewPCG(1, 2))); err != nil {
		t.Fatal(err)
	}
	for i := range m.EntW {
		m.EntW[i] = float32(i%7) * 0.01
	}
	for i := range m.EntP {
		m.EntP[i] = float32(i%7) * 0.01
	}
	inW := m.entInW()
	for q := 0; q < m.EntK; q++ {
		for j := EntityRawWidthV1; j < EntityRawWidth; j++ {
			m.EntW[q*inW+j] = 0
		}
	}
	truncated := make([]float32, m.EntK*(EntityRawWidthV1+m.H))
	for q := 0; q < m.EntK; q++ {
		copy(truncated[q*(EntityRawWidthV1+m.H):], m.EntW[q*inW:q*inW+EntityRawWidthV1])
		copy(truncated[q*(EntityRawWidthV1+m.H)+EntityRawWidthV1:], m.EntW[q*inW+EntityRawWidth:q*inW+inW])
	}

	var buf bytes.Buffer
	if err := writeCheckpointAs(m, &buf, EncoderHashEntityV1(), EntityRawWidthV1, truncated); err != nil {
		t.Fatal(err)
	}
	back, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("legacy checkpoint load: %v", err)
	}
	if back.Features != FeaturesEntity {
		t.Fatalf("loaded feature set %s", back.Features)
	}
	if !reflect.DeepEqual(back.EntW, m.EntW) {
		t.Fatal("widened EntW is not the zero-padded original")
	}

	ones := make([]float32, EntityRawWidth)
	for i := range ones {
		ones[i] = 1
	}
	st := ex.State
	st.Cards = []EntityCard{{Group: 1, Raw: ones, Rows: []Feature{{Row: 7, Value: 1}}}}
	st0 := st
	st0.Cards = []EntityCard{{Group: 1, Raw: append([]float32(nil), ones...), Rows: []Feature{{Row: 7, Value: 1}}}}
	clear(st0.Cards[0].Raw[EntityRawWidthV1:])
	opts := ex.Options
	wantS, wantV := m.Score(st, opts), m.Value(st)
	for _, tc := range []struct {
		name string
		net  *Model
		st   State
	}{{"widened", back, st}, {"widened-zero", back, st0}, {"source", m, st}} {
		gotS := tc.net.Score(tc.st, opts)
		if len(gotS) != len(wantS) {
			t.Fatalf("%s: %d scores", tc.name, len(gotS))
		}
		for i := range gotS {
			if math.Float32bits(gotS[i]) != math.Float32bits(wantS[i]) {
				t.Fatalf("%s: score %d %v vs %v", tc.name, i, gotS[i], wantS[i])
			}
		}
		if got := tc.net.Value(tc.st); math.Float32bits(got) != math.Float32bits(wantV) {
			t.Fatalf("%s: value %v vs %v", tc.name, got, wantV)
		}
	}

	for _, tc := range []struct {
		name   string
		hash   uint64
		entRaw int
		entW   []float32
	}{
		{"legacy-hash-wide", EncoderHashEntityV1(), EntityRawWidth, m.EntW},
		{"live-hash-narrow", EncoderHashFor(FeaturesEntity), EntityRawWidthV1, truncated},
	} {
		var b bytes.Buffer
		if err := writeCheckpointAs(m, &b, tc.hash, tc.entRaw, tc.entW); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadCheckpoint(bytes.NewReader(b.Bytes())); err == nil {
			t.Fatalf("%s: load succeeded, want a geometry error", tc.name)
		} else if !bytes.Contains([]byte(err.Error()), []byte("entity raw width")) {
			t.Fatalf("%s: error %q does not name the entity raw width", tc.name, err)
		}
	}
}

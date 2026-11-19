package policynet

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// podView is the four-seat Commander position the pod tests encode
// (gonettest.PodView is its copy for the gonet shape tests), from seat 0's view: seats 1 and
// 3 alive, seat 2 lost, seat 3 the monarch, seat 1 with the initiative and
// three poison counters, one of seat 0's creatures attacking seat 3 and
// goaded by seat 1, seat 3 with a partner pair (one in its command zone, one
// on the battlefield), and seat 0 dealt 6 and 9 combat damage by the pair.
func podView() view.View {
	v := view.View{Turn: 9, Step: "declare-blockers", Phase: "combat", Active: 0, Priority: 0, DayNight: "night"}
	for p := 0; p < 4; p++ {
		id := state.ObjID(100 + p)
		v.Players = append(v.Players, view.PlayerView{
			ID: state.PlayerID(p), Life: 40 - int32(p),
			Commanders:     []view.CardView{{ID: id, Name: "Commander", Owner: state.PlayerID(p), Controller: state.PlayerID(p)}},
			CommanderCasts: []int32{0},
		})
		v.Players[p].Command = []view.CardView{{ID: id, Name: "Commander", Owner: state.PlayerID(p), Controller: state.PlayerID(p), Types: "Legendary Creature"}}
	}
	atk := state.PlayerID(3)
	v.Players[0].Battlefield = []view.CardView{
		{ID: 10, Name: "Bear", Types: "Creature", Power: 2, Toughness: 2, Attacking: true, AttackingPlayer: &atk,
			GoadedBy: []state.PlayerID{1}, Controller: 0, Owner: 0},
		{ID: 11, Name: "Forest", Types: "Basic Land", Controller: 0, Owner: 0},
	}
	v.Players[0].Hand = []view.CardView{{ID: 12, Name: "Bear", Types: "Creature", ManaCost: "1 G", Controller: 0, Owner: 0}}
	v.Players[0].HandSize = 1
	v.Players[1].HasInitiative = true
	v.Players[1].Counters = map[string]int32{"POISON": 3}
	v.Players[1].Battlefield = []view.CardView{{ID: 20, Name: "Wall", Types: "Creature", Power: 0, Toughness: 4, Controller: 1, Owner: 1}}
	v.Players[2].Lost = true
	v.Players[3].IsMonarch = true
	v.Players[3].Commanders = []view.CardView{
		{ID: 103, Name: "Commander A", Owner: 3, Controller: 3},
		{ID: 113, Name: "Commander B", Owner: 3, Controller: 3},
	}
	v.Players[3].CommanderCasts = []int32{0, 1}
	v.Players[3].Command = []view.CardView{{ID: 103, Name: "Commander A", Owner: 3, Controller: 3, Types: "Legendary Creature"}}
	v.Players[3].Battlefield = []view.CardView{{ID: 113, Name: "Commander B", Types: "Legendary Creature", Power: 3, Toughness: 3, Owner: 3, Controller: 3}}
	v.Players[0].CmdDamage = map[state.ObjID]int32{103: 6, 113: 9}
	return v
}

// TestPodDenseFourSeat pins the pod blocks of FeaturesPod against podView:
// the monarch, initiative, poison and lost seats land in their relative
// seat's block, each partner's commander damage in its own column, the
// command-zone and partner bits, and the global round and night columns;
// the first 92 columns are FeaturesEntity's.
func TestPodDenseFourSeat(t *testing.T) {
	v := podView()
	st := EncodeStateWith(FeaturesPod, v, 0, nil)
	ent := EncodeStateWith(FeaturesEntity, v, 0, nil)
	if len(st.Dense) != DenseWidthPod || DenseWidthPod <= DenseWidth {
		t.Fatalf("dense width %d, want %d", len(st.Dense), DenseWidthPod)
	}
	for i := 0; i < DenseWidth; i++ {
		if st.Dense[i] != ent.Dense[i] {
			t.Fatalf("dense[%d] = %v, the entity set's %v", i, st.Dense[i], ent.Dense[i])
		}
	}
	col := func(r, f int) float32 { return st.Dense[DenseWidth+r*PodSeatWidth+f] }
	for _, c := range []struct {
		name string
		got  float32
		want float32
	}{
		{"seat 3 monarch", col(3, PodMonarch), 1},
		{"seat 1 not monarch", col(1, PodMonarch), 0},
		{"seat 1 initiative", col(1, PodInitiative), 1},
		{"seat 1 poison 3/10", col(1, PodPoison), 0.3},
		// seat 0's damage from relative seat 3 (k = 3): slots 0 and 1
		{"seat 0 from seat 3 slot 0", col(0, PodCmdDamage0+4), float32(6) / 21},
		{"seat 0 from seat 3 slot 1", col(0, PodCmdDamage0+5), float32(9) / 21},
		{"seat 3 slot 0 in command zone", col(3, PodCmdInZone0), 1},
		{"seat 3 slot 1 on the battlefield", col(3, PodCmdInZone0+1), 0},
		{"seat 3 partner", col(3, PodPartner), 1},
		{"seat 0 no partner", col(0, PodPartner), 0},
		// round: 1 + (9-1)/3 alive = 3
		{"round bucket", st.Dense[podGlobal0+podRoundBucket], float32(3) / 20},
		{"night", st.Dense[podGlobal0+podNight], 1},
		{"day", st.Dense[podGlobal0+podDay], 0},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	// The entity set's bag has no command-zone id; the pod set's has one
	// per commander there.
	cmd := hashID("Commander A|command")
	count := func(fs []Feature) int {
		n := 0
		for _, f := range fs {
			if f.Row == cmd {
				n++
			}
		}
		return n
	}
	if count(ent.Sparse) != 0 || count(st.Sparse) != 1 {
		t.Errorf("Commander A|command rows: entity %d, pod %d; want 0 and 1", count(ent.Sparse), count(st.Sparse))
	}
}

// TestPodCheckpointRoundTrip writes and loads a FeaturesPod model: the hash
// names the pod set, the geometry is the pod widths, and the loaded model
// scores the pod view exactly as the written one; the entity set's hash is
// a different one, so neither loads as the other.
func TestPodCheckpointRoundTrip(t *testing.T) {
	m, err := NewPodModel(TableRows, 8, 16, 4, rand.New(rand.NewPCG(1, 2)))
	if err != nil {
		t.Fatal(err)
	}
	if m.StateDenseW() != DenseWidthPod || m.Features != FeaturesPod {
		t.Fatalf("pod model: dense %d features %s", m.StateDenseW(), m.Features)
	}
	if EncoderHashFor(FeaturesPod) == EncoderHashFor(FeaturesEntity) {
		t.Fatal("the pod set's encoder hash is the entity set's")
	}
	if fs, ok := FeaturesForHash(EncoderHashFor(FeaturesPod)); !ok || fs != FeaturesPod {
		t.Fatalf("FeaturesForHash(pod hash) = %s, %v", fs, ok)
	}
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	l, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	v := podView()
	st := EncodeStateWith(FeaturesPod, v, 0, nil)
	if a, b := m.Value(st), l.Value(st); a != b {
		t.Errorf("loaded pod model's value %v, written %v", b, a)
	}
}

// TestMZLifeRowsSkipLostSeat (t1) pins the pod set's life and lethal rows
// at four seats from seat 2's view: seat 1 has lost at 0 life, so the first
// opponent slot is seat 3 (next in turn order), then seat 0; seat 1 has no
// slot, and its 0 life switches on no lethal row. The mz set keeps its rows
// (others[0], seat 0) as they were.
func TestMZLifeRowsSkipLostSeat(t *testing.T) {
	v := view.View{Turn: 5, Step: "main1", Active: 2, Priority: 2}
	for p := 0; p < 4; p++ {
		v.Players = append(v.Players, view.PlayerView{ID: state.PlayerID(p), Life: []int32{30, 0, 20, 12}[p]})
	}
	v.Players[1].Lost = true
	v.Players[2].Battlefield = []view.CardView{{ID: 50, Name: "Bear", Types: "Creature", Power: 2, Toughness: 2, Controller: 2}}
	st := EncodeStateWith(FeaturesPod, v, 2, nil)
	has := func(fs []Feature, s string) bool {
		r := hashID(s)
		for _, f := range fs {
			if f.Row == r {
				return true
			}
		}
		return false
	}
	for _, want := range []string{
		"mz|pod|opp0|life=6",      // seat 3: 12/2
		"mz|pod|lifediff0=9",      // (20-12)/3 + 7
		"mz|pod|opp1|life=15",     // seat 0: 30/2
		"mz|pod|lifediff1=" + "4", // (20-30)/3 + 7 = -3 + 7
		"mz|me|life=10",
	} {
		if !has(st.Sparse, want) {
			t.Errorf("pod set: no %q row", want)
		}
	}
	for _, not := range []string{"mz|pod|opp2|life=0", "mz|me|lethal-unblocked", "mz|opp|lethal-unblocked", "mz|opp|life=15", "mz|lifediff=4"} {
		if has(st.Sparse, not) {
			t.Errorf("pod set: unexpected %q row", not)
		}
	}
	// The mz set is unchanged: its rows read seat 0 (others[0]).
	mz := EncodeStateWith(FeaturesMZ, v, 2, nil)
	if !has(mz.Sparse, "mz|opp|life=15") || has(mz.Sparse, "mz|pod|opp0|life=6") {
		t.Error("mz set: the life rows moved")
	}
}

// TestPodEntitiesFourSeat pins the pod entity rows against podView: the
// relative-seat groups, the controller/owner/attacked/goader one-hots,
// entMine left zero, and the command-zone commanders listed last in the
// command group; an option naming a command-zone commander points at it.
func TestPodEntitiesFourSeat(t *testing.T) {
	v := podView()
	st := EncodeStateWith(FeaturesPod, v, 0, nil)
	byID := map[state.ObjID]EntityCard{}
	order := []state.ObjID{10, 11, 20, 113, 12, 100, 101, 102, 103}
	if len(st.Cards) != len(order) {
		t.Fatalf("%d entity cards, want %d", len(st.Cards), len(order))
	}
	for _, c := range st.Cards {
		if len(c.Raw) != EntityRawWidthPod {
			t.Fatalf("raw width %d, want %d", len(c.Raw), EntityRawWidthPod)
		}
	}
	for i, id := range order {
		byID[id] = st.Cards[i]
	}
	groups := map[state.ObjID]uint8{10: 0, 11: 0, 20: 1, 113: 3, 12: PodGroupHand, 100: PodGroupCommand, 103: PodGroupCommand}
	for id, g := range groups {
		if byID[id].Group != g {
			t.Errorf("card %d group %d, want %d", id, byID[id].Group, g)
		}
	}
	bear := byID[10].Raw
	for _, c := range []struct {
		name string
		col  int
		want float32
	}{
		{"bear controller rel 0", PodEntCtrl0, 1},
		{"bear owner rel 0", PodEntOwner0, 1},
		{"bear attacks rel 3", PodEntAttacked0 + 3, 1},
		{"bear attacks no permanent", PodEntAttacksPerm, 0},
		{"bear goaded by rel 1", PodEntGoadedBy0 + 1, 1},
		{"bear entMine zero", entMine, 0},
		{"bear not a commander", PodEntCommander, 0},
	} {
		if bear[c.col] != c.want {
			t.Errorf("%s: raw[%d] = %v, want %v", c.name, c.col, bear[c.col], c.want)
		}
	}
	if r := byID[20].Raw; r[PodEntCtrl0+1] != 1 || r[PodEntCtrl0] != 0 {
		t.Errorf("seat 1's wall: controller one-hot %v", r[PodEntCtrl0:PodEntCtrl0+4])
	}
	if r := byID[113].Raw; r[PodEntCommander] != 1 || r[PodEntCommandZone] != 0 || r[PodEntCtrl0+3] != 1 {
		t.Errorf("seat 3's battlefield commander: commander %v command zone %v ctrl %v", r[PodEntCommander], r[PodEntCommandZone], r[PodEntCtrl0:PodEntCtrl0+4])
	}
	if r := byID[103].Raw; r[PodEntCommander] != 1 || r[PodEntCommandZone] != 1 || r[PodEntOwner0+3] != 1 {
		t.Errorf("seat 3's command-zone commander: commander %v command zone %v owner %v", r[PodEntCommander], r[PodEntCommandZone], r[PodEntOwner0:PodEntOwner0+4])
	}
	// the entity set has none of it
	ent := EncodeStateWith(FeaturesEntity, v, 0, nil)
	if len(ent.Cards) != 5 || len(ent.Cards[0].Raw) != EntityRawWidth || ent.Cards[0].Raw[entMine] != 1 {
		t.Errorf("entity set: %d cards, raw %d, bear entMine %v", len(ent.Cards), len(ent.Cards[0].Raw), ent.Cards[0].Raw[entMine])
	}
	var x OptionEncoder
	x.Reset(&v, 0)
	o := x.Encode(FeaturesPod, decision.KPriority, decision.Option{Kind: "cast", Obj: 100}, 0, 1)
	if o.EntA != int32(1+indexOf64(order, 100)) {
		t.Errorf("a command-zone cast's EntA = %d, want %d", o.EntA, 1+indexOf64(order, 100))
	}
}

func indexOf64(ids []state.ObjID, id state.ObjID) int {
	for i, x := range ids {
		if x == id {
			return i
		}
	}
	return -1
}

// TestPodMLPScoresFourSeat runs a FeaturesPod model's forward over the
// four-seat Commander state: the entity encoder reads the 120-wide rows
// into seven pooled groups, the scores are finite, and a pod entity column
// (the attacker's attacked seat) moves them once its weights are nonzero.
func TestPodMLPScoresFourSeat(t *testing.T) {
	m, err := NewPodModel(TableRows, 8, 16, 4, rand.New(rand.NewPCG(3, 4)))
	if err != nil {
		t.Fatal(err)
	}
	if m.entPoolW() != EntityGroupsPod*2*m.EntK || m.entInW() != EntityRawWidthPod+m.H {
		t.Fatalf("pod model geometry: pool %d in %d", m.entPoolW(), m.entInW())
	}
	// the warm start zeroes the pooled projection: give it signal
	for i := range m.EntP {
		m.EntP[i] = float32(i%7)*0.01 - 0.03
	}
	v := podView()
	st := EncodeStateWith(FeaturesPod, v, 0, nil)
	opts := EncodeOptionsWith(FeaturesPod, &v, 0, decision.KPriority,
		[]decision.Option{{Kind: "cast", Obj: 100}, {Kind: "pass"}}, nil)
	a := m.Score(st, opts)
	for _, x := range a {
		if x != x {
			t.Fatalf("scores %v", a)
		}
	}
	st.Cards[0].Raw[PodEntAttacked0+3], st.Cards[0].Raw[PodEntAttacked0+1] = 0, 1
	b := m.Score(st, opts)
	if a[0] == b[0] && a[1] == b[1] {
		t.Errorf("the attacked-seat column does not move the scores %v", a)
	}
}

package shadow_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/shadow"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// deckNames is a deck's card names, one per copy, in deck order.
func deckNames(tbl *kop.Table, deck []kop.CardID) []string {
	out := make([]string, len(deck))
	for i := range deck {
		out[i] = tbl.Cards[deck[i]].Name
	}
	return out
}

// layoutGame plays one lockstep game of env and, at the first priority
// decision at or after a random one (that env wants, when gated), stages both the verifier's
// true world (its ceased tokens read from the replay) and the Layout's world
// from the same view. The Layout is dealt the view of every priority decision
// before it, as a seat's is, so it has seen the tokens that left. It returns ""
// or a failure text.
func layoutGame(reg *cards.Registry, env *stageEnv, gated bool, seed uint64, compared *int, staged *int) string {
	r := rand.New(rand.NewPCG(seed, 0x5748))
	decks := [2][]kop.CardID{env.deck(r), env.deck(r)}
	want := 2 + r.IntN(160)
	m, err := oracle.KernelMatchup(reg, env.tbl, [2]string{"a", "b"}, decks, nil, 0)
	if err != nil {
		return err.Error()
	}
	k, err := env.kernel(m, seed)
	if err != nil {
		return err.Error()
	}
	defer oracle.CloseSide(k)
	kstate, _, _ := oracle.KernelStateOf(k)
	layout, err := shadow.NewLayout(env.tbl, [][]string{deckNames(env.tbl, decks[0]), deckNames(env.tbl, decks[1])})
	if err != nil {
		return "NewLayout: " + err.Error()
	}
	n, done, fail := 0, false, ""
	check := func(ref *oracle.GorgeSide, _ oracle.Side) *oracle.Divergence {
		n++
		if done {
			return nil
		}
		d := ref.Pending()
		if d == nil || d.Kind != "priority" {
			return nil
		}
		g := ref.E.G
		if n < want {
			seen := view.ProjectFor(g, ref.E, 0, view.Seat, d)
			if _, _, err := layout.Deal(&seen, stageCfg(), seed); err != nil {
				fail = fmt.Sprintf("seed %d: Layout.Deal before the position: %v", seed, err)
				return errDone
			}
			return nil
		}
		viewer := state.PlayerID(r.IntN(2))
		v := view.ProjectFor(g, ref.E, viewer, view.Seat, d)
		if gated && !env.want(seed, &v, d) {
			if _, _, err := layout.Deal(&v, stageCfg(), seed); err != nil {
				fail = fmt.Sprintf("seed %d: Layout.Deal before the position: %v", seed, err)
				return errDone
			}
			return nil
		}
		// The verifier's true input, exactly as stageGame builds it.
		hidden := shadow.Hidden{}
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.Zone(state.ZHand, p) {
				hidden.Hand[p] = append(hidden.Hand[p], card(ref, g, id))
			}
			for _, id := range g.Zone(state.ZLibrary, p) {
				hidden.Library[p] = append(hidden.Library[p], card(ref, g, id))
			}
		}
		trueIn := &shadow.Input{
			View: v, Hidden: hidden, Tbl: env.tbl, Cfg: stageCfg(), Seed: seed,
			ObjOf: func(id int) (dirac.Obj, bool) { return dirac.Obj(ref.InstRef(state.ObjID(id))), true },
		}
		var kp dirac.Projection
		kstate.Project(&kp)
		var terr error
		if trueIn.Hidden.Ceased, terr = ceasedTokens(kstate, env.tbl, int(kp.NObj)); terr != nil {
			return errDone // a token in a zone no view shows: the verifier refuses the position
		}
		layoutIn, _, lerr := layout.Deal(&v, stageCfg(), seed)
		if lerr != nil {
			fail = fmt.Sprintf("seed %d: Layout.Deal: %v", seed, lerr)
			return errDone
		}
		trueWorld, trueErr := shadow.WorldFromView(trueIn)
		var trueState *dirac.State
		if trueErr == nil {
			trueState, trueErr = dirac.NewFromWorld(&trueWorld)
		}
		layoutState, layoutErr := shadow.StageFromView(layoutIn)
		*compared++
		if (trueErr == nil) != (layoutErr == nil) {
			if trueErr == nil {
				fail = fmt.Sprintf("seed %d: true stages, layout does not: %v", seed, layoutErr)
			} else {
				fail = fmt.Sprintf("seed %d: true does not stage (%v), layout stages", seed, trueErr)
			}
			return errDone
		}
		if trueErr != nil {
			var tu, lu dirac.ErrUnstageable
			if !errors.As(trueErr, &tu) || !errors.As(layoutErr, &lu) {
				fail = fmt.Sprintf("seed %d: non-ErrUnstageable: true %v, layout %v", seed, trueErr, layoutErr)
			} else if tu.Reason != lu.Reason {
				fail = fmt.Sprintf("seed %d: reasons differ: true %s (%s), layout %s (%s)", seed, tu.Reason, tu.Detail, lu.Reason, lu.Detail)
			}
			return errDone
		}
		if msg := compareLayoutProjections(trueState, layoutState, viewer); msg != "" {
			fail = fmt.Sprintf("seed %d: %s", seed, msg)
			return errDone
		}
		*staged++
		return errDone
	}
	res := oracle.Lockstep(m, seed, k, oracle.RandomPolicy(seed), oracle.RunConfig{MaxDecisions: 4000, MaxTurns: 60, Check: check})
	done = true
	if res.Div != nil && res.Div != errDone {
		return fmt.Sprintf("seed %d: lockstep diverged: %s", seed, res.Div)
	}
	return fail
}

// TestLayoutDealMatchesTruth stages the same positions from the verifier's
// true input and from a Layout built only from the view and the open deck
// lists, and requires the projections to agree.
func TestLayoutDealMatchesTruth(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	t.Run("vanilla", func(t *testing.T) { layoutTruth(t, reg, vanillaEnv(), false, 120) })
	t.Run("mbc", func(t *testing.T) { layoutTruth(t, reg, cardsEnv(t), true, 120) })
}

func layoutTruth(t *testing.T, reg *cards.Registry, env *stageEnv, gated bool, games int) {
	var compared, staged int
	for seed := uint64(1); seed <= uint64(games); seed++ {
		if msg := layoutGame(reg, env, gated, seed, &compared, &staged); msg != "" {
			t.Error(msg)
			break
		}
	}
	t.Logf("%d games: %d positions compared, %d staged both ways", games, compared, staged)
	if compared < 100 {
		t.Errorf("only %d positions compared, need at least 100", compared)
	}
	if staged < compared*8/10 {
		t.Errorf("only %d of %d compared positions staged both ways", staged, compared)
	}
}

// compareLayoutProjections checks two staged states that number their objects
// differently: every non-object field (except Draws and the object indices in
// Stack), and the objects as (owner, zone) card multisets and, for the ordered
// zones, as sequences of their visible state, with Attached and Blocks compared
// through the (zone, ZoneIdx) of the object they name.
func compareLayoutProjections(a, b *dirac.State, viewer state.PlayerID) string {
	var pa, pb dirac.Projection
	a.Project(&pa)
	b.Project(&pb)
	if msg := compareNonObjects(&pa, &pb); msg != "" {
		return msg
	}
	if msg := compareObjectMultisets(&pa, &pb, viewer); msg != "" {
		return msg
	}
	if msg := compareObjectSequences(&pa, &pb, viewer); msg != "" {
		return msg
	}
	return compareStacks(&pa, &pb)
}

// compareNonObjects compares every Projection field but the object array, the
// draw count and the stack (whose object references are numbered differently).
func compareNonObjects(a, b *dirac.Projection) string {
	va, vb := reflect.ValueOf(a).Elem(), reflect.ValueOf(b).Elem()
	for i := 0; i < va.NumField(); i++ {
		name := va.Type().Field(i).Name
		switch name {
		case "Objs", "Draws", "Stack":
			continue
		}
		if !reflect.DeepEqual(va.Field(i).Interface(), vb.Field(i).Interface()) {
			return fmt.Sprintf("%s differs: %v vs %v", name, va.Field(i).Interface(), vb.Field(i).Interface())
		}
	}
	return ""
}

type objKey struct {
	owner uint8
	zone  kop.Zone
}

func objectGroups(p *dirac.Projection) map[objKey][]int {
	m := map[objKey][]int{}
	for i := 0; i < int(p.NObj); i++ {
		o := &p.Objs[i]
		k := objKey{o.Owner, o.Zone}
		m[k] = append(m[k], i)
	}
	return m
}

func sortedKeys(m map[objKey][]int) []objKey {
	keys := make([]objKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].owner != keys[j].owner {
			return keys[i].owner < keys[j].owner
		}
		return keys[i].zone < keys[j].zone
	})
	return keys
}

func compareObjectMultisets(a, b *dirac.Projection, viewer state.PlayerID) string {
	ga, gb := objectGroups(a), objectGroups(b)
	if len(ga) != len(gb) {
		return fmt.Sprintf("object (owner, zone) groups differ: %d vs %d", len(ga), len(gb))
	}
	// A non-viewer's hand and library are dealt arbitrarily: only their
	// combined multiset is knowable from the list. The viewer's hand is shown,
	// so its library is the exact remaining multiset and compares per zone.
	var hiddenA, hiddenB [dirac.MaxSeats][]dirac.CardID
	for _, k := range sortedKeys(ga) {
		if k.owner != uint8(viewer) && (k.zone == kop.ZoneHand || k.zone == kop.ZoneLibrary) {
			hiddenA[k.owner] = append(hiddenA[k.owner], cardIDs(a, ga[k])...)
			hiddenB[k.owner] = append(hiddenB[k.owner], cardIDs(b, gb[k])...)
			continue
		}
		ca, cb := cardIDs(a, ga[k]), cardIDs(b, gb[k])
		if !reflect.DeepEqual(ca, cb) {
			return fmt.Sprintf("seat %d %v cards differ: %v vs %v", k.owner, k.zone, ca, cb)
		}
	}
	for s := range hiddenA {
		sort.Slice(hiddenA[s], func(i, j int) bool { return hiddenA[s][i] < hiddenA[s][j] })
		sort.Slice(hiddenB[s], func(i, j int) bool { return hiddenB[s][i] < hiddenB[s][j] })
		if !reflect.DeepEqual(hiddenA[s], hiddenB[s]) {
			return fmt.Sprintf("seat %d hidden hand+library cards differ: %v vs %v", s, hiddenA[s], hiddenB[s])
		}
	}
	return ""
}

func cardIDs(p *dirac.Projection, idx []int) []dirac.CardID {
	out := make([]dirac.CardID, len(idx))
	for i, j := range idx {
		out[i] = p.Objs[j].Card
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// objTuple is the visible state of one object the ordered-zone comparison
// holds to. A reference to another object is its (zone, ZoneIdx) place.
type objTuple struct {
	card     dirac.CardID
	zoneIdx  uint8
	flags    uint8
	damage   uint8
	counters [kop.NumCounterKinds2]uint16
	token    bool
	attZone  uint8
	attIdx   uint8
	attached bool
	blkZone  uint8
	blkIdx   uint8
	blocking bool
}

func tupleOf(p *dirac.Projection, i int) objTuple {
	o := &p.Objs[i]
	t := objTuple{
		card: o.Card, zoneIdx: o.ZoneIdx, flags: o.Flags, damage: o.Damage,
		counters: o.Counters, token: o.Token,
	}
	t.attZone, t.attIdx, t.attached = objectPlace(p, o.Attached)
	t.blkZone, t.blkIdx, t.blocking = objectPlace(p, o.Blocks)
	return t
}

func objectPlace(p *dirac.Projection, o dirac.Obj) (uint8, uint8, bool) {
	if o == dirac.NoObj || int(o) >= int(p.NObj) {
		return 0, 0, false
	}
	x := &p.Objs[o]
	return uint8(x.Zone), x.ZoneIdx, true
}

func compareObjectSequences(a, b *dirac.Projection, viewer state.PlayerID) string {
	ga, gb := objectGroups(a), objectGroups(b)
	for _, k := range sortedKeys(ga) {
		ordered := k.zone == kop.ZoneBattlefield || k.zone == kop.ZoneGraveyard || k.zone == kop.ZoneStack ||
			k.zone == kop.ZoneHand && k.owner == uint8(viewer)
		if !ordered {
			continue
		}
		ia, ib := append([]int(nil), ga[k]...), append([]int(nil), gb[k]...)
		if k.zone == kop.ZoneStack {
			ia, ib = stackOrder(a, k.owner, ia), stackOrder(b, k.owner, ib)
		} else {
			sort.SliceStable(ia, func(i, j int) bool { return a.Objs[ia[i]].ZoneIdx < a.Objs[ia[j]].ZoneIdx })
			sort.SliceStable(ib, func(i, j int) bool { return b.Objs[ib[i]].ZoneIdx < b.Objs[ib[j]].ZoneIdx })
		}
		if len(ia) != len(ib) {
			return fmt.Sprintf("seat %d %v has %d vs %d ordered objects", k.owner, k.zone, len(ia), len(ib))
		}
		for n := range ia {
			ta, tb := tupleOf(a, ia[n]), tupleOf(b, ib[n])
			if ta != tb {
				return fmt.Sprintf("seat %d %v object %d differs: %+v vs %+v", k.owner, k.zone, n, ta, tb)
			}
		}
	}
	return ""
}

// stackOrder orders a group's stack objects by their spell entries, bottom to
// top, and leaves any object no entry names after them in (card, ZoneIdx) order.
func stackOrder(p *dirac.Projection, owner uint8, idx []int) []int {
	at := map[int]int{}
	for i := 0; i < int(p.NStack); i++ {
		if p.Stack[i].Kind != 0 {
			continue
		}
		if o := int(p.Stack[i].Source); o < int(p.NObj) && p.Objs[o].Zone == kop.ZoneStack && p.Objs[o].Owner == owner {
			if _, ok := at[o]; !ok {
				at[o] = i
			}
		}
	}
	sort.SliceStable(idx, func(i, j int) bool {
		ai, aok := at[idx[i]]
		bi, bok := at[idx[j]]
		if aok != bok {
			return aok
		}
		if aok {
			return ai < bi
		}
		return p.Objs[idx[i]].Card < p.Objs[idx[j]].Card
	})
	return idx
}

// pairObjects maps each object index of a to the object index of b with the
// same place, so Stack's object references can be compared.
func pairObjects(a, b *dirac.Projection) (map[dirac.Obj]dirac.Obj, string) {
	m := map[dirac.Obj]dirac.Obj{}
	sa, sb := stackSpellObjects(a), stackSpellObjects(b)
	if len(sa) != len(sb) {
		return nil, fmt.Sprintf("stack spell objects differ: %d vs %d", len(sa), len(sb))
	}
	for i := range sa {
		m[sa[i]] = sb[i]
	}
	ga, gb := objectGroups(a), objectGroups(b)
	for _, k := range sortedKeys(ga) {
		if k.zone == kop.ZoneStack {
			continue
		}
		ia, ib := append([]int(nil), ga[k]...), append([]int(nil), gb[k]...)
		less := func(p *dirac.Projection, x, y int) bool {
			if k.zone == kop.ZoneLibrary {
				if p.Objs[x].Card != p.Objs[y].Card {
					return p.Objs[x].Card < p.Objs[y].Card
				}
			}
			return p.Objs[x].ZoneIdx < p.Objs[y].ZoneIdx
		}
		sort.SliceStable(ia, func(i, j int) bool { return less(a, ia[i], ia[j]) })
		sort.SliceStable(ib, func(i, j int) bool { return less(b, ib[i], ib[j]) })
		if len(ia) != len(ib) {
			return nil, fmt.Sprintf("seat %d %v objects differ: %d vs %d", k.owner, k.zone, len(ia), len(ib))
		}
		for i := range ia {
			m[dirac.Obj(ia[i])] = dirac.Obj(ib[i])
		}
	}
	return m, ""
}

func stackSpellObjects(p *dirac.Projection) []dirac.Obj {
	var out []dirac.Obj
	for i := 0; i < int(p.NStack); i++ {
		e := &p.Stack[i]
		if e.Kind != 0 {
			continue
		}
		if int(e.Source) < int(p.NObj) && p.Objs[e.Source].Zone == kop.ZoneStack {
			out = append(out, e.Source)
		}
	}
	return out
}

func compareStacks(a, b *dirac.Projection) string {
	m, msg := pairObjects(a, b)
	if msg != "" {
		return msg
	}
	if a.NStack != b.NStack {
		return fmt.Sprintf("stack length differs: %d vs %d", a.NStack, b.NStack)
	}
	for i := 0; i < int(a.NStack); i++ {
		ea, eb := a.Stack[i], b.Stack[i]
		if ea.Source != dirac.NoObj {
			mapped, ok := m[ea.Source]
			if !ok {
				return fmt.Sprintf("stack entry %d source %d has no partner", i, ea.Source)
			}
			ea.Source = mapped
		}
		for j := range ea.Targets {
			ea.Targets[j] = remapTarget(ea.Targets[j], m)
		}
		for j := 0; j < int(ea.NExtra); j++ {
			ea.Extra[j] = remapTarget(ea.Extra[j], m)
		}
		if ea != eb {
			return fmt.Sprintf("stack entry %d differs: %+v vs %+v", i, ea, eb)
		}
	}
	return ""
}

func remapTarget(c dirac.TCode, m map[dirac.Obj]dirac.Obj) dirac.TCode {
	if !dirac.IsObjTarget(c) {
		return c
	}
	if mapped, ok := m[dirac.Obj(c)]; ok {
		return dirac.TCode(mapped)
	}
	return c
}

func repeatNames(name string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = name
	}
	return out
}

func layoutOpenLists() [][]string {
	one := append(repeatNames("Plains", 7), repeatNames("Savannah Lions", 13)...)
	return [][]string{one, append([]string(nil), one...)}
}

// TestLayoutDealReasons: a shown card outside the list and a hidden count off
// by one are ReasonHidden, and NewLayout refuses each malformed deck list.
func TestLayoutDealReasons(t *testing.T) {
	t.Parallel()
	cfg := dirac.LockstepConfig(dirac.SurfaceFull)
	base := reasonsInput()
	lists := layoutOpenLists()
	l, err := shadow.NewLayout(vanilla.Table, lists)
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	in0, _, err := l.Deal(&base.View, cfg, 1)
	if err != nil {
		t.Fatalf("the legal view does not deal: %v", err)
	}
	if in0.Tbl != vanilla.Table || in0.Cfg != cfg || in0.Seed != 1 {
		t.Errorf("Input carries table %p cfg %+v seed %d, want the Layout's table, cfg %+v and seed 1", in0.Tbl, in0.Cfg, in0.Seed, cfg)
	}

	// A card the owner's list does not hold.
	in := reasonsInput()
	allPlains := [][]string{repeatNames("Plains", 20), repeatNames("Plains", 20)}
	l2, err := shadow.NewLayout(vanilla.Table, allPlains)
	if err != nil {
		t.Fatalf("NewLayout(all Plains): %v", err)
	}
	in.View.Players[0].Hand[0].Name = "Savannah Lions"
	checkReason(t, "a card beyond the list", func() error {
		_, _, err := l2.Deal(&in.View, cfg, 1)
		return err
	}, shadow.ReasonHidden)

	// The hidden count is off by one.
	in = reasonsInput()
	in.View.Players[0].LibrarySize++
	checkReason(t, "hidden count off by one", func() error {
		_, _, err := l.Deal(&in.View, cfg, 1)
		return err
	}, shadow.ReasonHidden)

	for name, tc := range map[string]struct {
		lists [][]string
	}{
		"one list":          {[][]string{repeatNames("Plains", 20)}},
		"five lists":        {[][]string{repeatNames("Plains", 7), repeatNames("Plains", 7), repeatNames("Plains", 7), repeatNames("Plains", 7), repeatNames("Plains", 7)}},
		"six-card list":     {[][]string{repeatNames("Plains", 6), repeatNames("Plains", 20)}},
		"unknown card name": {[][]string{repeatNames("Black Lotus", 20), repeatNames("Plains", 20)}},
	} {
		if _, err := shadow.NewLayout(vanilla.Table, tc.lists); err == nil {
			t.Errorf("NewLayout accepted %s", name)
		} else {
			var u dirac.ErrUnstageable
			if errors.As(err, &u) {
				t.Errorf("NewLayout(%s) returned an ErrUnstageable, want a plain error: %v", name, err)
			}
		}
	}
}

func checkReason(t *testing.T, what string, run func() error, want string) {
	t.Helper()
	err := run()
	var u dirac.ErrUnstageable
	if !errors.As(err, &u) || u.Reason != want {
		t.Errorf("%s: got %v, want reason %q", what, err, want)
	}
}

// tokenView is a two-seat view with no physical cards on the battlefield of
// seat 1 and the given tokens, seat 0 also holding a Plains and a hand of
// seven Plains in the hidden zones.
func tokenView(plains0 int, tokens0, tokens1 []view.CardView) view.View {
	v := view.View{Viewer: 0, Turn: 1, Step: "main1", Active: 0, Priority: 0}
	p0 := view.PlayerView{ID: 0, Life: 20, HandSize: 0, LibrarySize: plains0, Battlefield: []view.CardView{{ID: 500, Name: "Plains"}}}
	p0.Battlefield = append(p0.Battlefield, tokens0...)
	p1 := view.PlayerView{ID: 1, Life: 20, HandSize: 0, LibrarySize: 7, Battlefield: append([]view.CardView(nil), tokens1...)}
	v.Players = []view.PlayerView{p0, p1}
	return v
}

func token(id state.ObjID, owner state.PlayerID) view.CardView {
	return view.CardView{ID: id, Name: "Goblin Token", IsToken: true, Owner: owner, Controller: owner}
}

// TestLayoutTokensSticky: the token registry is sticky. Tokens first seen
// earlier keep lower indices for good, a departed token leaves its index (it is
// ceased, TestLayoutKeepsCeasedTokens), and ViewID inverts ObjOf for every
// shown card.
func TestLayoutTokensSticky(t *testing.T) {
	t.Parallel()
	cfg := dirac.LockstepConfig(dirac.SurfaceFull)
	l, err := shadow.NewLayout(vanilla.Table, [][]string{repeatNames("Plains", 7), repeatNames("Plains", 7)})
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	a, b, c := state.ObjID(100), state.ObjID(200), state.ObjID(201)
	v0 := tokenView(6, []view.CardView{token(a, 0)}, []view.CardView{token(b, 1)})
	in0, n0, err := l.Deal(&v0, cfg, 1)
	if err != nil {
		t.Fatalf("Deal 0: %v", err)
	}
	wantObj(t, n0, a, 14)
	wantObj(t, n0, b, 15)
	wantObj(t, n0, 500, 0)
	wantView(t, n0, 0, 500)
	wantView(t, n0, 14, a)
	wantView(t, n0, 15, b)
	if o, ok := in0.ObjOf(500); !ok || o != 0 {
		t.Errorf("Input.ObjOf(500) = %d, %v, want 0", o, ok)
	}

	// The first-seen token left: the next one keeps its index.
	v1 := tokenView(6, nil, []view.CardView{token(b, 1)})
	_, n1, err := l.Deal(&v1, cfg, 1)
	if err != nil {
		t.Fatalf("Deal 1: %v", err)
	}
	wantObj(t, n1, b, 15)
	wantView(t, n1, 15, b)
	wantNoView(t, n1, 14)

	// A new token is appended after the surviving sticky rank.
	v2 := tokenView(6, nil, []view.CardView{token(b, 1), token(c, 1)})
	_, n2, err := l.Deal(&v2, cfg, 1)
	if err != nil {
		t.Fatalf("Deal 2: %v", err)
	}
	wantObj(t, n2, b, 15)
	wantObj(t, n2, c, 16)
	for _, id := range []state.ObjID{a, b, c, 500} {
		o, ok := n2.ObjOf(int(id))
		if !ok {
			continue
		}
		if got, ok := n2.ViewID(o); !ok || got != int(id) {
			t.Errorf("ViewID(ObjOf(%d)) = %d, %v", id, got, ok)
		}
	}
}

func wantObj(t *testing.T, n *shadow.Numbering, id state.ObjID, want dirac.Obj) {
	t.Helper()
	if got, ok := n.ObjOf(int(id)); !ok || got != want {
		t.Errorf("ObjOf(%d) = %d, %v, want %d", id, got, ok, want)
	}
}

func wantView(t *testing.T, n *shadow.Numbering, o dirac.Obj, want state.ObjID) {
	t.Helper()
	if got, ok := n.ViewID(o); !ok || got != int(want) {
		t.Errorf("ViewID(%d) = %d, %v, want %d", o, got, ok, want)
	}
}

func wantNoView(t *testing.T, n *shadow.Numbering, o dirac.Obj) {
	t.Helper()
	if got, ok := n.ViewID(o); ok {
		t.Errorf("ViewID(%d) = %d, true, want none", o, got)
	}
}

// TestLayoutDealDeterministic: two Layouts over the same lists and view
// sequence deal deep-equal hidden zones and the same numbering.
func TestLayoutDealDeterministic(t *testing.T) {
	t.Parallel()
	cfg := dirac.LockstepConfig(dirac.SurfaceFull)
	lists := [][]string{repeatNames("Plains", 7), repeatNames("Plains", 7)}
	l1, err := shadow.NewLayout(vanilla.Table, lists)
	if err != nil {
		t.Fatalf("NewLayout 1: %v", err)
	}
	l2, err := shadow.NewLayout(vanilla.Table, lists)
	if err != nil {
		t.Fatalf("NewLayout 2: %v", err)
	}
	views := []view.View{
		tokenView(6, []view.CardView{token(100, 0)}, []view.CardView{token(200, 1)}),
		tokenView(6, nil, []view.CardView{token(200, 1)}),
		tokenView(6, nil, []view.CardView{token(200, 1), token(201, 1)}),
	}
	for i := range views {
		a, na, err := l1.Deal(&views[i], cfg, 7)
		if err != nil {
			t.Fatalf("Deal 1.%d: %v", i, err)
		}
		b, nb, err := l2.Deal(&views[i], cfg, 7)
		if err != nil {
			t.Fatalf("Deal 2.%d: %v", i, err)
		}
		if !reflect.DeepEqual(a.Hidden, b.Hidden) {
			t.Errorf("view %d: Hidden differs: %+v vs %+v", i, a.Hidden, b.Hidden)
		}
		for _, id := range []int{100, 200, 201, 500} {
			oa, oka := na.ObjOf(id)
			ob, okb := nb.ObjOf(id)
			if oka != okb || oa != ob {
				t.Errorf("view %d: ObjOf(%d) differs: %d/%v vs %d/%v", i, id, oa, oka, ob, okb)
			}
		}
		for o := dirac.Obj(0); o < 20; o++ {
			ia, oka := na.ViewID(o)
			ib, okb := nb.ViewID(o)
			if oka != okb || ia != ib {
				t.Errorf("view %d: ViewID(%d) differs: %d/%v vs %d/%v", i, o, ia, oka, ib, okb)
			}
		}
	}
}

// TestLayoutKeepsCeasedTokens: a token that disappears from the view keeps its
// index as a CeasedToken with the table name and owner it had when last shown,
// and a token seen after it takes the next index.
func TestLayoutKeepsCeasedTokens(t *testing.T) {
	t.Parallel()
	tbl := *vanilla.Table
	tbl.Cards = append(slices.Clone(vanilla.Table.Cards),
		kop.CardDef{Name: "r_1_1_goblin", Types: kop.TCreature | kop.TToken, Power: 1, Toughness: 1},
		kop.CardDef{Name: "w_2_2_knight", Types: kop.TCreature | kop.TToken, Power: 2, Toughness: 2})
	cfg := dirac.LockstepConfig(dirac.SurfaceFull)
	l, err := shadow.NewLayout(&tbl, [][]string{repeatNames("Plains", 7), repeatNames("Plains", 7)})
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	named := func(id state.ObjID, owner state.PlayerID, name, pt string) view.CardView {
		c := token(id, owner)
		c.Name, c.PrintedPT = name, pt
		return c
	}
	a, b, c := state.ObjID(100), state.ObjID(200), state.ObjID(201)
	deal := func(v view.View) (*shadow.Input, *shadow.Numbering) {
		t.Helper()
		in, n, err := l.Deal(&v, cfg, 1)
		if err != nil {
			t.Fatalf("Deal: %v", err)
		}
		return in, n
	}

	in0, n0 := deal(tokenView(6, []view.CardView{named(a, 0, "Goblin Token", "1/1")}, []view.CardView{named(b, 1, "Knight Token", "2/2")}))
	wantObj(t, n0, a, 14)
	wantObj(t, n0, b, 15)
	if len(in0.Hidden.Ceased) != 0 {
		t.Errorf("view 0 deals ceased tokens %+v, want none", in0.Hidden.Ceased)
	}

	// the goblin leaves: it keeps index 14 as a ceased token, the knight keeps 15
	in1, n1 := deal(tokenView(6, nil, []view.CardView{named(b, 1, "Knight Token", "2/2")}))
	wantObj(t, n1, b, 15)
	wantNoView(t, n1, 14)
	want1 := []shadow.CeasedToken{{Obj: 14, Name: "r_1_1_goblin", Owner: 0}}
	if !reflect.DeepEqual(in1.Hidden.Ceased, want1) {
		t.Errorf("view 1 ceased = %+v, want %+v", in1.Hidden.Ceased, want1)
	}

	// a token seen after it takes the next index and the goblin stays ceased
	in2, n2 := deal(tokenView(6, nil, []view.CardView{named(b, 1, "Knight Token", "2/2"), named(c, 1, "Goblin Token", "1/1")}))
	wantObj(t, n2, b, 15)
	wantObj(t, n2, c, 16)
	if !reflect.DeepEqual(in2.Hidden.Ceased, want1) {
		t.Errorf("view 2 ceased = %+v, want %+v", in2.Hidden.Ceased, want1)
	}

	// the knight leaves too: both are ceased, the goblin's slot first
	in3, _ := deal(tokenView(6, nil, []view.CardView{named(c, 1, "Goblin Token", "1/1")}))
	want3 := []shadow.CeasedToken{want1[0], {Obj: 15, Name: "w_2_2_knight", Owner: 1}}
	if !reflect.DeepEqual(in3.Hidden.Ceased, want3) {
		t.Errorf("view 3 ceased = %+v, want %+v", in3.Hidden.Ceased, want3)
	}
}

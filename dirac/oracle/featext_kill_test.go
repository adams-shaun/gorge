package oracle

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// featRowID is feat's row id of a raw token name ("mz|<key>"): FNV-1a 64
// from the offset basis, masked to the shared table (feat/fnv.go).
func featRowID(s string) uint16 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return uint16(h & (feat.TableRows - 1))
}

func extHasRow(x []feat.Feature, row uint16) bool {
	for _, f := range x {
		if f.Row == row {
			return true
		}
	}
	return false
}

func extStackCard(p *dirac.Projection, id kop.CardID) bool {
	for i := 0; i < int(p.NStack); i++ {
		if p.Stack[i].Kind == 0 && p.Stack[i].Card == id {
			return true
		}
	}
	return false
}

// TestExtCounterPreventsLethal walks counter-prevents-lethal: the burn is on
// the stack and lethal (486), my counter on it sets 490 while the burn is
// still under the counter (486 stays), and after the counter resolves neither
// is set. With the block off Ext is empty.
func TestExtCounterPreventsLethal(t *testing.T) {
	by := stackScenarios(t)
	sc, ok := by["counter-prevents-lethal"]
	if !ok {
		t.Fatal("counter-prevents-lethal scenario missing")
	}
	seat := sc.Seat
	if sc.Dec.Kind != dirac.DkPriority || sc.Dec.Player != seat {
		t.Fatalf("%s: decision kind %d player %d, want seat %d's priority", sc.Name, sc.Dec.Kind, sc.Dec.Player, seat)
	}
	var p dirac.Projection
	sc.State.Project(&p)
	if p.NStack == 0 {
		t.Fatal("empty stack")
	}
	burn := p.Stack[p.NStack-1]
	if burn.Kind != 0 || burn.Ctrl == seat {
		t.Fatalf("top entry kind %d ctrl %d, want the opponent's burn spell", burn.Kind, burn.Ctrl)
	}
	amt := sc.Pool.Effect(burn.Card).Amount
	if int16(p.Life[seat]) > amt {
		t.Fatalf("burn amount %d is not lethal at life %d: the 486 precondition fails", amt, p.Life[seat])
	}
	foundCtr := false
	for i := 0; i < int(p.NObj); i++ {
		o := &p.Objs[i]
		if o.Zone == kop.ZoneHand && o.Ctrl == seat && sc.Pool.Effect(o.Card).Class == feat.EffCounter {
			foundCtr = true
		}
	}
	if !foundCtr {
		t.Fatalf("no counter in seat %d's hand", seat)
	}

	pre := encStack(sc.Pool, feat.ExtAll, sc.State, &sc.Dec, seat)
	if _, ok := extSlot(pre, 486); !ok {
		t.Fatalf("486 not set with the lethal burn on the stack: %v", pre)
	}
	if _, ok := extSlot(pre, 490); ok {
		t.Fatalf("490 set before the counter was cast: %v", pre)
	}
	if off := encStack(sc.Pool, 0, sc.State, &sc.Dec, seat); len(off) != 0 {
		t.Fatalf("off Ext has %d entries", len(off))
	}

	st, d := castCounterOnTop(t, sc)
	var q dirac.Projection
	st.Project(&q)
	if !extStackCard(&q, burn.Card) || q.NStack < 2 {
		t.Fatalf("burn %d is not under the counter (stack %d)", burn.Card, q.NStack)
	}
	after := encStack(sc.Pool, feat.ExtAll, st, d, seat)
	if _, ok := extSlot(after, 486); !ok {
		t.Fatalf("486 cleared while the burn is still under the counter: %v", after)
	}
	if _, ok := extSlot(after, 490); !ok {
		t.Fatalf("490 not set with my counter on the lethal burn: %v", after)
	}

	for n := 0; n < 64; n++ {
		var r dirac.Projection
		st.Project(&r)
		if r.NStack == 0 {
			break
		}
		var a dirac.Action = d.Options[0]
		if d.Kind == dirac.DkPriority {
			for _, o := range d.Options[:d.N] {
				if o.Kind() == dirac.AkPass {
					a = o
					break
				}
			}
		}
		st.Apply(a)
		st.StepInto(d)
		if d.Kind == dirac.DkNone {
			t.Fatal("game ended before the counter resolved")
		}
	}
	st.Project(&q)
	if q.NStack != 0 {
		t.Fatalf("stack not empty after the counter resolved: %d entries", q.NStack)
	}
	inGrave := false
	for i := 0; i < int(q.NObj); i++ {
		if q.Objs[i].Card == burn.Card && q.Objs[i].Zone == kop.ZoneGraveyard {
			inGrave = true
		}
	}
	if !inGrave {
		t.Fatalf("burn %d is not in a graveyard: the counter did not resolve", burn.Card)
	}
	resolved := encStack(sc.Pool, feat.ExtAll, st, d, seat)
	if _, ok := extSlot(resolved, 486); ok {
		t.Fatalf("486 still set after the counter resolved: %v", resolved)
	}
	if _, ok := extSlot(resolved, 490); ok {
		t.Fatalf("490 still set after the counter resolved: %v", resolved)
	}
}

// TestExtKillEqualAcrossPaths: ExtAll on every constructed scenario gives the
// same Ext through the full, incremental and parts paths.
func TestExtKillEqualAcrossPaths(t *testing.T) {
	c, _ := testCorpus(t)
	ss, err := FeatScenarios(rawSet("fdn"), c)
	if err != nil {
		t.Fatal(err)
	}
	nonEmpty := 0
	for i := range ss {
		sc := &ss[i]
		mk := func() *feat.Encoder {
			e := feat.NewEncoder(sc.Pool)
			e.SetExt(feat.ExtAll)
			return e
		}
		full, inc, parts := mk(), mk(), mk()
		x := append([]feat.Feature(nil), full.EncodeStateFor(sc.State, &sc.Dec, sc.Seat).Ext...)
		y := append([]feat.Feature(nil), inc.EncodeStateIncFor(sc.State, &sc.Dec, sc.Seat).Ext...)
		z := append([]feat.Feature(nil), parts.EncodeParts(sc.State, &sc.Dec, sc.Seat).Ext...)
		if extAny(x, 448, 493) {
			nonEmpty++
		}
		if !reflect.DeepEqual(x, y) || !reflect.DeepEqual(x, z) {
			t.Fatalf("%s: Ext differs across paths:\nfull  %v\ninc   %v\nparts %v", sc.Name, x, y, z)
		}
	}
	if nonEmpty != len(ss) {
		t.Fatalf("%d of %d scenarios have no group K slot in Ext", len(ss)-nonEmpty, len(ss))
	}
}

// killPair builds the dirac state and feat pool of one FDN pairing the way
// SetMatchup/SetSide do.
func killPair(t *testing.T, c *Corpus, p Pairing) (*dirac.State, *feat.Pool, bool) {
	t.Helper()
	m, err := c.Matchup(p.A, p.B, 0)
	if err != nil {
		return nil, nil, false
	}
	var es [2][]cardset.Entry
	for s := 0; s < 2; s++ {
		f, err := c.deckFile(m.Names[s])
		if err != nil {
			return nil, nil, false
		}
		for _, e := range f.Cards {
			es[s] = append(es[s], cardset.Entry{Name: e.Name, Count: e.Count})
		}
	}
	tbl, d0, d1, err := rawSet("fdn").Matchup(es[0], es[1])
	if err != nil {
		return nil, nil, false
	}
	fp, err := FeatPool(c.Reg, tbl, nil)
	if err != nil {
		t.Fatal(err)
	}
	return dirac.NewConfig(tbl, d0, d1, p.Seed, dirac.Config{}), fp, true
}

// TestExtLethalCrossingsKeepMzRows: over FDN random-game decisions, group K
// leaves the sparse bag bit-identical, and 492 is set only where the bag holds
// mz|me|lethal-unblocked.
func TestExtLethalCrossingsKeepMzRows(t *testing.T) {
	c, pool := testCorpus(t)
	rowLethal := featRowID("mz|me|lethal-unblocked")
	rowCastI := featRowID("mz|hand|castable|instant-speed")
	seen, withMz, with492, with493 := 0, 0, 0, 0
	for _, p := range Pairings(pool, 40, 11) {
		s, fp, ok := killPair(t, c, p)
		if !ok {
			continue
		}
		on, off := feat.NewEncoder(fp), feat.NewEncoder(fp)
		on.SetExt(feat.ExtAll)
		pol := dirac.NewRandomPolicy(p.Seed)
		var d dirac.Decision
		for n := 0; n < 20000 && seen < 200; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			var proj dirac.Projection
			s.Project(&proj)
			s.ProjectChars(&proj)
			a := on.EncodeState(s, &d)
			b := off.EncodeState(s, &d)
			if !reflect.DeepEqual(a.Sparse, b.Sparse) {
				t.Fatalf("%s vs %s: ExtAll changed the sparse bag", p.A, p.B)
			}
			mz := extHasRow(a.Sparse, rowLethal)
			oppSpell := false
			for i := 0; i < int(proj.NStack); i++ {
				if proj.Stack[i].Kind == 0 && proj.Stack[i].Ctrl != d.Player {
					oppSpell = true
				}
			}
			_, s492 := extSlot(a.Ext, 492)
			// the projection decides the rule; the bag row is the spec's
			// reading (a hashed row can only collide into presence)
			if want := canAttackPower(&proj, d.Player) >= int32(proj.Life[1-d.Player]) && oppSpell; s492 != want {
				t.Fatalf("%s vs %s decision %d: 492 set=%v, want %v (opp-spell=%v)",
					p.A, p.B, seen, s492, want, oppSpell)
			}
			if s492 && !mz {
				t.Fatalf("%s vs %s decision %d: 492 set without mz|me|lethal-unblocked in the bag", p.A, p.B, seen)
			}
			_, s493 := extSlot(a.Ext, 493)
			// one way: the castable-instant row is hashed, so its presence
			// can only over-approximate
			if s493 && !(canAttackPower(&proj, 1-d.Player) >= int32(proj.Life[d.Player]) && extHasRow(a.Sparse, rowCastI)) {
				t.Fatalf("%s vs %s decision %d: 493 set without opponent lethal power and a castable instant", p.A, p.B, seen)
			}
			if mz {
				withMz++
			}
			if s492 {
				with492++
			}
			if s493 {
				with493++
			}
			seen++
			s.Apply(pol.Pick(&d))
		}
		if seen >= 200 {
			break
		}
	}
	if seen != 200 {
		t.Fatalf("visited %d decisions, want 200", seen)
	}
	if withMz == 0 {
		t.Fatalf("no decision held mz|me|lethal-unblocked: the crossing was never exercised")
	}
	t.Logf("200 decisions: %d with mz me-lethal, %d with 492, %d with 493", withMz, with492, with493)
}

// canAttackPower is mzGlobal's canAttackPower from the projection: the
// seat's untapped, non-sick (or hasty) non-defender creatures' power.
func canAttackPower(p *dirac.Projection, seat uint8) int32 {
	var sum int32
	for i := 0; i < int(p.NObj); i++ {
		o := &p.Objs[i]
		if o.Zone != kop.ZoneBattlefield || o.Ctrl != seat || p.Chars[i].Types&kop.TCreature == 0 {
			continue
		}
		kw := p.Chars[i].Kw
		if o.Flags&dirac.ProjTapped != 0 || o.Flags&dirac.ProjSick != 0 && kw&kop.KwHaste == 0 || kw&kop.KwDefender != 0 {
			continue
		}
		sum += int32(o.Power)
	}
	return sum
}

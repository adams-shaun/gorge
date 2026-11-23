package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The tag tallies of a slot a live StatUnspentMana static keeps: gorge's
// foldManaClear `continue`s before clearNonPersistent for a keep-letter slot
// (engine/events/apply_misc.go), so the slot's Snow/SnowTyped/TypedMana/
// ArtifactTyped entries survive the boundary with its pool units
// (engine/events/apply_state_helpers.go drains the tallies only for the slots
// it empties). The four kernel boundary emptiers (endStep, finishCleanup,
// persistEmpty, persistTurn) must keep those tallies for the protected
// colours too, beside the pool units (static_unspentmana.go) and the
// restricted batches (rbKeep); the tag tallies are per seat per colour, which
// is exactly the keep mask's per-slot index. The precondition each case pins
// is the emptier itself (a table without the static empties every tally, as
// snowClearAll/artNClearAll always did).

// provTable builds a scen table carrying the two UnspentMana spellings beside
// a snow reader (a card whose cost asks {S}: genPrep.anySnow, so the snow
// tally exists) and a Treasure (an artifact mana producer: useArtN, so the
// typed-unit tally exists); bearer picks which static is in play (0: neither).
func provTable(t *testing.T, bearer CardID) *scen {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Upwelling", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatUnspentMana, A: int16(kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG | kop.ColC)}}}},
		kop.CardDef{Name: "Test Omnath", Types: kop.TCreature, Power: 0, Toughness: 0,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatUnspentMana, A: int16(kop.ColG), StFlags: kop.StUnspentYou}}}},
		kop.CardDef{Name: "Test Snow Reader", Cost: kop.ManaCost{Snow: 1}},
		kop.CardDef{Name: "Test Gem", Types: kop.TArtifact,
			Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Mana: kop.ManaCost{Generic: 2}}, ManaColors: kop.ColR, ManaN: 1}}},
	)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if bearer != 0 {
		sc.perm(bearer, 0)
	}
	return sc
}

// provTag writes seat p's tag tallies of colour c: snow, typed (the cold
// tagged-unit tally and the Treasure tally) and the pool's colour bits
// (poolArtSnow rides any snow).
func provTag(s *State, p uint8, c int, snow, artn, tre uint8) {
	s.snowSetAt(p, c, snow)
	if ax := coldArtN.of(s); ax != nil {
		ax[int(SeatIx(p))*kop.NumManaKinds+c] = artn
	}
	s.g.poolTre[SeatIx(p)][c] = tre
	s.g.poolArt[SeatIx(p)] |= 1 << uint(c)
	if snow != 0 {
		s.g.poolArt[SeatIx(p)] |= poolArtSnow
	}
}

// provWant asserts seat p's whole tag state of colour c: the pool unit, the
// snow tally, the typed tallies and the pool's bits.
func provWant(t *testing.T, s *State, p uint8, c int, pool, snow, artn, tre uint8, bit bool) {
	t.Helper()
	i := SeatIx(p)
	if got := s.pl[i].pool[c]; got != pool {
		t.Errorf("pool %d, want %d", got, pool)
	}
	if got := s.snowAt(p, c); got != snow {
		t.Errorf("snow %d, want %d", got, snow)
	}
	artnGot := uint8(0)
	if ax := coldArtN.of(s); ax != nil {
		artnGot = ax[int(i)*kop.NumManaKinds+c]
	}
	if artnGot != artn {
		t.Errorf("typed tally %d, want %d", artnGot, artn)
	}
	if got := s.g.poolTre[i][c]; got != tre {
		t.Errorf("treasure tally %d, want %d", got, tre)
	}
	if got := s.g.poolArt[i]&(1<<uint(c)) != 0; got != bit {
		t.Errorf("poolArt colour bit %v, want %v", got, bit)
	}
}

// provWantSnowBit asserts the seat-wide poolArtSnow bit (it is per seat, not
// per colour): set exactly when a snow tally survived the boundary.
func provWantSnowBit(t *testing.T, s *State, p uint8, want bool) {
	t.Helper()
	if got := s.g.poolArt[SeatIx(p)]&poolArtSnow != 0; got != want {
		t.Errorf("poolArt snow bit %v, want %v", got, want)
	}
}

// TestUnspentManaKeepsProvenance: a boundary emptier keeps a protected slot's
// snow and typed provenance with its units (endStep, finishCleanup,
// persistEmpty, the turn change), keeping only the keep-letter colours: the
// bare spelling keeps both provenances of every colour, the You-scoped green
// one keeps green's alone (red's tag tallies empty with the slot), and a
// table without the static empties every tally.
func TestUnspentManaKeepsProvenance(t *testing.T) {
	upwelling, omnath := CardID(len(v.Table.Cards)), CardID(len(v.Table.Cards)+1)
	for _, c := range []struct {
		name                 string
		bearer               CardID
		wantRPool, wantRSnow uint8
		wantRArtn, wantRTre  uint8
		wantGPool, wantGSnow uint8
		wantGArtn, wantGTre  uint8
	}{
		{"Upwelling: every seat's provenance of every colour", upwelling, 2, 1, 1, 1, 1, 1, 1, 1},
		{"You scope: the controller's green provenance only", omnath, 0, 0, 0, 0, 1, 1, 1, 1},
		{"no static: every tally empties", 0, 0, 0, 0, 0, 0, 0, 0, 0},
	} {
		for _, b := range []struct {
			name string
			run  func(*State)
		}{
			{"endStep", func(s *State) { s.endStep(kop.StepBeginCombat) }},
			{"finishCleanup", func(s *State) { s.finishCleanup(s.k()) }},
			{"persistEmpty", func(s *State) { s.persistEmpty(false) }},
			{"turn change", func(s *State) { s.persistTurn() }},
		} {
			t.Run(c.name+"/"+b.name, func(t *testing.T) {
				sc := provTable(t, c.bearer)
				// the precondition: a snow unit, a typed (Treasure) unit and
				// the tags beside them, in a red slot and a green one. The
				// tallies must not exceed their pools (snowSync/artSync clamp
				// them), which they do here.
				provTag(sc.s, 0, kop.R, 1, 1, 1)
				provTag(sc.s, 0, kop.G, 1, 1, 1)
				sc.s.pl[0].pool[kop.R] = 2
				sc.s.pl[0].pool[kop.G] = 1
				b.run(sc.s)
				provWant(t, sc.s, 0, kop.R, c.wantRPool, c.wantRSnow, c.wantRArtn, c.wantRTre,
					c.wantRPool != 0)
				provWant(t, sc.s, 0, kop.G, c.wantGPool, c.wantGSnow, c.wantGArtn, c.wantGTre,
					c.wantGPool != 0)
				provWantSnowBit(t, sc.s, 0, c.wantRSnow != 0 || c.wantGSnow != 0)
			})
		}
	}
}

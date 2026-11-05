package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Vannifar, Evolved Enigma's combat trigger (dirac lowers its DB$ Cloak with
// the Choices$ hand pick). The cloak picks are counted
// from the canonical actions; the empty-hand positions from the kernel's
// pending-trigger view at each agreed decision.
type kcloakCounts struct {
	games, picks, modeAsks, triggers, cloakPicks, emptyRes int
	// expectPick: a cloak-mode pick is awaiting the face_down_choice ask;
	// still set at the NEXT cloak-mode ask means that resolution asked
	// nothing -- an empty hand at resolution time (gorge asks for any
	// nonempty hand, so no ask is the empty-hand path).
	expectPick bool
}

// kcloakDeck: both seats run Vannifar beside free creatures (the hand the
// cloak pick drains) and lands that can pay {2}{G}{U}.
var kcloakDeck = []cardset.Entry{
	{Name: "Vannifar, Evolved Enigma", Count: 4},
	{Name: "Ornithopter", Count: 12},
	{Name: "Forest", Count: 12},
	{Name: "Island", Count: 12},
}

// kcloakIsVannifarCharm reports whether the table ability is the trigger
// whose Charm body holds the cloak (its program carries the OpCloak).
func kcloakIsVannifarCharm(tbl *kop.Table, card dirac.CardID, ab uint8) bool {
	if int(card) >= len(tbl.Cards) {
		return false
	}
	abs := tbl.Cards[card].Abilities
	if int(ab) >= len(abs) {
		return false
	}
	for _, in := range abs[ab].Prog {
		if in.Op == kop.OpCloak {
			return true
		}
	}
	return false
}

// kcloakPolicy casts Vannifar and the free creatures, takes the cloak mode
// while its controller holds a card to cloak and the counter mode otherwise,
// and counts the cloak picks and mode asks.
func kcloakPolicy(seed uint64, counts *kcloakCounts, pickCode canon.CanonKind) Policy {
	heu := HeuristicPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		var pick, counter, cloak *Canon
		for i := range c.D.Options {
			a := c.D.Options[i]
			switch a.Kind() {
			case pickCode:
				pick = &c.D.Options[i]
			case canon.ActMode:
				if a.Mode() == 0 {
					cloak = &c.D.Options[i]
				} else {
					counter = &c.D.Options[i]
				}
			}
		}
		if pick != nil {
			counts.picks++
			counts.expectPick = false
			return *pick
		}
		if cloak != nil {
			counts.modeAsks++
			if counts.expectPick {
				// the previous cloak resolution asked nothing: an empty hand
				counts.emptyRes++
			}
			counts.cloakPicks++
			counts.expectPick = true
			if c.G != nil {
				p := c.D.Player
				if p < 2 && len(c.G.E.G.Zone(state.ZHand, state.PlayerID(p))) == 0 {
					counts.triggers++ // the empty-hand position takes the counter mode
					return *counter
				}
			}
			return *cloak
		}
		return heu.Pick(c)
	})
}

// TestCloakSourcesLockstep: gorge and the kernel play Vannifar, Evolved
// Enigma decision by decision -- the combat trigger takes the cloak mode and
// cloaks a chosen hand card (one face_down_choice pick), and resolves without
// an ask when the hand emptied between the mode ask and the resolution (the
// empty-hand position).
func TestCloakSourcesLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cloaksources")
	if err != nil {
		t.Fatal(err)
	}
	pickCode, ok := canon.ChooseCode("face_down_choice")
	if !ok {
		t.Fatal("face_down_choice is not a canonical choose kind")
	}
	var counts kcloakCounts
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kcloakDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kcloakDeck, kcloakDeck)
	if err != nil {
		t.Fatal(err)
	}
	{
		for seed := uint64(1); seed <= 96; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcloak-a", "kcloak-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			counts.expectPick = false
			check := func(_ *GorgeSide, side Side) *Divergence {
				ks, ok := side.(*realSide)
				if !ok {
					return nil
				}
				s := &ks.s
				for i := 0; i < s.NumPending(); i++ {
					_, card, ab, _, _ := s.PendingTrigger(i)
					if !kcloakIsVannifarCharm(tbl, card, ab) {
						continue
					}
					counts.triggers++
				}
				return nil
			}
			res := Lockstep(m, seed, k, kcloakPolicy(seed, &counts, canon.ActChoose+canon.CanonKind(pickCode)),
				RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30, Check: check})
			CloseSide(k)
			counts.games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, mode asks %d, cloak-mode picks %d, face_down_choice asks %d, Vannifar triggers pending %d, empty-hand resolutions %d",
		counts.games, counts.modeAsks, counts.cloakPicks, counts.picks, counts.triggers, counts.emptyRes)
	if counts.picks < 8 {
		t.Errorf("only %d face_down_choice picks", counts.picks)
	}
	if counts.emptyRes < 1 {
		t.Errorf("no empty-hand cloak resolution in %d games: %+v", counts.games, counts)
	}
}

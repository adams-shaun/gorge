package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The opponent-caused discard-to-battlefield lockstep (Wilt-Leaf Liege, CR
// 614.1): a Liege discarded by a spell its owner's opponent controls enters
// the battlefield; the owner's own looting and the cleanup discard keep the
// graveyard. Each deck isolates one case; gorge is the reference.
func TestDiscardToBattlefieldLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("discardetb")
	if err != nil {
		t.Fatal(err)
	}
	liege := cardset.Entry{Name: "Wilt-Leaf Liege", Count: 8}
	cases := []struct {
		name         string
		deck         []cardset.Entry
		wantBF, want bool // a Liege enters from hand / goes to the graveyard from hand
		noBF         bool
	}{
		{"opponent-discard", []cardset.Entry{liege, {Name: "Mind Rot", Count: 8}, {Name: "Swamp", Count: 24}}, true, false, false},
		{"own-looting", []cardset.Entry{liege, {Name: "Faithless Looting", Count: 8}, {Name: "Swamp", Count: 24}}, false, true, true},
		{"cleanup", []cardset.Entry{{Name: "Wilt-Leaf Liege", Count: 20}, {Name: "Swamp", Count: 20}}, false, true, true},
	}
	for _, tc := range cases {
		md := [2][]cardset.Entry{tc.deck, tc.deck}
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("%s deck %d: %d cards", tc.name, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		toBF, toGrave, seen := 0, 0, 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				e := &evs[seen]
				if e.Kind != events.MoveZone || e.From != state.ZHand {
					continue
				}
				o := ref.E.G.Obj(e.Obj)
				if o == nil || o.Card == nil || len(o.Card.Faces) == 0 || o.Card.Faces[0].Name != "Wilt-Leaf Liege" {
					continue
				}
				switch e.To {
				case state.ZBattlefield:
					toBF++
				case state.ZGraveyard:
					toGrave++
				}
			}
			return nil
		}
		for seed := uint64(1); seed <= 48; seed++ {
			seen = 0
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"discetb-a", "discetb-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, ranklePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 12000, MaxTurns: 200, Check: check})
			CloseSide(k)
			if res.Stall != "" {
				t.Errorf("%s seed %d: stalled: %s", tc.name, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("%s seed %d last actions %v", tc.name, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", tc.name, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		t.Logf("%s: %d hand->battlefield, %d hand->graveyard", tc.name, toBF, toGrave)
		if tc.wantBF && toBF == 0 {
			t.Errorf("%s: no Liege entered from hand: the replacement was not exercised", tc.name)
		}
		if tc.want && toGrave == 0 {
			t.Errorf("%s: no Liege reached the graveyard from hand", tc.name)
		}
		if tc.noBF && toBF != 0 {
			t.Errorf("%s: a Liege entered the battlefield from hand %d times", tc.name, toBF)
		}
	}
}

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The lockstep: constructed matchups over the generated grantsac pool, whose
// only coloured mana is a Forest and whose only black is the any-colour ability
// Great Divide Guide grants every land and Ally. Deadly Precision ({B}, plus {4}
// or a sacrifice) is therefore payable only through that grant, the guide being
// the creature it sacrifices after it has been tapped for the {B} (CR 601.2g
// pays mana before 601.2h pays costs). gorge's planner has no granted source,
// so its prefix search taps the guide first and the cast is offered; the kernel
// must offer it in the same windows.

var kgrantsacDeck = []cardset.Entry{
	{Name: "Deadly Precision", Count: 12}, {Name: "Great Divide Guide", Count: 12},
	{Name: "Forest", Count: 16},
}

type kgrantsacTally struct{ offers int }

func (y *kgrantsacTally) check(ref *GorgeSide, _ Side) *Divergence {
	d := ref.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return nil
	}
	for _, o := range d.Options {
		if o.Kind != "cast" {
			continue
		}
		if ob := ref.E.G.Obj(o.Obj); ob != nil && ob.Card != nil && len(ob.Card.Faces) != 0 && ob.Card.Faces[0].Name == "Deadly Precision" {
			y.offers++
			break
		}
	}
	return nil
}

func TestGrantSacLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("grantsac")
	if err != nil {
		t.Fatal(err)
	}
	md := [2][]cardset.Entry{kgrantsacDeck, kgrantsacDeck}
	decks := kcastDeck(t, reg, md)
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	var y kgrantsacTally
	games := 0
	for seed := uint64(1); seed <= 100; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kgrantsac-a", "kgrantsac-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: y.check})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			return
		}
	}
	t.Logf("%d games; windows where gorge offers Deadly Precision: %d", games, y.offers)
	if y.offers < 10 {
		t.Errorf("only %d windows offer Deadly Precision, want at least 10", y.offers)
	}
}

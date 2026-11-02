package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEOTEmblemObjectTrigger: an until-end-of-turn Effect's trigger (an
// UntilEOT emblem) fires for an object event (a creature entering) as the
// creating card's latent copy, and the cleanup drops it.
func TestEOTEmblemObjectTrigger(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	trig := kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigETB, Zone: kop.ZoneCommand}, Prog: gain}
	tbl.Emblems = append(tbl.Emblems, kop.Emblem{Abilities: []kop.Ability{trig}, UntilEOT: true})
	emb := len(tbl.Emblems)
	latent := trig
	latent.AFlags |= kop.AfLatent
	spell, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Sorcery", Types: kop.TSorcery, Abilities: []kop.Ability{latent}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(bear, 1) // any object stands in for the spent spell
	sc.s.g.emb[0][0], sc.s.g.embC[0][0], sc.s.g.embO[0][0] = uint8(emb), spell, src
	sc.s.g.nemb[0] = 1
	b := sc.perm(bear, 1)
	n0 := sc.s.npend
	sc.s.fireObj(sc.k, kop.TrigETB, sc.slot(b), b, 1, 0)
	if sc.s.npend != n0+1 || sc.s.pend[n0].trigObj != b || sc.s.pend[n0].ctrl != 0 {
		t.Fatalf("pending %d (want %d), want the emblem owner's trigger on the entering creature", sc.s.npend, n0+1)
	}
	sc.s.cleanupGen(sc.k)
	if sc.s.g.nemb[0] != 0 {
		t.Fatalf("%d emblems after the cleanup, want 0", sc.s.g.nemb[0])
	}
}

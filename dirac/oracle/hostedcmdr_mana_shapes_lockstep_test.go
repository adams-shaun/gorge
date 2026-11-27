package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestHostedCmdrManaShapesLockstep lowers the hosted-Commander mana shapes
// whose Forge lines kgen used to refuse. This first case is Virtue of
// Strength's ProduceMana replacement:
//
//	R:Event$ ProduceMana | ActiveZones$ Battlefield | ValidActivator$ You |
//	  ValidCard$ Land.Basic | ReplaceWith$ ProduceThrice
//	SVar:ProduceThrice:DB$ ReplaceMana | ReplaceAmount$ 3
//
// kgen lowers the exact shape to the kernel's tap-extra static in reflected
// mode (kop.StTapReflectDefer, TapAmt 2): the tapped basic land makes its own
// mana plus two more of the same type. The plan prices the source at its own
// production only (gorge does not fold the replacement) and the extra floats
// at the tap (Virtue Probe, the inline copy of the line, is cast first).
//
// The probe is {5}{G}{G}. Seven nonbasic Grove Probes (not Land.Basic, so the
// replacement does not reach them) pay for it; the deck's only Forest is kept
// in hand until the probe is on the battlefield, then played and tapped for a
// Grizzly Bears. That leaves exactly one green source untapped, so the payment
// plan has no source-choice tie-break to differ on: the lockstep compares the
// replacement's three mana itself. The case FAILS while kgen refuses the shape
// (printedDelveSetIn prints the table) and FAILS unless some seed casts the
// probe and then gets the Forest to produce three mana, so a passing lockstep
// is not vacuous. The real Virtue of Strength is checked by the lowering test
// in dirac/kgen (its split-card file name keeps it out of this corpus).
const virtueProbeScript = "Name:Virtue Probe\nManaCost:5 G G\nTypes:Enchantment\n" +
	"R:Event$ ProduceMana | ActiveZones$ Battlefield | ValidActivator$ You | ValidCard$ Land.Basic | ReplaceWith$ ProduceThrice | Description$ If you tap a basic land for mana, it produces three times as much of that mana instead.\n" +
	"SVar:ProduceThrice:DB$ ReplaceMana | ReplaceAmount$ 3\n" +
	"Oracle:If you tap a basic land for mana, it produces three times as much of that mana instead.\n"

const cubProbeScript = "Name:Cub Probe\nManaCost:G\nTypes:Creature Bear\nPT:2/2\n" +
	"Oracle:2/2\n"

const groveProbeScript = "Name:Grove Probe\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T Sac<1/CARDNAME/this land> | Produced$ G | SpellDescription$ Add {G}.\n" +
	"Oracle:{T}, Sacrifice this land: Add {G}.\n"

func TestHostedCmdrManaShapesLockstep(t *testing.T) {
	dir := filters11Corpus(t, map[string]string{"Virtue Probe": virtueProbeScript, "Grove Probe": groveProbeScript, "Cub Probe": cubProbeScript},
		"Forest")
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	decks := [2][]string{
		append(deckOf(4, "Virtue Probe"), append(deckOf(28, "Cub Probe"),
			append(deckOf(7, "Grove Probe"), "Forest")...)...),
		deckOf(40, "Forest"),
	}
	if len(decks[0]) != 40 || len(decks[1]) != 40 {
		t.Fatalf("deck sizes = %d/%d, want 40/40", len(decks[0]), len(decks[1]))
	}
	var cardDecks [2][]*cards.Card
	for s, names := range decks {
		for _, n := range names {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			cardDecks[s] = append(cardDecks[s], card)
		}
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	set := printedDelveSetIn(t, dir, decks)
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	seen := false
	played := 0
	for seed := uint64(1); seed <= 80 && !(seen && played >= 3); seed++ {
		m, err := NewMatchup(reg, [2]string{"hms-probe-a", "hms-probe-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			if !seen && virtueProbeTripled(g) {
				seen = true
			}
		}
		res := Lockstep(m, seed, k, hostedCmdrManaShapesPolicy(m, seed), rc)
		if res.Div != nil {
			d := res.Div
			n := len(res.Actions) - 8
			if n < 0 {
				n = 0
			}
			var names []string
			for _, a := range res.Actions[n:] {
				names = append(names, fmt.Sprintf("%v:%q", a, m.InstanceCard(int(a.Src()))))
			}
			CloseSide(k)
			t.Fatalf("seed %d: divergence at %d: %s %s %v detail=%q last=%v\ngorge %v\ndirac %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, names, d.RefOptions, d.KerOptions)
		}
		if res.Decisions > 100 {
			played++
		}
		CloseSide(k)
	}
	if played < 3 {
		t.Fatalf("only %d seeds played a real game", played)
	}
	if !seen {
		t.Fatalf("the ProduceMana replacement was not reached on any seed played, so the lockstep is vacuous")
	}
}

// virtueProbeTripled: the probe entered the battlefield under seat 0 and a
// later ManaAdd event carried three units (the replacement's rewritten
// production).
func virtueProbeTripled(g *GorgeSide) bool {
	probe := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Virtue Probe" && o.Controller == 0 {
				probe = true
			}
		}
		if probe && e.Kind == events.ManaAdd && e.Amount >= 3 {
			return true
		}
	}
	return false
}

// virtueOnBattlefield reports whether seat 0 controls a Virtue Probe.
func virtueOnBattlefield(g *GorgeSide) bool {
	if g == nil || g.E == nil {
		return false
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Virtue Probe" {
			return true
		}
	}
	return false
}

// hostedCmdrManaShapesPolicy plays the scripted line: seat 0 casts the probe
// as soon as its seven Grove Probes can pay, then plays its one Forest and
// casts a single Cub Probe with the Forest's (tripled) mana. It never
// casts a second spell, so after the one bear the game only plays lands and
// passes; seat 1's all-Forest deck has nothing to cast. Keeping the Forest in
// hand until the probe is out makes it the only untapped green source for the
// bear, which keeps the payment out of the planners' source-choice tie-break.
func hostedCmdrManaShapesPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	cubs := 0
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || p.G == nil || p.D.Player != 0 {
			return rnd.Pick(p)
		}
		if !virtueOnBattlefield(p.G) {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Virtue Probe" {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand && m.InstanceCard(int(a.Src())) == "Grove Probe" {
					return a
				}
			}
		} else {
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand && m.InstanceCard(int(a.Src())) == "Forest" {
					return a
				}
			}
			if cubs < 1 {
				for _, a := range p.D.Options {
					if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Cub Probe" {
						cubs++
						return a
					}
				}
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		return rnd.Pick(p)
	})
}

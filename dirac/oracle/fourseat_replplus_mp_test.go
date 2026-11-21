package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
)

// A static damage prevention beside an Ext PreventDamageScope record: CR 616.1
// (the affected permanent's controller orders the replacements) and 615.7
// (Temple Altisaur prevents all but 1 of the damage; Arachnogenesis's Effect
// prevents all combat damage by non-Spiders). gorge collects both as
// candidates of one combat event and asks their order (engine/rules
// replacement_damage.go; the Effect's first, then the card's own line); the
// kernel's combat walk poses the same ask (dirac/repl_staticprevent.go
// spCombatCands, spBesidePrevAll) and applies each pick as made.

// spsrMix is a deck list from (count, name) pairs.
func spsrMix(parts ...any) []string {
	var d []string
	for i := 0; i < len(parts); i += 2 {
		d = append(d, deckOf(parts[i].(int), parts[i+1].(string))...)
	}
	return d
}

func spsrArt() []any {
	return []any{4, "Silver Myr", 4, "Containment Construct", 3, "Steel Overseer", 3, "Millikin"}
}

// spsrDecks are the pod's four decks: Temple Altisaur's seat, Arachnogenesis's
// seat and two seats of plain attackers.
func spsrDecks() [][]string {
	return [][]string{
		spsrMix(append(spsrArt(), 4, "Losheel, Clockwork Scholar", 4, "Temple Altisaur", 4, "Ranging Raptors", 8, "Plains", 6, "Forest")...),
		spsrMix(append(spsrArt(), 5, "Arachnogenesis", 6, "Pugnacious Hammerskull", 14, "Forest")...),
		spsrMix(append(spsrArt(), 4, "Humble Defector", 4, "Ranging Raptors", 4, "Elvish Mystic", 14, "Mountain")...),
		spsrMix(append(spsrArt(), 4, "Humble Defector", 4, "Ranging Raptors", 4, "Elvish Mystic", 14, "Mountain")...),
	}
}

// spsrSeeds reach Arachnogenesis and Temple Altisaur on one combat event
// (seeds 8, 13: the pair alone; 7, 39: beside a second Altisaur
// or Arachnogenesis).
var spsrSeeds = []uint64{8, 13, 7, 39}

// spsrRun plays spsrSeeds in lockstep and returns the first divergence, any
// kernel stop, and how many order asks named Arachnogenesis beside a Temple
// Altisaur.
func spsrRun(t *testing.T) (div, stop string, asks int) {
	t.Helper()
	return spsrPlay(t, spsrDecks(), spsrSeeds, "Arachnogenesis", "Temple Altisaur")
}

// spsrPlay plays seeds over decks in lockstep and counts the order asks that
// name both a and b.
func spsrPlay(t *testing.T, decks [][]string, seeds []uint64, a, b string) (div, stop string, asks int) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("edh")
	if err != nil {
		t.Fatal(err)
	}
	var lists [][]cardset.Entry
	for _, d := range decks {
		cnt := map[string]int{}
		var es []cardset.Entry
		for _, n := range d {
			if cnt[n] == 0 {
				es = append(es, cardset.Entry{Name: n})
			}
			cnt[n]++
		}
		for i := range es {
			es[i].Count = cnt[es[i].Name]
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range seeds {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]}, Seed: seed, PassElision: true, Surface: "full"}
		for i := 2; i < len(decks); i++ {
			r.MoreDecks = append(r.MoreDecks, "x"+string(rune('0'+i)))
			r.MoreCards = append(r.MoreCards, decks[i])
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 6000, 40
		rc.Check = func(g *GorgeSide, _ Side) *Divergence {
			d := g.E.Pending()
			if d == nil || d.Kind != decision.KReplacement {
				return nil
			}
			var names []string
			for i := range d.Options {
				if o := &d.Options[i]; o.Kind == "replacement" {
					names = append(names, chName(g.E.G, o.Obj))
				}
			}
			if all := strings.Join(names, "+"); strings.Contains(all, a) && strings.Contains(all, b) {
				asks++
			}
			return nil
		}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, combatFirstPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil && div == "" {
			div = fmt.Sprintf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if (res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix)) && stop == "" {
			stop = fmt.Sprintf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	return div, stop, asks
}

// TestFourSeatStaticPrevBesideScopeRecordLockstep plays four-seat pods holding
// Temple Altisaur and Arachnogenesis (the pair the refusal named): the order
// ask is posed and answered in lockstep with gorge, and some game reached it.
func TestFourSeatStaticPrevBesideScopeRecordLockstep(t *testing.T) {
	div, stop, asks := spsrRun(t)
	if div != "" || stop != "" {
		t.Fatalf("%s %s", div, stop)
	}
	if asks == 0 {
		t.Fatal("no game reached the Arachnogenesis beside Temple Altisaur order ask")
	}
}

// TestFourSeatStaticPrevBesideScopeRecordMutation: with the kernel leaving the
// static prevention out of the ask beside the record, a game diverges.
func TestFourSeatStaticPrevBesideScopeRecordMutation(t *testing.T) {
	dirac.DebugSpBesidePcolOff = true
	defer func() { dirac.DebugSpBesidePcolOff = false }()
	if div, _, _ := spsrRun(t); div == "" {
		t.Fatal("the mutation did not diverge")
	}
}

// combatFirstPolicy casts, activates, plays lands, attacks and blocks whenever offered
// (the carriers are expensive and combat is where the order ask arises), else
// random.
func combatFirstPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate || k == ActPlayLand || k == ActAttack || k == ActBlock {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// A bare combat prevention beside an amount changer: CR 616.1. Losheel prevents all
// combat damage dealt to attacking artifact creatures its controller has; Thor,
// Asgard's Avenger adds one to another source's damage and Angrath's Marauders
// doubles it. Whichever is picked first, nothing is dealt, but gorge asks the order
// (the candidates in object scan order) and the kernel's combat walk poses the
// same ask (dirac/repl_staticprevent.go spBesideAmounts, spMergeAmounts).

// spamDecks: Losheel's seat (attacking artifact creatures), a Thor seat, a
// Marauders seat and a plain attacker seat.
func spamDecks() [][]string {
	art := []any{4, "Silver Myr", 4, "Containment Construct", 3, "Steel Overseer", 3, "Millikin"}
	return [][]string{
		spsrMix(append(art, 4, "Losheel, Clockwork Scholar", 4, "Ranging Raptors", 8, "Plains", 6, "Forest")...),
		spsrMix(append(art, 4, "Thor, Asgard's Avenger", 4, "Humble Defector", 4, "Elvish Mystic", 14, "Mountain")...),
		spsrMix(append(art, 4, "Angrath's Marauders", 4, "Humble Defector", 4, "Elvish Mystic", 14, "Mountain")...),
		spsrMix(append(art, 4, "Humble Defector", 4, "Ranging Raptors", 4, "Elvish Mystic", 14, "Mountain")...),
	}
}

var spamSeeds = []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

// TestFourSeatStaticPrevBesideAmountsLockstep: pods holding Losheel beside Thor and
// Angrath's Marauders play in lockstep, and some game reached the order ask.
func TestFourSeatStaticPrevBesideAmountsLockstep(t *testing.T) {
	var asks int
	for _, n := range []string{"Thor, Asgard's Avenger", "Angrath's Marauders"} {
		div, stop, a := spsrPlay(t, spamDecks(), spamSeeds, "Losheel", n)
		if div != "" || stop != "" {
			t.Fatalf("%s %s", div, stop)
		}
		asks += a
	}
	if asks == 0 {
		t.Fatal("no game reached a Losheel beside Thor or Marauders order ask")
	}
}

// TestFourSeatStaticPrevBesideAmountsMutation: with the prevention left out of the
// ask beside the amount changers, a game diverges.
func TestFourSeatStaticPrevBesideAmountsMutation(t *testing.T) {
	dirac.DebugSpBesideAmountsOff = true
	defer func() { dirac.DebugSpBesideAmountsOff = false }()
	if div, _, _ := spsrPlay(t, spamDecks(), spamSeeds, "Losheel", "Angrath's Marauders"); div == "" {
		t.Fatal("the mutation did not diverge")
	}
}

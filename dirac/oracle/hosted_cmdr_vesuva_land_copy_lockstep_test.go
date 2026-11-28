package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Vesuva (pro-shaper's hosted deck): an optional Copy enters replacement whose
// Clone body picks from Land.Other with IntoPlayTapped$ True. Played as a
// land it asks the as-enters copy election (CR 614.12, 707.2) over every other
// land on the battlefield, plus "enter as itself"; an elected copy enters
// tapped (IntoPlayTapped$ True) and a decline untapped, and a copied land
// taps for the template's mana. Each variant puts a land in the pool whose
// own as-enters choice must be asked with Vesuva still in hand once Vesuva
// copies it: Thriving Grove's colour choice, Overgrown Tomb's pay-life
// election (the shock lands of the hosted-cmdr pool). The kernel must pose
// the same asks at the same decisions and expose the copied mana ability to
// its mana walk. printedDelveSetIn runs kgen over the decks, so the case is
// RED while kgen refuses the land template.
var vesuvaLandCopyTemplates = []string{"Thriving Grove", "Overgrown Tomb", "Game Trail"}

// vesuvaLandCopyDecks is the mirror whose Vesuva seat also holds tmpl.
func vesuvaLandCopyDecks(tmpl string) [2][]string {
	return [2][]string{
		append(append(append(deckOf(10, "Vesuva"), deckOf(7, "Forest")...), deckOf(7, tmpl)...), deckOf(16, "Grizzly Bears")...),
		append(deckOf(24, "Mountain"), deckOf(16, "Grizzly Bears")...),
	}
}

// vesuvaCopiedLandTapped: a Vesuva entered as a copy of another land (its
// face is no longer Vesuva's) and that same permanent later tapped while on
// the battlefield (it entered tapped, so a later tap is its copied mana
// ability paying for a spell).
func vesuvaCopiedLandTapped(g *GorgeSide) bool {
	copied := map[state.ObjID]bool{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch e.Kind {
		case events.MoveZone:
			if e.To != state.ZBattlefield {
				delete(copied, e.Obj)
				continue
			}
			o := g.E.G.Obj(e.Obj)
			if o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Vesuva" && o.Face() != nil && o.Face().Name != "Vesuva" {
				copied[e.Obj] = false
			}
		case events.Untap:
			if _, ok := copied[e.Obj]; ok {
				copied[e.Obj] = true
			}
		case events.Tap:
			if untapped, ok := copied[e.Obj]; ok && untapped {
				return true
			}
		}
	}
	return false
}

// vesuvaCopied: some seed must take the land whose template carries its own
// as-enters choice, so the case does not pass by copying only the
// choice-free Forest.
func vesuvaCopied(g *GorgeSide, tmpl string) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.MoveZone || e.To != state.ZBattlefield {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Vesuva" && o.Face() != nil && o.Face().Name == tmpl {
			return true
		}
	}
	return false
}

// vesuvaLandCopyPolicy plays Vesuva ahead of any other land, casts whatever
// it can and never attacks; at a copy election it takes tmpl so the copied
// land's own as-enters choice is asked, and the tail is the seed's own random
// stream.
func vesuvaLandCopyPolicy(m *Matchup, seed uint64, tmpl string) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		for _, a := range p.D.Options {
			if a.Kind() == ActChoose && a.Mode() == 0 && m.InstanceCard(int(a.Src())) == tmpl {
				return a
			}
		}
		switch p.D.Kind {
		case DPriority:
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand && m.InstanceCard(int(a.Src())) == "Vesuva" {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand || a.Kind() == ActCast {
					return a
				}
			}
		case DAttackers:
			for _, a := range p.D.Options {
				if a.Kind() != ActAttack {
					return a
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestHostedCmdrVesuvaLandCopyLockstep plays the Vesuva mirror in both
// engines, gorge leading, once per template variant, and fails on a refused
// card, on any divergence, unless each variant copies its template with
// Vesuva, and unless some seed taps a copied land for mana.
func TestHostedCmdrVesuvaLandCopyLockstep(t *testing.T) {
	dir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, tmpl := range vesuvaLandCopyTemplates {
		if vesuvaLandCopyVariant(t, dir, reg, tmpl) {
			seen = true
		}
	}
	if !seen {
		t.Fatal("no seed tapped a Vesuva that entered as a copy of a land: the lockstep is vacuous")
	}
}

// vesuvaLandCopyVariant runs the mirror holding tmpl and reports whether some
// seed tapped a copied land for mana.
func vesuvaLandCopyVariant(t *testing.T, dir string, reg *cards.Registry, tmpl string) bool {
	t.Helper()
	names := vesuvaLandCopyDecks(tmpl)
	var decks [2][]*cards.Card
	var entries [2][]cardset.Entry
	for s, ns := range names {
		for _, n := range ns {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], card)
			entries[s] = append(entries[s], cardset.Entry{Name: n, Count: 1})
		}
		if len(decks[s]) != 40 {
			t.Fatalf("%s: seat %d deck has %d cards, want 40", tmpl, s, len(decks[s]))
		}
	}
	set := printedDelveSetIn(t, dir, names) // a refused card fails here
	tbl, _, _, err := set.Matchup(entries[0], entries[1])
	if err != nil {
		t.Fatal(err)
	}
	seen, chosen, played := false, false, 0
	for seed := uint64(1); seed <= 40 && !(seen && chosen && played >= 3); seed++ {
		m, err := NewMatchup(reg, [2]string{"vesuva-a", "vesuva-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 4000, MaxTurns: 30}
		rc.Finish = func(g *GorgeSide) {
			if !seen && vesuvaCopiedLandTapped(g) {
				seen = true
			}
			if !chosen && vesuvaCopied(g, tmpl) {
				chosen = true
			}
		}
		res := Lockstep(m, seed, k, vesuvaLandCopyPolicy(m, seed, tmpl), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("%s seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", tmpl, seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
		}
		if res.Decisions > 100 {
			played++
		}
	}
	if played < 3 {
		t.Fatalf("%s: only %d seeds played a real game", tmpl, played)
	}
	if !chosen {
		t.Fatalf("no seed copied %s: the template's own as-enters choice was never asked", tmpl)
	}
	return seen
}

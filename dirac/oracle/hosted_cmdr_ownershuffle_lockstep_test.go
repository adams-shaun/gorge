package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander census blocker this shape lowers, played decision by
// decision against gorge (gorge leads) so the kernel takes the shape the
// same:
//
//   - Struggle // Survive: Survive's SP$ ChangeZoneAll | Defined$ Player |
//     ChangeType$ Card | Origin$ Graveyard | Destination$ Library |
//     Shuffle$ True ("Each player shuffles their graveyard into their
//     library"). gorge's changeZoneAllPlayers sweeps every player,
//     AliveFrom(controller) (Defined$ Player; engine/effects/zone_changeall.go);
//     the kernel's sweep walks every seat when the op names no player, and
//     the kop.CzShuffleOwners|CzShuffleScope arm of the per-player zone sweep
//     starts at the controller and shuffles every library that received a
//     card (first-placement order), then every in-scope library that
//     received nothing -- gorge's changeZoneAllShuffleLibraries on an
//     explicit scope. The case runs with the probe on seat 0 AND on seat 1
//     (the controller-first order only differs when seat 1 casts it).
//
// The case builds the kernel table with printedDelveSetIn (kgen), which is
// the RED signal: while kgen refuses the shape (its reason: ChangeZoneAll
// Defined$ "Player"), printing the table fails -- the real Struggle //
// Survive rides seat 1's deck for that. The mechanism itself is driven by
// "Survive Probe", an inline {1}{G} sorcery carrying the same A: parameters
// (the real Survive is Aftermath-only, so a seed reaches the graveyard far
// too slowly to gate the case on it): its case FAILS unless some seed
// shuffles a graveyard holding a resolved Survive Probe into its owner's
// library. The shuffles themselves are enforced by the lockstep: a missing
// or extra shuffle shows as a divergent library order when a later decision
// reads the top of one.
//
// The policy casts the probe whenever it can, plays lands, attacks with
// everything on seat 0 (seat 1 never attacks), and answers the rest from the
// seed's own random stream.

const surviveProbeScript = "Name:Survive Probe\nManaCost:1 G\nTypes:Sorcery\n" +
	"A:SP$ ChangeZoneAll | Shuffle$ True | Destination$ Library | Origin$ Graveyard | Defined$ Player | ChangeType$ Card | SpellDescription$ Everyone shuffles a graveyard into a library.\n" +
	"Oracle:Everyone shuffles a graveyard into a library.\n"

// surviveOwnerShuffleReached: a resolved Survive Probe (in some player's
// graveyard) moved graveyard -> library -- the ChangeZoneAll sweep itself.
func surviveOwnerShuffleReached(g *GorgeSide, owner state.PlayerID) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind == events.MoveZone && e.From == state.ZGraveyard && e.To == state.ZLibrary {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Survive Probe" && o.Owner == owner {
				return true
			}
		}
	}
	return false
}

// surviveShufflePolicy casts Survive Probe first, then lands, then anything
// else, attacks with everything on seat 0 and declares none on seat 1; the
// tail is the seed's own random stream.
func surviveShufflePolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DAttackers {
			if p.D.Player == 0 {
				for _, a := range p.D.Options {
					if a.Kind() == ActAttack {
						return a
					}
				}
			} else {
				for _, a := range p.D.Options {
					if a.Kind() != ActAttack {
						return a
					}
				}
			}
		}
		if p.D.Kind == DPriority {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Survive Probe" {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActCast || a.Kind() == ActActivate {
					return a
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestHostedCmdrSurviveOwnerShuffleLockstep plays Survive's all-players
// graveyard shuffle decision for decision against gorge: kgen must accept
// the printed split card (printedDelveSetIn fails while the shape refuses),
// the kernel's every-seat sweep and per-receiving-library shuffles must
// match gorge, and some seed must really shuffle a filled graveyard into
// its owner's library so the case cannot pass vacuously.
func TestHostedCmdrSurviveOwnerShuffleLockstep(t *testing.T) {
	for _, probeSeat := range []int{0, 1} {
		t.Run(fmt.Sprintf("probe-seat-%d", probeSeat), func(t *testing.T) { surviveOwnerShuffleLockstep(t, probeSeat) })
	}
}

func surviveOwnerShuffleLockstep(t *testing.T, probeSeat int) {
	fixtures := map[string]string{"Survive Probe": surviveProbeScript}
	decks := [2][]string{
		append(append(deckOf(10, "Survive Probe"), deckOf(10, "Grizzly Bears")...), deckOf(20, "Forest")...),
		append(append(deckOf(4, "Struggle // Survive"), deckOf(16, "Grizzly Bears")...), deckOf(20, "Forest")...),
	}
	if probeSeat == 1 {
		decks[0], decks[1] = decks[1], decks[0]
	}
	src := corpusDirOrSkip(t)
	dir := filters11Corpus(t, fixtures, "Forest", "Grizzly Bears")
	// filters11Corpus' name mapper leaves the split card's slashes in the
	// script path, so the real script is copied in by hand.
	body, err := os.ReadFile(filepath.Join(src, "cardsfolder", "s", "struggle_survive.txt"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "cardsfolder", "s")
	if err := os.MkdirAll(dst, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "struggle_survive.txt"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
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
		if len(cardDecks[s]) != 40 {
			t.Fatalf("seat %d deck has %d cards, want 40", s, len(cardDecks[s]))
		}
	}
	// printedDelveSetIn runs kgen over the decks: a refused card fails here.
	set := printedDelveSetIn(t, dir, decks)
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	seen := false
	played := 0
	seats := [2]string{fmt.Sprintf("hcs-surv-a%d", probeSeat), fmt.Sprintf("hcs-surv-b%d", probeSeat)}
	for seed := uint64(1); seed <= 60 && !(seen && played >= 3); seed++ {
		m, err := NewMatchup(reg, seats, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			if !seen && surviveOwnerShuffleReached(g, state.PlayerID(probeSeat)) {
				seen = true
			}
		}
		res := Lockstep(m, seed, k, surviveShufflePolicy(m, seed), rc)
		if res.Div != nil {
			d := res.Div
			CloseSide(k)
			t.Fatalf("seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
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
		t.Fatal("the graveyard shuffle was not reached on any seed played, so the lockstep is vacuous")
	}
}

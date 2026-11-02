package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorgeMosswoodFixTime is gorge cr-mayplay-adventure-from-graveyard (merge
// e28306134, 2026-10-26 09:17:16 -0600 = 15:17:16Z, on gorge main): before it
// the graveyard "cast as an Adventure" permission offered no cast. gorgePinAtLeast
// compares against a module pseudo-version's UTC stamp, so the constant is UTC
// (every other gorge*FixTime is); at a pin whose develop tip does not carry the
// merge the test must skip, not run against an unfixed gorge.
const gorgeMosswoodFixTime = "20261026151716"

// The Mosswood Dreadknight lockstep: "When it dies, you may cast it
// from your graveyard as an Adventure until the end of your next turn"
// (Effect MayPlay, Affected$ Card.IsRemembered+Adventure; the permission
// offers Dread Whispers only, never the creature). Shock, Prodigal Sorcerer
// and Hill Giant kill it; the expected answer at every decision is gorge's.
var kmosswoodMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Mosswood Dreadknight", Count: 12}, {Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Shock", Count: 6}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Suntail Hawk", Count: 4}, {Name: "Mountain", Count: 11}, {Name: "Island", Count: 9},
	},
}, {
	{
		{Name: "Mosswood Dreadknight", Count: 12}, {Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Mosswood Dreadknight", Count: 12}, {Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
	},
}}

// kmosswoodTally counts, on gorge's side, casts of Dread Whispers from a
// graveyard (the permission used) and games.
type kmosswoodTally struct{ casts, games int }

func kmosswoodCheck(tal *kmosswoodTally) func(*GorgeSide, Side) *Divergence {
	seen := 0
	var cur *GorgeSide
	var inGrave map[state.ObjID]bool
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen, inGrave = ref, 0, map[state.ObjID]bool{}
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			switch ev.Kind {
			case events.MoveZone:
				if ev.To == state.ZGraveyard {
					inGrave[ev.Obj] = true
				} else if ev.From == state.ZGraveyard {
					delete(inGrave, ev.Obj)
				}
			case events.CastInfo:
				// the graveyard card is cast as its Adventure
				if o := ref.E.G.Obj(ev.Obj); inGrave[ev.Obj] && o != nil && o.Face() != nil && o.Face().Name == "Dread Whispers" {
					tal.casts++
					delete(inGrave, ev.Obj)
				}
			}
		}
		return nil
	}
}

func TestMosswoodLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeMosswoodFixTime) {
		t.Skip("gorge before cr-mayplay-adventure-from-graveyard (e28306134)")
	}
	var tal kmosswoodTally
	sonRun(t, "mosswood", kmosswoodMatchups, 300, true, nil, kmosswoodCheck(&tal))
	t.Logf("%d games: %d Adventure casts from a graveyard", tal.games, tal.casts)
	if tal.casts < 30 {
		t.Errorf("Adventure casts from the graveyard %d; want at least 30", tal.casts)
	}
}

// TestMosswoodLockstepMutation: with the permission dropped, some seed
// diverges.
func TestMosswoodLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeMosswoodFixTime) {
		t.Skip("gorge before cr-mayplay-adventure-from-graveyard (e28306134)")
	}
	n := sonRun(t, "mosswood", kmosswoodMatchups, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpMayPlay {
						p[x] = kop.Instr{Op: kop.OpNop}
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with the permission dropped")
	}
}

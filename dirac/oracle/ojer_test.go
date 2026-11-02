package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Ojer Axonil, Deepest Might lockstep: (1) a red source its
// controller controls that would deal noncombat damage below Ojer's power to
// an opponent deals Ojer's power instead (DamageDone replacement, DB$
// ReplaceEffect of DamageAmount); (2) when Ojer dies it returns to the
// battlefield tapped and transformed (ChangeZone Transformed$); (3) Temple of
// Power's {2}{R},{T}: transform, only as a sorcery and only if red sources its
// controller controlled dealt 4 or more noncombat damage this turn
// (Count$NonCombatDamageThisTurn Card.Red+YouCtrl Any). The expected answer
// at every decision is gorge's.
var kojerMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Ojer Axonil, Deepest Might", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Prodigal Pyromancer", Count: 4}, {Name: "Goblin Chainwhirler", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Mountain", Count: 16},
	},
	{
		{Name: "Ojer Axonil, Deepest Might", Count: 4}, {Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 6},
		{Name: "Prodigal Sorcerer", Count: 4}, {Name: "Prey Upon", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 2}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 3}, {Name: "Island", Count: 3},
	},
}, {
	// the order ask: Twinflame Tyrant (doubling) beside Ojer's raise; two
	// applicable replacements stall loud (gorge asks for the CR 616.1 order)
	{
		{Name: "Ojer Axonil, Deepest Might", Count: 5}, {Name: "Twinflame Tyrant", Count: 2},
		{Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 4}, {Name: "Prodigal Pyromancer", Count: 4},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Mountain", Count: 17},
	},
	{
		{Name: "Ojer Axonil, Deepest Might", Count: 4}, {Name: "Twinflame Tyrant", Count: 2}, {Name: "Shock", Count: 6},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Mountain", Count: 16},
	},
}}

// kojerTally counts, on gorge's side, the noncombat damage events to a player
// raised to Ojer's power (a 1-, 2- or 3-damage card landing 4), the Ojers
// that returned transformed from the graveyard and the Temples that
// transformed back.
type kojerTally struct {
	games, raised, returned, temple int
}

func kojerCheck(tal *kojerTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			switch ev.Kind {
			case events.DamageProvenance:
				if _, pl := ev.IDs[0].PlayerRef(); pl && ev.Amount == 4 && !strings.HasPrefix(ev.Text, events.DamageProvenanceCombat) {
					if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil {
						switch o.Face().Name {
						case "Shock", "Lightning Bolt", "Prodigal Pyromancer", "Goblin Chainwhirler":
							tal.raised++
						}
					}
				}
			case events.FlipFace:
				if o := ref.E.G.Obj(ev.Obj); o == nil || o.Card == nil || o.Card.Faces[0].Name != "Ojer Axonil, Deepest Might" {
					continue
				}
				if seen+1 < len(evs) && evs[seen+1].Kind == events.MoveZone && evs[seen+1].Obj == ev.Obj && evs[seen+1].From == state.ZGraveyard {
					tal.returned++
				} else {
					tal.temple++
				}
			}
		}
		return nil
	}
}

func TestOjerLockstep(t *testing.T) {
	t.Parallel()
	var tal kojerTally
	_, stalled := sonRunStalls(t, "ojer", kojerMatchups, 600, true, nil, kojerCheck(&tal))
	t.Logf("%d games: %d noncombat damage events raised to 4, %d Ojers returned transformed, %d Temples transformed; %d stalled loud on the damage-replacement order ask", tal.games, tal.raised, tal.returned, tal.temple, stalled)
	if tal.raised < 200 || tal.returned < 500 || tal.temple < 25 {
		t.Errorf("raised %d (want >= 200), returned transformed %d (want >= 500), Temple transforms %d (want >= 25)", tal.raised, tal.returned, tal.temple)
	}
}

// TestOjerLockstepMutation: with the damage raise off, then with the
// tally off, then with the transformed return off, some seed diverges.
func TestOjerLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []string{"raise", "tally", "return"} {
		var mut func(*kop.Table)
		switch m {
		case "raise":
			mut = func(tbl *kop.Table) {
				cards := append([]kop.CardDef(nil), tbl.Cards...)
				for i := range cards {
					abs := append([]kop.Ability(nil), cards[i].Abilities...)
					for j := range abs {
						if abs[j].Kind == kop.AbReplacement {
							abs[j].Repl.C &^= kop.ReplDmgRaise
						}
					}
					cards[i].Abilities = abs
				}
				tbl.Cards = cards
			}
		case "tally":
			dirac.DebugNoRedNC = true
		case "return":
			dirac.DebugIgnoreTransformed = true
		}
		n := sonRun(t, "ojer", kojerMatchups, 100, false, mut, nil)
		dirac.DebugNoRedNC, dirac.DebugIgnoreTransformed = false, false
		if n == 0 {
			t.Errorf("mutation %s: no seed diverged", m)
		}
	}
}

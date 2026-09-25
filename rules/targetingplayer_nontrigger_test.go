package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Opponent specifications in non-triggered asks use the same deterministic
// rule as trigger asks: the first living seat other than the controller in
// AliveFrom(0) answers. Forge does not specify which of multiple opponents
// gets to choose, so this engine-level fallback is explicit rather than
// pretending the controller selected one.
func TestTargetingPlayerOpponentSpellAndActivation(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name, ability string
		kind          string
	}{
		{name: "Evangelize", ability: "SP", kind: "spell"},
		{name: "Echo Chamber", ability: "AB", kind: "activation"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			e, _ := combatTriggerBoard(t, reg,
				[]string{tc.name},
				[]string{"Name:Own Creature\nManaCost:0\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"},
				nil,
				[]string{"Name:Opponent Creature\nManaCost:0\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"})
			carrier := mustCorpusCard(t, reg, tc.name)
			var sa *cards.SA
			for _, candidate := range carrier.Faces[0].Abilities {
				if candidate.Kind == tc.ability && candidate.Params["TargetingPlayer"] == "Player.Opponent" {
					sa = candidate
					break
				}
			}
			if sa == nil {
				t.Fatalf("%s compiled %s ability with TargetingPlayer$ Player.Opponent not found", tc.name, tc.ability)
			}
			var source state.ObjID
			for i := range e.G.Objs {
				o := &e.G.Objs[i]
				if o.Owner == 0 && o.Card == carrier && o.Zone == state.ZBattlefield {
					source = o.ID
					break
				}
			}
			if source == 0 {
				t.Fatalf("%s source is not on the battlefield", tc.name)
			}
			own := findBattlefield(t, e, 0, "Own Creature", 0)
			opponent := findBattlefield(t, e, 1, "Opponent Creature", 0)
			if e.G.Obj(own).Controller == e.G.Obj(opponent).Controller {
				t.Fatal("candidate fixture does not contain different controllers")
			}

			// This directly drives the shared cast/activation target-ask choke
			// point with the real compiled card ability; target legality remains
			// calculated from controller 0, while only the answering seat moves.
			e.askTarget(0, source, sa)
			d := e.Pending()
			if d == nil || d.Kind != decision.KTarget {
				t.Fatalf("pending ask = %+v, want target decision", d)
			}
			if d.Player != 1 {
				t.Fatalf("chooser = %d, want first living opponent seat 1", d.Player)
			}
			if !targetOptionContains(d.Options, own) || !targetOptionContains(d.Options, opponent) {
				t.Fatalf("legal candidates unexpectedly changed with chooser: %+v", d.Options)
			}
			bad := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{len(d.Options) + 10}}
			if err := d.Validate(bad); err == nil {
				t.Fatal("chooser can submit a target not present in the legal option set")
			}
		})
	}
}

func targetOptionContains(options []decision.Option, obj state.ObjID) bool {
	for _, o := range options {
		if o.Obj == obj {
			return true
		}
	}
	return false
}

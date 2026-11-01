package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// repeatEachSacSpell is a fixture sorcery with Organ Harvest's loop body
// (RepeatEach over RepeatPlayers$ <%s>, each iteration's remembered player
// may sacrifice any number of creatures).
const repeatEachSacSpell = "Name:Mass Offering\nManaCost:B\nTypes:Sorcery\n" +
	"A:SP$ RepeatEach | RepeatSubAbility$ DBSac | RepeatPlayers$ %s | SpellDescription$ x\n" +
	"SVar:DBSac:DB$ Sacrifice | Defined$ Remembered | Amount$ Creatures | SacValid$ Creature | RememberSacrificed$ True | Optional$ True | SubAbility$ DBCleanup\n" +
	"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\nSVar:Creatures:Count$Valid Creature\nOracle:x\n"

// TestRepeatEachAsksEachRepeatedPlayerToSacrifice pins this shape on a
// three-seat free-for-all board: a RepeatEach whose body
// is an optional sacrifice by the iteration's player poses one sacrifice ask
// per repeated player, in turn order, each owned by that player, and each
// answer moves that player's creature. RepeatPlayers$ NonOpponent is the
// caster alone in a free-for-all game (no teammates, CR 102.2), so Organ
// Harvest's spelling asks once; Player asks all three seats and Opponent the
// two others.
func TestRepeatEachAsksEachRepeatedPlayerToSacrifice(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		players string
		want    []state.PlayerID
	}{
		{"Player", []state.PlayerID{0, 1, 2}},
		{"NonOpponent", []state.PlayerID{0}},
		{"Opponent", []state.PlayerID{1, 2}},
	} {
		e := threeSeatEngine(t)
		e.G.Active, e.G.Priority, e.G.Step = 0, 0, state.StepMain1
		var bears []state.ObjID
		for p := state.PlayerID(0); p < 3; p++ {
			bears = append(bears, onBoardReady(t, e, p, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n"))
		}
		spell := onBoard(t, e, 0, fmt.Sprintf(repeatEachSacSpell, tc.players))
		sa := e.G.Obj(spell).Face().SpellAbility()
		probeAfterPass(e, func() {
			effects.Resolve(e, &effects.Ctx{Source: spell, Controller: 0, SVars: e.G.Obj(spell).Face().SVars}, sa)
		})
		var asked []state.PlayerID
		for i := 0; i < 6; i++ {
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "sacrifice" {
				break
			}
			asked = append(asked, d.Player)
			pick := -1
			for _, o := range d.Options {
				if o.Kind == "sacrifice" && o.Obj == bears[d.Player] {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("%s: player %d's ask does not offer their own Bears: %+v", tc.players, d.Player, d.Options)
			}
			submitChoices(t, e, pick)
		}
		if fmt.Sprint(asked) != fmt.Sprint(tc.want) {
			t.Fatalf("RepeatPlayers$ %s: sacrifice asks for players %v, want %v", tc.players, asked, tc.want)
		}
		for _, p := range tc.want {
			if o := e.G.Obj(bears[p]); o.Zone != state.ZGraveyard {
				t.Fatalf("RepeatPlayers$ %s: player %d's Bears in %v after the answer, want graveyard", tc.players, p, o.Zone)
			}
		}
	}
}

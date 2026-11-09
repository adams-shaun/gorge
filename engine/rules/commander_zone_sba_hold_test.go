// CR 704.3 / 704.6d / 903.9a: the commander-zone choice is one of a
// state-based check's actions, so it is made before the check performs
// anything else. At the ask the board is the one from before the check: a
// token in a graveyard has not yet ceased to exist (CR 704.5d) and an
// Equipment on the dead commander is still attached (CR 704.5n). The check
// after the answer performs them.
package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR903_9aCommanderChoiceHoldsTheTokenCease: seat 0's commander and a
// creature token both take lethal damage. Both reach the graveyard; the ask
// is posed with the token still there, and only the answer's check makes it
// cease to exist.
func TestCR903_9aCommanderChoiceHoldsTheTokenCease(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{tinyCmdSrc}, {}})
	cmd := fieldCommander(t, e, 0, 0)
	tok := onBoardCard(t, e, 0, corpusCard(t, "Grizzly Bears"))
	e.G.Obj(tok).IsToken = true
	e.G.Obj(cmd).Damage = 1
	e.G.Obj(tok).Damage = 2

	e.checkStateBased()

	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 {
		t.Fatalf("pending = %s, want seat 0's commander_zone ask", pendingKind(e.Pending()))
	}
	if !commanderMovedTo(e.L, tok, state.ZGraveyard) {
		t.Fatal("fixture: the lethally damaged token must have died with the commander")
	}
	if !slices.Contains(e.G.Zone(state.ZGraveyard, 0), tok) {
		t.Fatalf("CR 704.3: at the commander-zone ask the token must still be in the graveyard (zone %v)", e.G.Obj(tok).Zone)
	}
	submit(t, e, 0)
	if got := e.G.Obj(cmd).Zone; got != state.ZCommand {
		t.Fatalf("accepted commander zone = %v, want command", got)
	}
	if slices.Contains(e.G.Zone(state.ZGraveyard, 0), tok) {
		t.Fatal("CR 704.5d: after the answer the token must have ceased to exist")
	}
}

// TestCR903_9aCommanderChoiceHoldsTheEquipmentUnattach: an Equipment on the
// dying commander stays attached through the ask and is unattached by the
// answer's check.
func TestCR903_9aCommanderChoiceHoldsTheEquipmentUnattach(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{tinyCmdSrc}, {}})
	cmd := fieldCommander(t, e, 0, 0)
	eq := onBoardCard(t, e, 0, corpusCard(t, "Bonesplitter"))
	e.emit(events.Event{Kind: events.Attach, Obj: eq, IDs: []state.ObjID{cmd}})
	if got := e.G.Obj(eq).AttachedTo; got != cmd {
		t.Fatalf("fixture: Bonesplitter attached to %d, want the commander %d", got, cmd)
	}
	e.G.Obj(cmd).Damage = 1

	e.checkStateBased()

	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 {
		t.Fatalf("pending = %s, want seat 0's commander_zone ask", pendingKind(e.Pending()))
	}
	if got := e.G.Obj(cmd).Zone; got != state.ZGraveyard {
		t.Fatalf("fixture: commander zone at the ask = %v, want graveyard", got)
	}
	if got := e.G.Obj(eq).AttachedTo; got != cmd {
		t.Fatalf("CR 704.3: at the commander-zone ask the Equipment must still be attached to the commander, attached to %d", got)
	}
	submit(t, e, 1) // leave it in the graveyard
	if got := e.G.Obj(eq).AttachedTo; got != 0 {
		t.Fatalf("CR 704.5n: after the answer the Equipment must be unattached, attached to %d", got)
	}
}

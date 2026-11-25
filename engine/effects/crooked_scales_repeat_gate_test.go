package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// crookedScalesScript is the card's ability paragraph inline as a fixture:
// Forge card scripts (GPL) live only in gitignored .cards/, so the test pins
// the behaviour on a verbatim copy of the corpus line's SVar chain.
const crookedScalesScript = `Name:Crooked Scales
ManaCost:4
Types:Artifact
A:AB$ TargetOnly | Cost$ 4 T | ValidTgts$ Creature.OppCtrl | TgtPrompt$ Select target creature an opponent controls | RememberObjects$ ThisTargetedCard | IsCurse$ True | StackDescription$ SpellDescription | SubAbility$ DBTarget
SVar:DBTarget:DB$ TargetOnly | ValidTgts$ Creature.YouCtrl | TgtPrompt$ Select target creature you control | ImprintCards$ ThisTargetedCard | StackDescription$ None | SubAbility$ DBRepeat
SVar:DBRepeat:DB$ Repeat | RepeatSubAbility$ DBFlip | RepeatDefined$ Imprinted | RepeatPresent$ Card | RepeatSVarCompare$ EQ0
SVar:DBFlip:DB$ FlipCoin | WinSubAbility$ DestroyOpp | LoseSubAbility$ DestroyYours | StackDescription$ None
SVar:DestroyOpp:DB$ Destroy | Defined$ Remembered | SubAbility$ DBCleanup
SVar:DestroyYours:DB$ Destroy | Defined$ Imprinted | UnlessCost$ 3 | UnlessPayer$ You | SubAbility$ DBCleanup | UnlessResolveSubs$ WhenNotPaid
SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True | ClearImprinted$ True
Oracle:Flip a coin. If you win the flip, destroy target creature an opponent controls. If you lose the flip, destroy target creature you control unless you pay {3} and repeat this process.
`

// unpayableUnlessHost is the double standing in for a rules host whose payer
// cannot pay the offered unless cost: it reports every cost unpayable, the
// same answer UnlessCostPayableFromCtx gives with no floating or tappable
// mana on the board.
type unpayableUnlessHost struct {
	askHost
}

func (h *unpayableUnlessHost) UnlessCostPayableFromCtx(state.PlayerID, string, *Ctx) bool {
	return false
}

// crookedScalesBoard builds the card's own battlefield — the Scales for p0,
// the Memnite (its "creature you control" target) also for p0, the Ornithopter
// (its "creature an opponent controls" target) for p1 — and the ability
// resolution the way the real cast-time chain binds it: the opponent-side
// target as the root's answer, the you-side one as DBTarget's recorded
// cast-time pre-ask (Forge asks the whole chain's targets before payment).
// The creature IDs come back, not pointers: state.Game.Objs is a value slice,
// so pointers into it go stale across the board's later AddObject calls.
func crookedScalesBoard(t *testing.T, g *state.Game) (h *fakeHost, head *cards.SA, c *Ctx, memnite, ornith state.ObjID) {
	t.Helper()
	card := mkCard(t, crookedScalesScript)
	scales := g.AddObject(card, 0)
	repeatTestZone(g, scales, state.ZBattlefield, 0)
	memniteObj := g.AddObject(mkCard(t, "Name:Memnite\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	repeatTestZone(g, memniteObj, state.ZBattlefield, 0)
	ornithObj := g.AddObject(mkCard(t, "Name:Ornithopter\nTypes:Creature\nPT:0/2\nOracle:x\n"), 1)
	repeatTestZone(g, ornithObj, state.ZBattlefield, 1)
	if g.Obj(memniteObj.ID).Zone != state.ZBattlefield || g.Obj(ornithObj.ID).Zone != state.ZBattlefield ||
		g.Obj(scales.ID).Zone != state.ZBattlefield {
		t.Fatal("board fixture is not on the battlefield")
	}
	head = card.Faces[0].Abilities[0]
	dbTarget := head.Sub
	if dbTarget == nil || dbTarget.Sub == nil || dbTarget.Sub.API != "Repeat" {
		t.Fatalf("Crooked Scales DBTarget link not compiled: %+v", head.Sub)
	}
	h = &fakeHost{g: g}
	c = &Ctx{
		Source:         scales.ID,
		Controller:     0,
		TargetsOffered: true,
		OfferedSA:      head,
		Targets:        []state.Target{{Obj: ornithObj.ID}},
		SVars:          card.Faces[0].SVars,
		SubPreAsk:      map[string][]state.Target{dbTarget.Line: {{Obj: memniteObj.ID}}},
	}
	return h, head, c, memniteObj.ID, ornithObj.ID
}

// flipsIn reports how many coin flips the host's log recorded.
func flipsIn(h *fakeHost) int {
	n := 0
	for _, ev := range h.log {
		if _, _, ok := FlipNoteResult(ev); ok {
			n++
		}
	}
	return n
}

// TestCrookedScalesWinningFlipDestroysOpponentCreatureOnce pins the
// between-iteration gate Forge's RepeatEffect runs over the defined pile: the
// win branch's Cleanup empties it, and the gate — which compares the pile's
// size against Forge's RepeatCompare GE1 default — stops the process after
// the one flip. A win ends the process, destroying exactly the opponent's
// creature.
func TestCrookedScalesWinningFlipDestroysOpponentCreatureOnce(t *testing.T) {
	h, head, c, memnite, ornith := crookedScalesBoard(t, state.NewGame(names(2)))
	Resolve(h, c, head)
	if h.Suspended() {
		t.Fatal("resolution suspended on the effects double")
	}
	if flips := flipsIn(h); flips != 1 {
		t.Fatalf("winning first flip ran %d flips, want 1 (a win ends the process)", flips)
	}
	if z := h.g.Obj(ornith).Zone; z != state.ZGraveyard {
		t.Fatalf("Ornithopter zone = %v, want graveyard (destroyed by the win branch)", z)
	}
	if z := h.g.Obj(memnite).Zone; z != state.ZBattlefield {
		t.Fatalf("Memnite zone = %v, want battlefield (a win never destroys your creature)", z)
	}
}

// TestCrookedScalesUnpaidLossDestroysYourCreatureOnce pins the loss side with
// {3} unaffordable: the election is not payable, so the deterministic decline
// applies, DestroyYours destroys your creature, its Cleanup empties the
// imprint pile, and the gate stops — no second flip after an unpaid loss
// (Forge and XMage destroy exactly one creature, the one you control).
func TestCrookedScalesUnpaidLossDestroysYourCreatureOnce(t *testing.T) {
	fh, head, c, memnite, ornith := crookedScalesBoard(t, state.NewGame(names(2)))
	th := tailsHost{fh}
	Resolve(th, c, head)
	if th.Suspended() {
		t.Fatal("resolution suspended on the effects double")
	}
	if flips := flipsIn(fh); flips != 1 {
		t.Fatalf("losing first flip ran %d flips, want 1 (an unpaid loss ends the process)", flips)
	}
	if z := fh.g.Obj(memnite).Zone; z != state.ZGraveyard {
		t.Fatalf("Memnite zone = %v, want graveyard (destroyed when {3} was not paid)", z)
	}
	if z := fh.g.Obj(ornith).Zone; z != state.ZBattlefield {
		t.Fatalf("Ornithopter zone = %v, want battlefield (an unpaid loss never reaches the opponent's creature)", z)
	}
}

// TestUnlessCostUnaffordableOffersOnlyDecline pins the CR 118.3 half of the
// symptom on the shared gate: a payer the host reports as unable to pay
// ({3}, no mana of any kind) is offered only the decline option — never a
// pay option they cannot honestly choose.
func TestUnlessCostUnaffordableOffersOnlyDecline(t *testing.T) {
	h := &unpayableUnlessHost{}
	h.g = state.NewGame(names(2))
	Resolve(h, &Ctx{Controller: 0}, sa(t, "DB$ GainLife | Defined$ You | LifeAmount$ 2 | UnlessCost$ 3 | UnlessPayer$ You"))
	if h.asked == nil {
		t.Fatal("the unaffordable unless election was never posed")
	}
	if len(h.asked.Options) != 1 || h.asked.Options[0].Mode != decision.ModeUnlessDecline {
		t.Fatalf("unaffordable {3} options = %+v, want the lone decline", h.asked.Options)
	}
	if h.g.Players[0].Life != 22 {
		t.Fatalf("life = %d, want 22 (the unswitched body still runs on the decline)", h.g.Players[0].Life)
	}
}

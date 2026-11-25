package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRepeatEachRememberedAttachLowers plays Reckless Crew's per-token
// optional Attach in lockstep with gorge over the
// repeatrememberedattach fixture pool
// (dirac/cardset/pools/fixtures/repeatrememberedattach): "Create X 2/1 red
// Dwarf Berserker creature tokens ... For each of those tokens, you may
// attach an Equipment you control to it." The mint remembers the tokens
// (RememberTokens$), then DefinedCards$ RememberedCard repeats an
//
//	DB$ Attach | Choices$ Equipment.YouCtrl | Defined$ Remembered | Optional$ True
//
// once per remembered token, in remembered order. The loop is OpRepeatEach
// with C RepeatRemembered|RepeatPlain (the resumable remembered frame); the
// body is an optional object-side ChooseCard pick (AttachPickOpt) followed by
// an OpAttachObj of the pick onto RefEach. Gorge poses one Min-0 KChoose per
// token over the controller's battlefield Equipment that may legally attach
// to that token; a decline attaches nothing.
//
// Precondition: Reckless Crew is cast with at least one remembered token.
// Mutation: the pick attaches to the caster (RefController) instead of the
// repeated token, so gorge's per-token attachment and the kernel's no-op
// diverge.
func TestRepeatEachRememberedAttachLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run := runFixtureLockstep(t, "repeatrememberedattach", repeatRememberedAttachDecks, repeatRememberedAttachWatch, 200, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
		if run.casts["Reckless Crew"] < 10 {
			t.Errorf("only %d Reckless Crew casts (want 10)", run.casts["Reckless Crew"])
		}
	})
	t.Run("mutation/AttachToCaster", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeatrememberedattach", repeatRememberedAttachDecks, repeatRememberedAttachWatch, 200, false, func(tbl *kop.Table) {
			mutateCardIfPresent(t, tbl, "Reckless Crew", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpAttachObj && in.Obj == kop.RefEach {
					in.Obj = kop.RefController
					n++
				}
			})
		})
		if n == 0 {
			t.Fatal("no AttachObj onto RefEach in the table (the per-token attach is not lowered)")
		}
		run.requireDivergence(t, "the picked Equipment attached to the caster instead of the repeated token")
	})
}

var repeatRememberedAttachWatch = map[string]bool{"Reckless Crew": true}

var repeatRememberedAttachDecks = []fixtureDecks{
	{
		{
			{Name: "Reckless Crew", Count: 4}, {Name: "Bonesplitter", Count: 3}, {Name: "Short Sword", Count: 2},
			{Name: "Renegade Freighter", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Llanowar Elves", Count: 6},
			{Name: "Bonesplitter", Count: 2}, {Name: "Short Sword", Count: 2}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
		},
	},
	{
		{
			{Name: "Reckless Crew", Count: 4}, {Name: "Bonesplitter", Count: 2}, {Name: "Short Sword", Count: 2},
			{Name: "Renegade Freighter", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Bonesplitter", Count: 2}, {Name: "Short Sword", Count: 2}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
		},
	},
}

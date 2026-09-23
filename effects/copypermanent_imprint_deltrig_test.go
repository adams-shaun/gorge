package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestCopyPermanentImprintTokensDelTrigReadBack pins the ImprintTokens$ half
// of DB$ CopyPermanent END TO END at the effects boundary: the mint imprints
// the SOURCE with the copy (the Token-path precedent), and the following
// DelTrig's `RememberObjects$ ImprintedLKI` -- the read-back both corpus
// carrier-shaped DelTrig that Kharasha Foothills and Shredder, Shadow Master
// use -- resolves that imprint pile, so the delayed registration remembers
// EXACTLY the minted copies and the DelTrig body's `Defined$
// DelayTriggerRememberedLKI` names them. Before the ImprintedLKI case landed
// in definedSpec the registration carried NO ids and emitted one "unmodelled
// DelayedTrigger RememberObjects$ ImprintedLKI" Note, so the delayed
// exile/sacrifice found nothing.
//
// This test can fail three ways and each is the point: the copy is not
// imprinted on the source (the emit dropped), the registration carries no ids
// (the read-back dead again), or the registration retains the chain's
// remembered CARD alongside the minted copy.
func TestCopyPermanentImprintTokensDelTrigReadBack(t *testing.T) {
	h, c := fixtureHostWithTokens(t) // Game.Tokens: r_1_1_goblin, ...
	// The copy SOURCE plus a distinct creature to copy: the DelTrig read-back
	// must name the MINT, never the copied original.
	var target state.ObjID
	for i := range h.g.Objs {
		if h.g.Objs[i].Card != nil && h.g.Objs[i].ID != c.Source {
			target = h.g.Objs[i].ID
		}
	}
	if target == 0 || target == c.Source {
		t.Fatalf("precondition: no second board object to copy (target %d, source %d)", target, c.Source)
	}
	c.Remembered = []state.Target{{Obj: target}}

	Resolve(h, c, &cards.SA{Kind: "DB", API: "CopyPermanent", Params: map[string]string{
		"Defined": "Remembered", "ImprintTokens": "True"}})

	bf := h.Game().Zone(state.ZBattlefield, c.Controller)
	if len(bf) != 1 {
		t.Fatalf("battlefield = %v, want exactly the one minted copy", bf)
	}
	token := bf[0]
	if o := h.Game().Obj(token); o == nil || !o.IsToken {
		t.Fatalf("setup did not mint a token copy: %+v", o)
	}
	if o := h.Game().Obj(token); o != nil && o.ID == target {
		t.Fatal("the minted copy IS the copied original; the read-back below would be vacuous")
	}
	if got := h.Game().Obj(c.Source).ImprintTokens; len(got) != 1 || got[0] != token {
		t.Fatalf("source ImprintTokens = %v, want [%d] (the minted copy)", got, token)
	}
	// The pure read the DelTrig registration routes through: definedSpec's
	// ImprintedLKI case is exactly the source's imprint pile -- the minted
	// copy and nothing else (the remembered original must not leak in).
	pile := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "ImprintedLKI"}})
	if len(pile) != 1 || pile[0].IsPlayer || pile[0].Obj != token {
		t.Fatalf("Defined$ ImprintedLKI = %+v, want exactly the minted copy %d", pile, token)
	}

	// The carrier-shaped DelTrig: CopyPermanent's sibling DelayedTrigger reads
	// the imprint pile back. The corpus carriers' RepeatEach selector remains
	// unimplemented, so their complete chains are separately unreachable.
	before := len(h.log)
	Resolve(h, c, &cards.SA{Kind: "DB", API: "DelayedTrigger", Params: map[string]string{
		"Mode": "Phase", "Phase": "End Of Turn", "Execute": "TrigExile",
		"RememberObjects": "ImprintedLKI"}})
	var reg *events.Event
	for i := before; i < len(h.log); i++ {
		if h.log[i].Kind == events.DelayedRegister {
			reg = &h.log[i]
		}
	}
	if reg == nil {
		t.Fatalf("no DelayedRegister emitted; log %+v", h.log[before:])
	}
	if len(reg.IDs) != 1 || reg.IDs[0] != token {
		t.Fatalf("DelTrig registration ids = %v, want exactly the minted copy [%d]", reg.IDs, token)
	}
	for _, ev := range h.log[before:] {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unmodelled DelayedTrigger RememberObjects$") {
			t.Fatalf("the ImprintedLKI read-back still degrades to the unmodelled Note: %q", ev.Text)
		}
	}

	// And the DelTrig BODY (TrigExile's `Defined$ DelayTriggerRememberedLKI`)
	// resolves the registration's remember set to the minted copy -- the last
	// link the delayed exile/sacrifice needs.
	dctx := &Ctx{Source: c.Source, Controller: c.Controller,
		Remembered: []state.Target{{Obj: token}}}
	got := Defined(h, dctx, &cards.SA{Params: map[string]string{"Defined": "DelayTriggerRememberedLKI"}})
	found := false
	for _, tgt := range got {
		if !tgt.IsPlayer && tgt.Obj == token {
			found = true
		}
	}
	if !found {
		t.Fatalf("Defined$ DelayTriggerRememberedLKI = %+v, want the registered copy %d", got, token)
	}
}

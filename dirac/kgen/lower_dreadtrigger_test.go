package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestParanormalAnalystLowers: the real DSK card lowers with the Mode$
// ManifestDread trigger and the ChooseFromDefined$ TriggeredCards hidden
// pick (the game-wide graveyard pick with the trigger-object predicate).
func TestParanormalAnalystLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Paranormal Analyst")
	if !ok || card == nil {
		t.Fatal("Paranormal Analyst not in the corpus")
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	var trigMode, picks int
	for _, a := range lc.Abilities {
		if a.Trigger != nil && a.Trigger.Event == "ManifestDread" {
			trigMode++
			if a.Trigger.Whose != whoYou {
				t.Errorf("ManifestDread trigger Whose %v, want whoYou", a.Trigger.Whose)
			}
		}
		for _, in := range a.Prog {
			if in.Op != "ChooseMove" {
				continue
			}
			picks++
			if in.CS != "from=Graveyard|GameGrave|Mandatory" {
				t.Errorf("pick CS %q", in.CS)
			}
			if in.Obj.Kind != rYou || in.C != zHand || in.A.Kind != vConst || in.A.N != 1 {
				t.Errorf("pick Obj %v C %v A %v", in.Obj, in.C, in.A)
			}
			if in.F == nil || len(in.F.Clauses) != 1 || in.F.Clauses[0].CmpStat != cmpTriggerObj || in.F.Clauses[0].CmpN != 1 {
				t.Errorf("pick filter %v", in.F)
			}
		}
	}
	if trigMode != 1 || picks != 1 {
		t.Errorf("trigMode %d picks %d, want 1 and 1", trigMode, picks)
	}
	// and it builds, with the kop shapes the kernel reads
	_, all, pt, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	b := &all[0]
	sawTrig, sawPick := false, false
	for i := range b.def.Abilities {
		ab := &b.def.Abilities[i]
		if ab.Kind == kop.AbTriggered && ab.Trig.Event == kop.TrigManifestDread {
			sawTrig = true
		}
		for j := range ab.Prog {
			in := &ab.Prog[j]
			if in.Op != kop.OpChooseMove || in.F == 0 || pt.filters[in.F].Pred != kop.PredTriggerObj {
				continue
			}
			sawPick = true
			if kop.Zone(in.C&7) != kop.ZoneHand || in.C>>3&kop.SrMandatory == 0 || in.B.N&kop.SrGameGrave == 0 {
				t.Errorf("pick C %#x zone %d, B %#x", in.C, in.C&7, in.B.N)
			}
		}
	}
	if !sawTrig || !sawPick {
		t.Errorf("built card: TrigManifestDread %v PredTriggerObj pick %v", sawTrig, sawPick)
	}
}

// TestUnidentifiedHovershipLowers: the real DSK card lowers with the
// remembered-persist pair — the ETB exile remembers, the leave trigger
// manifests dread for the remembered card's owner (RefRememberedOwner) and
// clears the set.
func TestUnidentifiedHovershipLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Unidentified Hovership")
	if !ok || card == nil {
		t.Fatal("Unidentified Hovership not in the corpus")
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	if !lc.RemPersist {
		t.Error("RemPersist not set")
	}
	var dreads int
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			if in.Op != "ManifestDread" {
				continue
			}
			dreads++
			if in.Obj.Kind != rRememberedOwner {
				t.Errorf("ManifestDread Obj %v, want rRememberedOwner", in.Obj)
			}
		}
	}
	if dreads != 1 {
		t.Errorf("ManifestDread instructions %d, want 1", dreads)
	}
	_, all, pt, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	b := &all[0]
	_ = pt
	if !b.def.RemPersist {
		t.Error("CardDef.RemPersist not set")
	}
	saw := false
	for i := range b.def.Abilities {
		for j := range b.def.Abilities[i].Prog {
			in := &b.def.Abilities[i].Prog[j]
			if in.Op == kop.OpManifestDread && in.Obj == kop.RefRememberedOwner {
				saw = true
			}
		}
	}
	if !saw {
		t.Error("built card has no ManifestDread over RefRememberedOwner")
	}
}

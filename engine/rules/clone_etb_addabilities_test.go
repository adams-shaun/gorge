package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCloneETBAddAbilitiesMercurialPretender casts the real Mercurial
// Pretender with a creature its caster controls on the battlefield: the
// as-enters copy election is offered (AddAbilities$ is inside the ETB Clone
// scope, CR 614.1c / 707.9b), the Pretender enters as a copy of the bear, and
// the copy has the granted "{2}{U}{U}: Return this creature to its owner's
// hand." (it was the loud "unimplemented API Clone" fallback: a 0/0 that
// died).
func TestCloneETBAddAbilitiesMercurialPretender(t *testing.T) {
	t.Parallel()
	bear := corpusAlternativeCard(t, "Grizzly Bears")
	e := goadGrantEngine(t, corpusAlternativeCard(t, "Mercurial Pretender"))
	b := onBoardReadyCard(t, e, 0, bear)
	e.G.Players[0].Pool[state.MC] = 4
	e.G.Players[0].Pool[state.MU] = 1
	mp := e.G.Zone(state.ZHand, 0)[0]

	castMode(t, e, mp, "")
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("expected the ETB copy election, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == b {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("own bear not offered as the template: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 8)

	clone := e.G.Obj(mp)
	if clone == nil || clone.Zone != state.ZBattlefield || clone.Face().Name != "Grizzly Bears" || !hasEvent(e, events.ClonePermanent, mp) {
		t.Fatalf("pretender = %+v, want a Grizzly Bears copy on the battlefield", clone)
	}
	if hasNote(e, "unimplemented API Clone") {
		t.Fatal("the loud Clone fallback ran")
	}
	gr := e.grantedAbilities(0, mp)
	if len(gr) != 1 || gr[0].SA.API != "ChangeZone" {
		t.Fatalf("granted abilities %+v, want the one return-to-hand ability", gr)
	}
	if n := len(e.grantedAbilities(0, b)); n != 0 {
		t.Fatalf("the template has %d granted abilities, want none", n)
	}
	e.G.Players[0].Pool[state.MC] = 2
	e.G.Players[0].Pool[state.MU] = 2
	e.priorityRound()
	opt := -1
	if d := e.Pending(); d != nil {
		for _, o := range d.Options {
			if o.Kind == "ability" && o.Obj == mp && o.SVar != "" {
				opt = o.Index
			}
		}
	}
	if opt < 0 {
		t.Fatal("the granted return-to-hand ability is not offered at priority")
	}
	submitChoices(t, e, opt)
	passUntilStackEmpty(t, e, 8)
	inHand := false
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mercurial Pretender" {
			inHand = true
		}
	}
	if !inHand || e.G.Obj(b).Zone != state.ZBattlefield {
		t.Fatal("the granted ability did not return the Pretender (and only it) to its owner's hand")
	}
}

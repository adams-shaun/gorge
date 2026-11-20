package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const soldierTokenSrc = "Name:Soldier\nTypes:Creature Soldier\nPT:1/1"

// TestEssenceOfTheWildCreaturesEnterAsCopies: with Essence of the Wild on the
// battlefield, a creature entering under its controller (cast or token) is a
// copy of Essence (6/6, CR 614.1c/707.9), Essence itself is untouched, and an
// opponent's creature enters as itself (Creature.Other+YouCtrl).
func TestEssenceOfTheWildCreaturesEnterAsCopies(t *testing.T) {
	t.Parallel()
	e := goadGrantEngine(t, corpusAlternativeCard(t, "Grizzly Bears"))
	essence := onBoardReadyCard(t, e, 0, corpusAlternativeCard(t, "Essence of the Wild"))
	if o := e.G.Obj(essence); o.Face().Name != "Essence of the Wild" || e.Power(essence) != 6 {
		t.Fatalf("precondition: Essence is %q %d", o.Face().Name, e.Power(essence))
	}

	bears := e.G.Zone(state.ZHand, 0)[0]
	if e.G.Obj(bears).Face().Name != "Grizzly Bears" {
		t.Fatalf("hand card is %q, want Grizzly Bears", e.G.Obj(bears).Face().Name)
	}
	e.G.Players[0].Pool[state.MG] = 1
	e.G.Players[0].Pool[state.MC] = 1
	castMode(t, e, bears, "")
	e.resolveTop()
	passUntilStackEmpty(t, e, 8)
	o := e.G.Obj(bears)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Grizzly Bears = %+v, want it on the battlefield", o)
	}
	if o.Face().Name != "Essence of the Wild" || e.Power(bears) != 6 || e.Toughness(bears) != 6 {
		t.Fatalf("the cast creature entered as %q %d/%d, want a 6/6 Essence copy",
			o.Face().Name, e.Power(bears), e.Toughness(bears))
	}

	own := putToken(t, e, 0, soldierTokenSrc, state.ZBattlefield)
	passUntilStackEmpty(t, e, 8)
	if to := e.G.Obj(own); to == nil || !to.IsToken || to.Zone != state.ZBattlefield ||
		to.Face().Name != "Essence of the Wild" || e.Power(own) != 6 {
		t.Fatalf("own token = %+v, want a 6/6 Essence copy token", to)
	}

	opp := putToken(t, e, 1, soldierTokenSrc, state.ZBattlefield)
	passUntilStackEmpty(t, e, 8)
	if to := e.G.Obj(opp); to == nil || to.Zone != state.ZBattlefield || to.Controller != 1 ||
		to.Face().Name != "Soldier" || e.Power(opp) != 1 {
		t.Fatalf("opponent token = %+v, want an uncopied 1/1 Soldier", to)
	}
	if o := e.G.Obj(essence); o.Face().Name != "Essence of the Wild" {
		t.Fatalf("Essence itself became %q", o.Face().Name)
	}
}

// TestThunderbondVanguardEntryCopy: each creature TOKEN its controller gets
// enters as a copy of Thunderbond Vanguard (power and toughness equal to the
// number of creatures its controller controls); a cast nontoken creature and
// an opponent's token enter as themselves.
func TestThunderbondVanguardEntryCopy(t *testing.T) {
	t.Parallel()
	e := goadGrantEngine(t, corpusAlternativeCard(t, "Grizzly Bears"))
	vanguard := onBoardReadyCard(t, e, 0, corpusAlternativeCard(t, "Thunderbond Vanguard"))

	tok := putToken(t, e, 0, soldierTokenSrc, state.ZBattlefield)
	passUntilStackEmpty(t, e, 8)
	to := e.G.Obj(tok)
	if to == nil || !to.IsToken || to.Zone != state.ZBattlefield || to.Face().Name != "Thunderbond Vanguard" {
		t.Fatalf("own token = %+v, want a Thunderbond Vanguard copy token", to)
	}
	// Two creatures (the Vanguard and its token copy): the copy's */* is 2/2.
	if e.Power(tok) != 2 || e.Toughness(tok) != 2 || e.Power(vanguard) != 2 {
		t.Fatalf("copy token is %d/%d (Vanguard %d), want 2/2 from two creatures",
			e.Power(tok), e.Toughness(tok), e.Power(vanguard))
	}

	opp := putToken(t, e, 1, soldierTokenSrc, state.ZBattlefield)
	passUntilStackEmpty(t, e, 8)
	if oo := e.G.Obj(opp); oo == nil || oo.Zone != state.ZBattlefield || oo.Face().Name != "Soldier" {
		t.Fatalf("opponent token = %+v, want an uncopied Soldier", oo)
	}

	bears := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MG] = 1
	e.G.Players[0].Pool[state.MC] = 1
	castMode(t, e, bears, "")
	e.resolveTop()
	passUntilStackEmpty(t, e, 8)
	if bo := e.G.Obj(bears); bo == nil || bo.Zone != state.ZBattlefield || bo.Face().Name != "Grizzly Bears" {
		t.Fatalf("cast Grizzly Bears = %+v, want an uncopied nontoken Bears", bo)
	}
}

// TestFourteenthDoctorEntryCopy: the cast Doctor may enter as a copy of a
// Doctor card in a graveyard that was put there from a library this turn, and
// the copy gains haste until end of turn. A Doctor that did not come from the
// library and a non-Doctor that did are not offered.
func TestFourteenthDoctorEntryCopy(t *testing.T) {
	t.Parallel()
	e := goadGrantEngine(t, corpusAlternativeCard(t, "The Fourteenth Doctor"))
	mill := func(src string, fromLibrary bool) state.ObjID {
		o := e.G.AddObject(card(t, src), 0)
		o.Zone = state.ZGraveyard
		e.G.SetZone(state.ZGraveyard, 0, append(e.G.Zone(state.ZGraveyard, 0), o.ID))
		o.EnteredThisTurn = true
		o.EnteredFrom = state.ZHand
		if fromLibrary {
			o.EnteredFrom = state.ZLibrary
		}
		return o.ID
	}
	good := mill("Name:Eleventh Doctor\nTypes:Creature Time Lord Doctor\nPT:4/4", true)
	mill("Name:Tenth Doctor\nTypes:Creature Time Lord Doctor\nPT:3/3", false)
	mill("Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2", true)

	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MR] = 1
	e.G.Players[0].Pool[state.MW] = 1
	e.G.Players[0].Pool[state.MU] = 1
	castMode(t, e, id, "")
	// The hybrid {R/G} symbol asks how to pay: answer it with red.
	if d := e.Pending(); d != nil && len(d.Options) > 0 && d.Options[0].Kind == "pay_R" {
		submitChoices(t, e, 0)
	}
	// Both players pass: the cast trigger (reveal fourteen) resolves, then
	// the spell, which asks for the copy election as it enters.
	for i := 0; i < 8 && len(e.G.Stack) > 0; i++ {
		if d := e.Pending(); d != nil && d.ResumeKind == "etb" {
			break
		}
		passOnceP(t, e)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("expected the ETB copy election, got %+v", d)
	}
	idx, n := -1, 0
	for _, o := range d.Options {
		if o.Kind == "clone" && o.Obj != 0 {
			n++
			if o.Obj == good {
				idx = o.Index
			}
		}
	}
	if idx < 0 || n != 1 {
		t.Fatalf("clone options = %+v, want exactly the library-milled Doctor", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 8)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Eleventh Doctor" ||
		e.Power(id) != 4 {
		t.Fatalf("entered object = %+v, want a 4/4 Eleventh Doctor copy", o)
	}
	if !edhHasKeyword(o, "Haste") && !e.hasKeywordH(id, kwhHaste) {
		t.Fatal("the copy lacks haste until end of turn")
	}
}

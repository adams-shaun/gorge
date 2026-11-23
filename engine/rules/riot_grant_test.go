package rules

// kw:Riot (CR 702.136) granted by a static: "Nontoken creatures you control
// have riot" (Rhythm of the Wild, Uncivil Unrest) and "Other Spiders you
// control have riot" (Spider-Punk). CR 614.12 makes an as-enters replacement
// look at the battlefield as it would be after the entry, so the entrant's
// DERIVED keyword list (the same read bloodthirstEntryMatch makes) — printed
// K:Riot line plus layer-6 AddKeyword$ grant — is what poses the ask. The
// grant is invisible to the primitive ratchet (Face.Primitives walks printed
// keywords only); these pins run on real corpus carriers.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// riotGrantEngine deals the hand cards, puts the granting permanents on the
// battlefield, and returns the entering candidates' ids. A grant that is
// itself a riot carrier (Spider-Punk's printed K:Riot) poses its OWN ask as
// it enters; the setup answers it, then drops the stale priority snapshot
// the raw setup move left (the fixture convention, probeDroppingPending) so
// the candidate's move is what poses the next ask.
func riotGrantEngine(t *testing.T, grantee *cards.Card, grants ...*cards.Card) (*Engine, state.ObjID) {
	t.Helper()
	hand := append([]*cards.Card{grantee}, grants...)
	e := handEngineTokens(t, hand...)
	var grantIDs []state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0)[1:] {
		grantIDs = append(grantIDs, id)
	}
	for _, id := range grantIDs {
		placeOnBattlefield(t, e, id)
	}
	for i := 0; i < 5; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			break
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("settle grant-entry ask: %v", err)
		}
	}
	e.pending = nil
	return e, e.G.Zone(state.ZHand, 0)[0]
}

// riotGrantChoice emits the candidate's battlefield move and reads the
// as-enters ask the entry poses (nil when it poses none).
func riotGrantChoice(e *Engine, id state.ObjID) *decision.Decision {
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	return e.Pending()
}

func TestGrantedRiotPosesTheAsEntersAsk(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	rhythm, ok := reg.Lookup("Rhythm of the Wild")
	if !ok {
		t.Fatal("Rhythm of the Wild missing from corpus")
	}
	recruit := card(t, "Name:Riot Recruit\nManaCost:2 R\nTypes:Creature Goblin Warrior\nPT:2/2\nOracle:x\n")

	// The grant poses the counter/haste ask; the "counter" answer enters with
	// the +1/+1 counter and no haste.
	e, id := riotGrantEngine(t, recruit, rhythm)
	rd := riotGrantChoice(e, id)
	if rd == nil || rd.Kind != decision.KChoose || len(rd.Options) != 2 || rd.Options[1].Kind != "riot" {
		t.Fatalf("granted riot ask = %+v, want counter/haste choice", rd)
	}
	if err := e.Submit(decision.Intent{Seq: rd.Seq, Player: rd.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit granted riot counter choice: %v", err)
	}
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.Counter("P1P1") != 1 || e.HasKeyword(id, "Haste") {
		t.Fatalf("granted riot counter entry = %+v, want 1 counter and no haste", o)
	}

	// The "haste" answer grants haste and no counter.
	e2, id2 := riotGrantEngine(t, recruit, rhythm)
	rd = riotGrantChoice(e2, id2)
	if rd == nil || rd.Kind != decision.KChoose || len(rd.Options) != 2 {
		t.Fatalf("granted riot ask (haste) = %+v, want counter/haste choice", rd)
	}
	if err := e2.Submit(decision.Intent{Seq: rd.Seq, Player: rd.Player, Choices: []int{1}}); err != nil {
		t.Fatalf("submit granted riot haste choice: %v", err)
	}
	o = e2.G.Obj(id2)
	if o.Zone != state.ZBattlefield || o.Counter("P1P1") != 0 || !e2.HasKeyword(id2, "Haste") {
		t.Fatalf("granted riot haste entry = %+v, want haste and no counter", o)
	}

	// A token is outside the grant (Affected$ Creature.!token+YouCtrl): it
	// enters with no ask.
	e3, _ := riotGrantEngine(t, recruit, rhythm)
	tok := card(t, "Name:Goblin Token\nManaCost:0\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	to := e3.G.AddObject(tok, 0)
	to.IsToken = true
	to.Zone = state.ZHand
	e3.G.SetZone(state.ZHand, 0, append(e3.G.Zone(state.ZHand, 0), to.ID))
	if rd := riotGrantChoice(e3, to.ID); rd != nil {
		t.Fatalf("token entry under the riot grant posed an ask %+v, want none", rd)
	}
}

// TestGrantedRiotSpiderPunkFiltersType pins the Affected$ type filter of the
// grant: a Spider you control gets the ask, a non-Spider does not.
func TestGrantedRiotSpiderPunkFiltersType(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	spiderPunk, ok := reg.Lookup("Spider-Punk")
	if !ok {
		t.Fatal("Spider-Punk missing from corpus")
	}
	spider := card(t, "Name:Weaver Cub\nManaCost:1 G\nTypes:Creature Spider\nPT:1/2\nOracle:x\n")
	beast := card(t, "Name:Plain Beast\nManaCost:2 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")

	e, sid := riotGrantEngine(t, spider, spiderPunk)
	rd := riotGrantChoice(e, sid)
	if rd == nil || rd.Kind != decision.KChoose || len(rd.Options) != 2 || rd.Options[1].Kind != "riot" {
		t.Fatalf("Spider-Punk's granted riot ask = %+v, want counter/haste choice", rd)
	}
	if err := e.Submit(decision.Intent{Seq: rd.Seq, Player: rd.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit Spider-Punk riot counter choice: %v", err)
	}
	if o := e.G.Obj(sid); o.Zone != state.ZBattlefield || o.Counter("P1P1") != 1 {
		t.Fatalf("Spider-Punk riot counter entry = %+v, want 1 counter", o)
	}

	e2, bid := riotGrantEngine(t, beast, spiderPunk)
	if rd := riotGrantChoice(e2, bid); rd != nil {
		t.Fatalf("non-Spider entry under Spider-Punk posed an ask %+v, want none", rd)
	}
}

package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// gatekeeperScript exiles a permanent, returns it if you controlled it, and
// otherwise gives its last controller a Detective: Unyielding Gatekeeper's chain
// as one spell-ability root, so TokenOwner$ and ConditionPresent$ both run.
const gatekeeperScript = "Name:Fixture Gatekeeper\nManaCost:1 W\nTypes:Creature Cleric\nPT:3/2\n" +
	"A:SP$ ChangeZone | ValidTgts$ Permanent.nonLand+Other | Origin$ Battlefield | Destination$ Exile | RememberLKI$ True | SubAbility$ Back\n" +
	"SVar:Back:DB$ ChangeZone | Defined$ Remembered | Origin$ Exile | Destination$ Battlefield | GainControl$ True | ConditionDefined$ Remembered | ConditionPresent$ Card.YouCtrl | SubAbility$ Detective\n" +
	"SVar:Detective:DB$ Token | TokenScript$ wu_2_2_detective | TokenOwner$ RememberedController | ConditionDefined$ Remembered | ConditionPresent$ Card.YouCtrl | ConditionCompare$ EQ0 | SubAbility$ Forget\n" +
	"SVar:Forget:DB$ Cleanup | ClearRemembered$ True\n" +
	"Oracle:x\n"

type gatekeeperRun struct {
	victim *state.Object
	tokens [2]int
}

// runGatekeeper resolves the chain with the Gatekeeper on seat 0 (its
// controller) against a victim seat vOwner owns and seat vCtrl controls.
// seeded mirrors a real TurnFaceUp trigger: the Gatekeeper itself is the
// trigger's captured referent, seeded into Ctx.Remembered and Ctx.Captured.
func runGatekeeper(t *testing.T, vOwner, vCtrl state.PlayerID, seeded bool) gatekeeperRun {
	t.Helper()
	h := newHost(t, 2)
	h.g.Tokens = map[string]*cards.Card{
		"wu_2_2_detective": mkCard(t, "Name:Detective Token\nTypes:Creature Detective\nPT:2/2\nOracle:x\n"),
	}
	card := mkCard(t, gatekeeperScript)
	gk := h.g.AddObject(card, 0)
	gk.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), gk.ID))
	victim := h.g.AddObject(mkCard(t, "Name:Fixture Relic\nTypes:Artifact\nOracle:x\n"), vOwner)
	victim.Zone = state.ZBattlefield
	victim.Controller = vCtrl
	h.g.SetZone(state.ZBattlefield, vOwner, append(h.g.Zone(state.ZBattlefield, vOwner), victim.ID))
	if o := h.g.Obj(victim.ID); o == nil || o.Zone != state.ZBattlefield || o.Controller != vCtrl || o.Owner != vOwner {
		t.Fatalf("precondition: victim = %+v", o)
	}

	c := &Ctx{Source: gk.ID, Controller: 0, Targets: []state.Target{{Obj: victim.ID}}}
	if seeded {
		c.Remembered = []state.Target{{Obj: gk.ID}}
		c.Captured = []state.Target{{Obj: gk.ID}}
	}
	Resolve(h, c, card.Faces[0].Abilities[0])
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented") {
			t.Fatalf("handler not registered: %q", ev.Text)
		}
	}
	var out gatekeeperRun
	out.victim = h.g.Obj(victim.ID)
	for seat := 0; seat < 2; seat++ {
		for _, id := range h.g.Zone(state.ZBattlefield, state.PlayerID(seat)) {
			if o := h.g.Obj(id); o != nil && o.IsToken {
				out.tokens[seat]++
			}
		}
	}
	return out
}

// TestGatekeeperStolenPermanentTokenGoesToLastKnownController: seat 0 owns a
// permanent seat 1 controls. "If you controlled it, return it ... Otherwise,
// its controller creates a Detective": seat 0 did not, so the card stays
// exiled and seat 1 (its controller before the exile reset it to the owner)
// gets the token (CR 608.2h). Both the TokenOwner$ read and the
// ConditionPresent$ Card.YouCtrl gate must look back.
func TestGatekeeperStolenPermanentTokenGoesToLastKnownController(t *testing.T) {
	r := runGatekeeper(t, 0, 1, false)
	if r.victim == nil || r.victim.Zone != state.ZExile {
		t.Fatalf("victim = %+v, want it to stay exiled (Gatekeeper's controller did not control it)", r.victim)
	}
	if r.tokens != [2]int{0, 1} {
		t.Fatalf("tokens per seat = %v, want [0 1] (its last-known controller, seat 1)", r.tokens)
	}
}

// TestGatekeeperOwnPermanentStolenByUsReturns: seat 1 owns a permanent seat 0
// (the Gatekeeper's controller) controlled, so "you controlled it" holds: it
// returns and nobody gets a Detective. Reading the post-exile live controller
// (the owner, seat 1) got this backwards.
func TestGatekeeperOwnPermanentStolenByUsReturns(t *testing.T) {
	r := runGatekeeper(t, 1, 0, false)
	if r.victim == nil || r.victim.Zone != state.ZBattlefield {
		t.Fatalf("victim = %+v, want it returned to the battlefield", r.victim)
	}
	if r.tokens != [2]int{0, 0} {
		t.Fatalf("tokens per seat = %v, want none: the Gatekeeper's controller controlled it", r.tokens)
	}
}

// TestGatekeeperTriggerReferentIsNotRemembered is the tracked Level-B repro
// (MKM Unyielding Gatekeeper/trigger#0.0): the firing trigger seeds the
// Gatekeeper itself into Ctx.Remembered, and the token's RememberedController
// must not read that referent's controller -- the opponent's permanent makes
// exactly ONE Detective, for its controller, none for the Gatekeeper's.
func TestGatekeeperTriggerReferentIsNotRemembered(t *testing.T) {
	r := runGatekeeper(t, 1, 1, true)
	if r.victim == nil || r.victim.Zone != state.ZExile {
		t.Fatalf("victim = %+v, want exiled", r.victim)
	}
	if r.tokens != [2]int{0, 1} {
		t.Fatalf("tokens per seat = %v, want [0 1]: the seeded trigger referent made its controller a token", r.tokens)
	}
}

// TestTokenOwnerRememberedControllerCarrierCensus is the corpus ratchet for
// the mechanism: every Token line whose TokenOwner$ is RememberedController
// reads a remembered object's controller AFTER the chain may have moved it.
// The carrier set is pinned; a new carrier must be reviewed for whether its
// chain moves the remembered card (and so needs the RememberLKI$ capture
// rememberedControllersLKI reads) before it is added here.
func TestTokenOwnerRememberedControllerCarrierCensus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	known := map[string]bool{"Unyielding Gatekeeper": true, "The Curse of Fenric": true, "Terastodon": true}
	found := map[string]bool{}
	for _, card := range reg.AllCards() {
		for _, f := range card.Faces {
			for _, body := range f.SVars {
				if strings.Contains(body, "DB$ Token") && strings.Contains(body, "TokenOwner$ RememberedController") {
					found[f.Name] = true
				}
			}
			for _, sa := range f.Abilities {
				if sa != nil && sa.API == "Token" && strings.HasPrefix(strings.TrimSpace(sa.ParamStr(cards.PKTokenOwner)), "RememberedController") {
					found[f.Name] = true
				}
			}
		}
	}
	if len(found) == 0 {
		t.Fatal("census found no carriers: the scan is not reading the corpus")
	}
	for name := range found {
		if !known[name] {
			t.Errorf("new TokenOwner$ RememberedController carrier %q: review whether its chain moves the remembered card", name)
		}
	}
	for name := range known {
		if !found[name] {
			t.Errorf("pinned carrier %q no longer found", name)
		}
	}
	// Unyielding Gatekeeper is the one carrier whose first step exiles the
	// remembered card, so its chain must keep the RememberLKI$ capture the
	// last-known-controller read depends on.
	gk, ok := reg.Lookup("Unyielding Gatekeeper")
	if !ok || !strings.Contains(gk.Faces[0].SVars["TrigExile"], "RememberLKI$ True") {
		t.Fatalf("Unyielding Gatekeeper no longer remembers LKI: %+v", gk)
	}
}

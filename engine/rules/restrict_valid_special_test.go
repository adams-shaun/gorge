package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 116.2b/f/h/k/m: turning a permanent face up, suspending, foretelling,
// plotting and unlocking a door are special actions, neither spells nor
// activated abilities. A RestrictValid$ "spend only on spells" (CR 106.6)
// mana cannot pay one; a Static.* restriction naming the action can. Forge
// reads the same split (SpellAbility.java: Spell needs isSpell, Static needs
// an AbilityStatic root; nonSpell is !isSpell).

// restrictedMana floats n units of colour c in seat 0's pool under the given
// RestrictValid$ text, exactly the ManaAdd a restricted mana ability emits.
func restrictedMana(e *Engine, c string, n int32, valid string) {
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: c, Amount: n,
		Text: events.ManaRestrictionText(valid, 0)})
}

// manifestedBearEngine is a manifestEngine game with seat 0's Grizzly Bears
// manifested face down, priority at seat 0's main phase.
func manifestedBearEngine(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Grizzly Bears")
	id := manifestCard(t, e, 0, "Grizzly Bears")
	if o := e.G.Obj(id); o == nil || !o.FaceDown || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the manifested bear is not a face-down permanent: %+v", o)
	}
	return e, cfg, id
}

func optionsOfKind(e *Engine, kind string, id state.ObjID) []decision.Option {
	var out []decision.Option
	for _, o := range e.legalActions(0) {
		if o.Kind == kind && o.Obj == id {
			out = append(out, o)
		}
	}
	return out
}

// Overgrown Zealot's mana ("spend only to turn permanents face up",
// Static.isTurnFaceUp) pays the {1}{G} turn-up of a manifested Grizzly Bears.
func TestRestrictValidStaticTurnFaceUpPaysTurnUp(t *testing.T) {
	t.Parallel()
	e, cfg, id := manifestedBearEngine(t)
	restrictedMana(e, "G", 2, "Static.isTurnFaceUp")
	if got := restrictedBatchUnits(restrictedBatchesOf(e, 0, "Static.isTurnFaceUp")); got != 2 {
		t.Fatalf("precondition: %d restricted units floating, want 2", got)
	}
	e.priorityRound()
	if len(manifestTurnUpOptions(t, e, id)) == 0 {
		t.Fatalf("Static.isTurnFaceUp mana did not offer the turn-up: %+v", e.Pending().Options)
	}
	submitManifestTurnUp(t, e, id, "manifest")
	if e.G.Obj(id).FaceDown {
		t.Fatal("the manifested bear is still face down after the turn-up")
	}
	if left := e.G.Players[0].Pool.Total(); left != 0 {
		t.Fatalf("pool after the turn-up = %d, want 0", left)
	}
	if hasNote(e, "no longer payable") {
		t.Fatal("the turn-up aborted although the offer proved it payable")
	}
	replayCheck(t, e, cfg)
}

// "Spend only to cast spells" mana cannot turn a permanent face up: the
// turn-up is not offered (it used to be offered, then abort with a Note).
func TestSpellRestrictedManaNeverOffersTurnUp(t *testing.T) {
	t.Parallel()
	// Control: the same two mana, unrestricted, DOES offer the turn-up.
	e, _, id := manifestedBearEngine(t)
	addMana(t, e, 0, "GG")
	if len(manifestTurnUpOptions(t, e, id)) == 0 {
		t.Fatalf("precondition: plain GG does not offer the {1}{G} turn-up: %+v", e.Pending().Options)
	}
	for _, valid := range []string{"Spell", "Spell.Creature", "Activated", "Spell.Enchantment,Static.Unlock"} {
		e, cfg, id := manifestedBearEngine(t)
		restrictedMana(e, "G", 2, valid)
		e.priorityRound()
		if got := manifestTurnUpOptions(t, e, id); len(got) != 0 {
			t.Fatalf("%q mana offered the turn-up: %+v", valid, got)
		}
		// A stale option still degrades to a no-op: face down, pool kept.
		e.turnFaceUp(0, decision.Option{Kind: "turn_face_up", Obj: id, Mode: "manifest"})
		if !e.G.Obj(id).FaceDown || e.G.Players[0].Pool.Total() != 2 {
			t.Fatalf("%q mana paid a stale turn-up: faceDown=%v pool=%d", valid, e.G.Obj(id).FaceDown, e.G.Players[0].Pool.Total())
		}
		if hasNote(e, "no longer payable") {
			t.Fatalf("%q: the turn-up started and aborted instead of never being offered", valid)
		}
		replayCheck(t, e, cfg)
	}
}

func lockedRoomEngine(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := handEngine(t)
	room := cfCorpusPerm(t, e, 0, "Dazzling Theater")
	if lf := roomLockedFace(e.G.Obj(room)); lf == nil || lf.Name != "Prop Room" {
		t.Fatalf("precondition: the placed room's locked door is not Prop Room: %+v", lf)
	}
	return e, room
}

// Smoky Lounge's mana (Static.Unlock) pays the {2}{W} unlock of Prop Room.
func TestRestrictValidStaticUnlockPaysUnlock(t *testing.T) {
	t.Parallel()
	e, room := lockedRoomEngine(t)
	cfPool(e, "WWW")
	if len(optionsOfKind(e, "unlock", room)) != 1 {
		t.Fatal("precondition: plain WWW does not offer the {2}{W} unlock")
	}
	cfPool(e, "")
	restrictedMana(e, "W", 3, "Static.Unlock")
	if len(optionsOfKind(e, "unlock", room)) != 1 {
		t.Fatal("Static.Unlock mana did not offer the unlock")
	}
	e.priorityRound()
	var idx = -1
	for _, o := range e.Pending().Options {
		if o.Kind == "unlock" && o.Obj == room {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no unlock option in the priority decision: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	if !e.G.Obj(room).Unlocked {
		t.Fatal("the room is still locked after paying with Static.Unlock mana")
	}
	if left := e.G.Players[0].Pool.Total(); left != 0 {
		t.Fatalf("pool after the unlock = %d, want 0", left)
	}
}

// An activated-ability-only restriction is not an unlock: the unlock is not
// offered (it used to be offered and then silently do nothing).
func TestActivatedRestrictedManaNeverOffersUnlock(t *testing.T) {
	t.Parallel()
	for _, valid := range []string{"Activated", "Spell.Room", "Spell"} {
		e, room := lockedRoomEngine(t)
		restrictedMana(e, "W", 3, valid)
		if got := optionsOfKind(e, "unlock", room); len(got) != 0 {
			t.Fatalf("%q mana offered the unlock: %+v", valid, got)
		}
		if e.G.Obj(room).Unlocked || e.G.Players[0].Pool.Total() != 3 {
			t.Fatalf("%q: unlocked=%v pool=%d", valid, e.G.Obj(room).Unlocked, e.G.Players[0].Pool.Total())
		}
	}
}

// Foretelling is not casting: bare Spell mana is refused (it used to pay
// silently), a Static.Foretelling batch (Karfell Harbinger) pays it.
func TestSpellRestrictedManaCannotForetell(t *testing.T) {
	t.Parallel()
	foretellOffered := func(e *Engine) bool {
		for _, o := range e.legalActions(0) {
			if o.Mode == "foretell" {
				return true
			}
		}
		return false
	}
	for _, valid := range []string{"Spell", "Spell.Instant,Spell.Sorcery", "Activated"} {
		e := handEngine(t, corpusAlternativeCard(t, "Haunting Voyage"))
		restrictedMana(e, "C", 2, valid)
		if foretellOffered(e) {
			t.Fatalf("%q mana offered the foretell", valid)
		}
	}
	for _, valid := range []string{"Static.Foretelling", "Spell.Instant,Static.Foretelling,Spell.Sorcery", "Static", "nonSpell"} {
		e := handEngine(t, corpusAlternativeCard(t, "Haunting Voyage"))
		id := e.G.Zone(state.ZHand, 0)[0]
		restrictedMana(e, "C", 2, valid)
		if !foretellOffered(e) {
			t.Fatalf("%q mana did not offer the foretell", valid)
		}
		foretellIt(t, e, id)
		if left := e.G.Players[0].Pool.Total(); left != 0 {
			t.Fatalf("%q: pool after the foretell = %d, want 0", valid, left)
		}
	}
}

// nonSpell (Thran Turbine) is Forge's !isSpell: it pays cumulative upkeep and
// every other non-spell payment, never a spell.
func TestRestrictValidNonSpellPaysCumulativeUpkeep(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Haunting Voyage"))
	id := e.G.Zone(state.ZHand, 0)[0]
	restrictedMana(e, "C", 1, "nonSpell")
	one := ParseCost("1")
	classes := []struct {
		name string
		d    pay.Descriptor
		want bool
	}{
		{"cumulative upkeep", pay.Descriptor{ID: id, Class: pay.PurposeCumulativeUpkeep, Cost: &one}, true},
		{"other", pay.Descriptor{ID: id, Class: pay.PurposeOther, Cost: &one}, true},
		{"activated", pay.Descriptor{ID: id, Class: pay.PurposeActivated, Cost: &one}, true},
		{"special", pay.SpecialDescriptor(id, pay.SpecialForetell, one), true},
		{"spell", pay.Descriptor{ID: id, Class: pay.PurposeSpell, Cost: &one}, false},
	}
	for _, c := range classes {
		if got := pay.CostPayableClass(asPayer(e), 0, c.d, pipRider{}, one); got != c.want {
			t.Errorf("nonSpell mana payable for %s = %v, want %v", c.name, got, c.want)
		}
	}
	if pay.PayManaFor(asPayer(e), 0, id, false, one, nil, pipRider{}) {
		t.Fatal("nonSpell mana paid a spell")
	}
	if !pay.PayManaCumulative(asPayer(e), 0, id, one, nil) {
		t.Fatal("nonSpell mana did not pay cumulative upkeep")
	}
	if left := e.G.Players[0].Pool.Total(); left != 0 {
		t.Fatalf("pool after the cumulative upkeep = %d, want 0", left)
	}
}

// The four Standard carriers' real RestrictValid$ text, admitted and refused
// per special action, with the card filter read off the paid object.
func TestRestrictValidStandardCarriersAdmitTheirSpecials(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _, bear := manifestedBearEngine(t)
	restrictionOf := func(name string) string {
		c := searchCorpusCard(t, reg, name)
		for _, f := range c.Faces {
			for _, ab := range f.Abilities {
				if rv := effects.ManaOf(ab).RestrictValid; rv != "" {
					return rv
				}
			}
			for _, sv := range f.SVars {
				if _, rv, ok := strings.Cut(sv, "RestrictValid$"); ok {
					rv, _, _ = strings.Cut(rv, "|")
					return strings.TrimSpace(rv)
				}
			}
		}
		t.Fatalf("%s carries no RestrictValid$", name)
		return ""
	}
	one := ParseCost("1")
	spec := func(s pay.Special) pay.Descriptor { return pay.SpecialDescriptor(bear, s, one) }
	spell := pay.Descriptor{ID: bear, Class: pay.PurposeSpell, Cost: &one}
	ability := pay.Descriptor{ID: bear, Class: pay.PurposeActivated, Cost: &one}
	rows := []struct {
		card string
		d    pay.Descriptor
		want bool
	}{
		{"Overgrown Zealot", spec(pay.SpecialMorphUp), true},
		{"Overgrown Zealot", spec(pay.SpecialManifestUp), true},
		{"Overgrown Zealot", spec(pay.SpecialUnlock), false},
		{"Overgrown Zealot", spec(pay.SpecialForetell), false},
		{"Overgrown Zealot", spell, false},
		{"Overgrown Zealot", ability, false},
		{"Creeping Peeper", spec(pay.SpecialUnlock), true},
		{"Creeping Peeper", spec(pay.SpecialCloakUp), true},
		{"Creeping Peeper", spec(pay.SpecialForetell), false},
		{"Smoky Lounge // Misty Salon", spec(pay.SpecialUnlock), true},
		{"Smoky Lounge // Misty Salon", spec(pay.SpecialDisguiseUp), false},
		{"Tin Street Gossip", spec(pay.SpecialDisguiseUp), true}, // the bear is a creature
		{"Tin Street Gossip", spec(pay.SpecialUnlock), false},
		{"Karfell Harbinger", spec(pay.SpecialForetell), true},
		{"Karfell Harbinger", spec(pay.SpecialPlot), false},
		{"Qarsi Deceiver", spec(pay.SpecialManifestUp), true},
		{"Qarsi Deceiver", spec(pay.SpecialMorphUp), true},
		{"Qarsi Deceiver", spec(pay.SpecialDisguiseUp), false},
		{"Qarsi Deceiver", spec(pay.SpecialCloakUp), false},
	}
	for _, r := range rows {
		valid := restrictionOf(r.card)
		if got := pay.RestrictValidMatches(asPayer(e), 0, r.d, valid, 0); got != r.want {
			t.Errorf("%s %q admits %d/%d = %v, want %v", r.card, valid, r.d.Class, r.d.Special, got, r.want)
		}
	}
}

// Every RestrictValid$ term in the corpus names a kind the matcher handles:
// bare (dotless) or as a dotted <kind>.<filter> head. A kind it does not
// handle fails closed, silently making its mana unspendable, so a new corpus
// term must fail here rather than there.
func TestRestrictValidCorpusTermKindsAllHandled(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _, bear := manifestedBearEngine(t)
	one, cu := ParseCost("1"), ParseCost("1")
	xCost := Cost{X: 1}
	cCost := ParseCost("C")
	other := func(c *Cost) pay.Descriptor { return pay.Descriptor{ID: bear, Class: pay.PurposeOther, Cost: c} }
	// admits[kind] is a payment the bare kind (or kind. with an empty filter)
	// must admit.
	admits := map[string]pay.Descriptor{
		"Spell":                     {ID: bear, Class: pay.PurposeSpell, Cost: &one},
		"Activated":                 {ID: bear, Class: pay.PurposeActivated, Cost: &one},
		"Static":                    pay.SpecialDescriptor(bear, pay.SpecialUnlock, one),
		"nonSpell":                  other(&one),
		"CantCastNonArtifactSpells": other(&one),
		"CantCastSpellFromHand":     other(&one),
		"CostContainsX":             other(&xCost),
		"CostContainsC":             other(&cCost),
		"CantPayGenericCosts":       other(&Cost{}),
		"CumulativeUpkeep":          {ID: bear, Class: pay.PurposeCumulativeUpkeep, Cost: &cu},
	}
	seen := map[string]string{} // term kind -> first card naming it
	text := func(card *cards.Card, s string) {
		for {
			_, rest, ok := strings.Cut(s, "RestrictValid$")
			if !ok {
				return
			}
			val, tail, _ := strings.Cut(rest, "|")
			for term := range strings.SplitSeq(strings.TrimSpace(val), ",") {
				kind, _, _ := strings.Cut(strings.TrimSpace(term), ".")
				if _, ok := seen[kind]; !ok {
					seen[kind] = card.Faces[0].Name + " " + term
				}
			}
			s = tail
		}
	}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, ab := range f.Abilities {
				text(c, ab.Line)
			}
			for _, sv := range f.SVars {
				text(c, sv)
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no RestrictValid$ term found in the corpus: the census read nothing")
	}
	kinds := make([]string, 0, len(seen))
	for k := range seen {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		d, ok := admits[kind]
		if !ok {
			t.Errorf("corpus term kind %q (%s) has no handled-kind row", kind, seen[kind])
			continue
		}
		// The three kinds that head a dotted <kind>.<filter> term are probed
		// with an empty filter; the rest are bare words.
		probe := kind
		if kind == "Spell" || kind == "Activated" || kind == "Static" {
			probe += "."
		}
		if !pay.RestrictValidMatches(asPayer(e), 0, d, probe, 0) {
			t.Errorf("corpus term kind %q (%s): the matcher does not admit %q for class %d", kind, seen[kind], probe, d.Class)
		}
	}
}

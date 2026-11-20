package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The .OtherShape gates these tests widen. Each behaviour test names the
// card family's implemented shape; the census is the class record of which
// corpus cards are playable and which stay gated, each with the mechanism it
// would need.

const (
	secondAuraSrc  = "Name:Test Second Aura\nManaCost:1 U\nTypes:Enchantment Aura\nK:Enchant:Land\nOracle:x\n"
	osLandSrc      = "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"
	creatureAura   = "Name:Test Creature Aura\nManaCost:1 U\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"
	artifactAura   = "Name:Test Artifact Aura\nManaCost:1 U\nTypes:Enchantment Aura\nK:Enchant:Artifact\nOracle:x\n"
	testArtifact   = "Name:Test Bauble\nManaCost:2\nTypes:Artifact\nOracle:x\n"
	redInstantSrc  = "Name:Test Red Bolt\nManaCost:1 R\nTypes:Instant\nA:SP$ DealDamage | NumDmg$ 1 | ValidTgts$ Player | SpellDescription$ Zap.\nOracle:x\n"
	testVanillaCre = "Name:Test Vanilla\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n"
)

// craftOption returns the stalactite's Craft activation option at seat 0's
// re-posed priority decision, if the expander installed one.
func craftOption(t *testing.T, e *Engine, id state.ObjID) (decision.Option, bool) {
	t.Helper()
	ppPriority(t, e)
	for _, o := range e.Pending().Options {
		if o.Obj == id && (o.Kind == "ability" || o.Kind == "activate") && strings.Contains(o.Label, "Craft") {
			return o, true
		}
	}
	return decision.Option{}, false
}

// TestOreRichStalactiteCraftShape: the graveyard-only material head
// (ExileFromGrave<X/...> beside XMin4) is the same announced-X craft the
// Enigma Jewel family already pays; it must install the CR 702.167a
// activation, exile four or more red instants/sorceries from the graveyard
// and return the card transformed (Cosmium Catalyst).
func TestOreRichStalactiteCraftShape(t *testing.T) {
	stalactite := corpusCardNamed(t, "Ore-Rich Stalactite")
	seat0 := []*cards.Card{stalactite}
	for range 4 {
		seat0 = append(seat0, card(t, redInstantSrc))
	}
	e, cfg := tokenReplGame(t, 7712, seat0...)
	id := moveSeededCard(t, e, 0, stalactite, state.ZBattlefield)
	var bolts []state.ObjID
	for i := 0; i < 4; i++ {
		bolts = append(bolts, moveSeededCard(t, e, 0, seat0[i+1], state.ZGraveyard))
	}
	addMana(t, e, 0, "CCCCCCCCRR") // {3}{R}{R} + X=4: seven generic (the floor) and the two red pips
	o, ok := craftOption(t, e, id)
	if !ok {
		t.Fatalf("craft activation not offered; options %+v", e.Pending().Options)
	}
	d := e.Pending()
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
		t.Fatal(err)
	}
	// The craft ask: announce X (the floor is 4) and pick the four materials.
	// Every choose the flow poses takes its minimum count; a priority hand-off
	// is passed so the resolving craft ability runs -- until the card is back
	// on the battlefield transformed.
	for i := 0; i < 60; i++ {
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield && o.FaceIdx == 1 {
			break
		}
		d = e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KPriority {
			passPriorityOnce(t, e)
			continue
		}
		if len(d.Options) == 0 {
			t.Fatalf("craft flow stuck at %+v with no options", d)
		}
		// A multi-pick ask (the four material exiles) takes its minimum
		// count; single-pick asks take option 0.
		n := d.Min
		if n < 1 || n > len(d.Options) {
			n = 1
		}
		choices := make([]int, 0, n)
		for i := 0; i < n; i++ {
			choices = append(choices, d.Options[i].Index)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
			t.Fatalf("craft flow ask %+v: %v", d, err)
		}
	}
	if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield && o.Face() != nil &&
		o.Face().Name == "Cosmium Catalyst" {
		// entered transformed
	} else {
		t.Fatalf("stalactite did not return transformed; obj=%+v", e.G.Obj(id))
	}
	exiled := 0
	for _, b := range bolts {
		if e.G.Obj(b).Zone == state.ZExile {
			exiled++
		}
	}
	if exiled != 4 {
		t.Fatalf("exiled %d of the 4 red instants, want 4", exiled)
	}
	replayCheck(t, e, cfg)
}

// castOptionIndex finds the cast option for id in the pending priority
// decision, -1 when the cast is not offered.
func castOptionIndex(e *Engine, id state.ObjID) int {
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			return o.Index
		}
	}
	return -1
}

// auraCastAvoiding casts an Aura card in hand at expect's ask: the target ask
// must not offer forbidden (0 skips the check; the CR 601.2c census a
// CantAttach Aura static withholds the protected bearer from), expect is
// chosen, and the Aura resolves attached to it.
func auraCastAvoiding(t *testing.T, e *Engine, id, forbidden, expect state.ObjID) {
	t.Helper()
	ppPriority(t, e)
	idx := castOptionIndex(e, id)
	if idx < 0 {
		t.Fatalf("aura cast not offered; options %+v", e.Pending().Options)
	}
	d := e.Pending()
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatal(err)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target ask pending: %+v", d)
	}
	choice := -1
	var offered []state.ObjID
	for _, o := range d.Options {
		if o.Obj != 0 {
			offered = append(offered, o.Obj)
		}
		if o.Obj == expect {
			choice = o.Index
		}
		if forbidden != 0 && o.Obj == forbidden {
			t.Fatalf("forbidden bearer %d still offered as a target: %v", forbidden, offered)
		}
	}
	if choice < 0 {
		t.Fatalf("expected target %d not offered: %v", expect, offered)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{choice}}); err != nil {
		t.Fatal(err)
	}
	resolveSpell(t, e, id)
	if z, att := e.G.Obj(id).Zone, e.G.Obj(id).AttachedTo; z != state.ZBattlefield || att != expect {
		t.Fatalf("aura not attached to %d: zone=%s attachedTo=%d", expect, z, att)
	}
}

// TestConsecrateLandCantBeEnchantedByOthers: the Aura-other CantAttach shape
// -- another Aura spell cannot target the land Consecrate Land enchants.
func TestConsecrateLandCantBeEnchantedByOthers(t *testing.T) {
	consecrate := corpusCardNamed(t, "Consecrate Land")
	second := card(t, secondAuraSrc)
	lands := [](*cards.Card){card(t, osLandSrc), card(t, osLandSrc)}
	e, cfg := tokenReplGame(t, 7713, append([]*cards.Card{consecrate, second}, lands...)...)
	land := moveSeededCard(t, e, 0, lands[0], state.ZBattlefield)
	id := castAuraOnto(t, e, consecrate, land, "W")
	if got := e.G.Obj(id).AttachedTo; got != land {
		t.Fatalf("consecrate attached to %d, want %d", got, land)
	}
	id2 := moveSeededCard(t, e, 0, second, state.ZHand)
	addMana(t, e, 0, "UU")
	other := moveSeededCard(t, e, 0, lands[1], state.ZBattlefield)
	auraCastAvoiding(t, e, id2, land, other)
	replayCheck(t, e, cfg)
}

// TestAntiMagicAuraCantBeEnchantedByOthers: the Aura-other shape on a
// creature bearer. The spell route is closed by Anti-Magic Aura's own
// CantTarget static ("can't be the target of spells"), so the gate is
// exercised the CR 303.4g way: a non-cast Aura entry with no legal object to
// enchant stays in its zone.
func TestAntiMagicAuraCantBeEnchantedByOthers(t *testing.T) {
	antimagic := corpusCardNamed(t, "Anti-Magic Aura")
	second := card(t, creatureAura)
	e, cfg := tokenReplGame(t, 7714, antimagic, second, card(t, testVanillaCre))
	cre := putCreature(t, e, 0, testVanillaCre)
	id := castAuraOnto(t, e, antimagic, cre, "UUU")
	if got := e.G.Obj(id).AttachedTo; got != cre {
		t.Fatalf("anti-magic aura attached to %d, want %d", got, cre)
	}
	id2 := moveSeededCard(t, e, 0, second, state.ZHand)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id2, From: state.ZHand, To: state.ZBattlefield})
	if z := e.G.Obj(id2).Zone; z != state.ZHand {
		t.Fatalf("second Aura entered attached to the protected creature (zone %s); want the entry swallowed", z)
	}
	replayCheck(t, e, cfg)
}

// TestGuardianBeastCantEnchantUntappedArtifacts: Guardian Beast's untapped
// IsPresent$ shape -- while it is untapped a noncreature artifact you control
// cannot be enchanted, a tapped Guardian Beast leaves it open, and an Aura
// already attached is NOT swept (the printed "This effect doesn't remove
// Auras already attached").
func TestGuardianBeastCantEnchantUntappedArtifacts(t *testing.T) {
	beast := corpusCardNamed(t, "Guardian Beast")
	aura := card(t, artifactAura)
	aura2 := card(t, artifactAura)
	baubleCard := card(t, testArtifact)
	e, cfg := tokenReplGame(t, 7715, beast, aura, aura2, baubleCard)
	bauble := moveSeededCard(t, e, 0, baubleCard, state.ZBattlefield)
	id := moveSeededCard(t, e, 0, beast, state.ZBattlefield)
	if e.G.Obj(id).Tapped {
		t.Fatal("guardian beast seeded tapped")
	}
	// While tapped the artifact is enchantable: attach the first Aura.
	e.emit(events.Event{Kind: events.Tap, Obj: id})
	attached := castAuraOnto(t, e, aura, bauble, "UU")
	// Untap: the static is live again, but the attached Aura stays put.
	e.emit(events.Event{Kind: events.Untap, Obj: id})
	if got := e.G.Obj(attached).AttachedTo; got != bauble {
		t.Fatalf("already-attached Aura %d was swept on untap: attachedTo=%d", attached, got)
	}
	id3 := moveSeededCard(t, e, 0, aura2, state.ZHand)
	addMana(t, e, 0, "UU")
	ppPriority(t, e)
	if idx := castOptionIndex(e, id3); idx >= 0 {
		t.Fatal("artifact Aura cast offered while Guardian Beast is untapped: no legal target exists")
	}
	// Tap the beast again: the artifact is enchantable and a second Aura may
	// join the first.
	e.emit(events.Event{Kind: events.Tap, Obj: id})
	addMana(t, e, 0, "UU")
	auraCastAvoiding(t, e, id3, 0, bauble)
	replayCheck(t, e, cfg)
}

// castAuraOnto casts an Aura card from hand onto bearer (answering the target
// ask with the bearer's option) and returns the Aura's id once attached.
func castAuraOnto(t *testing.T, e *Engine, c *cards.Card, bearer state.ObjID, mana string) state.ObjID {
	t.Helper()
	id := moveSeededCard(t, e, 0, c, state.ZHand)
	addMana(t, e, 0, mana)
	ppPriority(t, e)
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("aura %s cast not offered", c.Faces[0].Name)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatal(err)
	}
	d = e.Pending()
	choice := -1
	for _, o := range d.Options {
		if o.Obj == bearer {
			choice = o.Index
		}
	}
	if choice < 0 {
		t.Fatalf("bearer %d not offered as an Aura target: %+v", bearer, d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{choice}}); err != nil {
		t.Fatal(err)
	}
	resolveSpell(t, e, id)
	if e.G.Obj(id).Zone != state.ZBattlefield || e.G.Obj(id).AttachedTo != bearer {
		t.Fatalf("aura not attached: zone=%v attachedTo=%d", e.G.Obj(id).Zone, e.G.Obj(id).AttachedTo)
	}
	return id
}

// resolveSpell passes priority (whoever holds it) and answers chooses with
// option 0 until the spell id leaves the stack.
func resolveSpell(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZStack {
			return
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("spell stuck on the stack with nothing pending")
		}
		if d.Kind == decision.KPriority {
			passPriorityOnce(t, e)
			continue
		}
		if len(d.Options) == 0 {
			t.Fatalf("stuck at %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("spell still on the stack after 40 rounds")
}

// TestBendOrBreakTwoPiles: the per-player battlefield land pile -- each
// player separates their nontoken lands into two piles, an opponent of their
// choice picks one, the chosen piles are destroyed and the others tapped.
// Every land ends the spell either destroyed or tapped.
func TestBendOrBreakTwoPiles(t *testing.T) {
	bend := corpusCardNamed(t, "Bend or Break")
	lands0c := [](*cards.Card){card(t, osLandSrc), card(t, osLandSrc), card(t, osLandSrc)}
	lands1c := [](*cards.Card){card(t, osLandSrc), card(t, osLandSrc)}
	e, cfg := tokenReplGameSeats(t, 7716, append([]*cards.Card{bend}, lands0c...), lands1c)
	lands0 := []state.ObjID{
		moveSeededCard(t, e, 0, lands0c[0], state.ZBattlefield),
		moveSeededCard(t, e, 0, lands0c[1], state.ZBattlefield),
		moveSeededCard(t, e, 0, lands0c[2], state.ZBattlefield),
	}
	lands1 := []state.ObjID{
		moveSeededCard(t, e, 1, lands1c[0], state.ZBattlefield),
		moveSeededCard(t, e, 1, lands1c[1], state.ZBattlefield),
	}
	id := moveSeededCard(t, e, 0, bend, state.ZHand)
	addMana(t, e, 0, "RRRR")
	ppPriority(t, e)
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("bend or break cast not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatal(err)
	}
	resolveSpell(t, e, id)
	count := func(lands []state.ObjID, dead bool) int {
		n := 0
		for _, l := range lands {
			o := e.G.Obj(l)
			if (o.Zone == state.ZGraveyard) == dead {
				n++
			}
		}
		return n
	}
	for seat, lands := range map[int][]state.ObjID{0: lands0, 1: lands1} {
		if got := len(lands) - count(lands, false); got < 1 {
			t.Errorf("seat %d: no pile destroyed (%d/%d remain)", seat, count(lands, false), len(lands))
		}
		for _, l := range lands {
			o := e.G.Obj(l)
			if o.Zone == state.ZBattlefield && !o.Tapped {
				t.Errorf("seat %d land %d neither destroyed nor tapped", seat, l)
			}
		}
	}
	replayCheck(t, e, cfg)
}

// TestDeathOrGloryTwoPiles: the graveyard creature-card pile -- the creatures
// split into two piles, the opponent exiles one and the other returns to the
// battlefield; every creature card is accounted for.
func TestDeathOrGloryTwoPiles(t *testing.T) {
	dog := corpusCardNamed(t, "Death or Glory")
	e, cfg := tokenReplGame(t, 7717, dog, card(t, testVanillaCre), card(t, testVanillaCre))
	cre0 := addToGraveyard(t, e, 0, testVanillaCre)
	cre1 := addToGraveyard(t, e, 0, testVanillaCre)
	id := moveSeededCard(t, e, 0, dog, state.ZHand)
	addMana(t, e, 0, "WWWWW")
	ppPriority(t, e)
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("death or glory cast not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatal(err)
	}
	resolveSpell(t, e, id)
	sites := map[string]bool{}
	for _, cid := range []state.ObjID{cre0, cre1} {
		sites[string(e.G.Obj(cid).Zone)] = true
	}
	if len(sites) != 2 {
		t.Fatalf("both creatures ended in one zone: %v (%v, %v)", sites,
			e.G.Obj(cre0).Zone, e.G.Obj(cre1).Zone)
	}
	replayCheck(t, e, cfg)
}

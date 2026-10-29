package mapping_test

// Regression tests for the sbp-unmapped-edge-cases refusals: the
// duplicate_candidates shape (one ability, two payable costs), the repeatable
// modal ask beyond its printed mode count, an arrange over another player's
// library, a Dig whose remainder goes back on top, and a max-speed granted
// priority action.

import (
	"errors"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
)

// cardGame builds a 60-card deck of four copies of each wanted card and the
// rest basic lands and plays bots until pred holds. It scans a few secrets so
// a deck whose card is not cast in one opening hand still reaches the board.
func cardGame(t *testing.T, wanted []string, landName string, secretByte byte, pred func(*rules.Engine) bool) *gamecfg.Game {
	t.Helper()
	reg := testcorpus.Registry(t)
	land, ok := reg.Lookup(landName)
	if !ok {
		t.Fatalf("no basic land named %q", landName)
	}
	deck := make([]*cards.Card, 0, 60)
	for _, name := range wanted {
		card, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("no card named %q", name)
		}
		for i := 0; i < 4; i++ {
			deck = append(deck, card)
		}
	}
	for len(deck) < 60 {
		deck = append(deck, land)
	}
	for s := secretByte; s < secretByte+20; s++ {
		secret := make([]byte, 32)
		secret[0] = s
		g, err := gamecfg.New(reg, secrets.NewGame(secret), [2][]*cards.Card{deck, deck},
			gamecfg.Rules{Mulligan: "none", StartingSeat: state.PlayerID(0)})
		if err != nil {
			t.Fatal(err)
		}
		if testgame.RunUntil(t, g, testgame.Bots(uint64(s)), pred, 30000) {
			return g
		}
	}
	t.Fatalf("no game reached the wanted board for %v", wanted)
	return nil
}

// namedOnBoard returns the id of name on seat's battlefield, or 0.
func namedOnBoard(e *rules.Engine, seat state.PlayerID, name string) state.ObjID {
	for _, id := range e.G.Zone(state.ZBattlefield, seat) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	return 0
}

// TestDuplicateCandidatesDisambiguated covers the Gavel of the Righteous
// shape: one printed equip ability offered at its own cost and at its
// AlternateCost$ rider. Two engine options project to one
// {source, ability_index} pair, because v2's activate_ability names the
// ability, not the cost. They must collapse to one validator-clean candidate
// that still commits a native option (the printed one first).
func TestDuplicateCandidatesDisambiguated(t *testing.T) {
	seat := state.PlayerID(0)
	g := cardGame(t, []string{"Gavel of the Righteous", "Memnite"}, "Plains", 41,
		func(e *rules.Engine) bool {
			d := e.Pending()
			return d != nil && d.Kind == decision.KPriority &&
				namedOnBoard(e, seat, "Gavel of the Righteous") != 0 &&
				namedOnBoard(e, seat, "Memnite") != 0
		})
	gavel := namedOnBoard(g.E, seat, "Gavel of the Righteous")
	if gavel == 0 {
		t.Fatal("precondition: Gavel of the Righteous is not on the battlefield")
	}
	observed := rules.ObservedAbilities(g.E, seat, gavel)
	var printed decision.Option
	for _, o := range observed {
		if o.Ability >= 0 {
			printed = o
			break
		}
	}
	if printed.Ability < 0 || printed.Obj != gavel {
		t.Fatalf("precondition: no printed activated ability observed on Gavel: %+v", observed)
	}
	alt := printed
	alt.AltCostIndex = 1
	if printed.AltCostIndex != 0 || alt.AltCostIndex == printed.AltCostIndex {
		t.Fatalf("precondition: the two variants do not differ in AltCostIndex: %+v / %+v", printed, alt)
	}

	env := envFor(t, g)
	d := synthetic(g.E.Pending(), printed, alt)
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("Pose (old code fails here with ErrDuplicate): %v", err)
	}
	var acts []mapping.Cand
	for _, c := range p.Candidates {
		if c.Sem.Kind == "activate_ability" && sourceIs(c, "Gavel of the Righteous") {
			acts = append(acts, c)
		}
	}
	if len(acts) != 1 {
		t.Fatalf("posed %d activate_ability candidates for one ability, want 1: %s", len(acts), semJSON(p.Candidates))
	}
	if err := acts[0].Sem.Check(); err != nil {
		t.Fatalf("collapsed candidate fails the protocol shape: %v", err)
	}
	if acts[0].Op.Option != printed.Index {
		t.Fatalf("collapsed candidate commits native option %d, want the printed variant %d", acts[0].Op.Option, printed.Index)
	}
}

// sourceIs reports whether a candidate's activate_ability source names card.
func sourceIs(c mapping.Cand, card string) bool {
	src, ok := c.Sem.Fields["source"].(protocol.ObjectRef)
	return ok && src.CardName != nil && *src.CardName == card
}

// TestRepeatableModesOverPrinted pins the documented refusal: Season of Loss
// (CharmNum$ 5 over three printed modes) wants five repeatable picks, but
// Section 7.3's choose_spell_mode requires minimum <= maximum <= mode_count.
// v2 has no repeat field, so the mapping declines rather than posing
// candidates every validator rejects. The test also checks the validator
// itself rejects the forwarded 5-over-3 pair.
func TestRepeatableModesOverPrinted(t *testing.T) {
	g := repeatableCharmGame(t, "Season of Loss", "Swamp", 41)
	d := g.E.Pending()
	if d.Kind != decision.KModes || !d.Repeatable {
		t.Fatalf("ask %+v, want a repeatable modal decision", d)
	}
	if d.Min != 0 || d.Max != 5 || len(d.Options) != 3 {
		t.Fatalf("ask min %d max %d options %d, want 0/5/3", d.Min, d.Max, len(d.Options))
	}
	env := envFor(t, g)
	src, err := mapping.MustSource(env, d)
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.ChooseSpellMode(src, 0, 3, 0, 0, 5).Check(); err == nil ||
		!strings.Contains(err.Error(), "maximum 5") {
		t.Fatalf("ChooseSpellMode(0 of 3, max 5).Check() = %v, want a maximum 5 rejection", err)
	}
	_, err = mapping.Begin(env, d)
	if !errors.Is(err, mapping.ErrUnmapped) {
		t.Fatalf("Begin = %v, want ErrUnmapped", err)
	}
	if !strings.Contains(err.Error(), "modes_over_printed(min=0,max=5,printed=3,repeat=true)") {
		t.Fatalf("Begin error %q, want the documented modes_over_printed refusal", err)
	}
}

// TestArrangeOverTargetLibraryLooksAtOwner covers Natural Selection's
// "target player's library": the ask's Player is the chooser while the
// offered cards belong to the target. The mapping must read the cards'
// owner's library, not the chooser's.
func TestArrangeOverTargetLibraryLooksAtOwner(t *testing.T) {
	g := untilPending(t, "Wildfire", 1, func(d *decision.Decision, e *rules.Engine) bool {
		return d.Kind == decision.KPriority
	})
	chooser := state.PlayerID(0)
	owner := state.PlayerID(1)
	lib := g.E.G.Zone(state.ZLibrary, owner)
	if len(lib) < 2 {
		t.Fatalf("precondition: the target library has %d cards, want at least 2", len(lib))
	}
	env := envFor(t, g)
	d := &decision.Decision{Kind: decision.KArrange, Player: chooser, Min: 0, Max: 2, Restable: true,
		Options: []decision.Option{
			{Kind: "bottom", Obj: lib[0], Index: 0, Player: owner},
			{Kind: "bottom", Obj: lib[1], Index: 1, Player: owner},
		}}
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("Begin (old code fails here with looked_at_card): %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("Pose: %v", err)
	}
	n := 0
	for _, c := range p.Candidates {
		if c.Sem.Kind == "arrange_card" {
			n++
			if err := c.Sem.Check(); err != nil {
				t.Fatalf("arrange_card candidate fails the protocol shape: %v", err)
			}
		}
	}
	if n == 0 {
		t.Fatalf("no arrange_card candidates: %s", semJSON(p.Candidates))
	}
}

// TestDigRestOnTopMaps covers Mobile Homestead's
// DestinationZone2$ Library | LibraryPosition2$ 0: the remainder goes back on
// top in a chosen order, not to the bottom. newDig must map it as an
// arrangement whose rest destination is "top".
func TestDigRestOnTopMaps(t *testing.T) {
	g := untilPending(t, "Wildfire", 1, func(d *decision.Decision, e *rules.Engine) bool {
		return d.Kind == decision.KPriority
	})
	seat := state.PlayerID(0)
	lib := g.E.G.Zone(state.ZLibrary, seat)
	if len(lib) < 2 {
		t.Fatalf("precondition: the library has %d cards, want at least 2", len(lib))
	}
	d := &decision.Decision{Kind: decision.KChoose, Player: seat, Min: 1, Max: 1,
		Prompt: "Look at the top 2 card(s) of your library",
		ResumeSA: &cards.SA{Kind: "SP", API: "Dig", Params: map[string]string{
			"DigNum": "2", "ChangeNum": "1", "DestinationZone": "Battlefield",
			"DestinationZone2": "Library", "LibraryPosition2": "0", "ChangeValid": "Card.Land"}},
		Options: []decision.Option{
			{Kind: "dig", Obj: lib[0], Index: 0, Player: seat},
			{Kind: "dig", Obj: lib[1], Index: 1, Player: seat},
		}}
	env := envFor(t, g)
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("Begin (old code fails here with dig_shape): %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("Pose: %v", err)
	}
	top := 0
	for _, c := range p.Candidates {
		if c.Sem.Kind != "arrange_card" {
			continue
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("arrange_card candidate fails the protocol shape: %v", err)
		}
		if c.Sem.Fields["destination"] == "top" {
			top++
		}
	}
	if top == 0 {
		t.Fatalf("no top destination candidate for a rest-on-top Dig: %s", semJSON(p.Candidates))
	}
}

// TestMaxSpeedGrantedMapsToIndexedActivation covers a max-speed static's
// "granted" priority option (Deviant Skytech). The inventory must list the
// static's AddAbility$ so the option gets an object-stable ability_index, and
// the option's SVar anchor (which carries no GrantSource) must match it.
func TestMaxSpeedGrantedMapsToIndexedActivation(t *testing.T) {
	seat := state.PlayerID(0)
	g := cardGame(t, []string{"Deviant Skytech"}, "Island", 41,
		func(e *rules.Engine) bool {
			id := namedOnBoard(e, seat, "Deviant Skytech")
			if id == 0 {
				return false
			}
			if e.G.Players[seat].Speed < 4 {
				e.Emit(events.Event{Kind: events.SpeedChange, Player: seat, Amount: 4 - e.G.Players[seat].Speed})
				return false
			}
			d := e.Pending()
			if d == nil || d.Kind != decision.KPriority {
				return false
			}
			for _, o := range d.Options {
				if o.Kind == "granted" && o.Obj == id {
					return true
				}
			}
			return false
		})
	id := namedOnBoard(g.E, seat, "Deviant Skytech")
	if id == 0 || g.E.G.Players[seat].Speed != 4 {
		t.Fatalf("precondition: Deviant Skytech %d at speed %d", id, g.E.G.Players[seat].Speed)
	}
	d0 := g.E.Pending()
	var granted decision.Option
	for _, o := range d0.Options {
		if o.Kind == "granted" && o.Obj == id {
			granted = o
		}
	}
	if granted.SVar == "" {
		t.Fatalf("precondition: no granted option on Deviant Skytech: %+v", d0.Options)
	}
	observed := rules.ObservedAbilities(g.E, seat, id)
	want := -1
	for i, o := range observed {
		if o.SVar == granted.SVar && o.GrantSource == id {
			want = i
		}
	}
	if want < 0 {
		t.Fatalf("precondition: the inventory lacks %q on Deviant Skytech: %+v", granted.SVar, observed)
	}
	env := envFor(t, g)
	d := synthetic(d0, granted)
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("Pose (old code fails here with ability/grant_not_observed): %v", err)
	}
	for _, c := range p.Candidates {
		if c.Sem.Kind != "activate_ability" || !sourceIs(c, "Deviant Skytech") {
			continue
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("activate_ability candidate fails the protocol shape: %v", err)
		}
		if got := c.Sem.Fields["ability_index"]; got != uint32(want) {
			t.Fatalf("ability_index %v, want the observed position %d", got, want)
		}
		return
	}
	t.Fatalf("no activate_ability candidate for the granted option: %s", semJSON(p.Candidates))
}

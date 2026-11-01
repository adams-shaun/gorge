package view

// Reference: Magic: The Gathering Comprehensive Rules, 2026-08-07.
// CR 106.4a/106.4b: a player announces what mana is in their pool, which is
// public information (see engine/view/cr106_pool_conformance_test.go for the full
// citation block). A spend restriction on that mana is derived from a public
// battlefield permanent's own ability, so it is public in exactly the same
// way -- the seat needs it to understand why a floating symbol was refused.
// CR 605: a mana ability's restriction ("spend this mana only to ...") binds
// the mana, not the pool display.
//
// This file pins the projection behind a Cavern of Souls report: the engine
// refuses to offer a cast against a {B} restricted to the chosen creature type,
// so the seat's view must show the restriction, not a bare chip.

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// restrictionBoard builds a 2-seat game whose seat 0 has a Cavern-of-Souls
// stand-in on the battlefield with a recorded ChosenType, one restricted {B}
// batch naming it, and one unrestricted {B} batch. The unrestricted batch is
// the control: it must NOT produce an annotation (its Valid is empty), which
// is what makes the "one entry per restricted batch" assertion meaningful
// rather than "one entry per pool entry".
func restrictionBoard(t *testing.T) (*state.Game, state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"alice", "bob"})
	c, err := cards.ParseBytes("cavern.txt", []byte("Name:Cavern of Souls\nTypes:Land\nOracle:x\n"))
	if err != nil {
		t.Fatalf("parse card: %v", err)
	}
	c.Link()
	o := g.AddObject(c, 0)
	o.Zone = state.ZBattlefield
	o.ChosenType = "Demon"
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})

	// The pool holds one restricted {B} (Cavern's own) and one bare {B}
	// (another source), so the projection must distinguish them.
	g.Players[0].Pool[state.MB] = 2
	g.Players[0].RestrictedMana = []state.ManaRestriction{
		{Color: "B", Amount: 1, Valid: "Spell.Creature+ChosenType", NoCounter: "True", Source: o.ID},
		{Color: "B", Amount: 1},
	}
	return g, o.ID
}

// TestRestrictedManaIsProjected pins the projection of a restricted batch
// onto the wire. Each sub-case asserts its own precondition first: the batch
// really sits in state (so a projection that reads a different field cannot
// pass), and the produced text really differs from the raw-Valid fallback
// (so "the field is present but still raw" cannot pass).
func TestRestrictedManaIsProjected(t *testing.T) {
	g, src := restrictionBoard(t)

	// Precondition (1): the restricted batch is really in state, with the
	// exact shape this snapshot recorded, and the source carries Demon.
	if len(g.Players[0].RestrictedMana) != 2 {
		t.Fatalf("precondition: seat 0 holds %d restricted batches, want 2", len(g.Players[0].RestrictedMana))
	}
	if got := g.Players[0].RestrictedMana[0].Valid; got != "Spell.Creature+ChosenType" {
		t.Fatalf("precondition: batch[0].Valid = %q, want Spell.Creature+ChosenType", got)
	}
	if o := g.Obj(src); o == nil || o.ChosenType != "Demon" {
		t.Fatalf("precondition: source %d has no ChosenType Demon", src)
	}

	v := Project(g, flatChars{g}, 0, nil)
	var pv PlayerView
	for _, p := range v.Players {
		if p.ID == 0 {
			pv = p
		}
	}
	if len(pv.PoolRestrictions) != 1 {
		t.Fatalf("seat 0 projects %d pool restrictions, want 1 (the bare {B} carries no limit): %+v", len(pv.PoolRestrictions), pv.PoolRestrictions)
	}
	got := pv.PoolRestrictions[0]

	// Precondition (2): the text is NOT the raw Valid$ string, so a
	// formatter that silently fell through to the raw fallback fails here.
	if got.Text == "Spell.Creature+ChosenType" {
		t.Fatalf("precondition: the projection fell through to the raw Valid$ string instead of humanising it")
	}
	want := "spend only to cast a Demon creature spell"
	if got.Text != want {
		t.Fatalf("restricted {B} text = %q, want %q", got.Text, want)
	}
	if got.Color != "B" || got.Amount != 1 {
		t.Fatalf("restricted batch = %+v, want color B amount 1", got)
	}

	// The plain Spell.Creature shape must read naturally too, and the
	// source-relative resolution must not leak a chosen type into it.
	g2 := state.NewGame([]string{"alice", "bob"})
	g2.Players[0].Pool[state.MB] = 1
	g2.Players[0].RestrictedMana = []state.ManaRestriction{{Color: "B", Amount: 1, Valid: "Spell.Creature"}}
	v2 := Project(g2, flatChars{g2}, 0, nil)
	var pv2 PlayerView
	for _, p := range v2.Players {
		if p.ID == 0 {
			pv2 = p
		}
	}
	if len(pv2.PoolRestrictions) != 1 {
		t.Fatalf("seat 0 projects %d pool restrictions, want 1", len(pv2.PoolRestrictions))
	}
	if want := "spend only to cast a creature spell"; pv2.PoolRestrictions[0].Text != want {
		t.Fatalf("Spell.Creature text = %q, want %q", pv2.PoolRestrictions[0].Text, want)
	}
}

// TestPoolRestrictionsOmittedWhenEmpty pins that a seat with no restricted
// mana omits the field entirely (the Available omitempty convention), so
// every existing view serialises byte-identically.
func TestPoolRestrictionsOmittedWhenEmpty(t *testing.T) {
	g := state.NewGame([]string{"alice", "bob"})
	g.Players[0].Pool[state.MR] = 1
	v := Project(g, flatChars{g}, 0, nil)
	blob, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(blob); strings.Contains(got, `"pool_restrictions"`) {
		t.Fatalf("a seat with no restricted mana still serialises a pool_restrictions key: %s", got)
	}
	for _, pv := range v.Players {
		if pv.PoolRestrictions != nil {
			t.Fatalf("seat %d carries a non-nil PoolRestrictions with no restricted mana: %+v", pv.ID, pv.PoolRestrictions)
		}
	}
}

// TestRestrictedManaUnknownShapeReadsPlainly pins the fallback: a shape the
// formatter does not model reads as a generic sentence, never as the script
// string, and one unknown alternative falls the whole restriction back so
// the known half does not read as all of it.
func TestRestrictedManaUnknownShapeReadsPlainly(t *testing.T) {
	for _, valid := range []string{"Spell.someNewPredicate", "Spell.Creature,Spell.someNewPredicate", "someNewToken"} {
		if got := humanizeRestrictValid(nil, 0, valid); got != restrictionFallback {
			t.Errorf("humanizeRestrictValid(%q) = %q, want the fallback", valid, got)
		}
	}
}

// TestRestrictionWording pins the sentence for each family of script shapes.
func TestRestrictionWording(t *testing.T) {
	for _, tc := range []struct{ valid, want string }{
		{"Spell.Instant,Spell.Sorcery", "spend only to cast an instant spell or cast a sorcery spell"},
		{"Spell.Dragon,Activated.Dragon+inZoneBattlefield", "spend only to cast a Dragon spell or activate a Dragon ability"},
		{"Spell.Time Lord", "spend only to cast a Time Lord spell"},
		{"Spell.Creature+Dragon", "spend only to cast a Dragon creature spell"},
		{"Spell.nonCreature", "spend only to cast a noncreature spell"},
		{"Spell.MultiColor", "spend only to cast a multicolored spell"},
		{"Spell.cmcGE4", "spend only to cast a spell with mana value 4 or greater"},
		{"Spell.nonColorless+!hasXCost", "spend only to cast a colored spell without {X} in its cost"},
		{"Spell.wasCastFromExile", "spend only to cast a spell from exile"},
		{"Spell.IsCommander+YouOwn", "spend only to cast a commander spell you own"},
		{"Spell.Equipment,Activated.Equip", "spend only to cast an Equipment spell or equip"},
		{"Static.Unlock,Spell.Room", "spend only to unlock a door or cast a Room spell"},
		{"CumulativeUpkeep", "spend only to pay cumulative upkeep"},
		{"CantPayGenericCosts", "can't be spent on generic costs"},
	} {
		if got := humanizeRestrictValid(nil, 0, tc.valid); got != tc.want {
			t.Errorf("humanizeRestrictValid(%q) = %q, want %q", tc.valid, got, tc.want)
		}
	}
}

// TestEveryCorpusRestrictionReadsAsPlainWords walks every RestrictValid$ in the
// card corpus: each must have a sentence of its own. A new shape fails here
// rather than reaching a player as the generic fallback.
func TestEveryCorpusRestrictionReadsAsPlainWords(t *testing.T) {
	root := cards.CorpusDir(testutil.CorpusDirOrSkip(t))
	param := regexp.MustCompile(`RestrictValid\$ ([^|\n]*)`)
	shapes := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range param.FindAllSubmatch(text, -1) {
			valid := strings.TrimSpace(string(m[1]))
			shapes++
			if _, ok := restrictSentence(nil, 0, valid); !ok {
				t.Errorf("%s: no sentence for RestrictValid$ %q", filepath.Base(path), valid)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if shapes == 0 {
		t.Fatal("found no RestrictValid$ in the corpus")
	}
}

// TestPoolRestrictionIsPublicForEveryViewer pins that, like Pool, the
// restriction is projected for every seat under every visibility: it is
// derived from a public battlefield permanent's ability, and the player who
// needs it most may be an opponent reasoning about a seat's mana.
func TestPoolRestrictionIsPublicForEveryViewer(t *testing.T) {
	g, _ := restrictionBoard(t)
	ch := flatChars{g}
	for _, vis := range []Visibility{Seat, Public, Omniscient} {
		for viewer := state.PlayerID(0); viewer < 2; viewer++ {
			v := ProjectFor(g, ch, viewer, vis, nil)
			var seat0 *PlayerView
			for i := range v.Players {
				if v.Players[i].ID == 0 {
					seat0 = &v.Players[i]
				}
			}
			if seat0 == nil {
				t.Fatalf("visibility %s viewer %d: seat 0 missing", vis, viewer)
			}
			if len(seat0.PoolRestrictions) != 1 {
				t.Fatalf("visibility %s viewer %d: seat 0 projects %d restrictions, want 1", vis, viewer, len(seat0.PoolRestrictions))
			}
			if want := "spend only to cast a Demon creature spell"; seat0.PoolRestrictions[0].Text != want {
				t.Fatalf("visibility %s viewer %d: seat 0 text = %q, want %q", vis, viewer, seat0.PoolRestrictions[0].Text, want)
			}
		}
	}
}

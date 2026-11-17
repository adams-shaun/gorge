package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTopCardOfLibraryIsGatesTheStatic pins Mul Daya Channelers' two "as long
// as the top card of your library is ..." statics. The top card is a real
// library card (the assertion below), and the granted +3/+3 / AnyMana ability
// must follow it at every moment (CR 611.3a): a creature card gives 5/5, a
// non-creature card gives 2/2, and only a land card offers the "{T}: Add two
// mana of any one color" ability.
func TestTopCardOfLibraryIsGatesTheStatic(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name     string
		top      string
		wantPT   string
		wantMana bool
	}{
		{"noncreature-top", "Shock", "2/2", false},
		{"creature-top", "Grizzly Bears", "5/5", false},
		{"land-top", "Forest", "2/2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := oracleScenario{
				Name: "mul-daya-" + tc.name,
				CR:   []string{"611.3a"},
				Why:  "a Continuous static's intervening-if follows the top card of the library",
				Setup: map[string]oracleSeat{
					"p0": {Battlefield: []string{"Mul Daya Channelers"}, LibraryTop: []string{tc.top, "Island", "Island"}},
				},
			}
			want := tc.wantMana
			sc.Expect = []oracleExpect{
				{Card: "p0:Mul Daya Channelers", Zone: "battlefield", PT: tc.wantPT},
				{Offered: &oracleOffered{Seat: 0, Kind: "activate", Card: "p0:Mul Daya Channelers", Label: "Add two mana of any one color"}, Want: &want},
			}
			fails, _, r := runOracleScenario(reg, sc)
			if len(fails) != 0 {
				t.Fatalf("scenario failed: %v", fails)
			}
			id, err := r.resolve("p0:Mul Daya Channelers")
			if err != nil {
				t.Fatal(err)
			}
			if o := r.e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
				t.Fatalf("precondition: Mul Daya Channelers zone = %v, want battlefield", o)
			}
			lib := r.e.G.Zone(state.ZLibrary, 0)
			if len(lib) == 0 {
				t.Fatal("precondition: p0's library is empty, so TopCardOfLibraryIs$ never reads " + tc.top)
			}
			top := r.e.G.Obj(lib[0])
			if top == nil || top.Face() == nil || top.Face().Name != tc.top {
				t.Fatalf("precondition: library top = %v, want %q", top, tc.top)
			}
		})
	}

	// The value is a general card filter: "Card.Black" is a colour word, not a
	// type. Vampire Nocturnus' +2/+1 and flying follow a black top card.
	for _, tc := range []struct {
		name    string
		top     string
		wantPT  string
		wantFly bool
	}{
		{"colour-top-black", "Doom Blade", "5/4", true},
		{"colour-top-not-black", "Forest", "3/3", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := oracleScenario{
				Name: "vampire-nocturnus-" + tc.name,
				CR:   []string{"611.3a"},
				Why:  "a colour-word TopCardOfLibraryIs$ follows the top card",
				Setup: map[string]oracleSeat{
					"p0": {Battlefield: []string{"Vampire Nocturnus"}, LibraryTop: []string{tc.top, "Island"}},
				},
			}
			expect := oracleExpect{Card: "p0:Vampire Nocturnus", PT: tc.wantPT}
			if tc.wantFly {
				expect.Keywords = []string{"Flying"}
			} else {
				expect.NoKeywords = []string{"Flying"}
			}
			sc.Expect = []oracleExpect{expect}
			fails, _, r := runOracleScenario(reg, sc)
			if len(fails) != 0 {
				t.Fatalf("scenario failed: %v", fails)
			}
			lib := r.e.G.Zone(state.ZLibrary, 0)
			if len(lib) == 0 {
				t.Fatal("precondition: p0's library is empty")
			}
			top := r.e.G.Obj(lib[0])
			if top == nil || top.Face() == nil || top.Face().Name != tc.top {
				t.Fatalf("precondition: library top = %v, want %q", top, tc.top)
			}
		})
	}

	// A static's effect applies at every moment (CR 611.3a). Drawing the
	// creature off the top must turn the boost off, and the land that becomes
	// the new top must turn the mana ability on, without any other event
	// forcing the layer memo to rescan.
	t.Run("top-change-on-draw", func(t *testing.T) {
		yes := true
		sc := oracleScenario{
			Name: "mul-daya-top-change",
			CR:   []string{"611.3a"},
			Why:  "drawing a new top card flips the continuous static",
			Setup: map[string]oracleSeat{
				"p0": {Battlefield: []string{"Mul Daya Channelers", "Mikokoro, Center of the Sea"}, LibraryTop: []string{"Grizzly Bears", "Forest", "Island"}, Mana: "CC"},
				"p1": {LibraryTop: []string{"Island", "Island"}},
			},
			Steps: []oracleStep{{
				Op: "activate", Seat: 0, Card: "p0:Mikokoro, Center of the Sea",
				Ability: "Each player draws a card", Mana: "CC",
				Expect: []oracleExpect{
					{Card: "p0:Mul Daya Channelers", PT: "5/5"},
					{Offered: &oracleOffered{Seat: 0, Kind: "activate", Card: "p0:Mul Daya Channelers", Label: "Add two mana of any one color"}, Want: boolp(false)},
				},
			}, {Op: "resolve"}},
			Expect: []oracleExpect{
				{Card: "p0:Mul Daya Channelers", PT: "2/2"},
				{Offered: &oracleOffered{Seat: 0, Kind: "activate", Card: "p0:Mul Daya Channelers", Label: "Add two mana of any one color"}, Want: &yes},
			},
		}
		fails, _, r := runOracleScenario(reg, sc)
		if len(fails) != 0 {
			t.Fatalf("scenario failed: %v", fails)
		}
		lib := r.e.G.Zone(state.ZLibrary, 0)
		if len(lib) == 0 {
			t.Fatal("precondition: p0's library is empty after the draw")
		}
		top := r.e.G.Obj(lib[0])
		if top == nil || top.Face() == nil || top.Face().Name != "Forest" {
			t.Fatalf("precondition: after the draw the top card is %v, want Forest", top)
		}
	})
}

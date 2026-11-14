package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runManaSpendScenario casts card with mana and asserts expect (a trigger's
// ManaSpent$ / ManaNotSpent$ clause, CR 601.2h: the gate reads the cast's
// per-colour spend, engine/rules/trigger_condition.go manaColourSpendHolds).
func runManaSpendScenario(t *testing.T, name, card, mana, expect string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"` + name + `","cr":["601.2h"],"why":"a trigger's mana-spend clause","setup":{"p0":{"hand":["` + card + `"],"battlefield":["Forest","Forest","Forest","Island","Island","Plains","Mountain"]}},"steps":[{"op":"cast","seat":0,"card":"p0:` + card + `","mana":"` + mana + `"},{"op":"resolve"}],"expect":` + expect + `}`
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil {
		t.Fatalf("scenario %s: %v", name, err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario %s: %v", name, res.Fails)
	}
}

// TestCourtHussarWhiteSpentStays: with {W} spent to cast it the
// ManaNotSpent$ W trigger does not fire, so Court Hussar stays.
func TestCourtHussarWhiteSpentStays(t *testing.T) {
	t.Parallel()
	runManaSpendScenario(t, "hussar-w", "Court Hussar", "WUU",
		`[{"card":"p0:Court Hussar","zone":"battlefield"}]`)
}

// TestCourtHussarNoWhiteSacrificed: with no {W} spent the trigger fires and
// sacrifices it.
func TestCourtHussarNoWhiteSacrificed(t *testing.T) {
	t.Parallel()
	runManaSpendScenario(t, "hussar-nw", "Court Hussar", "UUU",
		`[{"card":"p0:Court Hussar","zone":"graveyard"}]`)
}

// TestPatagiaViperBlueSpentStays: with {U} spent the ManaNotSpent$ U trigger
// does not fire.
func TestPatagiaViperBlueSpentStays(t *testing.T) {
	t.Parallel()
	runManaSpendScenario(t, "viper-u", "Patagia Viper", "UGGG",
		`[{"card":"p0:Patagia Viper","zone":"battlefield"}]`)
}

// TestPatagiaViperNoBlueSacrificed: with no {U} spent it is sacrificed (the
// token trigger has already made its snakes).
func TestPatagiaViperNoBlueSacrificed(t *testing.T) {
	t.Parallel()
	runManaSpendScenario(t, "viper-nu", "Patagia Viper", "GGGG",
		`[{"card":"p0:Patagia Viper","zone":"graveyard"}]`)
}

// TestGruulScrapperRedSpentHaste: ManaSpent$ R is the positive polarity --
// with {R} spent the ETB grants haste; with none it does not.
func TestGruulScrapperRedSpentHaste(t *testing.T) {
	t.Parallel()
	runManaSpendScenario(t, "gruul-r", "Gruul Scrapper", "RGGG",
		`[{"card":"p0:Gruul Scrapper","zone":"battlefield","keywords":["Haste"]}]`)
	runManaSpendScenario(t, "gruul-nr", "Gruul Scrapper", "GGGG",
		`[{"card":"p0:Gruul Scrapper","zone":"battlefield","no_keywords":["Haste"]}]`)
}

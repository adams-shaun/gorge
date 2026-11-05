package rules

// mayplay_selfflash_test.go pins CR 109.5 for a self-carried CastWithFlash
// static: while a player attempts to cast the card, the "you" on its own
// static is the would-be controller (the caster), not the card's controller
// in its origin zone. Illusion Spinners carries
// `Caster$ You | IsPresent$ Faerie.YouCtrl` on its own face; exiled by p0's
// Maralen, Fae Ascendant with a Faerie on p0's side, p0 may cast it at
// instant speed. The pre-fix binding resolved both gates against p1 (the
// owner in p1's exile) and withheld the cast.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// maralenMayPlaySelfFlashScenario puts four creatures (three Elves, two
// Faeries, Maralen itself is both) on p0's side, casts Maralen, and resolves
// its ETB Dig, which exiles p1's top two cards (Illusion Spinners, Savannah
// Lions). X (Count$Valid Elf.YouCtrl,Faerie.YouCtrl) is then 5, so Illusion
// Spinners' mana value 5 meets Maralen's Affected$ cmcLEX gate.
const maralenMayPlaySelfFlashScenario = `{"name":"maralen-mayplay-selfflash","cr":["109.5"],"why":"inline",` +
	`"setup":{"p0":{"battlefield":["Bloom Tender","Lys Alana Informant","Glamer Gifter","Dawnhand Eulogist"],` +
	`"hand":["Maralen, Fae Ascendant"]},"p1":{"library_top":["Illusion Spinners","Savannah Lions"]}},"steps":[` +
	`{"op":"cast","seat":0,"card":"p0:Maralen, Fae Ascendant","mana":"CCBGU"},` +
	`{"op":"resolve"},` +
	`{"op":"resolve"}]}`

func TestMayPlaySelfFlashYouIsWouldBeController(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(maralenMayPlaySelfFlashScenario))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, transcript, run := runOracleScenarioSpare(reg, sc, false, new(Spare))
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%v", fails, transcript)
	}
	e := run.e
	var maralen state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face().Name == "Maralen, Fae Ascendant" {
			maralen = id
		}
	}
	if maralen == 0 {
		t.Fatalf("precondition: Maralen is not on the battlefield\n%v", transcript)
	}
	var illu state.ObjID
	for _, id := range e.G.Zone(state.ZExile, 1) {
		if o := e.G.Obj(id); o != nil && o.Face().Name == "Illusion Spinners" {
			illu = id
			if o.ExiledWith != maralen {
				t.Fatalf("precondition: Illusion Spinners ExiledWith = %d, want Maralen %d", o.ExiledWith, maralen)
			}
		}
	}
	if illu == 0 {
		t.Fatalf("precondition: Illusion Spinners not in p1's exile (the ETB Dig did not run?)\n%v", transcript)
	}
	// The Affected$ spec (X = 5, mana value 5) matches: the may-play grant
	// itself covers the card, so the flash gate below is the only open
	// question. cmcLEX's SVar right-hand side needs the engine's own
	// evaluation (a bare MatchesSpecFrom cannot resolve it).
	if !e.matchesSpecFrom("Card.nonLand+cmcLEX+ExiledWithSource+ThisTurnEntered", illu, 0, maralen) {
		t.Fatalf("precondition: Illusion Spinners does not match Maralen's Affected$ spec\n%v", transcript)
	}
	// The fix itself: p0 (who controls a Faerie and attempts the cast) reads
	// the self static's "you" as itself.
	if !e.castWithFlash(0, illu) {
		t.Errorf("castWithFlash(0, Illusion Spinners) = false, want true (CR 109.5: the would-be controller reads the static's You)")
	}
	// The gate reads the caster, not the owner: p1 owns the card but controls
	// no Faerie.
	if e.castWithFlash(1, illu) {
		t.Errorf("castWithFlash(1, Illusion Spinners) = true, want false (p1 controls no Faerie)")
	}
	// No regression in the offer: the may-play free cast option still appears
	// in legalActions on this empty-stack main1 board (it did pre-fix too;
	// sorcery timing needs no flash there).
	offered := false
	for _, o := range e.legalActions(0) {
		if o.Obj == illu && o.Mode == "mayplay" {
			offered = true
		}
	}
	if !offered {
		t.Errorf("legalActions(0) offers no may-play cast of Illusion Spinners")
	}
}

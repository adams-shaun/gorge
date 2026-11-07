package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Mindskinner's damage replacement runs a Mill body. Each milled card's
// library-to-graveyard move is a new zone-change event (CR 614.6, 701.17), so
// Leyline of the Void's Moved "exile it instead" applies to it even though the
// Mill is emitted from inside the damage replacement body.
const mindskinnerLeylineScenario = `{"name":"mindskinner-with-leyline","turn":1,"cr":[],"why":"The Mindskinner's prevented damage mills into a Leyline of the Void.","setup":{"p0":{"battlefield":["Leyline of the Void","The Mindskinner"],"hand":[],"graveyard":[],"library_top":["Island","Island","Island","Island","Island"],"life":20},"p1":{"battlefield":["Island"],"hand":[],"graveyard":[],"library_top":["Pyroclasm","Island","Island","Island","Island"],"life":20}},"steps":[{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"attack","seat":0,"attackers":["p0:The Mindskinner"],"defender":"p1"},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1}]}`

const mindskinnerNoLeylineScenario = `{"name":"mindskinner-without-leyline","turn":1,"cr":[],"why":"The Mindskinner's prevented damage mills into the graveyard with no exile replacement.","setup":{"p0":{"battlefield":["The Mindskinner"],"hand":[],"graveyard":[],"library_top":["Island","Island","Island","Island","Island"],"life":20},"p1":{"battlefield":["Island"],"hand":[],"graveyard":[],"library_top":["Pyroclasm","Island","Island","Island","Island"],"life":20}},"steps":[{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"attack","seat":0,"attackers":["p0:The Mindskinner"],"defender":"p1"},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1}]}`

// DSK-s188_w17 (xe:RAND:1:18:w17): an Erratic Apparition hit is replaced by
// The Mindskinner's mill while Leyline of the Void is on the battlefield.
// Seized from Slumber is the first card milled.
const mindskinnerFarmWindowScenario = `{"name":"xengine window RAND-c1-g18-0-WG vs RAND-c1-g18-1-UB turn 17","turn":1,"cr":[],"why":"One turn of a recorded gorge game between two explore bots, replayed from its first precombat main phase.","setup":{"p0":{"battlefield":["Island","Island","Stalked Researcher","Swamp","Paranormal Analyst","Island","Swamp","Island","The Mindskinner","Erratic Apparition","Island","Leyline of the Void","Creeping Peeper"],"hand":["Meathook Massacre II","Malevolent Chandelier"],"graveyard":["Cynical Loner","Glimmerburst"],"library_top":["Island","Murky Sewer","Spectral Snatcher","Island","Clammy Prowler","Enter the Enigma","Island","Swamp","Swamp","Enduring Curiosity","Keys to the House","Swamp","Dashing Bloodsucker","Fear of Lost Teeth","Attack-in-the-Box","Island","Island","Get Out","Swamp","Fear of the Dark","Unwilling Vessel","Swamp","Daggermaw Megalodon"],"life":20},"p1":{"battlefield":["Plains","Forest","Plains","Plains","Cult Healer","Spineseeker Centipede","Plains","Defiant Survivor","Cautious Survivor","Hushwood Verge","Attack-in-the-Box","Fear of Surveillance","Keys to the House"],"tapped":["Plains","Forest","Hushwood Verge"],"graveyard":["Break Down the Door","Acrobatic Cheerleader","Plains","Exorcise","Emerge from the Cocoon","Split Up","Enduring Innocence"],"library_top":["Seized from Slumber","Plains","Forest","Plains","Overgrown Zealot","Forest","Forest","Altanak, the Thrice-Called","Wickerfolk Thresher","Flesh Burrower","Forest","Plains","Forest","Plains","Forest","Hardened Escort","Omnivorous Flytrap","Shepherding Spirits","Shardmage's Rescue","Plains"],"life":23}},"steps":[{"op":"activate","seat":0,"card":"p0:Island","ability_index":0},{"op":"activate","seat":0,"card":"p0:Island#2","ability_index":0},{"op":"activate","seat":0,"card":"p0:Island#3","ability_index":0},{"op":"activate","seat":0,"card":"p0:Swamp#2","ability_index":0},{"op":"activate","seat":0,"card":"p0:Island#4","ability_index":0},{"op":"activate","seat":0,"card":"p0:Island#5","ability_index":0},{"op":"cast","seat":0,"card":"p0:Malevolent Chandelier"},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"attack","seat":0,"attackers":["p0:Erratic Apparition","p0:Creeping Peeper"],"defender":"p1"},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"block","seat":1,"blocks":[["p1:Cult Healer","p0:Creeping Peeper"]]},{"op":"pass","seat":0},{"op":"pass","seat":1}]}`

func mindskinnerLastSnapshot(t *testing.T, scenario string) OracleSnapshot {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	if len(res.Snapshots) == 0 {
		t.Fatal("scenario recorded no snapshot")
	}
	return res.Snapshots[len(res.Snapshots)-1]
}

func mindskinnerHasPermanent(s OracleSnapshot, name string, controller int) bool {
	for _, p := range s.Permanents {
		if p.Name == name && p.Controller == controller {
			return true
		}
	}
	return false
}

func mindskinnerContains(cards []string, name string) bool {
	for _, c := range cards {
		if c == name {
			return true
		}
	}
	return false
}

func TestMindskinnerMillHonoursMovedReplacement(t *testing.T) {
	t.Parallel()
	last := mindskinnerLastSnapshot(t, mindskinnerLeylineScenario)
	if !mindskinnerHasPermanent(last, "The Mindskinner", 0) || !mindskinnerHasPermanent(last, "Leyline of the Void", 0) {
		t.Fatalf("precondition: replacement permanents missing from the final board: %+v", last.Permanents)
	}
	p1 := last.Players[1]
	if !mindskinnerContains(p1.Exile, "Pyroclasm") {
		t.Fatalf("milled cards were not exiled: exile=%v graveyard=%v", p1.Exile, p1.Graveyard)
	}
	if len(p1.Graveyard) != 0 {
		t.Fatalf("Leyline of the Void let %d milled card(s) reach the graveyard: %v", len(p1.Graveyard), p1.Graveyard)
	}
	if got := len(p1.Exile); got != 10 {
		t.Fatalf("exiled %d milled card(s), want the whole 10-card mill: %v", got, p1.Exile)
	}
	if p1.Life != 20 {
		t.Fatalf("The Mindskinner's damage was dealt (life %d), want it fully replaced by the mill", p1.Life)
	}
}

func TestMindskinnerMillNoLeylineControl(t *testing.T) {
	t.Parallel()
	last := mindskinnerLastSnapshot(t, mindskinnerNoLeylineScenario)
	if !mindskinnerHasPermanent(last, "The Mindskinner", 0) {
		t.Fatalf("precondition: The Mindskinner is not on the final board: %+v", last.Permanents)
	}
	if mindskinnerHasPermanent(last, "Leyline of the Void", 0) {
		t.Fatal("precondition: the control scenario has a Leyline of the Void")
	}
	p1 := last.Players[1]
	if !mindskinnerContains(p1.Graveyard, "Pyroclasm") {
		t.Fatalf("control milled cards did not reach the graveyard: graveyard=%v exile=%v", p1.Graveyard, p1.Exile)
	}
	if got := len(p1.Graveyard); got != 10 {
		t.Fatalf("control milled %d card(s) into the graveyard, want 10: %v", got, p1.Graveyard)
	}
	if len(p1.Exile) != 0 {
		t.Fatalf("control exiled %d card(s) with no exile replacement: %v", len(p1.Exile), p1.Exile)
	}
}

func TestMindskinnerMillFarmWindows(t *testing.T) {
	t.Parallel()
	last := mindskinnerLastSnapshot(t, mindskinnerFarmWindowScenario)
	if !mindskinnerHasPermanent(last, "The Mindskinner", 0) || !mindskinnerHasPermanent(last, "Leyline of the Void", 0) {
		t.Fatalf("precondition: replacement permanents missing from the final board: %+v", last.Permanents)
	}
	p1 := last.Players[1]
	if !mindskinnerContains(p1.Exile, "Seized from Slumber") {
		t.Fatalf("Seized from Slumber was not exiled: exile=%v graveyard=%v", p1.Exile, p1.Graveyard)
	}
	if mindskinnerContains(p1.Graveyard, "Seized from Slumber") {
		t.Fatalf("Seized from Slumber reached the graveyard: %v", p1.Graveyard)
	}
}

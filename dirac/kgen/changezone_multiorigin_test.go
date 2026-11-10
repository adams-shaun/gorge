package kgen

import (
	"strings"
	"testing"
)

// The sideboard origins of ChangeZone (lower_sideboardsearch.go): North Wind
// Avatar's pick from outside the game and Turtles Forever's three nodes.
// The probe scripts below are paraphrases that carry the same parameter
// shapes; each out-of-shape edit must refuse by name.

const sideProbeTrigger = `Name:Side Wish Probe
ManaCost:2 U
Types:Creature Spirit
PT:2/2
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigWish | TriggerDescription$ probe
SVar:TrigWish:DB$ ChangeZone | PICK
Oracle:probe
`

const sideProbeSpell = `Name:Side Find Probe
ManaCost:3 W
Types:Instant
A:SP$ ChangeZone | FIND | SubAbility$ DBPick
SVar:DBPick:DB$ ChangeZone | PICK | SubAbility$ DBAll
SVar:DBAll:DB$ ChangeZoneAll | ALL | SubAbility$ DBClean
SVar:DBClean:DB$ Cleanup | ClearChosenCard$ False | ClearRemembered$ True
Oracle:probe
`

const (
	sideWishPick = "Origin$ Sideboard | Destination$ Hand | ChangeType$ Card.YouOwn | ChangeNum$ 1 | Hidden$ True"
	sideFind     = "Origin$ Library,Sideboard | ChangeType$ Creature.Legendary | ChangeNum$ 4 | RememberChanged$ True | Exactly$ True | Reveal$ True | Shuffle$ False | DifferentNames$ True"
	sideTake     = "Chooser$ Opponent | NoLooking$ True | Hidden$ True | Origin$ Library,Sideboard | Destination$ Hand | ChangeType$ Card.IsRemembered | ChangeNum$ 2 | Mandatory$ True | Shuffle$ False | ForgetChanged$ True"
	sideAll      = "Origin$ Library,Sideboard | Destination$ Library | ChangeType$ Card.IsRemembered | Shuffle$ True"
)

func sideWishScript(pick string) string {
	return strings.Replace(sideProbeTrigger, "PICK", pick, 1)
}

func sideSpellScript(find, pick, all string) string {
	s := strings.Replace(sideProbeSpell, "FIND", find, 1)
	s = strings.Replace(s, "PICK", pick, 1)
	return strings.Replace(s, "ALL", all, 1)
}

// anyRefusal reports whether some refusal reason contains want.
func anyRefusal(rs []string, want string) bool {
	for _, r := range rs {
		if strings.Contains(r, want) {
			return true
		}
	}
	return false
}

func TestLowerChangezoneMultiorigin(t *testing.T) {
	// the probes in the printed shapes lower clean
	if rs := lowerScript(t, "Side Wish Probe", sideWishScript(sideWishPick)); len(rs) != 0 {
		t.Fatalf("wish probe refused: %v", rs)
	}
	if rs := lowerScript(t, "Side Find Probe", sideSpellScript(sideFind, sideTake, sideAll)); len(rs) != 0 {
		t.Fatalf("find probe refused: %v", rs)
	}

	// the cards lower, to the instructions the kernel reads
	nwa := lowerCorpusCard(t, "North Wind Avatar")
	for _, rf := range nwa.Refusals {
		t.Errorf("North Wind Avatar refused: %s", rf.Reason)
	}
	var search []*lInstr
	for _, a := range nwa.Abilities {
		for i := range a.Prog {
			if a.Prog[i].Op == "Search" {
				search = append(search, &a.Prog[i])
			}
		}
	}
	if len(search) != 1 {
		t.Fatalf("North Wind Avatar: %d Search, want 1", len(search))
	}
	if cs := search[0].CS; cs != "Side|SideOnly|Mandatory" || search[0].C != zHand {
		t.Errorf("North Wind Avatar: Search C=%d CS=%q, want a mandatory sideboard-only pick into the hand", search[0].C, cs)
	}

	tf := lowerCorpusCard(t, "Turtles Forever")
	for _, rf := range tf.Refusals {
		t.Errorf("Turtles Forever refused: %s", rf.Reason)
	}
	var ops []string
	var css []string
	for _, a := range tf.Abilities {
		for i := range a.Prog {
			ops = append(ops, a.Prog[i].Op)
			css = append(css, a.Prog[i].CS)
		}
	}
	want := []string{"Search", "Search", "ChangeZoneAll", "SkipIf", "Shuffle", "ClearRemembered"}
	if got := strings.Join(ops, ","); !strings.HasPrefix(got, strings.Join(want, ",")) {
		t.Fatalf("Turtles Forever ops %s, want %s first", got, strings.Join(want, ","))
	}
	for i, wcs := range []string{"Side|Exactly|DifferentNames|Remember|NoShuffle", "Side|ChooserOpp|Mandatory|ForgetChanged|NoShuffle", "from=Library,Sideboard"} {
		if css[i] != wcs {
			t.Errorf("Turtles Forever node %d CS=%q, want %q", i, css[i], wcs)
		}
	}

	// each out-of-shape variant refuses by name
	for _, tc := range []struct{ name, script, reason string }{
		{"wish to battlefield", sideWishScript(strings.Replace(sideWishPick, "Destination$ Hand", "Destination$ Battlefield", 1)), "ChangeZone Origin$ Sideboard to Battlefield"},
		{"wish unhidden", sideWishScript(strings.Replace(sideWishPick, " | Hidden$ True", "", 1)), "Origin$ Sideboard without Hidden$ True"},
		{"wish shuffled", sideWishScript(sideWishPick + " | Shuffle$ True"), "unread params [Shuffle]"},
		{"wish optional", sideWishScript(sideWishPick + " | Optional$ True"), "unread params [Optional]"},
		{"wish counts", sideWishScript(strings.Replace(sideWishPick, "ChangeNum$ 1", "ChangeNum$ 0", 1)), "Origin$ Sideboard with ChangeNum$"},
		{"find unrevealed", sideSpellScript(strings.Replace(sideFind, " | Reveal$ True", "", 1), sideTake, sideAll), "without Destination$ needs Reveal$ True"},
		{"find not exact", sideSpellScript(strings.Replace(sideFind, " | Exactly$ True", "", 1), sideTake, sideAll), "without Destination$ needs Exactly$ True"},
		{"find shuffled", sideSpellScript(strings.Replace(sideFind, "Shuffle$ False", "Shuffle$ True", 1), sideTake, sideAll), "without Destination$ needs Shuffle$ False"},
		{"find qualityless", sideSpellScript(strings.Replace(sideFind, "Creature.Legendary", "Card.YouOwn", 1), sideTake, sideAll), "states no quality"},
		{"find to hand", sideSpellScript(sideFind+" | Destination$ Hand", sideTake, sideAll), "Origin$ Library,Sideboard to Hand"},
		{"take by you", sideSpellScript(sideFind, strings.Replace(sideTake, "Chooser$ Opponent", "Chooser$ You", 1), sideAll), `Chooser$ "You"`},
		{"take optional", sideSpellScript(sideFind, strings.Replace(sideTake, " | Mandatory$ True", "", 1), sideAll), "to Hand needs Mandatory$ True"},
		{"take shuffled", sideSpellScript(sideFind, strings.Replace(sideTake, "Shuffle$ False", "Shuffle$ True", 1), sideAll), "to Hand needs Shuffle$ False"},
		{"take any card", sideSpellScript(sideFind, strings.Replace(sideTake, "Card.IsRemembered", "Card", 1), sideAll), "Origin$ Library,Sideboard to Hand"},
		{"all unshuffled", sideSpellScript(sideFind, sideTake, strings.Replace(sideAll, "Shuffle$ True", "Shuffle$ False", 1)), "ChangeZoneAll Origin$ Library,Sideboard outside"},
		{"all to hand", sideSpellScript(sideFind, sideTake, strings.Replace(sideAll, "Destination$ Library", "Destination$ Hand", 1)), "ChangeZoneAll Origin$ Library,Sideboard outside"},
		{"all of anything", sideSpellScript(sideFind, sideTake, strings.Replace(sideAll, "Card.IsRemembered", "Card", 1)), "ChangeZoneAll Origin$ Library,Sideboard outside"},
		{"all on top", sideSpellScript(sideFind, sideTake, sideAll+" | LibraryPosition$ 0"), "ChangeZoneAll Origin$ Library,Sideboard with LibraryPosition$"},
	} {
		script := tc.script
		name := "Side Wish Probe"
		if strings.Contains(script, "Side Find Probe") {
			name = "Side Find Probe"
		}
		if rs := lowerScript(t, name, script); !anyRefusal(rs, tc.reason) {
			t.Errorf("%s: refusals %q, want one containing %q", tc.name, rs, tc.reason)
		}
	}
}

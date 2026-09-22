package cards

import (
	"sort"
	"testing"
)

// Keyword expansion used to be one switch in keywords.go, which made every
// ticket touching a keyword edit the same file. It is now a table the
// per-keyword kw_*.go files register into. These tests pin the two properties
// that replace what the compiler used to give for free: no head silently loses
// its expander, and no two files claim the same head.

// expandedHeads is every keyword head the switch expanded before the split. A
// head dropping out of the table is a keyword that silently stops expanding --
// no test would otherwise fail, the carrier just quietly loses its ability.
var expandedHeads = []string{
	"etbCounter", "Devour", "ETBReplacement",
	"Undying", "Persist", "Evolve", "Exalted", "Dethrone", "Afflict",
	"Flanking", "Hideaway", "Prowess", "Extort", "Soulbond", "Myriad",
	"Annihilator", "Ward", "Storm", "Gravestorm", "Replicate", "Conspire",
	"Living Weapon", "For Mirrodin", "Cumulative upkeep", "Echo",
	"Equip", "Transmute", "Cycling", "TypeCycling", "Level up", "Affinity",
	"Enchant", "Mobilize", "Afterlife", "Encore", "Embalm", "Eternalize",
	// Squad (CR 702.66) was added after the split: the corpus carries only
	// the K:Squad:<cost> line, so the expansion supplies the ETB trigger
	// that creates one token copy per squad payment (Count$SquadPaid).
	"Squad",
	// CR 702.70 Training: a genuinely new expansion added after the split
	// (not a head the old switch covered), so it is listed here to keep the
	// table equal to the registered set.
	"Training",
	// Appended after the split (each is a keyword whose expansion the
	// pre-split switch never had): Exploit (CR 702.58, task exploit1).
	"Exploit",
	// Offspring (CR 702.175, task offspring1): the corpus carries only the
	// K:Offspring:<cost> line, so the expansion supplies the ETB trigger
	// that creates one 1/1 token copy when the additional cost was paid
	// (Count$OffspringPaid).
	"Offspring",
}

func TestEveryExpandedKeywordHasAnExpander(t *testing.T) {
	for _, head := range expandedHeads {
		if kwExpanders[head] == nil {
			t.Errorf("keyword head %q has no registered expander: it silently stops expanding", head)
		}
	}
}

func TestNoKeywordIsRegisteredThatTheSwitchNeverExpanded(t *testing.T) {
	// The mirror: a head in the table that was not in the switch means the
	// split invented an expansion, which is a behaviour change.
	known := map[string]bool{}
	for _, h := range expandedHeads {
		known[h] = true
	}
	var extra []string
	for head := range kwExpanders {
		if !known[head] {
			extra = append(extra, head)
		}
	}
	sort.Strings(extra)
	if len(extra) != 0 {
		t.Errorf("heads registered but not expanded before the split: %v", extra)
	}
}

func TestAnUnregisteredKeywordIsNotExpanded(t *testing.T) {
	// The switch had no default arm: a keyword whose meaning is a casting
	// option or a static property (Kicker, Flash, Protection) falls through
	// and is read directly by rules. The table must keep that, not panic.
	for _, head := range []string{"Kicker", "Flash", "Protection", "Indestructible"} {
		if kwExpanders[head] != nil {
			t.Errorf("%q must not be expanded: rules reads it directly", head)
		}
	}
	f := &Face{Keywords: []string{"Kicker:{2}"}}
	f.expandKeywords()
	if len(f.Triggers) != 0 || len(f.Repls) != 0 || len(f.Abilities) != 0 {
		t.Errorf("an unregistered head expanded anyway: %d T, %d R, %d A",
			len(f.Triggers), len(f.Repls), len(f.Abilities))
	}
}

func TestRegisteringOneKeywordTwicePanics(t *testing.T) {
	// Two files claiming one head is the merge accident the split makes
	// possible, so it must be loud at startup rather than an expansion that
	// quietly stopped being reached.
	const head = "TestOnlyDuplicateKeyword"
	fn := func(*Face, int, string, string, string, func(string, string) bool) {}
	registerKeyword(fn, head)
	defer delete(kwExpanders, head)

	defer func() {
		if recover() == nil {
			t.Error("registering a keyword head twice must panic")
		}
	}()
	registerKeyword(fn, head)
}

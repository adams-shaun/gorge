package kgen

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// probeFixture lowers a card in both modes and asserts the default mode
// reports only the node's first refusal, while the every-blocker mode also
// reports the probed param's refusal.
func probeFixture(t *testing.T, src, firstWant, want string) {
	t.Helper()
	first := abReasons(abLower(t, src, false))
	if first == "" {
		t.Fatal("precondition: the fixture lowers; its first node must refuse")
	}
	if !strings.Contains(first, firstWant) {
		t.Fatalf("precondition: the first refusal is %q, got %q", firstWant, first)
	}
	if strings.Contains(first, want) {
		t.Fatalf("the default mode already read the probed param %q: %q", want, first)
	}
	all := abReasons(abLower(t, src, true))
	if !strings.Contains(all, want) {
		t.Errorf("every-blocker mode lacks the probed refusal %q:\n%s", want, all)
	}
	if !strings.Contains(all, firstWant) {
		t.Errorf("every-blocker mode lost the first refusal:\n%s", all)
	}
}

// TestProbeReadsRefKeys: an unread Defined$ is read through the Defined
// grammar under -all-blockers, and only there.
func TestProbeReadsRefKeys(t *testing.T) {
	src := strings.Join([]string{
		"Name:Probe",
		"ManaCost:R",
		"Types:Instant",
		"A:SP$ DealDamage | ValidTgts$ Any.!EnchantedBy | NumDmg$ 1 | Defined$ Bogus$Head | SpellDescription$ Probe.",
	}, "\n")
	probeFixture(t, src, `unknown filter head "Any"`, `Defined$ "Bogus$Head"`)
}

// TestProbeReadsCondKeys: an unread ConditionCheckSVar$ is read through the
// value reader, so its missing SVar is a second blocker.
func TestProbeReadsCondKeys(t *testing.T) {
	src := strings.Join([]string{
		"Name:Probe",
		"ManaCost:R",
		"Types:Instant",
		"A:SP$ DealDamage | ValidTgts$ Any.!EnchantedBy | NumDmg$ 1 | ConditionCheckSVar$ Bogus | SpellDescription$ Probe.",
	}, "\n")
	probeFixture(t, src, `unknown filter head "Any"`, `names no SVar`)
}

// TestProbeReadsCostKeys: an unread UnlessCost$ and a sub-ability's unread
// Cost$ are read through the cost readers their nodes use, so each bad token
// is a second blocker.
func TestProbeReadsCostKeys(t *testing.T) {
	t.Run("unless", func(t *testing.T) {
		// PutCounter's own Defined$ Targeted.<qualifier> guard refuses before
		// effect reads UnlessCost$, leaving the cost unread.
		src := strings.Join([]string{
			"Name:Probe",
			"ManaCost:R",
			"Types:Instant",
			"A:SP$ PutCounter | ValidTgts$ Creature | Defined$ Targeted.!BogusWord | UnlessCost$ Bogus | UnlessPayer$ You | SpellDescription$ Probe.",
		}, "\n")
		probeFixture(t, src, `!BogusWord`, `cost token "Bogus"`)
	})
	t.Run("sub-ability", func(t *testing.T) {
		// The trigger's Execute node refuses in its own Defined$ guard before
		// effect reads its resolution Cost$.
		src := strings.Join([]string{
			"Name:Probe",
			"ManaCost:R",
			"Types:Creature Human",
			"PT:1/1",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ TrigBody | TriggerDescription$ Probe.",
			"SVar:TrigBody:AB$ PutCounter | ValidTgts$ Creature | Defined$ Targeted.!BogusWord | Cost$ Bogus",
		}, "\n")
		probeFixture(t, src, `!BogusWord`, `cost token "Bogus"`)
	})
	t.Run("numtimes-skipped", func(t *testing.T) {
		// effect hands a Mana<...\NumTimes> cost to numTimesPay, not to a
		// cost reader: the probe leaves it unread instead of a false blocker.
		src := strings.Join([]string{
			"Name:Probe",
			"ManaCost:R",
			"Types:Creature Human",
			"PT:1/1",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ TrigBody | TriggerDescription$ Probe.",
			"SVar:TrigBody:AB$ PutCounter | ValidTgts$ Creature | Defined$ Targeted.!BogusWord | Cost$ Mana<1\\NumTimes>",
		}, "\n")
		c := abLower(t, src, true)
		all := abReasons(c)
		if !strings.Contains(all, `!BogusWord`) {
			t.Fatalf("precondition: the node refuses in its Defined$ guard:\n%s", all)
		}
		if strings.Contains(all, "NumTimes") {
			t.Errorf("the NumTimes cost became a probed blocker:\n%s", all)
		}
		unread := ""
		for _, u := range c.Unreadable {
			unread += u.Reason + "\n"
		}
		if !strings.Contains(unread, "Cost") {
			t.Errorf("the NumTimes cost left no unread row: %q", unread)
		}
	})
}

// dumpSetList lowers a named set list with the every-blocker probes present
// but off, and returns every card's IR dump plus the card count.
func dumpSetList(t *testing.T, list, cardsDir string) (string, int) {
	t.Helper()
	names, err := readCardList(filepath.Join(testSetlistDir, list+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadCardNames(names, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	if p.allBlockers {
		t.Fatal("precondition: the every-blocker mode must be off")
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, c := range lw.cards {
		if len(c.Unreadable) != 0 {
			t.Fatalf("%s carries an unread-after-refusal row with the mode off: %+v", c.Name, c.Unreadable)
		}
		for i := range c.Abilities {
			if c.Abilities[i].Name == "Probe" {
				t.Fatalf("%s carries a probe ability with the mode off", c.Name)
			}
		}
		dumpLIR(&sb, c)
	}
	return sb.String(), len(lw.cards)
}

// TestAllBlockersOffIsIdentical: with the mode off the probes never run, so
// a named set list lowers byte-identically across two passes and carries no
// probe ability or unread row.
func TestAllBlockersOffIsIdentical(t *testing.T) {
	cardsDir := ratchetCards(t)
	first, n := dumpSetList(t, "BIG", cardsDir)
	if n == 0 {
		t.Fatal("precondition: the named set list lowers no cards")
	}
	second, _ := dumpSetList(t, "BIG", cardsDir)
	if first != second {
		t.Fatalf("the mode-off lowering of BIG differs between passes (%d bytes vs %d)", len(first), len(second))
	}
}

// unreadTableRow parses one list row of the "## Unread after refusal" table.
func unreadTableRow(t *testing.T, md, list string) (gap, partial int) {
	t.Helper()
	prefix := "| " + list + " | "
	for _, line := range strings.Split(md, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		cols := strings.Split(line, "|")
		if len(cols) < 5 {
			t.Fatalf("malformed %s row %q", list, line)
		}
		var err error
		if gap, err = strconv.Atoi(strings.TrimSpace(cols[2])); err != nil {
			t.Fatalf("%s gap: %v", list, err)
		}
		if partial, err = strconv.Atoi(strings.TrimSpace(cols[3])); err != nil {
			t.Fatalf("%s partial: %v", list, err)
		}
		return gap, partial
	}
	t.Fatalf("no %s row in the unread table:\n%s", list, md)
	return 0, 0
}

// TestUnreadShareTable: the ALL row counts of an every-blocker census equal a
// recount over the result rows (refused cards and cards carrying a partial
// row). Only that mode writes unread-after-refusal rows, so the census runs
// in it and the partial column must be non-zero: a 0 == 0 pass is no check.
func TestUnreadShareTable(t *testing.T) {
	// Sequential, never heavyParallel: the mode is the package global
	// censusAllBlockers, which the shared census tests read unset.
	std, err := readCardList(filepath.Join("../../", standardListPath))
	if err != nil {
		t.Fatal(err)
	}
	cardsDir := ratchetCards(t)
	censusAllBlockers = true
	defer func() { censusAllBlockers = false }()
	res, err := computeSetlistCensus(testSetlistDir, cardsDir, std)
	if err != nil {
		t.Fatal(err)
	}
	var md strings.Builder
	writeStdCensus(&md, res)
	if !strings.Contains(md.String(), "## Unread after refusal") {
		t.Fatalf("census has no unread table:\n%s", md.String())
	}
	gap, partial := unreadTableRow(t, md.String(), "ALL")

	refused := map[string]bool{}
	unread := map[string]bool{}
	for _, r := range res.tsv {
		if r.set != "ALL" {
			continue
		}
		if r.class == partialClass {
			unread[r.card] = true
			continue
		}
		if r.class != "" {
			refused[r.card] = true
		}
	}
	if gap != len(refused) {
		t.Errorf("ALL gap cards = %d, recount over rows = %d", gap, len(refused))
	}
	if partial != len(unread) {
		t.Errorf("ALL cards with unread params = %d, recount over rows = %d", partial, len(unread))
	}
	if partial == 0 {
		t.Fatal("precondition: the every-blocker census holds no card with unread params")
	}
	if partial > gap {
		t.Errorf("ALL unread cards %d exceed gap cards %d", partial, gap)
	}
}

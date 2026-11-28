package kgen

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestDefinedPairShapes: the `Defined$ A & B` compositions of
// lower_defineduniononplayers.go and lower_definedset.go. A player pair
// lowers to one node per part in script order; an object union whose parts
// never meet lowers to one SET line ending SetUnion; the shapes gorge would
// play differently refuse with their own reason.
func TestDefinedPairShapes(t *testing.T) {
	corpus := []struct {
		name, defined string
		prog          *regexp.Regexp // the lowered program the pair must leave
	}{
		{"Tenuous Truce", "TriggeredPlayer & You", regexp.MustCompile(`(?m)^ +0 Draw obj=TriggeredPlayer A=1\n +1 Draw obj=You A=1$`)},
		{"Zurzoth, Chaos Rider", "TriggeredAttackedTarget & You", regexp.MustCompile(`(?m)^ +0 Draw obj=Defending A=1\n +1 Draw obj=You A=1$`)},
		{"Cait, Cage Brawler", "You & TriggeredDefendingPlayer", regexp.MustCompile(`(?m)^ +0 Draw obj=You A=1\n +1 Draw obj=Defending A=1$`)},
		{"Alaborn Zealot", "TriggeredAttackerLKICopy & Self", regexp.MustCompile(`(?m)^ +0 Destroy obj=Triggered\n +1 Destroy obj=Self$`)},
		{"Loyal Sentry", "TriggeredAttackerLKICopy & Self", regexp.MustCompile(`(?m)^ +0 Destroy obj=Triggered\n +1 Destroy obj=Self$`)},
		{"Leaf-Leap Guide", "Self & TriggeredCardLKICopy", regexp.MustCompile(`(?m)^  SET 0 SetSelf SetTriggerObj SetUnion$`)},
		{"Dragon Turtle", "Targeted & Self", regexp.MustCompile(`(?m)^  SET 0 SetTarget\(0\) SetSelf SetUnion$`)},
	}
	for _, tc := range corpus {
		t.Run(tc.name, func(t *testing.T) {
			c := lowerCorpusCard(t, tc.name)
			lir := cardLIR(c)
			for _, r := range c.Refusals {
				// a Discard/Mill pair gorge batches refuses by name (the
				// "a discard pair gorge batches" row below): not this pair
				if strings.Contains(r.Reason, `"`+tc.defined+`"`) && !strings.HasSuffix(r.Reason, "in one batch") {
					t.Fatalf("the pair refused: %s\n%s", r.Reason, lir)
				}
			}
			if !tc.prog.MatchString(lir) {
				t.Fatalf("no %s in:\n%s", tc.prog, lir)
			}
		})
	}
	t.Run("Dragon Turtle lowers whole", func(t *testing.T) {
		c := lowerCorpusCard(t, "Dragon Turtle")
		if len(c.Refusals) != 0 {
			t.Fatalf("refused: %s", abReasons(c))
		}
		if !strings.Contains(cardLIR(c), "obj=Set0") {
			t.Fatalf("no instruction reads Set0:\n%s", cardLIR(c))
		}
	})
	t.Run("Scry pair", func(t *testing.T) {
		c, lir := definedSetLIR(t, definedSetSrc(
			"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ Opponent | TriggerZones$ Battlefield | Execute$ ProbeTrigScry",
			"SVar:ProbeTrigScry:DB$ Scry | Defined$ TriggeredPlayer & You | ScryNum$ 2"))
		if len(c.Refusals) != 0 {
			t.Fatalf("refused: %s\n%s", abReasons(c), lir)
		}
		if !regexp.MustCompile(`(?m)^ +0 Scry obj=TriggeredPlayer A=2\n +1 Scry obj=You A=2$`).MatchString(lir) {
			t.Fatalf("no two Scry nodes in script order:\n%s", lir)
		}
	})

	refuses := []struct {
		name, src, reason string
	}{
		{"a targeted player and you", definedSetSrc("A:AB$ Draw | Cost$ T | ValidTgts$ Player | NumCards$ 1 | SubAbility$ ProbeDBDraw",
			"SVar:ProbeDBDraw:DB$ Draw | NumCards$ 1 | Defined$ TargetedPlayer & You"),
			`Defined$ "TargetedPlayer & You": the parts may name the same player`},
		{"a phase player of any seat and you", definedSetSrc(
			"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ Player | TriggerZones$ Battlefield | Execute$ ProbeTrigDraw",
			"SVar:ProbeTrigDraw:DB$ Draw | Defined$ TriggeredPlayer & You | NumCards$ 1"),
			`Defined$ "TriggeredPlayer & You": the parts may name the same player`},
		{"a target that may be the source", definedSetSrc("A:AB$ Pump | Cost$ T | ValidTgts$ Creature | NumAtt$ +1 | SubAbility$ ProbeDBPump",
			"SVar:ProbeDBPump:DB$ Pump | NumDef$ +1 | Defined$ Targeted & Self"),
			`Defined$ "Targeted & Self": the parts may name the same object`},
		{"a trigger object that may be the source", definedSetSrc(
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature | TriggerZones$ Battlefield | Execute$ ProbeTrigPump",
			"SVar:ProbeTrigPump:DB$ Pump | NumAtt$ +1 | Defined$ Self & TriggeredCard"),
			`Defined$ "Self & TriggeredCard": the parts may name the same object`},
		{"an object union on an op whose order reaches a decision", definedSetSrc(
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other | TriggerZones$ Battlefield | Execute$ ProbeTrigExile",
			"SVar:ProbeTrigExile:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Defined$ Self & TriggeredCard"),
			`Defined$ "Self & TriggeredCard": union order reaches a decision on ChangeZone`},
		{"a discard pair gorge batches", definedSetSrc(
			"T:Mode$ Attacks | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ ProbeTrigDiscard",
			"SVar:ProbeTrigDiscard:DB$ Discard | Mode$ TgtChoose | NumCards$ 1 | Defined$ TriggeredDefendingPlayer & You"),
			`Defined$ "TriggeredDefendingPlayer & You": gorge discards for both players in one batch`},
	}
	for _, tc := range refuses {
		t.Run(tc.name, func(t *testing.T) {
			c, lir := definedSetLIR(t, tc.src)
			if strings.Contains(lir, "  SET ") {
				t.Fatalf("a refused shape kept a SET line:\n%s", lir)
			}
			got := abReasons(c)
			if !slices.ContainsFunc(strings.Split(got, "\n"), func(r string) bool { return strings.HasSuffix(r, tc.reason) }) {
				t.Fatalf("want a refusal ending %q, got:\n%s", tc.reason, got)
			}
		})
	}
}

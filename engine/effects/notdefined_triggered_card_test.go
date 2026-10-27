package effects

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNotDefinedTriggeredCardPredicate pins Forge's NotDefinedTriggeredCard
// filter predicate (Jackdaw Savior, Puca's Covenant: "another target ...
// card"): the candidate is anything but the card the triggering event
// captured in TriggerContext.TriggerCard. An unbound referent fails closed --
// the spec matches nothing, and neither spelling inverts the absence.
func TestNotDefinedTriggeredCardPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	dead := corpusObject(t, reg, g, "Grizzly Bears")
	other := corpusObject(t, reg, g, "Hill Giant")
	if dead.ID == other.ID {
		t.Fatalf("precondition failed: dead and other share id %d", dead.ID)
	}
	bound := SpecContext{You: 0}
	bound.TriggerCard = dead.ID
	if bound.TriggerCard != dead.ID || !MatchesObjectCtx(g, "Creature.TriggeredCard", dead, bound) {
		t.Fatalf("precondition failed: the bound context must name the dead creature as TriggeredCard")
	}

	const spec = "Creature.YouOwn+NotDefinedTriggeredCard"
	if MatchesObjectCtx(g, spec, dead, bound) {
		t.Errorf("%s must not match the triggering card", spec)
	}
	if !MatchesObjectCtx(g, spec, other, bound) {
		t.Errorf("%s must match another creature card", spec)
	}
	unbound := SpecContext{You: 0}
	for _, s := range []string{spec, "Creature.YouOwn+!NotDefinedTriggeredCard"} {
		if MatchesObjectCtx(g, s, dead, unbound) || MatchesObjectCtx(g, s, other, unbound) {
			t.Errorf("%s with no bound TriggerCard must match nothing", s)
		}
	}
	if got := UnknownPredicates(spec); len(got) != 0 {
		t.Errorf("UnknownPredicates(%s) = %v, want empty (the census must agree with the matcher)", spec, got)
	}
}

// TestNotDefinedTriggeredAttackerPredicate pins Forge's TriggeredAttacker
// filter property and its NotDefined spelling (Saw: "a permanent other than
// the triggered attacker or CARDNAME"): the candidate is, or is anything but,
// the attacker captured in TriggerContext.TriggerCard. An unbound referent
// fails closed under every spelling.
func TestNotDefinedTriggeredAttackerPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	attacker := corpusObject(t, reg, g, "Grizzly Bears")
	other := corpusObject(t, reg, g, "Hill Giant")
	if attacker.ID == other.ID {
		t.Fatalf("precondition failed: attacker and other share id %d", attacker.ID)
	}
	if attacker.Zone != state.ZBattlefield || other.Zone != state.ZBattlefield {
		t.Fatalf("precondition failed: both creatures must be on the battlefield (%v, %v)", attacker.Zone, other.Zone)
	}
	bound := SpecContext{You: 0}
	bound.TriggerCard = attacker.ID

	const pos = "Creature.YouOwn+TriggeredAttacker"
	const neg = "Creature.YouOwn+NotDefinedTriggeredAttacker"
	if !MatchesObjectCtx(g, pos, attacker, bound) {
		t.Errorf("%s must match the captured attacker", pos)
	}
	if MatchesObjectCtx(g, pos, other, bound) {
		t.Errorf("%s must not match another creature", pos)
	}
	if MatchesObjectCtx(g, neg, attacker, bound) {
		t.Errorf("%s must not match the captured attacker", neg)
	}
	if !MatchesObjectCtx(g, neg, other, bound) {
		t.Errorf("%s must match another creature", neg)
	}
	unbound := SpecContext{You: 0}
	for _, s := range []string{pos, neg, "Creature.YouOwn+!NotDefinedTriggeredAttacker"} {
		if MatchesObjectCtx(g, s, attacker, unbound) || MatchesObjectCtx(g, s, other, unbound) {
			t.Errorf("%s with no bound TriggerCard must match nothing", s)
		}
	}
	for _, s := range []string{pos, neg} {
		if got := UnknownPredicates(s); len(got) != 0 {
			t.Errorf("UnknownPredicates(%s) = %v, want empty (the census must agree with the matcher)", s, got)
		}
	}
}

// notDefinedUnrecognised is the ratchet of corpus NotDefined<X> filter
// predicates the engine does not yet evaluate (each still fails closed). It
// only shrinks: an entry that becomes recognised must be deleted, and a new
// unrecognised NotDefined<X> in the corpus fails the census.
var notDefinedUnrecognised = []string{
	"NotDefinedOriginalHost",
	"NotDefinedParentTarget",
	"NotDefinedReplacedSimultaneousETB",
	"NotDefinedTriggeredAttackers",
}

var notDefinedTokenRe = regexp.MustCompile(`NotDefined[A-Za-z]+`)

// TestNotDefinedPredicateCensus walks the corpus for every NotDefined<X>
// filter predicate and checks the recognised/unrecognised split against
// notDefinedUnrecognised, so the family cannot silently grow an
// unrecognised spelling.
func TestNotDefinedPredicateCensus(t *testing.T) {
	root := cards.CorpusDir(testutil.CorpusDirOrSkip(t))
	seen := map[string]bool{}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".txt") {
			return err
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, tok := range notDefinedTokenRe.FindAllString(string(b), -1) {
			seen[tok] = true
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk corpus: %v", walkErr)
	}
	if !seen["NotDefinedTriggeredCard"] {
		t.Fatalf("precondition failed: the corpus carries no NotDefinedTriggeredCard (Jackdaw Savior, Puca's Covenant)")
	}
	var unknown []string
	for tok := range seen {
		if len(UnknownPredicates("Card."+tok)) > 0 {
			unknown = append(unknown, tok)
		}
	}
	sort.Strings(unknown)
	if !equalStringSlices(unknown, notDefinedUnrecognised) {
		t.Errorf("unrecognised NotDefined<X> predicates changed:\n  got  %v\n  want %v", unknown, notDefinedUnrecognised)
	}
}

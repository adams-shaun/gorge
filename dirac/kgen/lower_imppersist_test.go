package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestImpPersistLowers: an Imprinted list on the source outlives the
// resolution that writes it (Dino DNA reads what another ability imprinted;
// Journey to the Lost City and Sivriss, Nightmare Speaker write without a
// same-chain Cleanup). The persisting shape lowers under CardDef.ImpPersist.
// Mirror Golem stays refused: ForgetImprinted$ is outside that shape
// (ticket 04).
func TestImpPersistLowers(t *testing.T) {
	for _, name := range []string{"Dino DNA", "Journey to the Lost City", "Sivriss, Nightmare Speaker", "Saproling Burst"} {
		c := lowerCorpusCard(t, name)
		if r := probeRefusals(c); r != "" {
			t.Errorf("%s refused: %s", name, r)
		}
		if !c.ImpPersist {
			t.Errorf("%s lowered without CardDef.ImpPersist", name)
		}
	}
	if r := probeRefusals(lowerCorpusCard(t, "Mirror Golem")); r == "" {
		t.Errorf("Mirror Golem lowered; ForgetImprinted$ is outside the ImpPersist shape")
	}
}

// impPersistProbe lowers a synthetic script against the corpus registry (its
// TokenScript$ names a real token, as Saproling Burst's does).
func impPersistProbe(t *testing.T, name, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: testutil.CorpusRegistry(t)}
	return lowerCard(p, name, card, false)
}

// TestImpPersistLeavesReader: a token writer whose list is read by the
// source's own leaves trigger (Saproling Burst's shape) lowers as the
// persistent shape: gorge keeps Object.ImprintTokens through the departure
// for the trigger, and the kernel keeps the store records for the sn+1
// incarnation (zone_imprint.go impSeed, zone_srcmem.go smLiveFor).
func TestImpPersistLeavesReader(t *testing.T) {
	const src = `Name:Imprint Persist Leaves Probe
ManaCost:1 G
Types:Enchantment
A:AB$ Token | Cost$ T | TokenScript$ g_2_2_wolf | ImprintTokens$ True | SpellDescription$ Probe.
T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.Self | Execute$ DestroyRemembered | TriggerDescription$ Probe.
SVar:DestroyRemembered:DB$ Destroy | Defined$ Imprinted
`
	c := impPersistProbe(t, "Imprint Persist Leaves Probe", src)
	if r := probeRefusals(c); r != "" {
		t.Fatalf("leaves reader refused: %s", r)
	}
	if !c.ImpPersist {
		t.Fatal("leaves reader lowered without CardDef.ImpPersist")
	}
	read := false
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			read = read || in.Op == "Destroy" && in.Obj.Kind == rImprinted
		}
	}
	if !read {
		t.Fatal("the leaves trigger does not read the persistent Imprinted ref")
	}
}

// TestImpPersistFoldOnlyExileWriters: the IsImprinted+ExiledWithSource fold
// is sound only when the two lists coincide, i.e. every writer is a
// ChangeZone exile. A token writer puts the token in the imprint association
// but not in Object.ExiledWith, and gorge requires both words, so the clause
// must keep its ExiledWith conjunct (and refuse outside a mover) rather than
// fold to a filter that answers true for the token.
func TestImpPersistFoldOnlyExileWriters(t *testing.T) {
	const src = `Name:Imprint Persist Token Fold Probe
ManaCost:1 G
Types:Enchantment
A:AB$ Token | Cost$ T | TokenScript$ g_2_2_wolf | ImprintTokens$ True | SpellDescription$ Probe.
A:AB$ Pump | Cost$ 1 | ValidTgts$ Creature.IsImprinted+ExiledWithSource | TgtZone$ Exile | SpellDescription$ Probe.
`
	got := strings.Join(lowerScript(t, "Imprint Persist Token Fold Probe", src), "; ")
	if !strings.Contains(got, "ExiledWithSource") {
		t.Fatalf("the token writer's IsImprinted+ExiledWithSource clause folded: refusals %q", got)
	}
}

// TestImpPersistStaticReaderRefuses: an IsImprinted reader in a static (the
// battlefield walk, which passes no filter source) or in a trigger/replacement
// condition is outside the shape; the shape refuses rather than reading the
// last resolution's set or judging a zone-change candidate's live zone.
func TestImpPersistStaticReaderRefuses(t *testing.T) {
	const src = `Name:Imprint Persist Static Probe
ManaCost:1 G
Types:Enchantment
A:AB$ Token | Cost$ T | TokenScript$ g_2_2_wolf | ImprintTokens$ True | SpellDescription$ Probe.
A:AB$ PutCounter | Cost$ 1 | Defined$ Imprinted | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ Probe.
S:Mode$ Continuous | Affected$ Creature.IsImprinted | AddPower$ 1 | Description$ Probe.
`
	card, diags := cards.ParseBytes("probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	lc := &lowerCtx{face: card.Faces[0]}
	if persist, _ := lc.impPersistShape(); persist {
		t.Fatal("a static IsImprinted reader was admitted to the persistent shape")
	}
}

// TestDefinedImprintedRepeatOnlyPersist: Defined$ Imprinted inside a
// RepeatEach body takes the loop subject only on the persist shape; the
// one-resolution shapes stay refused (they have no persistent pile to fall
// back to when the subject is gone).
func TestDefinedImprintedRepeatOnlyPersist(t *testing.T) {
	lc := &lowerCtx{repeatPl: 1}
	if _, err := lc.definedImprinted(); err == nil {
		t.Fatal("a non-persist RepeatEach lowered Defined$ Imprinted to the loop subject")
	}
	lc.impPersist = true
	ref, err := lc.definedImprinted()
	if err != nil || ref.Kind != rEach {
		t.Fatalf("the persist RepeatEach lowered to %+v, %v; want rEach", ref, err)
	}
}

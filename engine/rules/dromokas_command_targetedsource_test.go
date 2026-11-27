package rules

// Dromoka's Command's first mode is a remembered-source prevention: the
// Effect's RememberObjects$ TargetedSource must capture the targeted spell so
// its Card.IsRemembered static grants "prevent all damage that would be dealt
// by CARDNAME." Without the capture the Effect remembers nothing, the grant
// matches no object, and the Bolt still deals 3.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// castWhenOffered submits the cast option for name as soon as it is offered,
// passing any priority round in between (the opponent's response window).
func castWhenOffered(t *testing.T, e *Engine, name string) {
	t.Helper()
	for i := 0; i < 20 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while waiting to cast %q", name)
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("decision while waiting to cast %q = %+v, want priority", name, d)
		}
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Label == "Cast "+name {
				submitChoices(t, e, o.Index)
				return
			}
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			t.Fatalf("priority with no cast for %q and no pass: %+v", name, d.Options)
		}
		submitChoices(t, e, pass)
	}
	t.Fatalf("never offered a cast of %q", name)
}

func TestDromokasCommandRememberedSourcePreventsSpellDamage(t *testing.T) {
	t.Parallel()
	boltSrc := corpusCardText(t, "l/lightning_bolt.txt")
	commandSrc := corpusCardText(t, "d/dromokas_command.txt")
	e, cfg, _ := newFixtureDeck(t, 741, commandSrc, boltSrc)
	ensureInHand(t, e, 0, "Lightning Bolt")
	ensureInHand(t, e, 0, "Dromoka's Command")
	addMana(t, e, 0, "GWR")

	theirLife := e.G.Players[1].Life

	// Bolt at seat 1, then the Command in response, targeting the Bolt.
	castWhenOffered(t, e, "Lightning Bolt")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Bolt target ask = %+v, want KTarget", d)
	}
	player := optionForPlayer(d, 1)
	if player < 0 {
		t.Fatalf("precondition: seat 1 is not offered as the Bolt's target: %+v", d.Options)
	}
	submitChoices(t, e, player)
	bolt := spellOnStack(t, e, "Lightning Bolt")
	if o := e.G.Obj(bolt); o == nil || o.Zone != state.ZStack {
		t.Fatalf("precondition: Bolt %d is not on the stack: %+v", bolt, o)
	}

	castWhenOffered(t, e, "Dromoka's Command")
	d = e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("Command mode ask = %+v, want KModes", d)
	}
	var modes []int
	for _, text := range []string{"Prevent all damage", "sacrifices an enchantment"} {
		idx := -1
		for _, o := range d.Options {
			if strings.Contains(o.Label, text) {
				idx = o.Index
				break
			}
		}
		if idx < 0 {
			t.Fatalf("mode %q absent from %+v", text, d.Options)
		}
		modes = append(modes, idx)
	}
	submitChoices(t, e, modes...)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 2 || d.Max != 2 {
		t.Fatalf("Command target ask = %+v, want the single 2-of-N KTarget", d)
	}
	boltIdx := indexOfObjOption(d, bolt)
	if boltIdx < 0 {
		t.Fatalf("precondition: Bolt %d is not offered as a target: %+v", bolt, d.Options)
	}
	player = optionForPlayer(d, 1)
	if player < 0 {
		t.Fatalf("precondition: seat 1 is not offered as a target: %+v", d.Options)
	}
	if boltIdx == player {
		t.Fatalf("precondition: the Bolt and seat 1 must be distinct choices")
	}
	submitChoices(t, e, boltIdx, player)

	passUntilStackEmpty(t, e, 60)
	drainPopPending(t, e)

	if got := theirLife - e.G.Players[1].Life; got != 0 {
		t.Fatalf("bolt dealt %d damage; want 0 (prevented)", got)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack still holds %v after resolution", e.G.Stack)
	}
	for _, ce := range e.continuous {
		if strings.Contains(ce.Affects, "IsRemembered") {
			t.Fatalf("a Card.IsRemembered registration survived resolution: %+v", ce)
		}
	}
	replayCheck(t, e, cfg)
}

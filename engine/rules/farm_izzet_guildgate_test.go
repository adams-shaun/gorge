package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm window class 3way:FX|G (FDN-s504/s505/s508, field `permanents`, Izzet
// Guildgate on the board): a creature its owner p0 holds under two Confiscates,
// p0's own and p1's, is seat 0's in gorge and seat 1's in Forge and XMage
// ("gorge c0 o0 High Fae Trickster vs forge c1 o0 High Fae Trickster @ setup").
// CR 613.7: control-changing effects apply in timestamp order and the later
// one wins. The setup places p0's Confiscate before p1's, so p1's is later and
// p1 controls the creature. gorge registered a static control grant only
// where it moved the bearer, so p0's older Confiscate, which changed nothing
// when it was attached, registered on a later emit, after p1's grant, and won
// by registration order. Izzet Guildgate is only a bystander on those boards.
func TestFarmIzzetGuildgate(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	run := func(t *testing.T, scenario string) map[string]OracleSnapPerm {
		t.Helper()
		res, err := RunOracleScenarioJSON(reg, []byte(scenario))
		if err != nil {
			t.Fatalf("scenario JSON: %v", err)
		}
		if len(res.Fails) != 0 {
			t.Fatalf("%v", res.Fails)
		}
		out := map[string]OracleSnapPerm{}
		for _, p := range res.Snapshots[len(res.Snapshots)-1].Permanents {
			out[p.Ref] = p
		}
		return out
	}
	t.Run("opponent's later Confiscate wins over the owner's earlier one", func(t *testing.T) {
		t.Parallel()
		b := run(t, `{
		  "name": "farm-izzet-guildgate-two-confiscates",
		  "setup": {
		    "p0": {"battlefield": ["High Fae Trickster", "Confiscate", "Izzet Guildgate"], "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}]},
		    "p1": {"battlefield": ["Confiscate"], "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}]}
		  },
		  "steps": [{"op": "pass", "seat": 0}]
		}`)
		// Precondition: both Auras are on the creature, so the two statics compete.
		for _, ref := range []string{"p0:Confiscate", "p1:Confiscate"} {
			if b[ref].AttachedTo != "p0:High Fae Trickster" {
				t.Fatalf("%s attached to %q, want the Trickster", ref, b[ref].AttachedTo)
			}
		}
		if got := b["p0:High Fae Trickster"]; got.Owner != 0 || got.Controller != 1 {
			t.Fatalf("Trickster is c%d o%d, want c1 o0 (p1's later Confiscate, CR 613.7)", got.Controller, got.Owner)
		}
	})
	t.Run("an older Confiscate re-attached later wins by its new timestamp", func(t *testing.T) {
		t.Parallel()
		b := run(t, `{
		  "name": "farm-izzet-guildgate-reattached-confiscate",
		  "setup": {
		    "p0": {"battlefield": ["High Fae Trickster", "Grizzly Bears", "Confiscate"], "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}]},
		    "p1": {"battlefield": ["Confiscate"], "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}]}
		  },
		  "steps": [
		    {"op": "attach", "seat": 0, "card": "p0:Confiscate", "attached_to": "p0:Grizzly Bears"},
		    {"op": "attach", "seat": 0, "card": "p0:Confiscate", "attached_to": "p0:High Fae Trickster"},
		    {"op": "pass", "seat": 0}
		  ]
		}`)
		// Precondition: the returned Aura is the latest effect on the bearer
		// (CR 613.7e restamps it on each attach), so it must beat p1's Aura.
		if b["p0:Confiscate"].AttachedTo != "p0:High Fae Trickster" {
			t.Fatalf("p0:Confiscate attached to %q, want the Trickster", b["p0:Confiscate"].AttachedTo)
		}
		if got := b["p0:High Fae Trickster"]; got.Owner != 0 || got.Controller != 0 {
			t.Fatalf("Trickster is c%d o%d, want c0 o0 (the re-attached Confiscate's new layer timestamp, CR 613.7e)", got.Controller, got.Owner)
		}
	})
	t.Run("an Act of Treason above the statics survives the newer static leaving", func(t *testing.T) {
		t.Parallel()
		b := run(t, `{
		  "name": "farm-izzet-guildgate-treason-over-statics",
		  "players": 3,
		  "setup": {
		    "p0": {"battlefield": ["Grizzly Bears", "Confiscate"], "attached": [{"card": "Confiscate", "to": "p0:Grizzly Bears"}]},
		    "p1": {"battlefield": ["Confiscate"], "attached": [{"card": "Confiscate", "to": "p0:Grizzly Bears"}]},
		    "p2": {"hand": ["Act of Treason"], "mana": "RRR"}
		  },
		  "steps": [
		    {"op": "pass_to", "seat": 2, "step": "main1", "active": "p2"},
		    {"op": "cast", "seat": 2, "card": "p2:Act of Treason", "mana": "RRR", "targets": ["p0:Grizzly Bears"]},
		    {"op": "resolve", "seat": 2},
		    {"op": "move", "seat": 2, "card": "p1:Confiscate", "to": "graveyard"},
		    {"op": "pass", "seat": 2}
		  ]
		}`)
		// Precondition: the newer Aura really left, and the older one holds.
		if b["p1:Confiscate"].Ref != "" {
			t.Fatalf("p1:Confiscate still on the battlefield: %+v", b["p1:Confiscate"])
		}
		if b["p0:Confiscate"].AttachedTo != "p0:Grizzly Bears" {
			t.Fatalf("p0:Confiscate attached to %q, want the Bears", b["p0:Confiscate"].AttachedTo)
		}
		if got := b["p0:Grizzly Bears"]; got.Owner != 0 || got.Controller != 2 {
			t.Fatalf("Bears is c%d o%d, want c2 o0 (Act of Treason is later than either static)", got.Controller, got.Owner)
		}
	})
	t.Run("a later own Confiscate wins over the opponent's earlier one", func(t *testing.T) {
		t.Parallel()
		b := run(t, `{
		  "name": "farm-izzet-guildgate-later-own-confiscate",
		  "setup": {
		    "p0": {"battlefield": ["Confiscate"], "attached": [{"card": "Confiscate", "to": "p1:High Fae Trickster"}]},
		    "p1": {"battlefield": ["High Fae Trickster", "Confiscate"], "attached": [{"card": "Confiscate", "to": "p1:High Fae Trickster"}]}
		  },
		  "steps": [{"op": "pass", "seat": 0}]
		}`)
		for _, ref := range []string{"p0:Confiscate", "p1:Confiscate"} {
			if b[ref].AttachedTo != "p1:High Fae Trickster" {
				t.Fatalf("%s attached to %q, want the Trickster", ref, b[ref].AttachedTo)
			}
		}
		if got := b["p1:High Fae Trickster"]; got.Owner != 1 || got.Controller != 1 {
			t.Fatalf("Trickster is c%d o%d, want c1 o1 (p1's later Confiscate, CR 613.7)", got.Controller, got.Owner)
		}
	})
	t.Run("a re-appearing Fealty want stays below a later Act of Treason", func(t *testing.T) {
		t.Parallel()
		reg := testutil.CorpusRegistry(t)
		ft, ok := reg.Lookup("Fealty to the Realm")
		if !ok {
			t.Fatal("corpus missing Fealty to the Realm")
		}
		treason, ok := reg.Lookup("Act of Treason")
		if !ok {
			t.Fatal("corpus missing Act of Treason")
		}
		e, cfg, bear := staticGainControlGame(t, 7341, []*cards.Card{ft}, 1, treason)
		castControlAura(t, e, "Fealty to the Realm", bear)
		if !e.G.IsMonarch(0) {
			t.Fatalf("precondition: monarch = %d/%v, want seat 0 (Fealty's ETB)", e.G.Monarch, e.G.HasMonarch)
		}
		if got := e.G.Obj(bear).Controller; got != 0 {
			t.Fatalf("precondition: bear controller %d, want 0 (the monarch)", got)
		}
		// Seat 1's later until-EOT steal, resolved off the real card face.
		trID := findAndMoveToHand(t, e, 1, "Act of Treason")
		addMana(t, e, 1, "RRR")
		trObj := e.G.Obj(trID)
		effects.Resolve(e, &effects.Ctx{Source: trID, Controller: 1,
			Targets: []state.Target{{Obj: bear}}, TargetsOffered: true,
			SVars: trObj.Face().SVars}, trObj.Face().SpellAbility())
		if got := e.G.Obj(bear).Controller; got != 1 {
			t.Fatalf("precondition: bear controller after the steal %d, want 1", got)
		}
		// The monarch designation moves to seat 1 and back. The Fealty want
		// disappears and re-appears with its ORIGINAL layer timestamp (CR 613.7d),
		// so Act of Treason, which began later, must stay on top.
		e.emit(events.Event{Kind: events.MonarchChange, Player: 1})
		e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
		if got := e.G.Obj(bear).Controller; got != 1 {
			t.Fatalf("bear controller after the monarch returned = %d, want 1 (Act of Treason is the later effect, CR 613.7)", got)
		}
		replayCheck(t, e, cfg)
	})
}

package rules

// Regressions for the may-play grant of a FLASH CREATURE exiled on the
// controller's own turn: the grant (`Duration$ UntilTheEndOfYourNextTurn |
// ForgetOnMoved$ Exile` delivered by a DB$ Effect; Blazing Crescendo's dig and
// Hex Magic's hand exile are the recorded shapes) runs to the end of the
// controller's NEXT turn (CR 611.2a), the intervening opponent's turn does not
// end it, and the card's Flash makes the cast legal at any priority decision
// (CR 702.8a, 117.1a) -- including the opponent's-turn and the next-turn
// upkeep. A non-Flash creature exiled by the same grant stays sorcery-timing:
// it is NOT offered at an upkeep decision, only in a main phase.

import "testing"

// TestMayPlayExileFlashOfferedAtUpkeep: Hex Magic's shape -- one grant
// remembering several exiled cards. The Flash creature is offered at every
// funded priority decision across the opponent's turn and the controller's
// next-turn upkeep; the non-Flash creature only in a main phase; the exiled
// instant wherever the mana covers it.
func TestMayPlayExileFlashOfferedAtUpkeep(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "hex-magic-flash-creature-offered-at-upkeep",
	  "setup": {"p0": {"hand": ["Hex Magic", "Aboleth Spawn", "Grizzly Bears", "Lightning Bolt"],
	                   "battlefield": ["Island", "Island", "Mountain", "Forest", "Forest"],
	                   "library_top": ["Forest", "Forest", "Forest", "Forest", "Forest"]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Hex Magic", "mana": "CCR"},
	    {"op": "resolve", "expect": [{"card": "p0:Aboleth Spawn", "zone": "exile"},
	                                 {"card": "p0:Grizzly Bears", "zone": "exile"},
	                                 {"card": "p0:Lightning Bolt", "zone": "exile"}]},
	    {"op": "mana", "seat": 0, "mana": "UUG",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Aboleth Spawn"}, "want": true},
	                {"offered": {"seat": 0, "kind": "cast", "card": "p0:Grizzly Bears"}, "want": true},
	                {"offered": {"seat": 0, "kind": "cast", "card": "p0:Lightning Bolt"}, "want": false}]},
	    {"op": "mana", "seat": 0, "mana": "R",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Lightning Bolt"}, "want": true}]},
	    {"op": "pass_to", "step": "upkeep", "active": "p1"},
	    {"op": "pass", "seat": 1},
	    {"op": "mana", "seat": 0, "mana": "UUG",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Aboleth Spawn"}, "want": true},
	                {"offered": {"seat": 0, "kind": "cast", "card": "p0:Grizzly Bears"}, "want": false},
	                {"offered": {"seat": 0, "kind": "cast", "card": "p0:Lightning Bolt"}, "want": false}]},
	    {"op": "pass_to", "step": "upkeep", "active": "p0"},
	    {"op": "mana", "seat": 0, "mana": "UUG",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Aboleth Spawn"}, "want": true},
	                {"offered": {"seat": 0, "kind": "cast", "card": "p0:Grizzly Bears"}, "want": false},
	                {"offered": {"seat": 0, "kind": "cast", "card": "p0:Lightning Bolt"}, "want": false}]},
	    {"op": "pass_to", "step": "main1", "active": "p0"},
	    {"op": "cast", "seat": 0, "card": "p0:Aboleth Spawn", "mana": "UUG"},
	    {"op": "resolve"}
	  ],
	  "expect": [{"card": "p0:Aboleth Spawn", "zone": "battlefield"}]
	}`)
}

// TestMayPlayExileFlashBlazingCrescendo: the dig shape -- the recorded game's
// Blazing Crescendo exiled the Flash creature and granted the play through its
// own DB$ Effect. The cast is offered right after the resolution, across the
// opponent's turn, at the controller's next-turn upkeep, and is exercisable
// there.
func TestMayPlayExileFlashBlazingCrescendo(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "blazing-crescendo-flash-creature-offered-at-upkeep",
	  "setup": {"p0": {"hand": ["Blazing Crescendo"],
	                   "battlefield": ["Grizzly Bears", "Island"],
	                   "library_top": ["Aboleth Spawn", "Forest", "Forest"]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Blazing Crescendo", "mana": "CR", "targets": ["p0:Grizzly Bears"]},
	    {"op": "resolve", "expect": [{"card": "p0:Aboleth Spawn", "zone": "exile"}]},
	    {"op": "mana", "seat": 0, "mana": "UUU",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Aboleth Spawn"}, "want": true}]},
	    {"op": "pass_to", "step": "upkeep", "active": "p1"},
	    {"op": "pass", "seat": 1},
	    {"op": "mana", "seat": 0, "mana": "UUU",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Aboleth Spawn"}, "want": true}]},
	    {"op": "pass_to", "step": "upkeep", "active": "p0"},
	    {"op": "mana", "seat": 0, "mana": "UUU",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Aboleth Spawn"}, "want": true}]},
	    {"op": "pass_to", "step": "main1", "active": "p0"},
	    {"op": "cast", "seat": 0, "card": "p0:Aboleth Spawn", "mana": "UUU"},
	    {"op": "resolve"}
	  ],
	  "expect": [{"card": "p0:Aboleth Spawn", "zone": "battlefield"}]
	}`)
}

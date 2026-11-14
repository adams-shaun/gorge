package rules

// Focused regressions for the modes AGENTS.md's Effect StaticAbilities$/
// Triggers$ row tracks: each is an Effect-delivered mode the row once called
// unimplemented, run through the inline Oracle harness against the real corpus
// and replayed from its log. The committed census that pins which modes still
// Note is rules/effect_static_census_test.go.

import "testing"

// TestEffectStaticChandraFlameshaper: the +2's Effect grants a continuous static
// that lets its controller play the chosen card from exile (forgotten once it
// leaves exile), and it is real: the card chosen during resolution is castable
// from exile.
func TestEffectStaticChandraFlameshaper(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "chandra-flameshaper-chosen-card-castable-from-exile",
	  "setup": {"p0": {"battlefield": ["Chandra, Flameshaper"],
	                   "library_top": ["Lightning Bolt", "Forest", "Grizzly Bears", "Craw Wurm"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Chandra, Flameshaper", "ability": "Add RRR"},
	    {"op": "resolve", "answers": [{"kind": "any", "pick": ["p0:Lightning Bolt"]}],
	     "expect": [{"card": "p0:Lightning Bolt", "zone": "exile"}]},
	    {"op": "cast", "seat": 0, "card": "p0:Lightning Bolt", "targets": ["p1"]},
	    {"op": "resolve"}
	  ],
	  "expect": [{"life": {"p1": 17}}, {"card": "p0:Lightning Bolt", "zone": "graveyard"}]
	}`)
}

// TestEffectStaticStrongboxRaider: the enter ability's `Play:Mode$ Continuous |
// MayPlay$ True | Duration$ UntilTheEndOfYourNextTurn | ForgetOnMoved$ Exile`
// grant keeps the chosen card playable through the opponent's turn and the
// controller's next turn.
func TestEffectStaticStrongboxRaider(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "strongbox-raider-playable-through-next-turn",
	  "setup": {"p0": {"hand": ["Strongbox Raider"], "battlefield": ["Grizzly Bears"],
	                   "library_top": ["Lightning Bolt", "Shock"]}},
	  "steps": [
	    {"op": "attack", "seat": 0, "attackers": ["p0:Grizzly Bears"], "defender": "p1"},
	    {"op": "pass_to", "step": "main2"},
	    {"op": "cast", "seat": 0, "card": "p0:Strongbox Raider", "mana": "CCRR"},
	    {"op": "resolve", "answers": [{"kind": "choose", "pick": ["p0:Lightning Bolt"]}]},
	    {"op": "pass_to", "step": "upkeep", "active": "p1"},
	    {"op": "pass", "seat": 1},
	    {"op": "mana", "seat": 0, "mana": "R",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Lightning Bolt"}, "want": true}]},
	    {"op": "pass_to", "step": "main1", "active": "p0"},
	    {"op": "cast", "seat": 0, "card": "p0:Lightning Bolt", "mana": "R", "targets": ["p1"]},
	    {"op": "resolve"}
	  ],
	  "expect": [{"life": {"p1": 15}}, {"card": "p0:Lightning Bolt", "zone": "graveyard"}]
	}`)
}

// TestEffectStaticKellanPlanarTrailblazer: the gained Effect's continuous
// static, which lets its controller play the remembered card from exile
// (forgotten once it leaves exile), lets the exiled top card be cast this turn.
func TestEffectStaticKellanPlanarTrailblazer(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "kellan-planar-trailblazer-exiled-card-castable",
	  "setup": {"p0": {"battlefield": ["Kellan, Planar Trailblazer"],
	                   "library_top": ["Lightning Bolt", "Shock"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Kellan, Planar Trailblazer", "mana": "CR", "ability": "Detective"},
	    {"op": "resolve"},
	    {"op": "attack", "seat": 0, "attackers": ["p0:Kellan, Planar Trailblazer"], "defender": "p1"},
	    {"op": "pass_to", "step": "main2",
	     "expect": [{"card": "p0:Lightning Bolt", "zone": "exile"}]},
	    {"op": "cast", "seat": 0, "card": "p0:Lightning Bolt", "mana": "R", "targets": ["p1"]},
	    {"op": "resolve"}
	  ],
	  "expect": [{"life": {"p0": 20, "p1": 15}}, {"card": "p0:Lightning Bolt", "zone": "graveyard"}]
	}`)
}

// TestEffectStaticZulAshurLichLord: the activated ability's Effect grants a
// continuous static that lets its controller play the remembered card from the
// graveyard (exiled if it would leave it), so the targeted Zombie can be cast
// from the graveyard.
func TestEffectStaticZulAshurLichLord(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "zul-ashur-lich-lord-cast-zombie-from-graveyard",
	  "setup": {"p0": {"battlefield": ["Zul Ashur, Lich Lord"], "graveyard": ["Walking Corpse"]}},
	  "steps": [
	    {"op": "mana", "seat": 0, "mana": "CB",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Walking Corpse"}, "want": false}]},
	    {"op": "activate", "seat": 0, "card": "p0:Zul Ashur, Lich Lord", "targets": ["p0:Walking Corpse"]},
	    {"op": "resolve"},
	    {"op": "cast", "seat": 0, "card": "p0:Walking Corpse"},
	    {"op": "resolve"}
	  ],
	  "expect": [{"card": "p0:Walking Corpse", "zone": "battlefield"},
	             {"card": "p0:Zul Ashur, Lich Lord", "tapped": true}, {"pool": {"p0": ""}}]
	}`)
}

// TestEffectStaticKaitoCunningInfiltrator: the +1's Effect grants a CantBlockBy
// static on the remembered attacker, which makes the targeted creature
// unblockable this turn while another attacker still can be blocked.
func TestEffectStaticKaitoCunningInfiltrator(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "kaito-cunning-infiltrator-target-unblockable",
	  "setup": {"p0": {"battlefield": ["Kaito, Cunning Infiltrator", "Grizzly Bears", "Savannah Lions"],
	                   "hand": ["Forest"], "library_top": ["Island"]},
	            "p1": {"battlefield": ["Hill Giant"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Kaito, Cunning Infiltrator",
	     "targets": ["p0:Grizzly Bears"], "ability": "can't be blocked"},
	    {"op": "resolve", "answers": [{"kind": "any", "pick": ["p0:Forest"]}]},
	    {"op": "attack", "seat": 0, "attackers": ["p0:Grizzly Bears", "p0:Savannah Lions"], "defender": "p1"},
	    {"op": "pass_to", "decision": "blockers",
	     "expect": [{"can_block": {"blocker": "p1:Hill Giant", "attacker": "p0:Grizzly Bears"}, "want": false},
	                {"can_block": {"blocker": "p1:Hill Giant", "attacker": "p0:Savannah Lions"}, "want": true}]}
	  ],
	  "expect": [{"card": "p0:Kaito, Cunning Infiltrator", "counters": {"loyalty": 4}}]
	}`)
}

// TestEffectStaticVampireGourmand: the attack trigger's `CantBlockBy`
// unblockable-this-turn grant makes the Gourmand unblockable after it
// sacrifices a creature.
func TestEffectStaticVampireGourmand(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "vampire-gourmand-sacrifice-makes-unblockable",
	  "setup": {"p0": {"battlefield": ["Vampire Gourmand", "Grizzly Bears", "Savannah Lions"]},
	            "p1": {"battlefield": ["Centaur Courser"]}},
	  "steps": [
	    {"op": "attack", "seat": 0, "attackers": ["p0:Vampire Gourmand", "p0:Savannah Lions"], "defender": "p1"},
	    {"op": "resolve", "answers": [{"kind": "choose", "pick": ["Sacrifice another creature"]},
	                                  {"kind": "choose", "pick": ["p0:Grizzly Bears"]}]},
	    {"op": "pass_to", "decision": "blockers",
	     "expect": [{"can_block": {"blocker": "p1:Centaur Courser", "attacker": "p0:Vampire Gourmand"}, "want": false},
	                {"can_block": {"blocker": "p1:Centaur Courser", "attacker": "p0:Savannah Lions"}, "want": true}]}
	  ],
	  "expect": [{"card": "p0:Grizzly Bears", "zone": "graveyard"}, {"hand_size": {"p0": 1}}]
	}`)
}

// TestEffectStaticVivienReid: the -8 emblem's permanent-duration continuous
// static (+2/+2, vigilance, trample and indestructible) pumps the controller's
// creatures and survives Vivien leaving the battlefield.
func TestEffectStaticVivienReid(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "vivien-reid-emblem-pumps-creatures",
	  "setup": {"p0": {"battlefield": ["Vivien Reid", "Grizzly Bears"]},
	            "p1": {"battlefield": ["Grizzly Bears"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Vivien Reid", "ability": "Look at the top four"},
	    {"op": "resolve"},
	    {"op": "pass_to", "step": "upkeep", "active": "p1"},
	    {"op": "pass_to", "step": "main1", "active": "p0"},
	    {"op": "activate", "seat": 0, "card": "p0:Vivien Reid", "ability": "Look at the top four"},
	    {"op": "resolve"},
	    {"op": "pass_to", "step": "upkeep", "active": "p1"},
	    {"op": "pass_to", "step": "main1", "active": "p0"},
	    {"op": "activate", "seat": 0, "card": "p0:Vivien Reid", "ability": "Look at the top four"},
	    {"op": "resolve"},
	    {"op": "pass_to", "step": "upkeep", "active": "p1"},
	    {"op": "pass_to", "step": "main1", "active": "p0",
	     "expect": [{"card": "p0:Vivien Reid", "counters": {"loyalty": 8}}]},
	    {"op": "activate", "seat": 0, "card": "p0:Vivien Reid", "ability": "emblem"},
	    {"op": "resolve"}
	  ],
	  "expect": [{"card": "p0:Vivien Reid", "zone": "graveyard"},
	             {"card": "p0:Grizzly Bears", "pt": "4/4",
	              "keywords": ["Vigilance", "Trample", "Indestructible"]},
	             {"card": "p1:Grizzly Bears", "pt": "2/2"}]
	}`)
}

// TestEffectStaticKaitoCunningInfiltratorEmblem: the -9 emblem's
// permanent-duration Effect, which carries a spell-cast trigger, is real. The +1 CantBlockBy grant is covered by the test above; this is the
// second behaviour the card routes through Effect. After the emblem resolves,
// casting a spell makes a 2/1 blue Ninja token, and the emblem keeps firing
// after Kaito has left the battlefield (a permanent-duration Effect trigger,
// not a delayed one).
func TestEffectStaticKaitoCunningInfiltratorEmblem(t *testing.T) {
	t.Parallel()
	runInlineOracle(t, `{
	  "name": "kaito-cunning-infiltrator-emblem-ninja-on-spell-cast",
	  "setup": {"p0": {"battlefield": ["Kaito, Cunning Infiltrator"],
	                   "hand": ["Lightning Bolt"],
	                   "counters": {"Kaito, Cunning Infiltrator": {"LOYALTY": 9}}}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Kaito, Cunning Infiltrator", "ability": "emblem"},
	    {"op": "resolve",
	     "expect": [{"card": "p0:Kaito, Cunning Infiltrator", "counters": {"loyalty": 3}}]},
	    {"op": "cast", "seat": 0, "card": "p0:Lightning Bolt", "mana": "R", "targets": ["p1"]},
	    {"op": "resolve"}
	  ],
	  "expect": [{"card": "p0:token:Ninja", "zone": "battlefield", "pt": "2/1"},
	             {"life": {"p1": 17}}]
	}`)
}

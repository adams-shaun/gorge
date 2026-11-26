package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEffectContainerMayPlay covers the Effect-container MayPlay carriers
// this slice lowers:
//
//   - a no-target, no-RememberObjects$ Effect whose one Continuous MayPlay$
//     static is a zone permission (Chainer, Liliana, Chandra Flame's
//     Catalyst, Wish): the static lowers through staticBody to the
//     MayPlayFrom static of an emblem, as printed permissions do;
//   - Nivix's remembered exiled instant-or-sorcery card with
//     ExileOnMoved$ Exile and Duration$ UntilYourNextTurn;
//   - Mission Briefing's chosen-card permission marked for the
//     graveyard-replacement exile (ExileOnMoved$ Stack).
//
// The shapes gorge implements nothing for stay refused by name in the
// second half: a shape is never silently dropped.
func TestEffectContainerMayPlay(t *testing.T) {
	// The zone-permission group: each lowers to one emblem carrying one
	// MayPlayFrom static in the right zone.
	for _, tc := range []struct {
		name  string
		zone  int
		flags string
	}{
		{"Chainer, Nightmare Adept", zGraveyard, "OncePerTurn"},
		{"Liliana, Untouched by Death", zGraveyard, ""},
		{"Chandra, Flame's Catalyst", zHand, "WithoutManaCost"},
		{"Wish", zSideboard, "OncePerTurn"},
	} {
		c := lowerNamedCard(t, tc.name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", tc.name, c.Refusals)
			continue
		}
		if len(c.Emblems) != 1 {
			t.Errorf("%s emblems = %d, want 1", tc.name, len(c.Emblems))
			continue
		}
		e := c.Emblems[0]
		if len(e.Statics) != 1 || e.Statics[0].Kind != "MayPlayFrom" {
			t.Errorf("%s emblem statics = %+v, want one MayPlayFrom", tc.name, e.Statics)
			continue
		}
		if e.Statics[0].Zone != tc.zone {
			t.Errorf("%s MayPlayFrom zone = %d, want %d", tc.name, e.Statics[0].Zone, tc.zone)
		}
		if tc.flags != "" && !mayPlayHasFlag(e.Statics[0].Flags, tc.flags) {
			t.Errorf("%s MayPlayFrom flags = %v, want %s", tc.name, e.Statics[0].Flags, tc.flags)
		}
	}

	// Nivix: a remembered exile permission for instants and sorceries,
	// until the controller's next turn.
	nivix := lowerNamedCard(t, "Nivix, Aerie of the Firemind")
	if len(nivix.Refusals) != 0 {
		t.Errorf("Nivix refusals = %v, want none", nivix.Refusals)
	} else if mp := firstProgOp(nivix, "MayPlay"); mp == nil {
		t.Error("Nivix lowers no MayPlay instruction")
	} else if mp.Obj.Kind != rRemembered {
		t.Errorf("Nivix MayPlay obj = %v, want Remembered", mp.Obj)
	} else if !hasMayPlayInstSorc(nivix) {
		t.Error("Nivix MayPlay does not restrict the offer to instants and sorceries")
	}

	// The Effect-delivered graveyard-to-exile replacement: Yawgmoth's Will,
	// Gaea's Will and Magus of the Will each lower one emblem whose one
	// static is the graveyard may-play permission and whose one replacement
	// is the any-zone dies-to-exile rewrite (ReplExileOnLeave, Repl.C 1),
	// until end of turn.
	for _, name := range []string{"Yawgmoth's Will", "Gaea's Will", "Magus of the Will"} {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		if len(c.Emblems) != 1 {
			t.Errorf("%s emblems = %d, want 1", name, len(c.Emblems))
			continue
		}
		e := c.Emblems[0]
		if !e.UntilEOT {
			t.Errorf("%s emblem UntilEOT = false, want the no-Duration$ lifetime", name)
		}
		if len(e.Statics) != 1 || e.Statics[0].Kind != "MayPlayFrom" || e.Statics[0].Zone != zGraveyard {
			t.Errorf("%s emblem statics = %+v, want one MayPlayFrom over the graveyard", name, e.Statics)
			continue
		}
		if mayPlayHasFlag(e.Statics[0].Flags, "OncePerTurn") {
			t.Errorf("%s MayPlayFrom flags = %v, want no once-per-turn limit", name, e.Statics[0].Flags)
		}
		if len(e.Repls) != 1 || e.Repls[0].Kind != "ExileOnLeave" || !e.Repls[0].AnyZone {
			t.Errorf("%s emblem repls = %+v, want one any-zone ExileOnLeave", name, e.Repls)
		}
	}

	// Memory Vessel: the multi-static remembered Effect. The per-owner
	// permission over the remembered exiled cards lowers to the MayPlay
	// instruction the remembered grant route emits (each player may play the
	// cards THEY exiled, until the controller's next turn); the two hand
	// locks lower onto one emblem whose lifetime is that same boundary.
	mv := lowerNamedCard(t, "Memory Vessel")
	if len(mv.Refusals) != 0 {
		t.Errorf("Memory Vessel refusals = %v, want none", mv.Refusals)
	} else {
		if mp := firstProgOp(mv, "MayPlay"); mp == nil {
			t.Error("Memory Vessel lowers no MayPlay instruction")
		} else {
			if mp.Obj.Kind != rRemembered {
				t.Errorf("Memory Vessel MayPlay obj = %v, want Remembered", mp.Obj)
			}
			if mp.C != zExile {
				t.Errorf("Memory Vessel MayPlay zone = %d, want exile", mp.C)
			}
			if mp.CS != "UntilYourNextTurn" {
				t.Errorf("Memory Vessel MayPlay lifetime = %q, want UntilYourNextTurn", mp.CS)
			}
			if int(mp.B.N)&int(kop.MayPlayOwner) == 0 {
				t.Errorf("Memory Vessel MayPlay flags = %v, want MayPlayOwner", mp.B.N)
			}
		}
		if len(mv.Emblems) != 1 {
			t.Errorf("Memory Vessel emblems = %d, want 1", len(mv.Emblems))
		} else {
			e := mv.Emblems[0]
			if !e.UntilNextTurn || e.UntilEOT {
				t.Errorf("Memory Vessel emblem lifetime = UntilEOT %v UntilNextTurn %v, want UntilNextTurn", e.UntilEOT, e.UntilNextTurn)
			}
			if len(e.Statics) != 2 {
				t.Errorf("Memory Vessel emblem statics = %+v, want the two hand locks", e.Statics)
			} else {
				if e.Statics[0].Kind != "CantPlayLand" || e.Statics[0].Zone != zHand {
					t.Errorf("Memory Vessel emblem static[0] = %+v, want CantPlayLand over the hand", e.Statics[0])
				}
				if e.Statics[1].Kind != "CantBeCast" || e.Statics[1].Zone != zHand {
					t.Errorf("Memory Vessel emblem static[1] = %+v, want CantBeCast over the hand", e.Statics[1])
				}
			}
		}
	}

	// Mission Briefing: the chosen card, marked so the stack exit exiles it.
	mission := lowerNamedCard(t, "Mission Briefing")
	if len(mission.Refusals) != 0 {
		t.Errorf("Mission Briefing refusals = %v, want none", mission.Refusals)
	} else {
		if mp := firstProgOp(mission, "MayPlay"); mp == nil {
			t.Error("Mission Briefing lowers no MayPlay instruction")
		} else if mp.Obj.Kind != rChosen {
			t.Errorf("Mission Briefing MayPlay obj = %v, want Chosen", mp.Obj)
		}
		if mk := firstProgOp(mission, "StackExileMark"); mk == nil {
			t.Error("Mission Briefing lowers no StackExileMark")
		} else if mk.Obj.Kind != rChosen {
			t.Errorf("Mission Briefing StackExileMark obj = %v, want Chosen", mk.Obj)
		}
	}

}

// TestEffectContainerMayPlayRemainingRefusals pins the carriers whose
// shape needs a kernel primitive this slice does not add: gorge implements
// them, so lowering only part of each would diverge. Each stays refused by
// name until its own ticket.
func TestEffectContainerMayPlayRemainingRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		why  string
	}{
		// permanent remembered permission plus a colourless-mana
		// ManaConvert static (no kernel C->AnyColor conversion)
		{"Abstruse Appropriation", "ManaConvert"},
		// target remembered permission plus a literal {8} mana RaiseCost
		// (no kernel mana raise on a may-play permission)
		{"Mavinda, Students' Advocate", "RaiseCost"},
		// EffectOwner$ Player.Opponent's opponent emblem: the kernel's
		// emblem trigger diverges from gorge's in lockstep (the follow-up
		// ticket carries the measured reproduction)
		{"Chandra, Awakened Inferno", "EffectOwner"},
		// the multi-static Effects whose primitives the kernel lacks: the
		// colourless-mana ManaConvert arm (Abstruse), the {8} mana raise on
		// a may-play permission (Mavinda); each keeps today's named refusal
		// until its own ticket lands the primitive
		{"Abstruse Appropriation", "ManaConvert"},
		{"Mavinda, Students' Advocate", "RaiseCost"},
	} {
		c := lowerNamedCard(t, tc.name)
		if len(c.Refusals) == 0 {
			t.Errorf("%s lowers; want its named refusal (needs %s)", tc.name, tc.why)
			continue
		}
		var got []string
		for _, r := range c.Refusals {
			got = append(got, r.Reason)
		}
		if joined := strings.Join(got, "; "); !strings.Contains(joined, tc.why) {
			t.Errorf("%s refusals = %q, want one mentioning %q", tc.name, joined, tc.why)
		}
	}
}

// firstProgOp returns the first emitted instruction with op in the card's
// abilities (a card-local helper; the lowering tests read the instruction
// stream this way).
func firstProgOp(c *lCard, op string) *lInstr {
	for i := range c.Abilities {
		for j := range c.Abilities[i].Prog {
			if c.Abilities[i].Prog[j].Op == op {
				return &c.Abilities[i].Prog[j]
			}
		}
	}
	return nil
}

func mayPlayHasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

// hasMayPlayInstSorc reports the inst/sorc flag on the MayPlay instruction's
// B operand (kop.MayPlayInstSorc).
func hasMayPlayInstSorc(c *lCard) bool {
	if mp := firstProgOp(c, "MayPlay"); mp != nil {
		return mp.B.Kind == vConst && int(mp.B.N)&int(kop.MayPlayInstSorc) != 0
	}
	return false
}

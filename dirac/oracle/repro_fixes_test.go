package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Games that once diverged at the end of their script, on the surface they
// diverged on: both engines now agree there, and for 200 decisions after.
func TestStdMkmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-mkm", []stdRepro{
		{"deathtouch_wither_hit", true},                // CR 704.5h: a counter-form hit marks no damage, and the mark alone is lethal
		{"dealt_once_each_damage", true},               // EachDamage lands on one referent twice: one trigger with the batch's total
		{"wither_from_departed_source", true},          // a dying Shady Informant's trigger keeps the wither Massacre Girl gave it
		{"potential_once_each_turn_mana", true},        // Gravestone Strider's {1} mana ability counts toward a trigger's own cost
		{"hold_out_ends_with_first_strike_step", true}, // the regular damage step is a new window with no StepChange
		{"dealt_damage_new_object", true},              // CR 400.7: a returned creature has dealt no damage this turn
		{"entering_is_not_leaving", true},              // Doorkeeper Thrull leaves "leave your graveyard" lines alone
		{"spell_gained_deathtouch_lifelink", true},     // Judith's mode on the spell that triggered her
		{"trigger_limit_look_back", true},              // CR 603.10a: Homicide Investigator's own death reads the limit it spent
		{"optional_cost_no_prefix_search", false},      // an OptionalCost card's plain cast: no search over a Strider
		{"optional_cost_granted_source", false},        // nor a witness from a granted source (Insidious Roots)
		{"fee_colour_hybrid_face", false},              // a filter's fee takes the colour the hybrid face can spare
		{"first_only_filter_in_window", false},         // a once-each-turn filter is listed once the pool pays its fee
		{"ward_window_first_only_filter", false},       // ... and a failed ward payment still spends it
		{"granted_colours_before_printed", false},      // Leyline of the Guildpact: a {C} land makes a granted colour first
		{"face_down_granted_trigger", false},           // Curious Inquiry on a face-down creature
		{"granted_source_hybrid_cast", false},          // Leyline of the Guildpact's {G/W}{G/U}{B/G}{R/G}: a hybrid cast is a potential play, and a granted source's prefix is searched on its pips as printed
		{"activation_plan_search_limit", false},        // Wispdrinker Vampire's {5}{W}{B} under Leyline of the Guildpact: the planner's node limit still arms its best plan
		{"granted_colours_before_dual_types", false},   // Leyline of the Guildpact: a dual land's type-derived {R} and {G} are intrinsic like the granted {B}, so Fuss's generic mana comes from its {G}, not a granted {B}
	})
}

// std-fdn's, on the same terms.
func TestStdFdnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-fdn", []stdRepro{
		{"surveil_own_replacement", true},     // a surveil's graveyard pile meets Darksteel Colossus's "from anywhere"
		{"color_gate_made_black", true},       // Knight of Grace counts what Eaten by Piranhas made black
		{"color_gate_recoloured", true},       // ... and not what Witness Protection made green and white
		{"exile_instead_planeswalker", true},  // CR 700.4: a planeswalker with no loyalty dies, so Obliterating Bolt exiles it
		{"lifelink_plus_per_source", true},    // two lifelink sources are two gains, each with Angel of Vitality's plus one
		{"lost_ability_no_shuffle", true},     // CR 613.1f: Witness Protection on Darksteel Colossus: it dies
		{"damage_map_per_recipient", true},    // Niv-Mizzet draws for the 3 to the opponent, not Drakuseth's whole 10
		{"copies_enter_together", true},       // CR 603.6a: each of Rite of Replication's five sees the other four
		{"flashback_restricted_source", true}, // Secluded Courtyard's any colour cannot pay Mystical Teachings' printed flashback
	})
}

// std-big's, on the same terms.
func TestStdBigReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-big", []stdRepro{
		{"copy_exception_types_count", false},               // Loot counts the card types among permanents, and a token copied "except it's an artifact creature" has the creature type
		{"discard_replacement_before_damage_removal", true}, // CR 514.1 then 514.2: two Rest in Peace ask which applies to a cleanup discard before damage wears off
	})
}

// std-tdm's, on the same terms.
func TestStdTdmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tdm", []stdRepro{
		{"shuffle_target_already_returned", false}, // CR 608.2b: Rite of Renewal's shuffled cards are in the graveyard still; one already returned to hand stays there
	})
}

// Count$AttackersDeclared counts each creature once however many combats it attacked
// in: one creature attacking in three combats of a turn is one attacker, so Case of the
// Gateway Express (three or more) stays unsolved at the end step, as in gorge and Forge.
func TestAttackersDeclaredCountsEachCreatureOnceLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-mkm", []stdRepro{
		{"attackers_declared_counts_each_creature_once", false},
	})
}

// A sorcery-speed activation is offered with the stack empty (CR 602.5d), as a cast is
// (CR 307.1): Ba Sing Se's earthbend, paid after Master Pakku's tap trigger, which
// Great Divide Guide's mana ability stacked, resolves.
func TestSorceryActivationWaitsForPrefixTriggerLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tla", []stdRepro{
		{"sorcery_activation_waits_for_prefix_trigger", false},
	})
}

// std-tla's: the first decision of each once diverged.
func TestStdTlaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tla", []stdRepro{
		{"sorcery_play_waits_for_prefix_trigger", false}, // CR 307.1: Guru Pathik is cast after Master Pakku's tap trigger resolves, and every granted source is tapped before the pass
	})
}

type stdRepro struct {
	name    string
	autopay bool
}

func stdReprosLockstep(t *testing.T, pool string, cases []stdRepro) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			replayRepro(t, reg, set, c.name, c.autopay)
		})
	}
}

// std-fin's: each of these once diverged at the end of its script.
func TestStdFinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-fin", []stdRepro{
		{"clash_chain_link_target_order", true},          // a mode's chain link is listed after the later modes' roots
		{"granted_death_trigger_last_known_power", true}, // CR 608.2h: Galuf's Final Act reads the pumped power
		{"peek_after_source_changed_hands", true},        // CR 109.5: "your library" after Stiltzkin gave the source away
		{"search_by_departed_targets_controller", true},  // CR 608.2h: Sandworm's "its controller" of a borrowed land
		{"free_cast_targets_resolving_ability", true},    // CR 608.2m: Buster Sword's offer may target the trigger itself
	})
}

// The same, on the pools the games came from.
func TestStdPoolReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range []struct {
		name string
		pool string
	}{
		{"convoke_colour_no_owed", "std-fra"},                        // a convoke cast's window owes no colour: Dedicated Commons makes its lowest
		{"convoke_colour_no_picks", "std-fra"},                       // ... with no creature announced, and the cast reversed
		{"improvise_treasure_census", "std-msh"},                     // Arc Reactor short with its Treasures counted: reversed, no Treasure spent
		{"excess_of_a_replaced_amount", "std-fra"},                   // CR 120.10: Violent Echoes' excess reads the damage as replaced
		{"free_cast_two_statics", "std-fra"},                         // two Omnipresences list one free cast
		{"ward_discard_is_a_discard", "std-fra"},                     // CR 701.9a: Tinybones sees the discard ward was paid with
		{"cast_trigger_second_target", "std-fra"},                    // Danitha, on the second object of a multi-object slot
		{"restricted_any_colour_window", "std-fra"},                  // Gideon's Memorial in a window makes the announced pip's colour
		{"ward_counters_the_targeting_ability", "std-fra"},           // Koth's two triggers: ward counters the one that targeted
		{"eot_pump_past_int8", "std-fra"},                            // Aerid Konstrari's pump past int8 stalls loudly
		{"attached_equipment_becomes_a_copy", "std-msh"},             // CR 704.5p: Shuri makes an attached Equipment a copy of a creature
		{"unless_evaluated_amount", "std-msh"},                       // Doc Samson's mana can pay an unless cost
		{"room_alt_granted_prefix_prices_the_other_door", "std-dsk"}, // Unholy Annex showing door 0 cast as Ritual Chamber: the granted-mana prefix is priced at {3}{B}{B}, not the shown door's {2}{B}
		{"lotus_pair_pays_two_colours", "std-fra"},                   // two Lotus tokens, three mana of one colour each, pay {R}{G}: the plan search keeps the second unit for the colour the first leaves owed
		{"granted_types_any_land_census_position", "std-mkm"},        // Leyline of the Guildpact: Public Thoroughfare (Produced$ Any) is listed with the fixed sources, so the plan taps Plains, not the Thoroughfare
	} {
		t.Run(c.name, func(t *testing.T) {
			replayRepro(t, reg, cardsettest.Synth(t, c.pool), c.name, true)
		})
	}
}

// The same, for scripts recorded on the full option surface.
func TestStdPoolReprosLockstepFull(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range []struct {
		name string
		pool string
	}{
		{"plan_limit_classes_by_alternatives", "std-mkm"}, // Leyline of the Guildpact: a surveil land and a Plains list the same alternatives, so the replayed node-limited search sees them as one class
		{"alt_add_cost_reduced_as_a_total", "std-tla"},    // CR 601.2f: Uncle Iroh's {1} off Redirect Lightning comes off {R} plus the {2} part, so the lowered cast floats two mana, not three
	} {
		t.Run(c.name, func(t *testing.T) {
			replayRepro(t, reg, cardsettest.Synth(t, c.pool), c.name, false)
		})
	}
}

// replayRepro plays a fixture's script in lockstep and 200 decisions past it.
func replayRepro(t *testing.T, reg *cards.Registry, set cardset.Set, name string, autopay bool) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = autopay
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
}

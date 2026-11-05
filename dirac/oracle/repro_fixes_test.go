package oracle

import (
	"path/filepath"
	"strings"
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
		{"deathtouch_wither_hit", true, false},                   // CR 704.5h: a counter-form hit marks no damage, and the mark alone is lethal
		{"dealt_once_each_damage", true, false},                  // EachDamage lands on one referent twice: one trigger with the batch's total
		{"wither_from_departed_source", true, false},             // a dying Shady Informant's trigger keeps the wither Massacre Girl gave it
		{"potential_once_each_turn_mana", true, false},           // Gravestone Strider's {1} mana ability counts toward a trigger's own cost
		{"hold_out_ends_with_first_strike_step", true, false},    // the regular damage step is a new window with no StepChange
		{"dealt_damage_new_object", true, false},                 // CR 400.7: a returned creature has dealt no damage this turn
		{"entering_is_not_leaving", true, false},                 // Doorkeeper Thrull leaves "leave your graveyard" lines alone
		{"spell_gained_deathtouch_lifelink", true, false},        // Judith's mode on the spell that triggered her
		{"trigger_limit_look_back", true, false},                 // CR 603.10a: Homicide Investigator's own death reads the limit it spent
		{"optional_cost_no_prefix_search", false, false},         // an OptionalCost card's plain cast: no search over a Strider
		{"optional_cost_granted_source", false, false},           // nor a witness from a granted source (Insidious Roots)
		{"fee_colour_hybrid_face", false, false},                 // a filter's fee takes the colour the hybrid face can spare
		{"first_only_filter_in_window", false, false},            // a once-each-turn filter is listed once the pool pays its fee
		{"ward_window_first_only_filter", false, false},          // ... and a failed ward payment still spends it
		{"granted_colours_before_printed", false, false},         // Leyline of the Guildpact: a {C} land makes a granted colour first
		{"face_down_granted_trigger", false, false},              // Curious Inquiry on a face-down creature
		{"granted_source_hybrid_cast", false, false},             // Leyline of the Guildpact's {G/W}{G/U}{B/G}{R/G}: a hybrid cast is a potential play, and a granted source's prefix is searched on its pips as printed
		{"activation_plan_search_limit", false, false},           // Wispdrinker Vampire's {5}{W}{B} under Leyline of the Guildpact: the planner's node limit still arms its best plan
		{"granted_colours_before_dual_types", false, false},      // Leyline of the Guildpact: a dual land's type-derived {R} and {G} are intrinsic like the granted {B}, so Fuss's generic mana comes from its {G}, not a granted {B}
		{"ward_failed_payment_granted_type_first", false, false}, // Leyline of the Guildpact: a ward payment too small to pay floats the Mountain's own {R}, the first of its abilities, not a granted {W}
		{"granted_types_any_land_hybrid_pool", true, false},      // Leyline of the Guildpact: Fuss after three {W} floated by hand pays its generic from the same lands in both engines
		{"adventure_card_copy_cast", true, false},                // CR 707.12, 715.3: Reenact the Crime's copy of Kellan, Inquisitive Prodigy is cast as its front face (gorge never offers the Adventure)
	})
}

// std-otj's, on the same terms.
func TestStdOtjReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-otj", []stdRepro{
		{"copied_vehicle_counts_its_crew", true, false}, // CR 707.2: Loan Shark copying Luxurious Locomotive makes a Treasure per creature that crewed it
		{"counters_skip_a_ceased_token", true, false},   // CR 111.7: Railway Brawler's counters for a Beau the legend rule removed land on nothing
		// Ornery Tumblewagg is cast through Jasper Flint's mayplay and paid
		// with a wide Stablemaster tap and a Mountain: the rider plan's
		// restricted mana floats, the payment admits and spends it first
		// (gorge pay.PlanPool, pay/plancost.go), so the plan taps only for
		// what the restricted mana leaves.
		{"rider_plan_wide_restricted_float", true, false},
		{"rider_plan_wide_restricted_float_full", false, false},
		// Bristly Bill, Spine Sower's doubling places 81920 +1/+1 counters:
		// past the kernel's 16-bit storage bound no value can agree with
		// gorge, so the game stops as the declared StallCounters capacity
		// (the divergence this replaces was the silent clamp).
		{"bristly_bill_counter_overflow_stalls", false, true},
		// Oko, the Ringleader's -5 copies each other permanent you control and
		// a copy's as-enters ask parks the sweep; the parked answer leaves
		// the sweep's zone batch to fire once, in entry order, when the sweep
		// ends (CR 603.6a): the Railway Brawler copies' "another creature
		// enters" see the Duelist and the later copies enter before the copy
		// the answer was made on
		{"copy_all_sweep_batch_spans_parked_ask", false, false},
		{"plot_cast_pays_additional_sacrifice", false, false}, // CR 118.9, 701.34d: a plotted Corrupted Conviction is cast free of its mana cost but still sacrifices a creature
		// Explosive Derailment ({R}, Spree, every mode {2}) on a board whose
		// only red source is Conduit Pylons' {1},{T} filter: the printed cost
		// is payable, but no mode is, so neither engine offers the cast (CR
		// 702.172a/702.183a: the mode cost is part of the cheapest legal total).
		{"explosive_derailment_needs_a_mode_cost", false, false},
		{"aegis_on_temporary_copy_bearer", true, false},     // CR 613.7e, 707.2: Assimilation Aegis attached to a creature Fleeting Reflection made a copy until end of turn copies over it (the later copy wins), and gives it back to that copy when unattached
		{"resolved_spell_copy_rests_in_exile", true, false}, // CR 707.10a: a copy of a spell is exiled as it leaves the stack (gorge's spellRestZone), not put in a graveyard and ceased later
	})
}

// std-blb's, on the same terms.
func TestStdBlbReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-blb", []stdRepro{
		{"blinked_creature_is_a_target_of_its_own_leave_trigger", true, false}, // CR 603.3d: Three Tree Scribe blinked and back is a legal target of its own trigger
		{"x_cast_leaves_a_target", false, false},                               // For the Common Good from one land and two Treasures: the largest-X plan consumes both copies targets, so the witness prices a smaller X whose payment keeps one Treasure as a target
	})
}

// std-trk's, on the same terms.
func TestStdTrkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-trk", []stdRepro{
		{"phased_out_source_granted_static_inert", true, false},         // CR 702.26b: Nog, Ambitious Cadet phased out stops granting "play lands from your graveyard", so the sacrificed Unexplored Territory is not offered as a land play
		{"granted_mana_taps_stack_one_at_a_time", false, false},         // CR 603.3b: Deep Space 9 paid by Rok-Tahk's {G}{G} from Beckett Mariner and Erica Ortegas: each tap's trigger is stacked before the next activation, and both resolve before the sorcery-speed cast
		{"entry_counter_put_by_returned_owner", false, false},           // CR 122.6a: Dr. Leonard "Bones" McCoy returns the opponent's Moopsy under its owner's control, so its owner puts the +1/+1 counter and the owner's Dr. Joseph M'Benga doubles it
		{"copy_each_target_token_replacement", false, false},            // CR 111.1, 614.1: Dominion Infiltration's three copies are created together, so the Holo-Engineer copy that entered first does not turn the Sarek copy into an Illusion
		{"entry_counter_put_by_opposing_grant", false, false},           // CR 122.6a: Overflow Submatrix's finality counter on the opponent's Storied Veteran is put by the Submatrix's controller, so Dr. Joseph M'Benga doubles only the Veteran's own grant
		{"stack_spell_controller_not_owner", true, false},               // CR 108.4, 110.2: Brazen Bluff asks the controller of the spell it targets, the player casting Transporter Malfunction from the opponent's exile, not its owner
		{"copy_cast_x_targets_ask_after_x", false, false},               // CR 601.2b, 601.2c: Q's Play of Dominion Infiltration announces X before its "X target creatures", so a copy cast no mana can pay still asks X and reaches its mana window
		{"refused_paid_copy_cast_keeps_window_tap", true, false},        // CR 733.1: Encounter the Unknown's copy cast that gorge cannot pay is reversed after its mana window, so the tapped source and its mana stay
		{"exile_grant_host_is_a_copy", false, false},                    // CR 707.2, 603.6a: Dominion Saboteur enters as Q, Stacking the Deck, so its enter trigger's exiled Assembly Specialist gains Q's exile-from-hand cast ability, which the copy's host offers
		{"turn_once_resets_while_phased_out", false, false},             // CR 602.5b, 702.26d: DOT-7 Repair Squad activated, then phased out through its turn's end, has its once-each-turn limit lifted when it phases back in
		{"phased_in_permanent_rederives_override_types", true, false},   // CR 702.26c, 613.1: an assimilated Lwaxana Troi and a Station-charged Borg Cube phase in during the untap step, and the next decision projects their continuous artifact/creature types
		{"clone_template_choice_asked_on_stack", false, false},          // CR 614.12, 707.9: Dominion Saboteur entering as a copy of Nyota Uhura is asked her "choose a number" with the Saboteur still on the stack, after the copy election
		{"returned_clone_order_ask_beside_counter_source", true, false}, // CR 616.1: Dominion Saboteur returned from exile as a copy of Troubling Tribbles beside Deanna Troi's extra counter is asked the replacement order with the card still in exile
		{"until_next_turn_pump_misses_returned_creature", false, false}, // CR 400.7, 611.2c: V'Ger's "-1/-0 until your next turn" does not reach Hologram Janeway that Ghost of Illyria blinked back, a new object
		{"copy_cast_x_spell_asks_x_before_abort", false, false},         // CR 601.2b, 601.2c: Q's Play of Khaaaaaaaaaaaannn! announces X before its target ask, so a copy cast no mana can pay still asks X = 0 and puts the copy on the stack before the target ask reverses it
		{"clone_entry_replacement_sees_copy", false, false},             // CR 614.12, 707.9b: Dominion Saboteur entering as a copy of U.S.S. Enterprise-D is a Spacecraft as it enters, so Zefram Cochrane's two extra charge counters join the copied one
		{"copy_cast_mode_target_unpayable", false, false},               // CR 601.2c, 733.1: Q's Play of Make It So announces its untap mode, whose required target the opponent's three untapped lands cannot pay for, so the cast is reversed at the target ask
		{"dilemma_cast_reversed_no_target", true, false},                // CR 601.2c, 733.1: Q's Play of Stray from History, a copy cast the opponent's two Mountains cannot pay, is reversed at the "target opponent" ask, after the dilemma and before any tap, as gorge withholds every target of an unpayable cast
		{"unpayable_x_copy_cast_asks_x", false, false},                  // CR 601.2b, 601.2c: Q's Play of Khaaaaaaaaaaaannn! with every source tapped still announces X before its target, so the copy sits on the stack at the X ask
		{"unpayable_modal_copy_cast_aborts_at_target", true, false},     // CR 601.2b, 601.2c: Q's Play of Set Phasers to . . . with every source tapped announces its mode, then aborts at the chosen mode's target ask
		{"graveyard_shuffle_moves_one_batch", false, false},             // CR 603.2c: Those Old Scientists' two-card shuffle is one event, so Sela's "one or more cards leave your graveyard" triggers once
		{"name_choice_beside_counter_rider_no_order", false, false},     // CR 614.12, 616.1: Kai Winn entering beside Deanna Troi's extra counter is asked only the name, as the two compose in either order
		{"name_hash_collision_both_offered", false, false},              // CR 201.4: Jadzia Dax and Xenobotanist hash alike as chosen names, yet both stay distinct options on each side
		{"exile_return_name_asked_mid_cost", false, false},              // CR 614.12, 610.3: In the Pale Moonlight's sacrifice of Malfunctioning Holodeck returns Kai Winn, who is asked her name in exile before the two Romulan tokens are made
		{"copy_cast_unaffordable_target_reverses", true, false},         // CR 601.2c, 733.1: Q's Play of Perils of the Past copies a {1}{R} instant with no red source untapped, so the cast is reversed before its target is asked
		{"stale_trigger_referent_reentered", false, false},              // CR 400.7: Support Mission's counter trigger fired for Dr. Leonard "Bones" McCoy's first entry puts nothing on the copy that left and was cast back in
		{"phased_out_permanent_grants_no_land_plays", false, false},     // CR 702.26b: Nog, Ambitious Cadet phased out by The City on the Edge of Forever does not exist, so its granted "play lands from your graveyard" offers no graveyard Forest
		{"unpayable_x_copy_cast_aborts_at_target", true, false},         // CR 601.2b, 601.2c: Q's Play of Khaaaaaaaaaaaannn! (a copy cast no source can pay) asks X, then aborts at its target ask, leaving no copy on the stack
		{"play_copy_cast_unpayable_reversed_at_target", false, false},   // CR 601.2c, 733.1: Perils of the Past exiled, its Play-granted copy cast the window cannot pay is reversed at its required target ask, leaving the stack as gorge has it
	})
}

// std-fdn's, on the same terms.
func TestStdFdnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-fdn", []stdRepro{
		{"surveil_own_replacement", true, false},                // a surveil's graveyard pile meets Darksteel Colossus's "from anywhere"
		{"color_gate_made_black", true, false},                  // Knight of Grace counts what Eaten by Piranhas made black
		{"color_gate_recoloured", true, false},                  // ... and not what Witness Protection made green and white
		{"exile_instead_planeswalker", true, false},             // CR 700.4: a planeswalker with no loyalty dies, so Obliterating Bolt exiles it
		{"lifelink_plus_per_source", true, false},               // two lifelink sources are two gains, each with Angel of Vitality's plus one
		{"lost_ability_no_shuffle", true, false},                // CR 613.1f: Witness Protection on Darksteel Colossus: it dies
		{"damage_map_per_recipient", true, false},               // Niv-Mizzet draws for the 3 to the opponent, not Drakuseth's whole 10
		{"copies_enter_together", true, false},                  // CR 603.6a: each of Rite of Replication's five sees the other four
		{"flashback_restricted_source", true, false},            // Secluded Courtyard's any colour cannot pay Mystical Teachings' printed flashback
		{"granted_any_mana_beside_own_trigger_x", false, false}, // New Horizons on a Forest adds an any-colour ability beside its own, so Wildborn Preserver's "you may pay {X}" counts it in the potential pool as gorge's walk does
		{"trigger_paid_x_zero_binds", false, false},             // CR 107.3: Wildborn Preserver paid X = 0 puts no counters, whatever X the creature that triggered it was cast with
		{"dies_trigger_reads_attacking_lki", true, false},       // CR 603.10a: Garna, Bloodfist of Keld draws for a Perforating Artist that died attacking, its last known status
		{"any_type_rider_hybrid_pips", true, false},             // CR 107.4e, 601.2b: Tinybones lets any mana pay Teach by Example's two hybrid pips, so W and B afford the cast
	})
}

// std-eoe's, on the same terms.
func TestStdEoeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-eoe", []stdRepro{
		{"restricted_grant_no_unit_for_hand_cast_x", false, false},          // Mm'menon grants an untapped Specimen Freighter blue for spells not cast from hand, so Zero Point Ballad from hand has no extra mana to announce X with
		{"restricted_grant_no_unit_for_hand_cast_x_two", false, false},      // ... and Devastating Onslaught's X likewise stops one short
		{"restricted_grant_no_unit_for_trigger_cost", true, false},          // ... and Oreplate Pangolin's "you may pay {1}" is no spell, so an artifact holding Mm'menon's blue pays nothing: only decline is asked
		{"granted_batch_attack_trigger_once_per_declaration", false, false}, // CR 508.1, 603.2: a granted "whenever you attack" trigger fires once for a declaration split across two defenders, not once per defender
		{"dies_trigger_power_threshold_lki", true, false},                   // CR 603.3b: Syr Vondam's own dies trigger queues before the dies trigger Perigee Beckoner granted it
		{"return_defender_asked_before_move", true, false},                  // CR 508.4: Chorale of the Void's attacking return asks its defender while the card is still in the graveyard, as gorge does, not after it entered
		{"station_reads_off_battlefield_cda_power", true, false},            // CR 604.3, 702.184a: Adagia's Station counts a sacrificed Harmonious Grovestrider by its lands-you-control power, which works in the graveyard, not by its printed 0
	})
}

// std-hob's, on the same terms.
func TestStdHobReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-hob", []stdRepro{
		{"sac_part_and_mana_share_a_treasure", false, false}, // CR 601.2g-h: the one Treasure that pays Stir Up Trouble's {B} is gone, so it is not also the artifact its either-or part sacrifices; the cast is not offered
	})
}

// std-big's, on the same terms.
func TestStdBigReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-big", []stdRepro{
		{"copy_exception_types_count", false, false},                        // Loot counts the card types among permanents, and a token copied "except it's an artifact creature" has the creature type
		{"discard_replacement_before_damage_removal", true, false},          // CR 514.1 then 514.2: two Rest in Peace ask which applies to a cleanup discard before damage wears off
		{"replacement_answer_mid_payment_pays_through_window", true, false}, // gorge's payment plan stops at a step that poses a decision, so Legion Extruder's second Treasure, behind Rest in Peace's order ask, is activated by the manual window, which asks its colour
		{"sba_batch_asks_before_tokens_cease", true, false},                 // CR 704.3: three Golems destroyed at once under Rest in Peace are each asked which replacement applies, and the two already in exile cease at the next check, not between the asks
		{"ability_target_before_cost_replacement", true, false},             // CR 601.2c: Harvester of Misery's target is a target of the ability before Rest in Peace asks where its discarded source goes
	})
}

// std-tdm's, on the same terms.
func TestStdTdmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tdm", []stdRepro{
		{"shuffle_target_already_returned", false, false}, // CR 608.2b: Rite of Renewal's shuffled cards are in the graveyard still; one already returned to hand stays there
	})
}

// Count$AttackersDeclared counts each creature once however many combats it attacked
// in: one creature attacking in three combats of a turn is one attacker, so Case of the
// Gateway Express (three or more) stays unsolved at the end step, as in gorge and Forge.
func TestAttackersDeclaredCountsEachCreatureOnceLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-mkm", []stdRepro{
		{"attackers_declared_counts_each_creature_once", false, false},
	})
}

// A sorcery-speed activation is offered with the stack empty (CR 602.5d), as a cast is
// (CR 307.1): Ba Sing Se's earthbend, paid after Master Pakku's tap trigger, which
// Great Divide Guide's mana ability stacked, resolves.
func TestSorceryActivationWaitsForPrefixTriggerLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tla", []stdRepro{
		{"sorcery_activation_waits_for_prefix_trigger", false, false},
	})
}

// A token copy of a transforming permanent is double-faced and each face takes the
// copy effect's exceptions (CR 707.8a, 707.9b): Saheeli's copy of Dowsing Device is
// an artifact on its front face and an artifact land once it transforms to Geode Grotto.
func TestTransformedTokenCopyKeepsCopyExceptionsLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-lci", []stdRepro{
		{"transformed_token_copy_keeps_copy_exceptions", false, false},
	})
}

// A Vehicle crewed again after Eaten by Piranhas resolved on it is an artifact creature
// again (CR 613.7): the second crew is a newer effect than the Aura's type removal, so it
// is not folded into the first crew's older position.
func TestCrewAgainUnderTypeRemovingAuraLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-lci", []stdRepro{
		{"crew_again_under_type_removing_aura", true, false},
	})
}

// std-tla's: the first decision of each once diverged.
func TestStdTlaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tla", []stdRepro{
		{"sorcery_play_waits_for_prefix_trigger", false, false},            // CR 307.1: Guru Pathik is cast after Master Pakku's tap trigger resolves, and every granted source is tapped before the pass
		{"airbent_card_with_self_flash", true, false},                      // CR 601.3b: Serpent of the Pass airbent to exile is cast at instant speed with three Lessons in the graveyard
		{"granted_mana_once_beside_another_grant", true, false},            // Great Divide Guide's granted mana is one ability of each grantee on a board with another global grant, so the prefix search finds Earthen Ally's {2}{W}{U}{B}{R}{G} within its budget
		{"waterbend_no_picks_falls_to_manual_window", false, false},        // CR 602.2b, 605.3a: a waterbend cost no printed source pays falls to the manual window, which taps Great Divide Guide's granted sources in battlefield order
		{"waterbend_no_picks_falls_to_manual_window_autopay", true, false}, // CR 602.2b, 605.3a: ... on the autopay surface too, where the planner's granted unit would otherwise pick a different source
		{"waterbend_credit_paid_source_live_pool", true, false},            // CR 701.67a: a paid mana ability earns the waterbend credit only while its fee is unpayable, so the prefix taps it once a grant floats the fee
		{"exiled_target_keeps_was_damaged", false, false},                  // CR 120.3a: Sold Out exiles its combat-damaged target, and the exile leaves WasDealtDamageThisTurn standing, so the "if that creature was dealt damage this turn" Clue is still made (gorge events.foldDamage)
		{"first_only_filter_funds_play", false, false},                     // CR 605.1a, 106.1: Barrels of Blasting Jelly's first-and-only {1} any-colour filter funds Waterbender Ascension's {1}{U} and Fire Nation Palace's {1}{R} activation, so both stay on the menu
		{"first_only_filter_funds_play_autopay", true, false},              // CR 605.1a, 106.1: ... and on the autopay surface, where the same first-and-only filter funds Fire Nation Palace's activation from the floating pool
		{"tap_cost_source_not_its_own_granted_mana", true, false},          // CR 602.2b, 605.3a: Agna Qel'a's {2}{U},{T} is not paid by the tap that is its cost, so Great Divide Guide's grant on it adds no third mana beside Guide and Badgermole Cub's extra {G}
	})
}

type stdRepro struct {
	name    string
	autopay bool
	stall   bool // the replay is expected to end in the declared kernel stall
}

func stdReprosLockstep(t *testing.T, pool string, cases []stdRepro) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			replayRepro(t, reg, set, c.name, c.autopay, c.stall)
		})
	}
}

// std-fin's: each of these once diverged at the end of its script.
func TestStdFinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-fin", []stdRepro{
		{"clash_chain_link_target_order", true, false},          // a mode's chain link is listed after the later modes' roots
		{"granted_death_trigger_last_known_power", true, false}, // CR 608.2h: Galuf's Final Act reads the pumped power
		{"peek_after_source_changed_hands", true, false},        // CR 109.5: "your library" after Stiltzkin gave the source away
		{"search_by_departed_targets_controller", true, false},  // CR 608.2h: Sandworm's "its controller" of a borrowed land
		{"free_cast_targets_resolving_ability", true, false},    // CR 608.2m: Buster Sword's offer may target the trigger itself
		{"granted_prefix_sacrifices_own_treasure", true, false}, // CR 704.5d: Gysahl Greens flashback, paid by Plains, a Treasure and The Earth Crystal activated at priority, so the sacrificed Treasure has ceased before the cast
		{"sacrifice_cost_granted_prefix", true, false},          // CR 601.2g-h: the granted mana is the prefix, so the printed plan leaves the Treasure to sacrifice
		{"modal_cost_granted_mana_prefix", false, false},        // CR 601.2b, 601.2g-h: Tifa's Limit Break's cheapest Tiered mode costs only its printed {G}, which the granted any-colour source (A Realm Reborn) pays through the prefix
		{"granted_mana_pays_tiered_cast", false, false},         // CR 601.2g: A Realm Reborn's granted any-colour mana from two untapped artifacts pays Ice Magic's {1}{U}, so the Tiered cast is offered
		{"free_tiered_mode_cost_reduction", false, false},       // CR 601.2f, 118.9d: The Wind Crystal's ReduceCost applies to the Tiered mode cost of a free cast
		{"graveyard_leave_sees_batch_entrant", true, false},     // CR 603.10a: Fang returned by Random Encounter beside the cards it returns did not see them leave
	})
}

// std-sos's, on the same terms.
func TestStdSosReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-sos", []stdRepro{
		{"printed_flashback_beside_granted", false, false},        // CR 702.34a: a flashback instance granted beside the printed one leaves the printed cast priced
		{"granted_and_printed_flashback_ambiguous", false, false}, // CR 613.1c: the printed Flashback and the granted one (Flashback's pump) price as separate potential casts, so the ambiguity scan never drops both
		{"graveyard_ability_source_new_object", true, false},      // CR 400.7: a second Postmortem Professor return ability resolves after the card came back and died again, and finds a new object
	})
}

// std-ecl's, on the same terms.
func TestStdEclReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-ecl", []stdRepro{
		{"each_color_among_valid_printed_read", false, false},        // CR 613.1e: Bloom Tender's batch reads Puca's Eye's layer-5 chosen colour, not its printed colourless face
		{"each_color_among_valid_printed_read_autopay", true, false}, // ... and the tap's mana is floated with the B in the pool itself
		{"each_color_among_valid_pay_unaffordable", true, false},     // CR 118.3: Trystan's "may pay {B}" upkeep ask is offered, as gorge's potential over-bound does, when Bloom Tender is the only untapped source
		{"convoke_tap_another_source_keeps_target", true, false},     // CR 601.2c, 601.2g: a Springleaf Drum left beside a convoke cast's picks keeps its target on gorge's menu, however short the window is
		{"evoke_hybrid_paid_from_floating_pool", false, false},       // CR 601.2h: Deceit evoked with a floating {U} and {B} pays its {U/B}{U/B} from the pool and taps no untapped Island
		{"planned_mana_sacrifice_token_waits_for_sba", true, false},  // CR 704.5d, 111.7: a Treasure sacrificed by the payment plan for Appeal to Eirdu stays in the graveyard until the next state-based check, as gorge leaves it
	})
}

// std-fra's, on the same terms.
func TestStdFraReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-fra", []stdRepro{
		{"sweep_returns_aura_after_earlier_entrants", true, false}, // CR 303.4f: Return to the Light Realms asks for Infinite Coursework's host with the cards before it entered and the ones after still in the graveyard
		{"sacrificed_mana_source_moves_its_aura", false, false},    // CR 704.3, 704.5m: the check after a mana source's sacrifice while paying a cast takes its Aura before the cast's next choice
	})
}

// std-lci's, on the same terms.
func TestStdLciReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-lci", []stdRepro{
		{"granted_sac_host_cost_poses_no_choice", false, false}, // CR 602.2b, 701.21a: Deconstruction Hammer's granted "Sacrifice Deconstruction Hammer" cost has one possible payment, so it is paid without a choice after the targets
	})
}

// std-spm's, on the same terms.
func TestStdSpmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-spm", []stdRepro{
		{"tapcost_source_not_paid_with_itself", false, false}, // CR 602.2b: Guy in the Chair's {T} activation is payable through Supportive Parents tapping two other creatures; gorge's prefix search no longer stops at the line whose forced tap takes Guy
	})
}

// std-dft's, on the same terms.
func TestStdDftReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-dft", []stdRepro{
		{"clone_entry_order_beside_counter", false, false},   // CR 616.1: Waxen Shapethief's as-enters copy and Thunderous Velocipede's counter replacement ask which applies first, with the card still on the stack
		{"clone_entry_order_three_candidates", false, false}, // CR 616.1, 616.1f: ... beside two Velocipedes the ask has three options, and a counter source named first moves the card in and asks the two left with it on the battlefield
		{"copy_entry_trigger_after_order_ask", false, false}, // CR 614.12/603.6a: Waxen Shapethief's as-enters copy and Thunderous Velocipede's counter replacement ask which applies first; the copy's Hulldrifter enters trigger queues after the bodies
	})
}

// std-tmt's: each of these once ended in a kernel stall, which replayRepro alone does not see.
func TestStdTmtReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tmt")
	for _, c := range []stdRepro{
		{"put_counters_from_unheard_kind_order", false, false}, // CR 603.3b: Donatello, Mutant Mechanic copies a dead artifact's +1/+1 and stun counters while a +1/+1 trigger listens: the stun is heard by none, so gorge's insertion order is unobservable
	} {
		t.Run(c.name, func(t *testing.T) {
			if res := replayReproRes(t, reg, set, c.name, c.autopay); res.Stall != "" {
				t.Fatalf("the game stalled: %s", res.Stall)
			}
		})
	}
}

// std-dsk's, on the same terms (this one recorded on the full surface).
func TestStdDskReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-dsk", []stdRepro{
		{"room_shown_face_plain_cast", false, false},             // CR 709.3: a Room back on the battlefield as Slimy Aquarium keeps that door, so its plain hand cast is the shown half's {3}{U}, which the planner must price
		{"sacrificed_panharmonicon_host_departs", false, false},  // CR 603.2d: a sacrificed Panharmonicon host is gone when Popular Egotist's "whenever you sacrifice" trigger is checked, so it echoes nothing
		{"returning_aura_during_paid_cost", false, false},        // CR 303.4f: a sacrificed "until this leaves" host returns its exiled Aura while the sacrifice cost's payment is still resolving: the Aura waits in exile and its owner chooses what it enchants
		{"lingering_aura_sweeps_before_order_ask", false, false}, // CR 704.3, 616.1: a dying creature's order ask is posed in its check: the Aura whose host left in that check is swept, and the token it moved ceases, at the check after the answer
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
		{"convoke_colour_no_owed", "std-fra"},                           // a convoke cast's window owes no colour: Dedicated Commons makes its lowest
		{"convoke_colour_no_picks", "std-fra"},                          // ... with no creature announced, and the cast reversed
		{"improvise_treasure_census", "std-msh"},                        // Arc Reactor short with its Treasures counted: reversed, no Treasure spent
		{"excess_of_a_replaced_amount", "std-fra"},                      // CR 120.10: Violent Echoes' excess reads the damage as replaced
		{"free_cast_two_statics", "std-fra"},                            // two Omnipresences list one free cast
		{"ward_discard_is_a_discard", "std-fra"},                        // CR 701.9a: Tinybones sees the discard ward was paid with
		{"cast_trigger_second_target", "std-fra"},                       // Danitha, on the second object of a multi-object slot
		{"restricted_any_colour_window", "std-fra"},                     // Gideon's Memorial in a window makes the announced pip's colour
		{"ward_counters_the_targeting_ability", "std-fra"},              // Koth's two triggers: ward counters the one that targeted
		{"eot_pump_past_int8", "std-fra"},                               // Aerid Konstrari's pump past int8 stalls loudly
		{"attached_equipment_becomes_a_copy", "std-msh"},                // CR 704.5p: Shuri makes an attached Equipment a copy of a creature
		{"unless_evaluated_amount", "std-msh"},                          // Doc Samson's mana can pay an unless cost
		{"room_alt_granted_prefix_prices_the_other_door", "std-dsk"},    // Unholy Annex showing door 0 cast as Ritual Chamber: the granted-mana prefix is priced at {3}{B}{B}, not the shown door's {2}{B}
		{"lotus_pair_pays_two_colours", "std-fra"},                      // two Lotus tokens, three mana of one colour each, pay {R}{G}: the plan search keeps the second unit for the colour the first leaves owed
		{"granted_mana_creature_is_no_ability_planner_unit", "std-tla"}, // Honest Work's Humble Merchant (Katara) is no unit of an ability's planner, so Flexible Waterbender's waterbend {3} falls to the window, which taps her before the Islands
		{"granted_types_any_land_census_position", "std-mkm"},           // Leyline of the Guildpact: Public Thoroughfare (Produced$ Any) is listed with the fixed sources, so the plan taps Plains, not the Thoroughfare
		{"delayed_return_skips_object_blinked_since", "std-fin"},        // CR 400.7: Random Encounter's end-step return finds Y'shtola Rhul already exiled and returned by her own trigger, a new object
		{"returned_creature_was_not_dealt_damage", "std-fin"},           // CR 400.7: Qutrub Forayer raised after combat damage killed it is dealt no damage this turn, so its destroy mode has no target
	} {
		t.Run(c.name, func(t *testing.T) {
			replayRepro(t, reg, cardsettest.Synth(t, c.pool), c.name, true, false)
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
		{"plan_limit_classes_by_alternatives", "std-mkm"},                // Leyline of the Guildpact: a surveil land and a Plains list the same alternatives, so the replayed node-limited search sees them as one class
		{"script_trigger_up_to_one_declined_first", "std-ecl"},           // Champions of the Shoal tapped to pay Springleaf Drum's cost before an equip: gorge's script search answers the trigger's "up to one target" ask with no target first, so the kernel's search does too and Sting-Slinger is not stunned
		{"script_trigger_up_to_one_declined_first_swat_away", "std-ecl"}, // the same search ahead of Swat Away's cast: the same Drum-paid script leaves the stun trigger unaimed on both sides
		{"maralen_blinked_regains_its_permission", "std-ecl"},            // CR 400.7: Personify cast free through Maralen blinks her, and the returned Maralen is a new object whose once-each-turn permission is unspent
		{"alt_add_cost_reduced_as_a_total", "std-tla"},                   // CR 601.2f: Uncle Iroh's {1} off Redirect Lightning comes off {R} plus the {2} part, so the lowered cast floats two mana, not three
		{"hybrid_trial_leaves_granted_sources_out", "std-tla"},           // Long Feng's {B/G}{B/G} over Great Divide Guide: gorge's resolution search has no witness for {B}{B} from a granted source, so it pays {G}{B}
		{"waterbend_armed_witness_stands_after_picks", "std-tla"},        // Katara's waterbend {6}: the offer armed Island, Island for the credit's {2}, four picks bring the cost to {2}, and gorge runs that witness rather than ranking again without the Waterskin
		{"window_combo_answers_its_own_colour", "std-tla"},               // Misty Palms Oasis, {W} or {B} printed and any colour granted, owing {R}: the window answers {W}, the granted option has no symbol
		{"sweep_damage_is_one_batch_for_dealt_once", "std-fin"},          // Nibelheim Aflame with Cecil, Dark Knight: his damage to three creatures is one batch, so his trigger queues once with the total
		{"trigger_cost_window_pays_through_converter", "std-tdm"},        // CR 601.2g, 118.3: Descendant of Storms' {1}{W} paid from two Swamps through Mardu Devotee: the window's floated W and B pay the cost and the Endure stacks
		{"token_bounced_if_you_do", "std-hob"},                           // CR 111.7: a token returned to hand did return, so Mirkwood Nurturer's "if you do" puts its counter
		{"token_in_graveyard_is_no_card", "std-woe"},                     // CR 111.7: a sacrificed Rat token waiting for the next check is no creature card, so Cruel Somnophage counts the graveyards without it

		{"sacrificed_token_is_no_card_in_graveyard_count", "std-lci"},       // CR 111.1, 704.5d: a Treasure sacrificed for mana waits for the next check, and is no permanent card for Souls of the Lost to count
		{"equipment_unattached_earlier_names_no_bearer", "std-lci"},         // CR 608.2h: Tarrian's Soulcleaver detached when its bearer stopped being a creature, so its dies trigger names no equipped creature
		{"spree_pays_with_its_only_target", "std-otj"},                      // Explosive Derailment aimed at the Treasure that pays for it: the cast is planned at {R}, targets chosen first, the window sacrifices the Treasure and the cast stands (CR 601.2c, 601.2g)
		{"twobrid_face_reached_through_converter", "std-tdm"},               // CR 601.2b: Temur Tawnyback's {2/R} is offered when an Island and Jeskai Devotee's {1} for {R} are the only red
		{"lose_gain_run_sacrifice_trigger_resolves_before_cast", "std-fin"}, // CR 603.3, 601.2g: Al Bhed Salvagers' "target opponent loses 1 life and you gain 1" fires on a Treasure sacrificed for Tonberry's mana and resolves before the spell is cast
		{"static_trigger_grants_list_before_effect_grants", "std-fin"},      // Summon: Primal Odin under Thief's Knife and Zantetsuken: the Knife's static grant lists its draw before the Animate grant's lose-game, so the one order ask means the same order on both sides (CR 603.3b)
	} {
		t.Run(c.name, func(t *testing.T) {
			replayRepro(t, reg, cardsettest.Synth(t, c.pool), c.name, false, false)
		})
	}
}

// replayRepro plays a fixture's script in lockstep and 200 decisions past it.
func replayRepro(t *testing.T, reg *cards.Registry, set cardset.Set, name string, autopay, stall bool) {
	t.Helper()
	res := replayReproRes(t, reg, set, name, autopay)
	if !stall && strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("kernel stalled: %s", res.Stall)
	}
}

// replayReproRes is replayRepro returning the game's result, for a fixture whose
// failure is a stall the game used to end on.
func replayReproRes(t *testing.T, reg *cards.Registry, set cardset.Set, name string, autopay bool) Result {
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
	return res
}

// A keyword removal that resolves after a grant is the newer effect (CR 613.1f, 613.7):
// The Fire Nation Drill's second removal is not folded into its first, older one, so the
// hexproof Enter the Avatar State gave in between is gone.
func TestKeywordRemovalAfterAGrantLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tla", []stdRepro{
		{"keyword_removal_resolved_after_a_grant", true, false},
	})
}

// std-woe's, on the same terms.
func TestStdWoeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-woe", []stdRepro{
		{"filter_fed_by_granted_mana", false, false},   // CR 605.3b, 601.2g: Prophetic Prism's {1} is paid by the Food Night of the Sweets' Revenge lets tap for {G}, so Embereth Veteran is castable
		{"act_paid_filter_second_colour", true, false}, // CR 601.2g, 602.2b: Restless Fortress's {1}{W}{B} animation is offered when a free filter (Crystal Grotto) makes one pip and Rootrider Faun's paid filter the other
	})
}

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
		{"deathtouch_wither_hit", true, false},                       // CR 704.5h: a counter-form hit marks no damage, and the mark alone is lethal
		{"dealt_once_each_damage", true, false},                      // EachDamage lands on one referent twice: one trigger with the batch's total
		{"wither_from_departed_source", true, false},                 // a dying Shady Informant's trigger keeps the wither Massacre Girl gave it
		{"potential_once_each_turn_mana", true, false},               // Gravestone Strider's {1} mana ability counts toward a trigger's own cost
		{"hold_out_ends_with_first_strike_step", true, false},        // the regular damage step is a new window with no StepChange
		{"dealt_damage_new_object", true, false},                     // CR 400.7: a returned creature has dealt no damage this turn
		{"entering_is_not_leaving", true, false},                     // Doorkeeper Thrull leaves "leave your graveyard" lines alone
		{"spell_gained_deathtouch_lifelink", true, false},            // Judith's mode on the spell that triggered her
		{"trigger_limit_look_back", true, false},                     // CR 603.10a: Homicide Investigator's own death reads the limit it spent
		{"optional_cost_no_prefix_search", false, false},             // an OptionalCost card's plain cast: no search over a Strider
		{"optional_cost_granted_source", false, false},               // nor a witness from a granted source (Insidious Roots)
		{"evidence_targets_cost_granted_source", false, false},       // CR 601.2b: Urgent Necropsy's collect evidence X is priced by printed sources only, not Insidious Roots' grant
		{"damage_all_planeswalker_clause", false, false},             // CR 120.3c: Incinerator of the Guilty's X damage to each planeswalker that player controls removes Kaya's loyalty
		{"fee_colour_hybrid_face", false, false},                     // a filter's fee takes the colour the hybrid face can spare
		{"first_only_filter_in_window", false, false},                // a once-each-turn filter is listed once the pool pays its fee
		{"ward_window_first_only_filter", false, false},              // ... and a failed ward payment still spends it
		{"granted_colours_before_printed", false, false},             // Leyline of the Guildpact: a {C} land makes a granted colour first
		{"face_down_granted_trigger", false, false},                  // Curious Inquiry on a face-down creature
		{"granted_source_hybrid_cast", false, false},                 // Leyline of the Guildpact's {G/W}{G/U}{B/G}{R/G}: a hybrid cast is a potential play, and a granted source's prefix is searched on its pips as printed
		{"activation_plan_search_limit", false, false},               // Wispdrinker Vampire's {5}{W}{B} under Leyline of the Guildpact: the planner's node limit still arms its best plan
		{"granted_colours_before_dual_types", false, false},          // Leyline of the Guildpact: a dual land's type-derived {R} and {G} are intrinsic like the granted {B}, so Fuss's generic mana comes from its {G}, not a granted {B}
		{"ward_failed_payment_granted_type_first", false, false},     // Leyline of the Guildpact: a ward payment too small to pay floats the Mountain's own {R}, the first of its abilities, not a granted {W}
		{"granted_types_any_land_hybrid_pool", true, false},          // Leyline of the Guildpact: Fuss after three {W} floated by hand pays its generic from the same lands in both engines
		{"adventure_card_copy_cast", true, false},                    // CR 707.12, 715.3: Reenact the Crime's copy of Kellan, Inquisitive Prodigy is cast as its front face (gorge never offers the Adventure)
		{"facedown_ward_of_a_printed_ward_card", false, false},       // CR 701.58a/702.168b: a cloaked Axebane Ferox (prints Ward) targeted by an opponent: the face-down ward trigger has no face trigger index (gorge 255)
		{"hybrid_activation_printed_witness", false, false},          // a lowered {G/W} activation resolves over printed mana only: Insidious Roots' granted any-colour token is not the {G} face gorge's planHybridCost prices
		{"each_target_aura_copy_host_ask", false, false},             // CR 303.4f: Doppelgang's copy of a targeted Curious Inquiry asks its host before the copies of the targets after it
		{"copy_each_target_aura_enchant", false, false},              // CR 303.4f: Doppelgang's copy of an Aura asks what it enchants as it enters, the later targets' copies after the answer
		{"animated_equipment_without_equip_ability", false, false},   // CR 704.5p: Cryptic Coat (Equipment, attached by its ETB trigger, no Equip ability) detaches when Behind the Mask animates it, so its Equipped static stops on the Forest
		{"mayplay_x_cast_census_rider", false, false},                // CR 609.4b, 601.2g-h: Worldsoul's Rage cast from Outrageous Robbery's exile: the prefix taps Insidious Roots' granted token for W, which the any-type rider spends on {R}, since gorge's census (no granted source) cannot pay the printed pips
		{"ward_queues_at_target_choice_before_payment", true, false}, // CR 601.2c, 603.3b: a cost's sacrifice dies during the payment, so its triggers queue behind the ward trigger the target answer queued, not ahead of it -- the CR 603.3b ordering ask indexes the ward first
		{"granted_before_printed_any", false, false},                 // CR 305.6: a granted basic type's intrinsic abilities precede a printed any-colour ability in the node-limited plan census
		// CR 601.2b, 601.2g-h: Fuss's {R/W} cast from Outrageous Robbery's exile is priced under the any-type rider, so the printed lands pay it and Insidious Roots' granted token stays untapped
		{"granted_any_prefix_respects_the_any_type_rider", false, false},
		{"cost_sacrifices_latch_one_or_more_die", false, false},
		{"mayplay_any_type_either_or_cost_offered", false, false}, // CR 601.2f-h: Demand Answers cast from Outrageous Robbery's exile with GGG floating and a card to discard: the any-type rider pays the recomposed {1}{R}
		// CR 118.9d, 601.2f: Reenact the Crime's free copy of Officious
		// Interrogation still owes its per-target RaiseCost; the pool cannot
		// pay it, so the cast window reverses the cast (std-mkm pair 2359)
		{"free_copy_raise_cost_per_target_reversed", false, false},
		{"collect_evidence_selection_stale", true, false},            // CR 701.59a: Forensic Researcher's evidence pick lists the whole graveyard after a declined Lamplight Phoenix cost left its reservation behind
		{"copy_each_target_copies_enter_together_two", false, false}, // ... and Doppelgang's copies of several targets likewise enter in one batch
		{"sacrifice_menu_keeps_stolen_creature", true, false},
		{"ward_cannot_counter_uncounterable_spell", false, false}, // CR 101.2, 702.21a: Long Goodbye can't be countered, so an unpaid ward leaves it on the stack
		{"most_creatures_counts_layer_types", false, false},       // CR 613.1d, 205.1: No Witnesses counts a solved Case (a Creature by its static) among the most creatures
		// CR 605.3b, 601.2g: Leering Onlooker's {2}{B}{B} is paid by two Scenes of the Crime's second ability (tap a creature: any colour) after the first Scene's ability ask is answered with its {C}
		{"second_tapx_mana_ability", true, false},
	})
}

// std-otj's, on the same terms.
func TestStdOtjReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-otj", []stdRepro{
		// CR 603.7c, 400.7: two Fortune, Loyal Steed end-of-combat delayed
		// triggers; the first blinks Fortune and its saddler, so the second
		// names a new object and moves nothing.
		{"delayed_self_skips_returned_source", false, false},
		{"aegis_copy_hides_older_copy_exceptions", true, false}, // CR 613.7e, 707.2: Oko's Assimilation Aegis copy of Daring Thunder-Thief is newer than his until-end-of-turn copy, so "except he has hexproof" is overwritten and returns with that layer
		{"self_copy_layer", false, false},                       // CR 707.2, 707.9b: Oko (animated a 3/3 creature by Emergent Haunting's permanent animate) targets himself: a new layer keeps his copiable values and adds hexproof
		{"copied_vehicle_counts_its_crew", true, false},         // CR 707.2: Loan Shark copying Luxurious Locomotive makes a Treasure per creature that crewed it
		{"aegis_rebase_keeps_once_per_turn", true, false},       // CR 602.5c, 613.1a: an Aegis copy ending while another keeps the same copy does not make its once-each-turn ability new
		{"enters_with_counts_entrant_controller", true, false},  // CR 109.5: Dust Animus returned by a foreign Mystical Tether counts its own controller's untapped lands
		{"counters_skip_a_ceased_token", true, false},           // CR 111.7: Railway Brawler's counters for a Beau the legend rule removed land on nothing
		// CR 707.10c, 115.3: Jace Reawakened's copy of Fleeting Reflection may
		// not move "up to one other target creature" onto the creature its
		// first target already names
		{"copy_retarget_unique_link", false, false},
		// CR 400.7: a Stubborn Burrowfiend saddled, bounced by Nurturing Pixie
		// and recast is a new object, so its first saddle that turn triggers
		// again (gorge's latch stops at the Mount's own zone change).
		{"rebought_mount_saddles_first_again", true, false},
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
		// CR 603.2d: Annie Joins Up makes Roxanne, Starfall Savant's triggered
		// mana ability trigger twice, so a Meteorite tap brings two extra mana
		// that gorge's planner prices as one: the kernel stops as the
		// declared StallTapExtra rather than floating one fewer
		{"tap_extra_trigger_echoed_by_panharmonicon_stalls", false, true},
		// CR 301.7b, 303.4f: One Last Job returns Demonic Ruckus "attached to
		// a creature you control" while a crewed Luxurious Locomotive is an
		// artifact creature, so the controller chooses between it and
		// Stingerback Terror (gorge read the Enchant spec off printed types)
		{"attach_filter_reads_crewed_vehicle_as_creature", false, false},
		// Oko, the Ringleader's -5 copies each other permanent you control and
		// a copy's as-enters ask parks the sweep; the parked answer leaves
		// the sweep's zone batch to fire once, in entry order, when the sweep
		// ends (CR 603.6a): the Railway Brawler copies' "another creature
		// enters" see the Duelist and the later copies enter before the copy
		// the answer was made on
		{"copy_all_sweep_batch_spans_parked_ask", false, false},
		// CR 707.2: Three Steps Ahead's token copy of an Assimilation Aegis
		// bearer is minted on the bearer's printed card (Shackle Slinger)
		// with the copied face (Geralf) as its copy basis, so the token is
		// projected by its printed name exactly as gorge mints it
		{"token_copy_of_aegis_bearer_name", false, false},
		{"plot_cast_pays_additional_sacrifice", false, false}, // CR 118.9, 701.34d: a plotted Corrupted Conviction is cast free of its mana cost but still sacrifices a creature
		{"plot_cast_needs_a_sacrifice_source", false, false},  // CR 118.9, 601.2h: the free plot cast is offered only while its additional sacrifice can be paid
		// Explosive Derailment ({R}, Spree, every mode {2}) on a board whose
		// only red source is Conduit Pylons' {1},{T} filter: the printed cost
		// is payable, but no mode is, so neither engine offers the cast (CR
		// 702.172a/702.183a: the mode cost is part of the cheapest legal total).
		{"explosive_derailment_needs_a_mode_cost", false, false},
		// CR 601.2c/f, 118.8: a Spree/Charm per-mode target ask withholds
		// Terror of the Peaks when its controller's "an additional 3 life"
		// leaves the caster unable to pay, as the root target ask does
		{"charm_mode_targets_skip_unaffordable_terror", false, false},
		// CR 601.2f: This Town Ain't Big Enough cast from Jasper Flint's
		// mayplay: the mayPlay arm prices the full price from the sources
		// first, and its target retry prices one matching target from the
		// pool alone.
		{"mayplay_target_dependent_several_targets", false, false},
		// CR 601.2f, 118.7: This Town Ain't Big Enough's two-target
		// declaration earns its {3} reduction when ONE target is a permanent
		// the caster controls. The potential planner prices the cheapest
		// legal target set, and a cast payable only by spending that target
		// pays it at CR 601.2g, after the targets are chosen.
		{"multi_target_reducecost_priced_at_cheapest_set", false, false},
		// CR 603.3b: One Last Job returns Assimilation Aegis attached; gorge scans the move (its ETB trigger) before the attach (its Attached trigger), so the ETB trigger is queued first
		{"entered_attached_etb_before_attached_trigger", false, false},
		{"aegis_on_temporary_copy_bearer", true, false},     // CR 613.7e, 707.2: Assimilation Aegis attached to a creature Fleeting Reflection made a copy until end of turn copies over it (the later copy wins), and gives it back to that copy when unattached
		{"clone_on_aegis_bearer_until_eot", true, false},    // CR 613.7e, 514.2: Fleeting Reflection's until-end-of-turn copy over an Assimilation Aegis bearer ends at cleanup, and the Aegis copy it covered shows again
		{"resolved_spell_copy_rests_in_exile", true, false}, // CR 707.10a: a copy of a spell is exiled as it leaves the stack (gorge's spellRestZone), not put in a graveyard and ceased later
		// Omenport Vigilante's double strike turns on mid-way through a
		// Boom Box activation: gorge records each target ask of an ability
		// as a TargetsChosen as it is applied, so the per-turn crime ledger
		// sets at every pick (CR 700.13), not only at placement, and the
		// kernel follows (act_commitcrime.go crimePick).
		{"crime_ledger_set_at_each_ability_target_pick", true, false},
		// A Boom Box its opponent stole with Shifting Grift is activated by
		// the thief against itself, its own creature and its own land: no
		// crime at the picks (CR 700.13 judges the targeting, CR 601.2c,
		// before the 601.2h cost), and the self-sacrificing cost does not
		// re-judge the target in the owner's graveyard at placement
		// (act_commitcrime.go crimeStep reads the entry's recorded verdict).
		{"crime_pick_verdict_not_rejudged", false, false},
		// Vadmir, New Blood's once-each-turn crime trigger is enqueued at the
		// CR 601.2c target answer (the ward walk's crime step); an unpayable
		// Spree cast then reverses, and the reversal gives back the
		// once-per-turn record the queued trigger consumed (gorge abortCast's
		// proposalLimits, CR 733.1), so the retried cast triggers again
		{"crime_limit_restored_on_reversed_cast", false, false},
		// Three Steps Ahead's chosen modes cost more than the board can pay:
		// the manual CR 601.2g window taps an any-colour source and the
		// colour ask carries no announced hybrid pip, so gorge answers its
		// lowest colour, not the {U} the composed cost still owes.
		{"spree_window_any_colour_lowest", false, false},
		// CR 601.2c, 601.2f: Throw from the Saddle's second target (its
		// SubAbility$ Bite) is announced on cast, and the caster is at 1
		// life against Terror of the Peaks' "spells your opponents cast that
		// target it cost an additional 3 life": the link's menu withholds
		// the Dragon because the total cost can't be paid, as the root
		// target's ask already did.
		{"sub_target_life_raise", false, false},
		// Intimidation Campaign, cast this turn under the opponent's High
		// Noon, returns to hand on its crime trigger: its finished cast is
		// still the one spell that turn, so neither engine offers it again
		// (CR 601.2e; gorge excluded every push of the same ObjID).
		{"cast_limit_counts_a_bounced_spell", false, false},
		{"clone_rebase_locked_face", false, false}, // CR 707.2b: a clone re-base keeps the values locked when its layer was applied
		// CR 733.1, 733.2: a granted-flashback cast's own grant clear during
		// the proposal is undone by its reversal, so it is no progress for the
		// hold-out window: the hand copy's second no-progress abort stays held
		// out instead of being re-offered (pair 478, Phantom Interference).
		{"flashback_grant_clear_keeps_holdout", false, false},
	})
}

// std-blb's, on the same terms.
func TestStdBlbReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-blb", []stdRepro{
		{"blinked_creature_is_a_target_of_its_own_leave_trigger", true, false}, // CR 603.3d: Three Tree Scribe blinked and back is a legal target of its own trigger
		{"x_cast_leaves_a_target", false, false},                               // For the Common Good from one land and two Treasures: the largest-X plan consumes both copies targets, so the witness prices a smaller X whose payment keeps one Treasure as a target
		{"activation_cost_removes_target_becomes_target", true, false},         // CR 601.2c, 601.2g-h: Nettle Guard's {1}, Sacrifice: Destroy ability aims at itself, so its Valiant BecomesTarget trigger fires before the sacrifice that is part of the same activation's cost
		{"emblem_granted_storm_trigger", false, false},                         // CR 702.40a: Ral, Crackling Wit's emblem grants Splash Portal storm, so its cast trigger is ordered beside the prowess triggers
		{"emblem_storm_grant", true, false},                                    // CR 702.40a: Ral, Crackling Wit's emblem grants storm to the instant cast after it, so the granted-storm cast trigger joins the ordering ask
		// CR 701.21a, 610.3d: Scavenger's Talent's sacrifice-three cost includes Banishing Light, so its card returns after all three are gone
		{"blb_sac_cost_exile_return_after_all_sacrificed", true, false},
		{"blb_sac_cost_exile_return_after_all_sacrificed_sword", true, false},
		// CR 601.2a, 601.2f, 106.6: Head of the Homestead pays its hybrid pips through Helga's floating restricted {G}, so the spell is on the stack when the target/pip asks come
		{"target_dependent_cost_spell_on_stack_at_first_ask", false, false},
		// CR 601.2c, 733.1: Wishing Well casts Sazacap's Brew from the graveyard with the hand empty, so its discard cost is unpayable and the cast is reversed at the target ask, after the gift election
		{"free_cast_unpayable_cost_aborts_at_target", true, false},
		{"common_good_double_x_no_autopay_panic", false, false},         // CR 601.2b: For the Common Good paid with two Treasures whose largest X eats both targets: the witness is the smaller X that consumes one, not the unpayable X = 0 that keeps both
		{"x_cast_unreachable_witness_no_prefix", false, false},          // CR 601.2b: a Treasure pays For the Common Good's {G} at X = 0 and is the only token it copies, so gorge's verdict is witness_unreachable and no prefix search runs
		{"x_cast_rider_keeping_plan", false, false},                     // CR 609.4b, 601.2b: Cruelclaw's Heist's any-type rider pays For the Common Good's {G} from a Swamp, so the keeping plan leaves the Treasure as its target
		{"x_cast_rider_keeping_plan_lowered", false, false},             // CR 609.4b, 601.2b: the same cast played: its payment taps the rider's keeping plan, not the smaller-X witness
		{"copy_keyword_exception_returns_when_loss_ends", false, false}, // CR 707.9b, 613.1f: Mockingbird copying Shoreline Looter, made to lose all abilities until its controller's next turn, has its copy exception's flying again when the loss ends
		{"any_one_colour_restricted_x_colour_choice", false, false},     // CR 106.6, 601.2b: Helga's two restricted mana count in the {X} ceiling of Mockingbird's cast, so its witness taps every source for X = 4
		// CR 106.6, 601.2g-h: Helga's own restricted any colour is tried before Clement's creature-only {G}, which reaches the same floated pool and tapped set, so gorge's search drops the state unpriced and offers no offspring cast of Bushy Bodyguard
		{"seen_state_drops_granted_colour_after_printed_any", true, false},
		// CR 106.6, 601.2g-h: gorge's search answers Helga's "choose a mana ability" ask with the colour the cost names first, so Clement's {G}{U} and Burrowguard Mentor's {G} find their prefix, while Thistledown Players' {W} leaves her own any colour first and the granted {G} is a seen state: no prefix
		{"needed_granted_colour_answered_before_printed_any", false, false},
		// CR 106.6, 302.6, 601.2g-h: Rockface Village's {R},{T} pump is paid by Helga's two restricted {R}, Clement's {G}, a Forest's {G} for Three Tree Mascot's summoning-sick Changeling filter, whose {1} needs no tap
		{"sick_changeling_filter_pays_prefix_activation", true, false},
		// CR 614.12, 707.2: the cards one return instruction returns each make
		// their as-enters choice before any of them is on the battlefield, so
		// Mockingbird's copy election is asked with the other targets still in
		// the graveyard
		{"return_batch_asenters_two_creatures", false, false},
		{"return_batch_asenters_three_creatures", false, false},
		{"return_batch_asenters_three_creatures_m1", false, false},
		{"restricted_witness_tap_survives_hybrid_lowering", false, false},
		{"granted_mana_ask_tries_needed_colour_first", false, false},
		{"clone_template_entry_condition_mockingbird_cutthroat", false, false},
		{"clone_template_entry_condition_mockingbird_sparkshooter", false, false},
		// CR 603.3b: Season of Loss makes each player sacrifice a creature, so Scavenger's Talent's die trigger and its per-sacrifice triggers are all queued: gorge walks each departure's triggers in line order (the die trigger first)
		{"sacrifice_triggers_walk_order_batch_sacrifice_283", true, false},
		{"sacrifice_triggers_walk_order_batch_sacrifice_1088", false, false},
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
		{"copy_cast_enters_as_cast", false, false},                      // CR 707.12: Q's Play of Crystalline Entity casts the copy, so its "if you cast it" enter trigger fires beside Sickbay's
		{"refused_paid_copy_cast_keeps_window_tap", true, false},        // CR 733.1: Encounter the Unknown's copy cast that gorge cannot pay is reversed after its mana window, so the tapped source and its mana stay
		{"exile_grant_host_is_a_copy", false, false},                    // CR 707.2, 603.6a: Dominion Saboteur enters as Q, Stacking the Deck, so its enter trigger's exiled Assembly Specialist gains Q's exile-from-hand cast ability, which the copy's host offers
		{"turn_once_resets_while_phased_out", false, false},             // CR 602.5b, 702.26d: DOT-7 Repair Squad activated, then phased out through its turn's end, has its once-each-turn limit lifted when it phases back in
		{"phased_in_permanent_rederives_override_types", true, false},   // CR 702.26c, 613.1: an assimilated Lwaxana Troi and a Station-charged Borg Cube phase in during the untap step, and the next decision projects their continuous artifact/creature types
		{"clone_template_choice_asked_on_stack", false, false},          // CR 614.12, 707.9: Dominion Saboteur entering as a copy of Nyota Uhura is asked her "choose a number" with the Saboteur still on the stack, after the copy election
		{"returned_clone_order_ask_beside_counter_source", true, false}, // CR 616.1: Dominion Saboteur returned from exile as a copy of Troubling Tribbles beside Deanna Troi's extra counter is asked the replacement order with the card still in exile
		{"until_next_turn_pump_misses_returned_creature", false, false}, // CR 400.7, 611.2c: V'Ger's "-1/-0 until your next turn" does not reach Hologram Janeway that Ghost of Illyria blinked back, a new object
		{"two_draw_mores_settle_unasked", true, false},                  // CR 616.1, 614.5: two mandatory "draw N instead" replacements compose as N1·N2 cards either order, so gorge settles in scan order unasked and the kernel walks the chain through the rest
		{"copy_cast_x_spell_asks_x_before_abort", false, false},         // CR 601.2b, 601.2c: Q's Play of Khaaaaaaaaaaaannn! announces X before its target ask, so a copy cast no mana can pay still asks X = 0 and puts the copy on the stack before the target ask reverses it
		{"clone_entry_replacement_sees_copy", false, false},             // CR 614.12, 707.9b: Dominion Saboteur entering as a copy of U.S.S. Enterprise-D is a Spacecraft as it enters, so Zefram Cochrane's two extra charge counters join the copied one
		{"copy_cast_mode_target_unpayable", false, false},               // CR 601.2c, 733.1: Q's Play of Make It So announces its untap mode, whose required target the opponent's three untapped lands cannot pay for, so the cast is reversed at the target ask
		{"dilemma_cast_reversed_no_target", true, false},                // CR 601.2c, 733.1: Q's Play of Stray from History, a copy cast the opponent's two Mountains cannot pay, is reversed at the "target opponent" ask, after the dilemma and before any tap, as gorge withholds every target of an unpayable cast
		{"unpayable_x_copy_cast_asks_x", false, false},                  // CR 601.2b, 601.2c: Q's Play of Khaaaaaaaaaaaannn! with every source tapped still announces X before its target, so the copy sits on the stack at the X ask
		{"unpayable_x_copy_cast_asks_x_then_target", false, false},      // CR 601.2b, 601.2c: the same X copy cast in pair 1331's game still asks X, then aborts at its required target when the announced X leaves it unpayable
		{"unpayable_modal_copy_cast_aborts_at_target", true, false},     // CR 601.2b, 601.2c: Q's Play of Set Phasers to . . . with every source tapped announces its mode, then aborts at the chosen mode's target ask
		{"graveyard_shuffle_moves_one_batch", false, false},             // CR 603.2c: Those Old Scientists' two-card shuffle is one event, so Sela's "one or more cards leave your graveyard" triggers once
		{"name_choice_beside_counter_rider_no_order", false, false},     // CR 614.12, 616.1: Kai Winn entering beside Deanna Troi's extra counter is asked only the name, as the two compose in either order
		{"name_hash_collision_both_offered", false, false},              // CR 201.4: Jadzia Dax and Xenobotanist hash alike as chosen names, yet both stay distinct options on each side
		{"exile_return_name_asked_mid_cost", false, false},              // CR 614.12, 610.3: In the Pale Moonlight's sacrifice of Malfunctioning Holodeck returns Kai Winn, who is asked her name in exile before the two Romulan tokens are made
		{"copy_cast_unaffordable_target_reverses", true, false},         // CR 601.2c, 733.1: Q's Play of Perils of the Past copies a {1}{R} instant with no red source untapped, so the cast is reversed before its target is asked
		{"stale_trigger_referent_reentered", false, false},              // CR 400.7: Support Mission's counter trigger fired for Dr. Leonard "Bones" McCoy's first entry puts nothing on the copy that left and was cast back in
		{"copy_cast_mode_target_unpayable_q_play", true, false},         // CR 601.2c, 733.1: Q's Play of Make It So paid with an exiled graveyard card announces its untap mode, whose required target the pool cannot cover, so the copy cast is reversed at the target ask with no window opened (autopay surface)
		{"phased_out_copy_statics_stop_applying", false, false},         // CR 702.26b: Dominion Saboteur entered as a copy of Sarcophagus, Ship of the Dead and phased out with it, so neither gives Species 8472 +1/+0
		{"phased_out_permanent_grants_no_land_plays", false, false},     // CR 702.26b: Nog, Ambitious Cadet phased out by The City on the Edge of Forever does not exist, so its granted "play lands from your graveyard" offers no graveyard Forest
		{"unpayable_x_copy_cast_aborts_at_target", true, false},         // CR 601.2b, 601.2c: Q's Play of Khaaaaaaaaaaaannn! (a copy cast no source can pay) asks X, then aborts at its target ask, leaving no copy on the stack
		{"play_copy_cast_unpayable_reversed_at_target", false, false},   // CR 601.2c, 733.1: Perils of the Past exiled, its Play-granted copy cast the window cannot pay is reversed at its required target ask, leaving the stack as gorge has it
		{"phased_out_copy_grant_survives", true, false},                 // CR 702.26b: a phased-out copy's granting static is silent, but phase-out unsubscribed only the printed card, so the copy's grant still fired
		{"copy_cast_zero_x_reaches_window", false, false},               // CR 601.2c, 601.2g, 733.1: Q's Play of Dominion Infiltration announces X = 0, so its "X target creatures" asks no menu and no affordability census reverses it; the cast reaches its mana window, and the taps and mana stand when it is then reversed
		{"copy_cast_zero_x_reaches_window_autopay", true, false},        // CR 601.2c, 601.2g, 733.1: ... the same on the autopay surface, where two Mountains tap for the {R}{R}{R} the pool cannot cover
		{"copy_card_play_on_stack_before_mode_asks", false, false},      // CR 601.2a, 707.12: Q's Play of an exiled Infinite Diversity puts the copy on the stack before its target ask, and the Treasure the census cannot price keeps that ask on the menu
		{"deanna_spacecraft_enterprise_a_s125", false, false},           // CR 614.12, 122.6: Deanna Troi's extra +1/+1 counter joins a Spacecraft entering with seven charge counters
		{"deanna_spacecraft_oberth_s162", false, false},                 // CR 614.12, 122.6: Deanna Troi's extra +1/+1 counter joins a Spacecraft entering with seven charge counters
		{"deanna_spacecraft_suurok_s193", false, false},                 // CR 614.12, 122.6: Deanna Troi's extra +1/+1 counter joins a Spacecraft entering with seven charge counters
		{"deanna_spacecraft_enterprise_a_s203", false, false},           // CR 614.12, 122.6: Deanna Troi's extra +1/+1 counter joins a Spacecraft entering with seven charge counters
		{"deanna_spacecraft_enterprise_d_s205", false, false},           // CR 614.12, 122.6: Deanna Troi's extra +1/+1 counter joins a Spacecraft entering with seven charge counters
		{"copy_each_target_copies_enter_together", true, false},         // CR 603.6a: Dominion Infiltration's copies of V'Ger and Captain Janeway enter together, so the Janeway copy also triggers for the V'Ger copy
		// CR 605.3a, 601.2g-h: Starfleet Academy's filter makes seven, past the relaxed census's six, so gorge's prefix search funds no fee and taps Rok-Tahk's granted {G}{G} on three creatures before the filter's {B} pays Artificial Anomaly
		{"granted_creatures_fund_unrelaxable_dynamic_filter", false, false},
		{"first_charge_counter_each_turn_after_bounce", false, false}, // CR 400.7, 603.2: U.S.S. Enterprise-D bounced and recast this turn is a new object, so its first charge counters after the recast queue "for the first time each turn" again
		{"unrelaxable_filter_no_grant_funding", false, false},         // CR 601.2g-h: Starfleet Academy's X=7 filter makes gorge's census unrelaxable, so the grant search funds no fee and Rok-Tahk's granted {G}{G} from three creatures pays the {5}
	})
}

// std-fdn's, on the same terms.
func TestStdFdnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-fdn", []stdRepro{
		{"surveil_own_replacement", true, false},                     // a surveil's graveyard pile meets Darksteel Colossus's "from anywhere"
		{"color_gate_made_black", true, false},                       // Knight of Grace counts what Eaten by Piranhas made black
		{"color_gate_recoloured", true, false},                       // ... and not what Witness Protection made green and white
		{"color_gate_beside_live_colour_effect", false, false},       // CR 613.1e: Knight of Malice counts Three Tree Mascot Witness Protection made green and white beside Imprisoned in the Moon's live colourless
		{"exile_instead_planeswalker", true, false},                  // CR 700.4: a planeswalker with no loyalty dies, so Obliterating Bolt exiles it
		{"lifelink_plus_per_source", true, false},                    // two lifelink sources are two gains, each with Angel of Vitality's plus one
		{"lost_ability_no_shuffle", true, false},                     // CR 613.1f: Witness Protection on Darksteel Colossus: it dies
		{"damage_map_per_recipient", true, false},                    // Niv-Mizzet draws for the 3 to the opponent, not Drakuseth's whole 10
		{"copies_enter_together", true, false},                       // CR 603.6a: each of Rite of Replication's five sees the other four
		{"flashback_restricted_source", true, false},                 // Secluded Courtyard's any colour cannot pay Mystical Teachings' printed flashback
		{"granted_any_mana_beside_own_trigger_x", false, false},      // New Horizons on a Forest adds an any-colour ability beside its own, so Wildborn Preserver's "you may pay {X}" counts it in the potential pool as gorge's walk does
		{"trigger_paid_x_zero_binds", false, false},                  // CR 107.3: Wildborn Preserver paid X = 0 puts no counters, whatever X the creature that triggered it was cast with
		{"dies_trigger_reads_attacking_lki", true, false},            // CR 603.10a: Garna, Bloodfist of Keld draws for a Perforating Artist that died attacking, its last known status
		{"any_type_rider_hybrid_pips", true, false},                  // CR 107.4e, 601.2b: Tinybones lets any mana pay Teach by Example's two hybrid pips, so W and B afford the cast
		{"deferred_mana_pays_x_announcement", true, false},           // CR 601.2b, 601.2g: Pyromancer's Goggles' deferred {R} is a source the window taps, so Steel Hellkite's {X} is offered for 1 (mana_deferred.go)
		{"name_look_ack_mid_resolution", true, false},                // CR 608.2n: a copy token's looked-hand name ask is acknowledged while the creating ability is still on the stack
		{"emblem_grant_after_ability_loss", false, false},            // CR 613.7: Vivien Reid's emblem, created after Witness Protection took Llanowar Elves' abilities, is the newer layer-6 effect, so its vigilance, trample and indestructible survive the loss
		{"unless_discard_is_effect_discard", true, false},            // CR 118.12: discarding Wilt-Leaf Liege to an opponent's Perforating Artist "unless" is that ability's discard, so the Liege enters the battlefield
		{"deferred_gap_beside_filter", true, false},                  // CR 605.3a, 602.2b: Pyromancer's Goggles is a census gap beside Three Tree Mascot's filter, so its {R} funds Tinybones' {3}{B}
		{"deferred_mana_funds_trigger_cost", true, false},            // CR 106.6: Pyromancer's Goggles' {R} has no spending restriction, so an untapped Goggles alone funds Frenzied Goblin's "you may pay {R}": pay is offered beside decline
		{"name_look_ack_mid_resolution_copy_spyglass", false, false}, // CR 614.12: Extravagant Replication's copy of Sorcerous Spyglass, entering mid-resolution, asks the name after the look at the hand, the trigger still on the stack
		{"filter_beside_deferred_script", true, false},               // a filter (Three Tree Mascot) beside a deferred TriggersWhenSpent$ source (Pyromancer's Goggles): the filter-only prefix search cannot fund the offer, so the restricted/deferred script pays Quick-Draw Katana's Equip {2}
		{"filter_beside_deferred_script_full_surface", false, false}, // CR 605.3a, 602.2b: the reported pair 36 full-surface Quick-Draw Katana Equip {2} offer: a filter (Three Tree Mascot) beside a deferred TriggersWhenSpent$ source (Pyromancer's Goggles) prices the restricted/deferred script, as gorge does
		{"deferred_gap_beside_filter", true, false},                  // CR 605.3a, 602.2b: Pyromancer's Goggles is a census gap beside Three Tree Mascot's filter, so its {R} funds Tinybones' {3}{B}
		{"control_untap_fires_equip_untaps", true, false},            // CR 701.21c, 603.2: Involuntary Employment untaps p0's Vanguard Seraph, so p0's Fishing Pole on it triggers
		{"control_untap_fires_equip_untaps_two", true, false},        // ... and the same with Clinquant Skymage
		{"effect_untap_fires_equip_untaps", true, false},             // ... and a resolved ability untapping Slumbering Cerberus triggers its Fishing Pole
		{"combat_damage_past_int16_kills", true, true},               // CR 120.3, 704.5g: 32768 combat damage from Heroes' Bane kills the blocking Guttersnipe; the damage record saturates (StallPT past int16), never wraps
		{"vizier_conversion_kicked_cast", true, false},               // CR 609.4b, 702.33: under Vizier of the Menagerie three Forests pay Gatekeeper of Malakir's kicked {B}{B}{B}, so the kicked cast is offered beside the plain one
		{"vizier_conversion_kicked_cast_two", true, false},           // ... the same in another game
		{"vizier_conversion_kicked_cast_three", true, false},         // ... and another
		{"vizier_conversion_kicked_cast_four", true, false},          // ... and another
		{"vizier_kicked_cast_granted_prefix", true, false},           // ... and Gnarlid Colony's kicked {3}{G}{G} beside Imprisoned in the Moon's granted {C}: the grant search prices the kicked total under the conversion too
		{"deferred_gap_beside_granted_land", true, false},            // CR 601.2g-h, 605.1a: Pyromancer's Goggles' deferred {R} beside Giant Cindermaw's Imprisoned-in-the-Moon {C} pays Pirate's Cutlass's Equip {2}: the restricted/deferred script search must count the unrestricted grant
		{"goggles_deferred_red_play_offered", false, false},          // CR 601.2g-h, 605.3a: Blossoming Sands and Pyromancer's Goggles' deferred {R}, one funding Three Tree Mascot's {1} for {U}, pay Negate: the filter search runs the Goggles as a gap step
		{"deferred_red_funds_first_filter_fee", false, false},        // ... and the Goggles' {R} alone funds the Mascot's {1}, so An Offer You Can't Refuse's {U} is offered
		{"counter_gap_beside_deferred_script", false, false},         // CR 601.2g: Ramos, Dragon Engine's counter-cost mana beside the Goggles pays Zetalpa, Primal Dawn: the deferred script's dead-end check plays the counter gap as the prefix does
		{"deferred_gap_beside_granted_mana", false, false},           // CR 601.2g-h, 605.1a: gorge's prefix search taps New Horizons' Mountain for {R}{R} and Pyromancer's Goggles' deferred {R}, both census gaps, so Lathliss, Dragon Queen's {4}{R}{R} is offered
	})
}

// std-eoe's, on the same terms.
func TestStdEoeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-eoe", []stdRepro{
		{"station_departed_creature_reads_toughness_static", true, false},      // CR 702.184a, 608.2h: Survey Mechan, tapped to station Uthros Scanship and then sacrificed, stations with its toughness under Tapestry Warden
		{"restricted_grant_no_unit_for_hand_cast_x", false, false},             // Mm'menon grants an untapped Specimen Freighter blue for spells not cast from hand, so Zero Point Ballad from hand has no extra mana to announce X with
		{"restricted_grant_no_unit_for_hand_cast_x_two", false, false},         // ... and Devastating Onslaught's X likewise stops one short
		{"restricted_grant_no_unit_for_hand_cast_pip", true, false},            // ... and a hand warp's {U/R} pip asks one colour, not the blue the Freighter's grant cannot pay a hand cast with (CR 106.6)
		{"restricted_grant_no_unit_for_trigger_cost", true, false},             // ... and Oreplate Pangolin's "you may pay {1}" is no spell, so an artifact holding Mm'menon's blue pays nothing: only decline is asked
		{"restricted_grant_no_unit_for_unless_window", true, false},            // CR 106.6: ... and no source for an unless cost either, so Divert Disaster's {2} taps Godless Shrine, not the Freighter holding the granted blue
		{"granted_batch_attack_trigger_once_per_declaration", false, false},    // CR 508.1, 603.2: a granted "whenever you attack" trigger fires once for a declaration split across two defenders, not once per defender
		{"dies_trigger_power_threshold_lki", true, false},                      // CR 603.3b: Syr Vondam's own dies trigger queues before the dies trigger Perigee Beckoner granted it
		{"return_defender_asked_before_move", true, false},                     // CR 508.4: Chorale of the Void's attacking return asks its defender while the card is still in the graveyard, as gorge does, not after it entered
		{"station_reads_off_battlefield_cda_power", true, false},               // CR 604.3, 702.184a: Adagia's Station counts a sacrificed Harmonious Grovestrider by its lands-you-control power, which works in the graveyard, not by its printed 0
		{"free_mayplay_joins_a_zone_grant", true, false},                       // Weftwalking's free-only static joins Mm'menon's top-of-library grant, so the first spell is cast for free (CR 118.9)
		{"granted_search_target_ask_gorge_order", true, false},                 // a grant search branches a stacked trigger's target ask over gorge's answer order (gorgeTargetOrder), not the kernel's slot order: Nanoform Sentinel's "untap another target permanent" untaps the first opponent permanent, not its controller's first land
		{"tap_another_activation_beside_granted_mana", true, false},            // CR 605.1a, 601.2g: the grant cap prunes a Lander's {2},{T},Sac activation the granted spell-only mana cannot pay, but a tap-another-permanent source beside it can, so the activation stays on autopay's menu
		{"tap_another_source_beside_restricted_grant", false, false},           // CR 601.2g-h: beside Mm'menon's hand-barred granted blue, Gene Pollinator's tap-another-permanent mana pays Mechanozoa's {U}{U} from hand, so the plain cast is offered
		{"each_return_asks_election_per_target", false, false},                 // CR 614.12, 608.2c: Pull Through the Weft returns its two shock lands one at a time, each one's pay-life election asked before the next moves
		{"unspendable_grant_is_a_prefix_gap", false, false},                    // CR 601.2g-h: Mm'menon's hand-barred granted blue prices no hand cast (Glacier Godmaw is not offered off it), yet gorge's prefix still taps Nanoform Sentinel for it as a gap step, the blue floating
		{"warp_cast_from_hand_spends_no_hand_barred_grant", true, false},       // CR 106.6, 702.185a: a warp cast is from the hand, so Nanoform Sentinel's Mm'menon-granted blue cannot pay Codecracker Hound's warp {2}{U} beside two Plains; the warp cast is not offered
		{"relaxed_proof_refuses_restricted_grant", true, false},                // CR 106.6, 602.2b: Mm'menon's spell-only granted blue is no relaxed unit for Atomic Microsizer's Equip {2}, so gorge proves it unpayable off Godless Shrine alone and never searches the Nanoform Sentinel untap line
		{"station_monitor_hand_cast_needs_no_hand_barred_grant", false, false}, // CR 106.6: Mm'menon's hand-barred granted blue is no relaxed unit for Station Monitor's plain hand cast, so neither copy is offered
		{"warp_hand_cast_no_hand_barred_grant_offer", true, false},             // CR 106.6, 702.185a: with every land tapped, only Mm'menon's hand-barred granted blue on the artifacts could pay Sinister Cryologist's warp {U}, and a warp cast is from the hand, so it is not offered
		{"warp_hand_cast_gated_filter_no_stand_down", true, false},             // CR 106.6, 702.184a: ... and Evendo, Waking Haven's {G},{T} filter is no mana ability below STATION 12, so it does not shield Mechanozoa's and Starbreach Whale's hand warp casts from the prune
		{"warp_hand_cast_tapped_filter_no_stand_down", false, false},           // CR 106.6, 302.6: ... and a tapped Uthros, Titanic Godcore past STATION 12 funds nothing either, so it does not shield Sinister Cryologist's hand warp cast from the prune
		{"ward_pays_through_tap_another_source", true, false},                  // CR 601.2g, 702.21a: the ward {2} pay answer opens the mana window, whose census leaves out Gene Pollinator's tap-another-permanent mana, so the spell is paid not countered
		{"ward_pays_through_tap_another_source_two", true, false},              // ... and Sunset Saboteur's attack trigger is paid the same way
		{"ward_failed_pay_skips_unspendable_grant", true, false},               // CR 106.6: the failed pay arm taps what the window lists, not Mm'menon's granted artifact blue a ward cannot spend
		{"grant_cap_tapx_activation_torpedo", false, false},                    // CR 605.1a, 601.2g-h: Thaumaton Torpedo's {6},{T},Sac is paid by five lands and Gene Pollinator beside Mm'menon's grant: the granted-only artifact is no unit to take out of the prefix search's availability
		{"grant_cap_tapx_activation_lander", false, false},                     // ... and a Lander token's {2},{T},Sac by one Island and Gene Pollinator
		{"trigger_order_menu_lists_same_pending_triggers", false, false},       // CR 603.10a: Devastating Onslaught's Encore sacrifices its two Lightless Evangel tokens together, so each token's sacrifice trigger sees the other go
		{"mmmenon_hand_barred_blue_not_spent_on_hand_cast", true, false},       // CR 106.6, 602.2b: Mm'menon's hand-barred granted blue is no relaxed unit for Atomic Microsizer's Equip {2} nor Sinister Cryologist's hand warp, and the tap-another search alone (Gene Pollinator tapping Nanoform Sentinel) may not keep the play gorge's one prefix search gave up on
		{"mmmenon_hand_barred_blue_not_spent_on_hand_cast_full", false, false}, // ... and Biosynthic Burst from the hand, on the full surface
		{"mmmenon_library_top_cast_binned_by_prefix_draw", true, false},        // CR 400.7: Mechan Navigator's tap draws the library-top Cryogen Relic and discards it, so the cast the granted blue would pay no longer exists
		{"plan_tap_skips_granted_mana_ability", true, false},                   // CR 106.6, 702.185a: the plan's All-Fates Scroll tap is its printed any-colour ability, not Mm'menon's granted hand-barred {U} the wheel also lists, so the Starwinder warp cast from hand is paid and made
		{"starwinder_warp_needs_second_blue", true, false},                     // CR 106.6, 702.185a: Starwinder's warp {2}{U}{U} has only Uthros's {U} beside Mm'menon's hand-barred artifact blue, and the live filter that makes grantCap stand down does not make the hand-barred grant a unit, so no warp cast is offered
		// CR 603.3, 602.2b: Cryoshatter on Zookeeper Mechan: its {T}: Add {R} fires the unvetted Taps watcher, so gorge's census never lists it and the {6}{R} sorcery pump it would help pay is not offered on autopay
		{"taps_deferred_source_pays_no_own_sorcery_activation", true, false},
		// ... and on the full surface
		{"taps_deferred_source_pays_no_own_sorcery_activation_full", false, false},
	})
}

// std-hob's, on the same terms.
func TestStdHobReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-hob", []stdRepro{
		{"equip_target_reducer_offered_from_lands", false, false},   // CR 601.2f/602.2b: Dwarven Mauler's {2}-less reduction for an equip targeting it makes Equip {4} payable from two lands, so the equip is offered
		{"sac_part_and_mana_share_a_treasure", false, false},        // CR 601.2g-h: the one Treasure that pays Stir Up Trouble's {B} is gone, so it is not also the artifact its either-or part sacrifices; the cast is not offered
		{"leaves_graveyard_looks_back_at_card_types", false, false}, // CR 603.10a: Tom, Bert, and William return as a noncreature artifact, yet a creature card left the graveyard, so Along the Crooked Way amasses
		{"graveyard_leave_sees_pre_move_types", true, false},        // CR 603.10a: Tom, Bert, and William returns from the graveyard as an artifact, and Along the Crooked Way still sees a creature card leave
		{"autopay_plan_prices_target_discount", true, false},        // CR 601.2f: Dwarven Mauler's {2}-less ValidTarget$ discount makes Ragged Short Spear's equip {3} payable from two lands, so the autopay witness prices it and the re-equip is offered
		{"hascards_graveyard_static_zero_a", false, false},          // CR 604.1: Master's Councillors' +2/+0 per graveyard with seven or more cards counts the graveyard that has them
		{"hascards_graveyard_static_zero_b", false, false},
		{"threshold_skips_a_waiting_token", false, false}, // CR 111.7, 704.5d: three Treasures sacrificed for mana wait in the graveyard, so they are no cards and do not switch Most Decrepit Old Bird's Threshold on
		// CR 603.3b, 704.5a: a Treasure spent for Duskwatch Hunter's hybrid
		// mana sets off The Sackville-Bagginses' drain before the cast; the
		// lowering's resolutions run their state-based check, which takes
		// the opponent's last life, so the spell stays in hand and the game
		// ends there rather than at a hybrid pip ask.
		{"lowered_hybrid_cast_lethal_sacrifice_trigger", false, false},
		// CR 106.4, 601.2h: Smaug, Wicked Worm's "mana from a Treasure was
		// spent" reads the mana the cast spent, not the Treasures tapped
		// before it: three Treasures sacrificed at priority leave their {G}
		// floating beside the Swamps' {B}, and Inside Information's {X}{B}{B}
		// is paid from the plain {B} (a plain unit goes before a Treasure's).
		{"floating_treasure_mana_unspent_by_cast", false, false},
		{"floating_treasure_mana_unspent_by_cast_two", false, false}, // ... and An Unexpected Party's
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
		{"cleanup_repeat_empties_window_mana", true, false},                 // CR 500.4, 514.3b: the mana Lotus Ring made in a cleanup priority window (a discard set off Hostile Investigator) empties as that step ends, before the repeated step's discard
		{"rest_in_peace_ask_between_cost_parts", true, false},               // CR 602.2b: the {T} part is paid before the discard, so the replacement ask the discard raises with two Rest in Peace sees Fomori Vault already tapped
		{"granted_sac_only_mana_prefix", true, false},                       // CR 602.2b, 704.5n: Lotus Ring's granted "Sacrifice this creature: Add three mana of any one color" is the prefix paying an activation, the equipped Hostile Investigator sacrificed and the Ring unattached
		{"granted_sac_only_mana_window", true, false},                       // CR 602.2b, 605.3a: Esoteric Duplicator's resolution-time {2} is paid in the manual mana window by sacrificing the Lost Jitte token Lotus Ring equips, a tapped creature whose granted ability has no {T}
		{"granted_sac_dies_token_copy_is_equip_target", true, false},        // CR 603.10a, 400.7: the prefix sacrifices the Lotus Ring-equipped Vaultborn Tyrant and its dies trigger's token copy, a new object, is the other Ring's Equip target; the search keys a payer state on its battlefield objects as gorge's scriptStateKey does
		{"explore_reveal_asks_replacement_order", true, false},              // CR 616.1, 701.44a: a Map's explore reveals a nonland card under two Rest in Peace, so its move to the graveyard asks which replacement applies; the card waits on top of the library for the answer
		{"granted_sac_mana_two_replacements", false, false},                 // CR 616.1, 602.2b: Lotus Ring's granted sacrifice mana, activated as the prefix with two Rest in Peace out, is asked which replacement applies and the script answers the first, so Oltec Matterweaver is exiled
		{"granted_sac_mana_sba_ask_after", false, false},                    // CR 704.3, 616.1: the sacrificed Construct token's death leaves its sibling too small, and the check's own replacement ask is answered first by the script, so both tokens are exiled and cease
		{"granted_sac_mana_offer_beside_trigger", false, false},             // CR 602.2b, 605.3a: with a trigger on the stack, the offer search of Esoteric Duplicator's {2} through Lotus Ring's granted sacrifice runs the sacrifice's replacement-order ask on its trial game, and the two sides agree on whether the activation is offered
		{"prefix_search_skips_sacrificing_own_source", true, false},
		{"sba_chain_tokens_wait_through_new_casualty", true, false}, // CR 704.3, 704.5d: combat casualties under two Rest in Peace are asked one by one; a creature that comes to die between the asks parks too, and the tokens already exiled by the earlier answers still wait for the pass that destroys nothing         // CR 602.2b: Lost Jitte's Equip {1} is paid by sacrificing the Bat token Lotus Ring equips, not by sacrificing the Jitte token itself (a prefix that removes the play's own source offers nothing)
	})
}

// std-tdm's, on the same terms.
func TestStdTdmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tdm", []stdRepro{
		{"shuffle_target_already_returned", false, false},          // CR 608.2b: Rite of Renewal's shuffled cards are in the graveyard still; one already returned to hand stays there
		{"renew_target_with_protection_not_offered", false, false}, // CR 115.2, 702.16b: Renew's source is the black card in the graveyard, so Ureni (protection from white and black) is no target and Alchemist's Assistant's ability is not offered
		{"declined_delve_picks_reverse_the_cast", false, false},    // CR 702.66a: declining the granted delve leaves the {2} unpayable; the mana window taps what it can and the cast is reversed (CR 733.1)
		{"omen_permanent_spell_copy", false, false},                // CR 720.2c, 720.4: an omen card is one card, so Flamehold Grappler's copy of an omen creature spell (Runescale Stormbrood) is an ordinary copy whose token is the front face, not a 707.10g double-faced token
		{"dig_aura_no_host_joins_the_rest", true, false},           // CR 303.4g: United Battlefront picks Fresh Start with nothing to enchant; it stays in the library and joins "the rest" on the bottom, so the six other window cards keep their relative order
		// Teval, Arbiter of Virtue's delve grant on an unresolved twobrid
		// (Gurmag Nightwatch): the cast is offered on the per-face credit
		// (CR 702.66a), and a payment the window's activations still cannot
		// finish reverses the cast keeping the floated mana (CR 733.1: a
		// player may reverse mana abilities activated for an illegal play)
		{"twobrid_delve_unpayable_window", true, false},
		// CR 107.4e, 702.66a: a card exiled by Cori Mountain Monastery's
		// may-play permission is still castable when Teval, Arbiter of
		// Virtue's granted delve pays the twobrid pips' generic faces, so
		// the offer is present from exile with an empty pool and six
		// graveyard cards (the hand walk already priced this).
		{"mayplay_twobrid_delve_offer", true, false},
		// CR 601.2g, 605.3a: Mardu Devotee's {1} mana ability's own fee is
		// paid from a Plains, so its {B} funds Caustic Exhale's {1} arm and
		// the cast is offered with nothing floated (std-tdm pair 419).
		{"fee_filter_source_funds_cast_offer", false, false},
		// CR 601.2g with CR 107.4e: the reversed twobrid cast's window taps
		// Opulent Palace's "Combo B G U" while both G and U are owed; gorge's
		// ask lists the Combo's printed order, so its answer is G, and the
		// floated mana must match (dirac answered the lowest WUBRG colour, U).
		{"window_combo_owed_colour_printed_order", false, false},
		{"printed_combo_window_colour_order", true, false}, // CR 601.2g: a printed Combo's mana window answers the ability's printed order, so Thornwood Falls makes {G} (before its {U}) and Frontier Bivouac stays untapped
		// CR 702.66a, 702.180a: a Harmonize creature whose power already
		// covers the whole generic part leaves no shortfall for Teval's
		// granted delve, so no graveyard card is offered (Ureni's Rebuff).
		{"delve_harmonize_covered_generic", true, false},
		{"copy_sacrificed_to_own_cost_reflexive_body", true, false},       // CR 707.2, 603.12: a copy that sacrifices itself to its ImmediateTrigger cost (Naga Fleshcrafter as Felothar) still queues the copied reflexive body
		{"alt_add_treasure_lowering_reaches_with_part_mana", true, false}, // CR 601.2f, 601.2g: Caustic Exhale ({B}, behold a Dragon or pay {1}) with a Treasure as the only {B}: gorge's lowering floats {B}{1} together and reaches the cast, so it is not a planned one; the kernel priced the witness without the {1}
		// CR 603.3b: Host of the Hereafter copies a legend's +1/+1 and flying
		// counters onto the survivor beside Hollowmurk Siege's Sultai
		// trigger, which hears both kinds only once each turn: the two orders
		// reach the same one trigger.
		{"counter_copy_once_listener", false, false},
		{"registered_effect_remembered_scope", true, false},
		{"registered_effect_remembered_scope", false, false},
		{"clone_returns_from_until_leaves_exile_asks_copy", true, false}, // CR 614.12, 707.2: a clone returning from "until this leaves" exile during a resolution is asked its enter-as-a-copy election, as gorge asks it
		// CR 601.2f: a per-target cost reprice hides the in-flight cast from the turn's cast counts, as the build's own pricing does
		{"per_target_reprice_hides_inflight_cast_from_cast_counts", false, false},
		// CR 601.2g-h: a hybrid cast whose dry census cannot price a filter's fee still completes when the real window pays it from the floated pool
		{"hybrid_cast_filter_fee_from_floated_pool", true, false},
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
		// CR 605.1a: White Lotus Hideout's Lesson-only any-colour ability is tapped in gorge's prefix search beside its {C} and the granted one, so Katara's GWU spends the 160-node budget there and is not offered
		{"restricted_printed_mana_branches_in_prefix_search", false, false},
		{"sorcery_play_waits_for_prefix_trigger", false, false},            // CR 307.1: Guru Pathik is cast after Master Pakku's tap trigger resolves, and every granted source is tapped before the pass
		{"airbent_card_with_self_flash", true, false},                      // CR 601.3b: Serpent of the Pass airbent to exile is cast at instant speed with three Lessons in the graveyard
		{"granted_mana_once_beside_another_grant", true, false},            // Great Divide Guide's granted mana is one ability of each grantee on a board with another global grant, so the prefix search finds Earthen Ally's {2}{W}{U}{B}{R}{G} within its budget
		{"granted_filter_funding_spends_search_budget", false, false},      // CR 605.3a: a filter's fee is funded by a non-gap source before the prefix search reaches Earthen Ally's {2}{W}{U}{B}{R}{G}, so the search spends its node budget as gorge's does and neither engine offers the activation
		{"waterbend_no_picks_falls_to_manual_window", false, false},        // CR 602.2b, 605.3a: a waterbend cost no printed source pays falls to the manual window, which taps Great Divide Guide's granted sources in battlefield order
		{"waterbend_no_picks_falls_to_manual_window_autopay", true, false}, // CR 602.2b, 605.3a: ... on the autopay surface too, where the planner's granted unit would otherwise pick a different source
		{"waterbend_credit_paid_source_live_pool", true, false},            // CR 701.67a: a paid mana ability earns the waterbend credit only while its fee is unpayable, so the prefix taps it once a grant floats the fee
		{"exiled_target_keeps_was_damaged", false, false},                  // CR 120.3a: Sold Out exiles its combat-damaged target, and the exile leaves WasDealtDamageThisTurn standing, so the "if that creature was dealt damage this turn" Clue is still made (gorge events.foldDamage)
		{"first_only_filter_funds_play", false, false},                     // CR 605.1a, 106.1: Barrels of Blasting Jelly's first-and-only {1} any-colour filter funds Waterbender Ascension's {1}{U} and Fire Nation Palace's {1}{R} activation, so both stay on the menu
		{"first_only_filter_funds_play_autopay", true, false},              // CR 605.1a, 106.1: ... and on the autopay surface, where the same first-and-only filter funds Fire Nation Palace's activation from the floating pool
		{"tap_cost_source_not_its_own_granted_mana", true, false},          // CR 602.2b, 605.3a: Agna Qel'a's {2}{U},{T} is not paid by the tap that is its cost, so Great Divide Guide's grant on it adds no third mana beside Guide and Badgermole Cub's extra {G}
		{"waterbend_taps_needed_counts_tap_cost_source", true, false},      // CR 602.2b, 701.67a: Yue, the Moon Spirit's {T} is part of the ability cost the waterbend ask is paying, so its granted any-colour ability is no window unit and the ask demands the pick the 4-mana pool leaves
		{"token_earthbend_return_queues_on_death", false, false},           // CR 603.7: a copy-token Forest earthbent by Earthbending Lesson and killed by Jet's damage still queues the return trigger (it does nothing: the token has ceased, CR 111.8), as gorge's id-keyed registration does
		{"replplus_commuting_order", true, false},                          // CR 616.1: two plus-N replacements are a sum no order changes
		{"waterbend_census_granted_source_tap_extra", false, false},        // CR 605.1b, 701.67a: Honest Work's Humble Merchant grant survives its lose-all abilities, so Badgermole Cub's tap extra makes the waterbend census incomplete and Foggy Swamp Vinebender's announcement offers Done
		{"waterbend_x_picked_source_not_mana", false, false},               // CR 701.67a, 601.2g-h: Katara's Waterbend<X> pick Plains (a 3/3 creature land) is tapped for the waterbend, so it is no mana source of the X announcement, which stops at 4 + 5 untapped lands
		{"waterbend_census_granted_tap_extra_capacity", false, false},      // CR 605.1b: the same merchant tapped later in the game floats its {G} tap extra, so the payment no longer stalls
		{"restricted_any_colour_mana_ability_sorts_first", true, false},    // CR 601.2g: White Lotus Hideout's Lesson-restricted any-colour mana sorts ahead of its {C} ability, so an announced {U} hybrid face is paid instead of stranding the cast
		{"restricted_any_colour_owed_tap", false, false},                   // CR 601.2g, 106.6: Jasmine Dragon Tea Shop has no unrestricted any-colour sibling, so the window must tap its restricted any-colour ability for the owed pip, not its useless {C} one, or the lowered hybrid cast strands
		{"unless_tap_extra_offers_pay", true, false},                       // CR 106.12a, 605.4a: Badgermole Cub's static TapsForMana trigger adds {G} to an animated Forest's tap, so the {2} unless cost is payable and gorge must offer Pay
		{"filter_tap_extra_funds_cast", false, false},                      // CR 605.1a, 106.12a: Badgermole Cub adds {G} when an earthbent Rumble Arena's {1},{T} filter taps it, so the filter's {B} plus the extra pays Merchant of Many Hats' {1}{B} and the cast stays on the menu
		{"resolved_this_turn_survives_zone_change", false, false},          // gorge events.ResolvedAbilityKey is (source, ability body) with no zone stamp: South Pole Voyager's tally still reads 2 after it is exiled and returned, so its second resolution draws
		{"grant_key_keeps_typed_mana", false, false},                       // CR 602.2b, 601.2g-h: Barrels of Blasting Jelly's {1} fee paid in W and its tagged W leave the pool counts equal but not gorge's typed unit (Player.ManaUnits), so the grant search's payer key must keep them apart and choose W
		{"filter_fee_paid_with_granted_mana", true, false},                 // CR 605.3a, 601.2g: Honest Work's Humble Merchant on Fire Sages pays Rumble Arena's {1} filter fee, so the one prefix search taps it before any printed land to pay Realm of Koh's token activation
		{"resolved_this_turn_across_zone_change", false, false},            // CR 707.10b: Airbender Ascension returns South Pole Voyager, whose resolution count keeps the turn's tally, so the second resolution overall does not draw Earth Kingdom Jailer
		{"filter_fee_barred_from_unusable_restricted_mana", false, false},  // CR 106.6, 605.3a: Professor Zei over Barrels of Blasting Jelly: the Tea Shop's Ally-only {U} cannot pay Barrels' {1}, so the second {U/R} pip cannot be paid {R} through it
		{"restricted_combination_tap_extra", false, false},                 // CR 106.6, 605.4a: Hermitic Herbalist's Lesson-only two-mana combination brings Badgermole Cub's {G} when the creature taps, so with an Island it pays Waterbending Lesson's {3}{U}
		{"granted_mana_funds_filter_fee", true, false},                     // CR 602.2b, 605.3a: Rumble Arena's {1},{T} filter fee is paid by the granted {C} the prefix search taps first in battlefield order, not by a land the filter-only script would fund it with
		{"crew_override_source_returned_is_new_object", false, false},      // CR 400.7: Fire Nation Warship destroyed in response to its own crew ability comes back as a new object; the crew effect finds nothing to animate
		{"hybrid_window_restricted_any_colour_admission", false, false},    // CR 106.6, 601.2g: the lowered hybrid window takes a restricted any-colour ability ahead of a useless fixed {C} only when the restriction admits the payment (an Ally-only source pays a hybrid face; a Lesson's generic takes the {C}), and the admitted mana floats restricted
		{"granted_gap_taps_ahead_of_filter_funding", true, false},          // CR 601.2g-h: on a board with a live granted mana gap (Honest Work's Humble Merchant) beside a filter (Rumble Arena), gorge's one prefix search taps the granted gap first and pays the filter's {1} fee from its mana, so no printed land is tapped for the fee
		{"filter_fee_from_granted_source", false, false},                   // CR 605.3a, 601.2g: a filter's {1} fee is funded by the play's granted {C} source, as gorge's one prefix search taps the census gaps in battlefield order
		{"filter_fee_funded_by_grant_before_non_gap", false, false},        // CR 601.2g, 605.3a: White Lotus Hideout's {1} filter fee is paid with Honest Work's granted {C} (a census gap) before a non-gap Plains, as gorge's gap-first prefix search orders it
		{"gated_mana_funds_activation", true, false},                       // CR 602.2b, 605.3a: White Lotus Hideout's {1},{T} any-colour ability funds Abandoned Air Temple's {3}{W},{T} fee beside the filter's own {1}, so dirac's scripted activation no longer outruns gorge's plan (c16ee5949)
		{"tla_sacrificed_aura_enchanted_lki_reused_slot", true, false},     // CR 608.2h, 400.7: Path to Redemption, sacrificed to pay, names Raven Eagle as it last existed; Lost Days put the Eagle into the library and its Clue took the slot, so nothing is exiled
		// CR 305.6, 605.1a: White Lotus Hideout under an Island grant lists gorge's seven node-limited alternatives ({C}, the granted {U}, the admitted any-colour ability's five), not its {1},{T} filter's or a WUBRG intrinsic run, so the alternative array no longer overflows
		{"plan_alts_lotus_hideout_island_grant", true, false},
		{"plan_alts_lotus_hideout_island_grant_late", true, false},
		{"plan_alts_lotus_hideout_island_grant_lesson", true, false},
		{"widened_restricted_source_granted_island_colour_first", true, false},  // CR 305.6, 601.2g: Jasmine Dragon Tea Shop made an Island taps for its intrinsic {U}, not its restricted any-colour {R}, toward Earthen Ally's {2}{W}{U}{B}{R}{G}: gorge's witness ranks an intrinsic ability before a printed one, so Avatar Kyoshi makes the six {R}
		{"attached_and_global_mana_grants_follow_the_static_walk", true, false}, // CR 613.7, 605.1a: Honest Work's {T}: Add {C} on a land of the other seat is listed after Great Divide Guide's any-colour grant, as gorge walks the statics by seat
		{"waterbend_self_taps_watcher_census", false, true},                     // CR 605.1b, 701.67a: Great Divide Guide's granted {T} ability on Compassionate Healer fires the Healer's own Taps watcher, which gorge's census cannot price, so the waterbend announcement offers Done
		{"free_waterbend_cast_stranded_reverses_untapped", true, false},         // CR 733.1: Benevolent River Spirit cast free off Solstice Revelations with Ty Lee waterbent leaves {4} no source pays; the cast reverses before the Mountain taps
		// CR 605.4a: Gran-Gran, deferred by its own tap trigger, still adds Badgermole Cub's extra {G} when tapped for Great Divide Guide's granted mana, so the relaxed proof cannot rule out Foggy Swamp Vinebender's Waterbend {5}
		{"deferred_source_keeps_tap_extra_mana", true, false},
		{"source_lki_power_keeps_unattached_equipment", true, false}, // CR 608.2h: Firebending Student killed by Razor Rings with its firebending trigger waiting adds its power as it last existed, Glider Staff's +1 included, though the Staff became unattached since
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
		{"window_any_colour_ability_owed_pip", false, false},    // CR 601.2g: an unpayable Tiered mode's window takes Starting Town's pay-life any-colour ability while a {W} pip is owed
		{"graveyard_leave_sees_batch_entrant", true, false},     // CR 603.10a: Fang returned by Random Encounter beside the cards it returns did not see them leave
		{"graveyard_entrant_watches_sibling", true, false},      // CR 603.10a: a card that entered in the sweep does not watch a sibling leave it
		{"equip_unattach_waits_for_legend_ask", true, false},    // CR 704.3, 704.5n: Magitek Scythe stays on Serah Farron, transformed beside another Crystallized Serah, until the legend answer
		// CR 510.2, 301.7b, 608.2h: Blitzball's "dealt combat damage by a legendary creature" reads the crewed Prima Vista as the creature it was
		{"blitzball_counts_crewed_legendary_vehicle", false, false},
		// CR 113.7a, 608.2h, 702.15b: Phoenix's chapter I, pending when the legend rule bins it, deals damage with the Phoenix's lifelink
		{"delayed_trigger_departed_source_lifelink", false, false},
		{"effect_lifelink_follows_source_controller", false, false},  // CR 702.15b: Phoenix's chapter damage, after Stiltzkin gave Phoenix away with the ability on the stack, gains life for Phoenix's controller
		{"effect_lifelink_departed_source_controller", false, false}, // CR 702.15b, 608.2h: Phoenix's chapter damage, after Stiltzkin donated Phoenix and Ice Magic returned it to hand, gains life for Phoenix's last-known controller
		// CR 601.2f-g: Valkyrie Aerial Unit's affinity counts both Treasures before one is sacrificed for mana, so {2}{U}{U} is payable
		{"count_reduction_fixed_before_treasure_mana", true, false},
		// CR 601.2f: Cloud, Planet's Champion's "Equip abilities you activate that target NICKNAME cost {2} less" prices Equip {3} at {1} for a target named only at CR 601.2c, so the offer and the payment plan agree
		{"equip_cost_target_reduction", true, false},
		// CR 601.2f: on the full surface too, Cloud, Planet's Champion's equip reduction prices Excalibur II's and Red Mage's Rapier's Equip {3} at {1}, so gorge offers both activations dirac offers
		{"equip_cost_target_reduction_full_surface", false, false},
		// CR 601.2f: Cloud, Planet's Champion's {2} less makes White Mage's Staff's equip onto it {1}, payable by tapping Balamb Garden
		{"equip_target_reduced_autopay_offer", true, false},
		// CR 601.2f, 602.2b: Cloud's {2} equip reduction prices Dragoon's Lance's Equip {4} for the target Cloud, so the autopay offer plans {2}
		{"equip_target_reduction_autopay_offer", true, false},
		// CR 601.2f: Cloud, Planet's Champion's ValidTarget$ discount makes Paladin's Arms' Equip {4} payable from three untapped sources when it targets Cloud
		{"target_priced_equip_from_untapped_lands", true, false},
		{"reversal_past_graveyard_exile_replacement", true, false},
		{"hidden_land_as_enters_choice_in_exile", true, false},
		{"instant_cast_lowering_life_run_trigger_below", false, false},
		// CR 601.2f, 602.2b: Cloud's {2} discount for an equip targeting it makes Ultima Weapon's Equip {7} payable from five lands
		{"equip_target_reduction_payable_from_lands", true, false},
		{"equip_target_reduction_payable_full_surface", false, false}, // CR 601.2f, 602.2b: the same equip offer on the full surface
		// CR 106.6, 601.2g: a Tiered cast's window asks Cargo Ship's abilities with its printed artifact-only {C} ahead of A Realm Reborn's grant, so the {C} floats unspent
		{"window_printed_restricted_before_grant", false, false},
		// CR 601.2f, 106.6: a Spree mode is priced beside the floating restricted mana the payment may spend, so the mode stays offered
		{"spree_mode_counts_floating_restricted_mana", true, false},
		// CR 601.2b, 601.2g, 733.1: Choco-Comet's largest-X lowered witness
		// would sacrifice a Treasure, and Al Bhed Salvagers' trigger would end
		// the game before the push, so gorge's potentialWitnessKeeping prices
		// the smaller X that keeps it and both engines ask X with the spell on
		// the stack
		{"choco_comet_x_witness_sacrifice_before_push", false, false},
		// CR 601.2g, 603.3b: Choco-Comet's largest-X witness would sacrifice the Treasure into Al Bhed Salvagers' lethal drain before the cast, so gorge plans X=5 from the lands and keeps the Treasure
		{"x_witness_lethal_sacrifice_trigger_keeps_treasure", false, false},
	})
}

// std-sos's, on the same terms.
func TestStdSosReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-sos", []stdRepro{
		{"printed_flashback_beside_granted", false, false},                  // CR 702.34a: a flashback instance granted beside the printed one leaves the printed cast priced
		{"granted_and_printed_flashback_ambiguous", false, false},           // CR 613.1c: the printed Flashback and the granted one (Flashback's pump) price as separate potential casts, so the ambiguity scan never drops both
		{"graveyard_ability_source_new_object", true, false},                // CR 400.7: a second Postmortem Professor return ability resolves after the card came back and died again, and finds a new object
		{"cascade_mana_value_evaluated_at_resolution", false, false},        // CR 202.3b: cascade reads the spell's mana value where the spell resolved, so the ward-counted Fractalize (X paid 4, now a card in the graveyard) prices 1 and does not hit Textbook Tabulator
		{"charm_gate_ignores_restricted_float", false, false},               // CR 106.6, 601.2c: the pre-target gate keeps a cast alive whose floating restricted mana its restriction admits, as gorge's pool does
		{"effetbctr_prepared_entry_order", false, false},                    // CR 616.1c, 616.1f: the effect's enters-with counters and the entrant's own prepared body compete over the entry, the order is asked and the own body first ends the rider's effect with the move it upgrades
		{"granted_flashback_when_printed_nonmana_unpayable", true, false},   // CR 702.34a: a printed flashback whose tap-three-creatures cost is unpayable leaves the granted mana-cost instance castable (Group Project)
		{"restricted_pool_cast_plans_sources_before_discard", false, false}, // CR 601.2b, 601.2g-h, 106.6: Seize the Spoils cast with Abstract Paintmage's restricted {U}{R} floating taps the planned Mountain at announcement, before the discard is chosen
		{"free_cast_unpayable_discard_returns_to_zone_end", true, false},    // CR 601.2b, 733.1: Improvisation Capstone's free Seize the Spoils with no card to discard is pushed and reversed, and returns behind the other exiled cards
		{"permission_cast_from_graveyard_not_own_target", false, false},     // CR 601.2a, 601.2c: Flashback cast from the graveyard through Tablet of Discovery's permission is on the stack when its target is chosen, so with no other instant or sorcery there it is not offered
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
		{"free_cast_either_or_cost_unaffordable", false, false},      // CR 118.9d: a cast without paying its mana cost from Maralen's exile still owes Mudbutton Cursetosser's behold-or-{2}; unpayable, it is not offered
		{"evoke_hybrid_paid_from_floating_pool", false, false},       // CR 601.2h: Deceit evoked with a floating {U} and {B} pays its {U/B}{U/B} from the pool and taps no untapped Island
		{"planned_mana_sacrifice_token_waits_for_sba", true, false},  // CR 704.5d, 111.7: a Treasure sacrificed by the payment plan for Appeal to Eirdu stays in the graveyard until the next state-based check, as gorge leaves it
		{"mayplay_selfflash_you_is_wouldbe_controller", true, false}, // CR 109.5: a may-play cast of a card exiled by Maralen, Fae Ascendant reads the card's own CastWithFlash static's "you" as the would-be controller
		// CR 702.51b: Sanar's "you may cast the exiled cards this turn"
		// permission casts Lofty Dreams ({3}{U}{U} Convoke) from exile, and
		// the convoke announcement filters by the cost beginCast charges
		// (MayPlayCastPaymentCost, payment_plan.go): the oracle caches a
		// priority-time cost for the may-play cast as it does for a plain
		// hand cast, and the kernel's convoke menu filters the same way, so
		// gorge's Submit never refuses a pick the menu no longer offers.
		{"mayplay_convoke_cast_cost", false, false},
		{"restricted_window_float_keeps_chosen_type", false, false},       // CR 106.6, 601.2h: the evoked pip window's owed tap of an any-colour ChosenType-restricted source floats the batch with the source's chosen creature type, so a spell of that type can spend it
		{"convoke_window_restricted_any_colour_ahead_of_c", false, false}, // CR 106.6, 601.2g: a convoke window with an unsettled generic remainder lists Eclipsed Realms' admitted any-colour ability ahead of its {C}, and a window with no mana payment answers that ability's lowest colour
		{"convoke_pays_with_floated_restricted_mana", false, false},       // CR 106.6, 601.2h: the convoke cast spends the restricted mana its window floated, as gorge's payment does
		{"mayplay_convoke_over_selection", false, false},                  // CR 702.51a: Temporal Cleansing cast from Sanar's exile convokes four blue creatures for {3}{U}; only one may pay {U}, the rest pay {1}
		{"mayplay_convoke_announcement_filtered_by_cost", false, false},   // CR 601.2f: Unexpected Assistance cast from Sanar's exile with Convoke offers no generic tap once the {3} is paid, as gorge refuses it
		{"subcounter_wildcard_flying_counter_kind", false, false},         // CR 122.1b: Reaping Willow's remove-any-two-counters cost names a Flying counter by its own kind, not as a +1/+1 counter

		{"behold_part_chosen_unpayable_by_grant_caster", true, false},              // CR 601.2h, 733.1: Silvergill Mentor cast from Maralen's exile by its non-owner, who controls no Merfolk; gorge offers the behold part last and choosing it reverses the cast
		{"free_cast_owes_either_or_additional_cost", true, false},                  // CR 601.2b, 118.9d: Kinsbaile Aspirant cast free from Maralen's exile still owes behold or {2}; with neither payable it is not offered
		{"behold_or_mana_fallback_window_colour", true, false},                     // CR 601.2g: a plan that fell back when the chosen mana part changed the cost has no ManaPayment, so the window answers the source's lowest colour, not the owed pip
		{"behold_or_mana_window_tap_another_source", false, false},                 // CR 601.2g: the fallback window lists a tap-another source (Springleaf Drum) at its battlefield place, so it taps that source and its creature before a later land
		{"either_or_mana_part_spends_restricted_source", false, false},             // CR 106.6, 601.2f: Kinsbaile Aspirant's {2} part with its {W} is affordable through Eclipsed Realms' chosen-type mana
		{"aura_etb_lki_bearer_flickered", true, false},                             // CR 400.7, 608.2h: Aquitect's Defenses' ETB pump, its Aura gone after Personify flickered Rimekin Recluse, gives the returned Rimekin nothing
		{"repeat_loss_after_static_grant", false, false},                           // CR 613.7: Abigale's second loss of abilities, after Chronicle of Victory entered, is newer than its first-strike-and-trample grant and takes it away from Prideful Feastling
		{"dig_entry_chosen_type_sees_batch", true, false},                          // CR 603.6a: Rimefire Torque, dug onto the battlefield last with its as-enters type asked, sees changeling Flock Impostor entering in the same batch
		{"either_or_unpayable_part_tap_another_source_keeps_target", false, false}, // CR 601.2g: Bogslither's Embrace's unaffordable {3} part goes on to its target menu while an untapped Springleaf Drum, with no creature to tap, leaves gorge's window census incomplete
		{"wide_restricted_source_funds_hand_ability", true, false},                 // CR 106.6, 601.2g: Flamebraider's two any-colour mana for an Elemental pays Kulrath Zealot's basic landcycling, a hand activation the census has no unit for
		{"behold_exile_cast_pays_restricted_wide_mana", false, false},              // CR 106.6, 601.2g: Champion of the Path cast from Sanar's exile is affordable through Flamebraider's restricted two mana, which gorge's prefix search taps ahead of the pricing
		{"behold_exile_cast_tap_another_source_not_census_gap", false, false},      // CR 601.2g: a plain hand Champions of the Shoal is not one of gorge's potential plays (shape:additional_cost is refused before the mana is priced), so no prefix search taps Springleaf Drum for it and dirac must not offer the cast
		// CR 610.3, 616.1: Temporal Cleansing's "second from top or bottom"
		// answer moves the owner's Liminal Hold into the library inside that
		// answer, so its until-leaves exile ends there and Rimefire Torque's
		// as-enters creature-type choice is asked with the card still in exile
		{"rimefire_torque_until_leaves_return_choose_type", true, false},
		{"mayplay_optional_cost_cast_offered_alike", false, false},               // CR 601.2g, 605.3a: Requiting Hex (optional blight) cast from Burning Curiosity's exile is a composed may-play cast, so gorge's prefix search taps Springleaf Drum for its {B} and dirac offers it too
		{"reversed_cast_keeps_same_taps", true, false},                           // CR 733.1: Sun-Dappled Celebrant's convoke picks strand its cost over Eclipsed Realms' Treefolk mana; the cast reverses before any land taps
		{"reversed_cast_keeps_mana_ability_trigger", false, false},               // CR 733.1: Springleaf Drum tapped Wanderwine Distracter for Appeal to Eirdu; the cast is reversed, the mana ability is not, so the "becomes tapped" trigger stays queued
		{"reversed_cast_trigger_push_is_progress", true, false},                  // CR 733.2: Silvergill Peddler's kept tap trigger is pushed after Disruptor of Currents' reversal, so a second no-progress reversal still leaves the cast offered
		{"until_leaves_return_asks_choice_after_top_bottom_answer", true, false}, // CR 614.12, 610.3: an as-enters choice of a card returning when its exiler is put second-from-top is asked with the card in exile
		{"behold_or_mana_mana_part_taps_planner_order", true, false},             // CR 601.2b, 601.2g-h: Mudbutton Cursetosser's chosen {2} part is paid by the payment plan's ObjID-ordered sources, not the manual window's battlefield order
		{"free_cast_altadd_mana_part_lowers_witness", false, false},              // CR 118.9d, 601.2b: Silvergill Mentor cast free from Maralen's exile with no Merfolk to behold owes its {2}: gorge's witness taps a Forest beside the floating {U} before the part is asked
		{"free_cast_altadd_mana_part_lowers_witness_autopay", true, false},       // ... and on the autopay surface, with an empty pool, it taps two lands
		{"behold_or_mana_generic_taps_planner_order", true, false},               // CR 601.2b, 601.2g-h: Kinsbaile Aspirant's {2} part taps the plan's sources, not the window's battlefield order
		{"behold_or_mana_full_surface_offer", false, false},                      // CR 601.2b/h: Silvergill Mentor's behold part is payable while its {2} part is not, so the free part's plan must not wait on the offer walk (which can only price the mana part); gorge now lists the cast on the full surface too
		{"behold_or_mana_relaxed_proof_tap_another_source", false, false},        // CR 601.2b, 601.2g: Mudbutton Cursetosser's {2} part, payable only through Springleaf Drum tapping a creature, is not proved unpayable by the relaxed census, so gorge searches the prefix and offers the cast after an aborted one
		{"behold_or_mana_unpayable_behold_chosen_window_colour", false, false},   // CR 601.2g, 733.1: Silvergill Mentor's unpayable behold chosen beside a plan armed for the {2} part falls back to a window with no ManaPayment, so Eclipsed Realms answers its lowest colour, not the owed pip, and the seat taps the Island that pays the pip, not the first Mountain
		{"behold_or_mana_chosen_mana_part_window_colour", false, false},          // CR 601.2g: Silvergill Mentor's {2} chosen beside a payable behold falls back to a window with no ManaPayment, so Springleaf Drum answers its lowest colour, not the owed pip, and the last pip is paid by the Island, not the first Plains
		{"behold_or_mana_abort_window_tap_another_pick", true, false},            // CR 601.2g, 733.1: Mudbutton Cursetosser's chosen {2} part falls back to the manual window, which poses Springleaf Drum's tap-another pick before it reverses the unpayable cast
		{"delayed_dies_trigger_last_known_power_m1m1", false, false},             // CR 608.2h: End-Blaze Epiphany's delayed dies trigger exiles the graveyard-bound creature's last-known 2 power (a -1/-1 counter), not its printed 3
		{"delayed_dies_trigger_last_known_power_aura", false, false},             // CR 608.2h: ... and its last-known 3 power with a +1/+0 Aura attached, not the printed 2
		{"delayed_dies_trigger_power_read_at_death", true, false},                // CR 603.10a, 608.2h: Squawkroaster's Vivid power is read as it dies, after Vibrance was sacrificed, not as End-Blaze Epiphany registered the trigger
		{"tapx_pick_strands_mana_aborts_activation", false, false},               // CR 601.2h, 602.2b, 733.1: Wanderbrine Trapper's tap-another pick takes Great Forest Druid, the only mana source, so its {1} cannot be paid and the activation is reversed at the target ask
		{"tapx_pick_strands_mana_aborts_activation_second", false, false},        // ... and the same reversal in a second game (pair 372)
	})
}

// std-fra's, on the same terms.
func TestStdFraReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-fra", []stdRepro{
		{"sweep_returns_aura_after_earlier_entrants", true, false},     // CR 303.4f: Return to the Light Realms asks for Infinite Coursework's host with the cards before it entered and the ones after still in the graveyard
		{"sacrificed_mana_source_moves_its_aura", false, false},        // CR 704.3, 704.5m: the check after a mana source's sacrifice while paying a cast takes its Aura before the cast's next choice
		{"flashback_dynamic_source_graveyard_amount", true, false},     // CR 117.1d, 605.3a: Bestial Incursion's flashback is paid by Loot, the Nexus tapped at its graveyard-time amount (Tarmogoyf one point larger), a lowered potential play and not a planned cast
		{"entry_counters_skip_entrants_own_replacement", true, false},  // CR 614.12: Generous Revival returns Yoshimaru, Beloved Companion with a +1/+1 counter; its own "that many plus one" does not apply, since the replacement affects a general subset, not only itself
		{"search_enters_together_canopy_two_forests", true, false},     // CR 603.2, 603.4: Hexhaven Invigorator's search puts two Forests onto the battlefield together, making six, so each sees five others and Roiling Canopy triggers twice
		{"search_enters_together_canopy_order_ask", true, false},       // CR 603.2, 603.4: the same search puts Forests #8 and #9 on beside three other lands, and both Canopy triggers are ordered
		{"behold_or_mana_restricted_mana_shortens_plan", false, false}, // CR 106.6, 601.2b, 601.2g: Countersculpt's chosen {1} part is planned from the pool first, Chandra's floating noncreature-only {U} paying a pip, so the plan taps for the two sources it has left
	})
}

// std-lci's, on the same terms.
func TestStdLciReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-lci", []stdRepro{
		{"tapxtype_plain_plan_leaves_candidates", false, false},       // CR 601.2g-h: nine lands cast Guardian of the Great Door: the plan paying {W}{W} from the Plains and Restless Anchorage leaves seven to tap, whatever the reservation order
		{"witness_waiting_trigger_other_answer", false, false},        // CR 601.2g, 603.3b: the Treasure sacrificed for Guardian of the Great Door's {W} sets off p0's Zoetic Glyph, whose discover waits for the cast, so its answer does not make the witness unreachable
		{"granted_sac_host_cost_poses_no_choice", false, false},       // CR 602.2b, 701.21a: Deconstruction Hammer's granted "Sacrifice Deconstruction Hammer" cost has one possible payment, so it is paid without a choice after the targets
		{"attack_trigger_pump_skips_returned_attacker", false, false}, // CR 400.7: Grasping Shadows' attack trigger does not grant deathtouch and lifelink to Aclazotz once it died and returned as Temple of the Dead
		{"lose_all_setter_overwrites_self_chosen_type", true, false},  // CR 613.6, 613.7: Eaten by Piranhas on Roaming Throne makes it only a Skeleton; the Throne's older chosen-type grant outlives the ability loss but is overwritten in timestamp order
		{"payment_trigger_counts_its_resolution", false, false},       // CR 603.2, 605.3b: Vito's trigger from a Treasure sacrificed paying for Guardian of the Great Door resolves before the cast and counts toward "the second time this ability has resolved this turn"
		{"counter_delayed_trigger_strips_creator", false, false},      // CR 603.7e: Tishana's Tidebinder counters the end-step sacrifice Saheeli, the Sun's Brilliance's copy registered; its source is Saheeli, who loses all abilities
		{"mana_float_unspent_observed_source", false, false},          // CR 601.2h: Barracks of the Thousand's mana floated by the X witness and left unspent (Jadelight Spelunker at X=0) is not mana that paid the cast
		{"aura_override_stops_when_host_leaves_filter", true, false},  // CR 704.5m, 613.1d: Zoetic Glyph's "enchanted artifact" override stops adding the Creature type once Treasure Map transforms into Treasure Cove, so the land is not a 0-toughness creature
		{"aura_type_add_filter_drops_after_transform", true, false},   // CR 613.1d, 704.5m: Zoetic Glyph's "enchanted artifact is a 5/4 Golem creature" stops applying when Matzalantli transforms into a land, so the land stays and only the Glyph falls off
		{"aura_type_add_drops_after_device_transform", true, false},   // CR 613.1d, 704.5m: Zoetic Glyph's "enchanted artifact" override stops applying when Dowsing Device transforms into Geode Grotto, so the land stays on the battlefield
		{"granted_tap_pay_life_mana_prefix", true, false},             // CR 119.4, 605.1a: Cavernous Maw's "{T}, Pay 1 life: Add one mana of any color" granted by Forgotten Monument is the prefix paying Restless Vents' {B}, and its life is paid
		{"granted_prefix_one_part_additional_cost", false, false},     // CR 601.2g-h, 605.1a: Dusk Rose Reliquary's {W} beside Hidden Necropolis (printed {B} only) and Forgotten Monument's granted any-colour Cave ability: the one-part additional-cost shape is lowered, so the grant search prices its printed {W} and floats the granted mana as the prefix, where the witness found no plan and panicked
	})
}

// std-spm's, on the same terms.
func TestStdSpmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-spm", []stdRepro{
		{"tapcost_source_not_paid_with_itself", false, false},               // CR 602.2b: Guy in the Chair's {T} activation is payable through Supportive Parents tapping two other creatures; gorge's prefix search no longer stops at the line whose forced tap takes Guy
		{"tap_two_forced_answer_taps_the_plays_own_source", false, false},   // CR 602.2b: the Parents' second activation has exactly two candidates, so nothing is asked and the forced tap takes Guy; the search drops that line and reaches the pair that leaves Spider Manifestation untapped within its budget
		{"web_watch_exile_x_restricted_mana", true, false},                  // CR 106.6, 601.2b: Morlun cast from exile under Interdimensional Web Watch's permission announces an X its two restricted mana pay for, as gorge's xAsk does
		{"face_down_exile_played_face_up_not_creature_entry", true, false},  // CR 708.2: a land exiled face down by Black Cat and played face up enters as a land, not the 2/2 face-down creature, so Spider-UK's end-step count stays at 1
		{"convoked_tap_another_source_keeps_census_complete", false, false}, // CR 601.2g, 733.1: Supportive Parents convoked for Web of Life and Destiny is no window source, so the stranded cast reverses before Guy in the Chair or a Forest taps
	})
}

// std-dft's, on the same terms.
func TestStdDftReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-dft", []stdRepro{
		{"clone_entry_order_beside_counter", false, false},                    // CR 616.1: Waxen Shapethief's as-enters copy and Thunderous Velocipede's counter replacement ask which applies first, with the card still on the stack
		{"clone_entry_order_three_candidates", false, false},                  // CR 616.1, 616.1f: ... beside two Velocipedes the ask has three options, and a counter source named first moves the card in and asks the two left with it on the battlefield
		{"copy_entry_trigger_after_order_ask", false, false},                  // CR 614.12/603.6a: Waxen Shapethief's as-enters copy and Thunderous Velocipede's counter replacement ask which applies first; the copy's Hulldrifter enters trigger queues after the bodies
		{"tap_power_value_lost_with_abilities", true, false},                  // CR 613.1f: Interface Ace under Flood the Engine has lost its crew-with-toughness static, so its 0 power does not crew Rangers' Refueler
		{"lookback_player_turn_uses_lki_controller", false, false},            // CR 603.10a: a stolen Ketramose exiled from the battlefield on its thief's turn triggers for the thief (PlayerTurn$ reads the last-known controller)
		{"own_counter_replacement_does_not_apply_to_own_entry", false, false}, // CR 614.12: Caradora's "a creature or Vehicle you control" plus-one does not apply to the +1/+1 counter Thunderous Velocipede grants it as it enters; a general subset excludes the entrant itself
		{"returned_aura_host_before_paid_draw", false, false},                 // CR 303.4f, 610.3: sacrificing Perilous Snare pays Chandra's "if you do, draw": its exiled Aura's host answer comes before the draw, as gorge asks it mid-payment
		{"lost_abilities_tappower_bonus", false, false},                       // CR 613.1f: a Pilot token under Flood the Engine loses the TapPowerValue static, so it contributes its printed power to a Saddle cost
		// CR 614.12, 610.3: a delayed sacrifice's returning card is asked its
		// as-enters choice with the card still in exile, before the delayed
		// trigger leaves the stack
		{"until_leaves_return_asenters_in_exile", false, false},
		{"actonly_pool_pays_surveyor_grave_grant", true, false}, // CR 106.6, 602.1: Boommobile's floating activated-only mana pays Leonin Surveyor's max-speed graveyard activation
		{"actonly_pool_pays_grave_return", false, false},        // CR 106.6, 602.1: Boommobile's floating activated-only mana pays Deathless Pilot's graveyard return
		{"actonly_pool_pays_battlefield_grant", true, false},    // CR 106.6, 602.1: Boommobile's floating activated-only mana pays Hour of Victory's max-speed battlefield grant
		{"outside_grants_distinct_codes", true, false},          // CR 205.3m, 613.1d: two Lifecraft Engines under one seat grant Kangaroo and Badger (different outside codes) to one Vehicle creature; a ChosenType read on it compares them instead of stalling as outsidetype
	})
}

// std-tmt's: each of these once ended in a kernel stall, which replayRepro alone does not see.
func TestStdTmtReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tmt")
	for _, c := range []stdRepro{
		{"put_counters_from_unheard_kind_order", false, false},                 // CR 603.3b: Donatello, Mutant Mechanic copies a dead artifact's +1/+1 and stun counters while a +1/+1 trigger listens: the stun is heard by none, so gorge's insertion order is unobservable
		{"second_ability_counter_cost_taps_source_for_generic", true, false},   // CR 605.3b, 601.2g: Weather Maker beside a plain {T} first ability: its "{T}, remove two charge counters: Add {C}{C}" pays Frog Butler's {2} reach activation when every land is tapped, so gorge offers it
		{"second_ability_counter_gap_tries_plain_ability_first", false, false}, // CR 605.3b, 601.2g: two Weather Makers pay Metalhead: gorge's prefix search answers the first one's ability ask with its plain ability (the colour ask behind it), then the second one's with its counters, so only the second loses them
		{"entry_body_rider_own_counter_replacement_excluded", false, false},    // CR 614.12: Michelangelo, Weirdness to 11 cast through Mikey & Don enters with one +1/+1 counter: its own "a creature you control" plus-one line is a general subset, so it does not modify how it enters, even through the Effect body's SubAbility$ rider
		{"attach_triggered_card_moved_skips_ninja", true, false},               // CR 400.7: Quintessential Katana's "attach to the entering Ninja" trigger waits on the stack while that Ninja returns to hand and re-enters, so its TriggeredCardLKICopy is a new object and the attach finds nothing
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
		{"nonpositive_life_pays_painless_plan", false, false},           // CR 119.4: a payer at life 0 or less (a Grimoire holds the game) still pays a painless plan; the planner's lethal guard needs life lost > 0
		{"manifest_dread_other_card_parks_exile_order", false, false},   // CR 701.62a, 616.1: a manifest dread window's other card under two Leylines of the Void parks for its order ask with the card still in the library, and the window's marker has already fired
		{"destroy_all_parks_whole_batch", true, false},                  // CR 616.1: two Leylines of the Void compete for one victim of Split Up's destroy-all, so the whole batch waits for the order ask; the other victims do not move first
		{"room_shown_face_plain_cast", false, false},                    // CR 709.3: a Room back on the battlefield as Slimy Aquarium keeps that door, so its plain hand cast is the shown half's {3}{U}, which the planner must price
		{"sacrificed_panharmonicon_host_departs", false, false},         // CR 603.2d: a sacrificed Panharmonicon host is gone when Popular Egotist's "whenever you sacrifice" trigger is checked, so it echoes nothing
		{"returning_aura_during_paid_cost", false, false},               // CR 303.4f: a sacrificed "until this leaves" host returns its exiled Aura while the sacrifice cost's payment is still resolving: the Aura waits in exile and its owner chooses what it enchants
		{"lingering_aura_sweeps_before_order_ask", false, false},        // CR 704.3, 616.1: a dying creature's order ask is posed in its check: the Aura whose host left in that check is swept, and the token it moved ceases, at the check after the answer
		{"restricted_float_affords_unadmitted_payment", false, false},   // a floating restricted batch counts as affordable for a spell its restriction does not admit, so the kernel taps one source fewer and floats one pip less
		{"restricted_float_paid_unadmitted_spell", false, false},        // the same admission check at the double-pip shape: a spell is paid straight out of the floating restricted pair both pips of which the payment may not touch
		{"room_other_face_trigger_reads_cast_face_svars", false, false}, // an unlocked Room's other-face trigger resolves against the shown face's SVar table, so a Branch arm named only on its own face does nothing
		{"return_ex_flags_hide_instant_stay", false, false},             // CR 304.4: OpReturnEx's C carries the move flags beside the zone, so the instant-stays guard read a non-battlefield zone and put a manifested instant back onto the battlefield
		{"granted_prefix_pays_painful_printed_colour", false, false},    // CR 119.4, 601.2g: an animated Haunted Screen under Enduring Vitality's grant answers the prefix search's ability ask with the {G} its pay-1-life ability makes, the colour Omnivorous Flytrap needs, so the life is paid
		{"mid_resolution_mana_sac_token_waits_sba", true, false},        // CR 704.3, 704.5d: a token sacrificed to pay a mid-resolution UnlessPaid fee waits in the graveyard for the state-based action, as a cast payment's does not
		{"multi_discard_pick_order_past_eight", false, false},           // CR 404.3: a ten-card discard keeps the owner's chosen arrangement past the old eight-pick buffer
		{"sacrifice_two_exile_sources_order_ask", false, false},         // CR 616.1: a sacrificed creature two Leylines of the Void would exile parks for its order ask, and the batch's later moves wait behind it
		{"returned_sorcery_stays_in_graveyard", false, false},           // CR 304.4: a manifested sorcery that died is a sorcery card, so Meathook Massacre II's paid return (with a finality counter) leaves it in the graveyard
		{"tapcost_spell_pays_life_mana_beside_part", false, false},      // CR 601.2g-h, 119.4: Fear of Exposure beside Haunted Screen is castable: the Screen's life-paid {G} pays the mana while two lands are tapped for the cost
		{"granted_prefix_pays_painful_needed_colour", false, false},     // CR 119.4: the granted-mana prefix taps an animated Haunted Screen for its painful {R} the play needs (gorge's ask lists it before the grant) and pays 1 life
		{"self_leaves_trigger_sees_remembered_clear", false, false},     // CR 400.7/603.2d: a leaves trigger whose source departed does not serve the fire-time remembered snapshot, so the first instance's Cleanup clear closes the second instance's window
		{"unless_tax_hides_unadmitted_restricted_mana", false, false},   // CR 106.6: Smoky Lounge's Room-only {R}{R} cannot pay Don't Make a Sound's {2} unless tax, so its controller is offered only the decline
		{"sac_after_aura_control_ends", false, false},                   // CR 613.1b, 701.21: Vile Mutilator's second sacrifice counts creatures after the first (an Aura) left, so the creature it stole is back with its owner and the lone manifest is forced
		{"painless_plan_at_nonpositive_life", false, false},             // CR 119.4, 601.2g: a player at life <= 0 (a lose-the-game replacement) still pays a {1} spell from painless lands; the pain guard cuts only a plan that pays life
		{"animate_rider_before_entry_triggers", true, false},            // CR 603.6a: an Enduring creature returned as an enchantment has its type-changing rider applied before an "another creature enters" watcher matches it, so no extra pump trigger
		{"tapxtype_cost_reserves_tap_candidates", false, false},         // CR 601.2g-h: Fear of Exposure's tap-two is priced from its composed cost, so the granted-mana prefix taps Enduring Vitality's creatures in battlefield order and leaves the rest for the pick
		{"tapxtype_restricted_source_not_free", false, false},           // CR 601.2g-h: Creeping Peeper's enchantment-only {U} paying Fear of Exposure is not also one of the two permanents it taps, so three mana sources and Patchwork Beastie do not cast it
		{"grant_prefix_paid_life_trigger", false, false},                // CR 119.4, 603.3, 307.1: Haunted Screen's 1 life in Overlord of the Hauntwoods' granted-mana prefix stacks Marina Vendrell's Grimoire's trigger, so the prefix taps on and passes for it to resolve before the cast
		{"tapxtype_lands_need_green_pip", false, false},                 // CR 601.2g-h: Fear of Exposure's {G} has no source among p1's untapped permanents, so tapping two of them for the cost does not make it castable
		{"tapxtype_floating_mana_beside_tapped_lands", false, false},    // CR 601.2g-h: floating U, B and G pay Fear of Exposure's mana cost, so two untapped lands remain to tap for its additional cost
		{"rider_mana_extends_x_announcement", false, false},             // CR 601.2b, 601.2g: Twitching Doll's counter-rider mana is a source the window taps, so Valgavoth's Onslaught's {X}{X} is offered for 2 beside four floating mana (mana_rider.go)
		{"unless_pay_reaches_deferred_rider_source", true, false},       // CR 118.3, 605.3b: Anthropede's pay-{2} election is offered when the second mana comes only from Twitching Doll's counter-rider mana, which the planner defers but the unless window taps
		{"unless_pay_reaches_deferred_rider_source_full", false, false}, // the same shape recorded on the full surface
		{"rider_mana_extends_x_announcement_second", false, false},      // CR 601.2b, 601.2g: the same counter-rider unit makes X = 1 payable from the floating {G}, the Doll and Abandoned Campground (mana_rider.go)
		{"rider_mana_pays_x_announcement", false, false},                // CR 601.2g: the same game answering X = 2: the window, with no auto-fill plan for {4}{G}, taps Strangled Cemetery then the Doll (its nest counter) and the spell stays on the stack
		{"rider_mana_pays_x_announcement_second", false, false},         // CR 601.2g: the second game answering X = 1: the Doll, first in battlefield order once the planner census falls short, then Abandoned Campground pay it
		{"x_witness_skips_deferred_rider_source", false, false},         // CR 601.2b, 605.3b: the {X} cast witness prices the planner's census, which never lists Twitching Doll's counter-rider mana, so The Rollercrusher Ride's lowered cast taps three lands for X = 0 and leaves the Doll untapped
		{"convoke_granted_impending_cast_plan_full_m1", false, false},   // CR 702.176a, 702.51a: Dazzling Theater grants Overlord of the Mistmoors convoke, but its {2}{W}{W} impending cost is payable without the optional credit, so the mode cast keeps its no-credit witness and is offered
		{"convoke_granted_impending_cast_plan_autopay_m0", true, false}, // the same at the other seat: two Overlords' impended casts priced without the granted convoke
		{"convoke_granted_impending_cast_plan_autopay_m1", true, false}, // and the post-attack window with the Overlord already on the battlefield: the hand's impending cast is offered beside the empty pool
		{"convoke_granted_impending_cast_menu", true, false},            // CR 702.176a, 702.51a: Overlord of the Mistmoors cast for its impending cost under Dazzling Theater's granted convoke poses the convoke announcement priced from the impending cost, as gorge's convoke ask does
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
		{"convoke_colour_no_owed", "std-fra"},                             // a convoke cast's window owes no colour: Dedicated Commons makes its lowest
		{"convoke_colour_no_picks", "std-fra"},                            // ... with no creature announced, and the cast reversed
		{"improvise_treasure_census", "std-msh"},                          // Arc Reactor short with its Treasures counted: reversed, no Treasure spent
		{"excess_of_a_replaced_amount", "std-fra"},                        // CR 120.10: Violent Echoes' excess reads the damage as replaced
		{"free_cast_two_statics", "std-fra"},                              // two Omnipresences list one free cast
		{"ward_discard_is_a_discard", "std-fra"},                          // CR 701.9a: Tinybones sees the discard ward was paid with
		{"cast_trigger_second_target", "std-fra"},                         // Danitha, on the second object of a multi-object slot
		{"restricted_any_colour_window", "std-fra"},                       // Gideon's Memorial in a window makes the announced pip's colour
		{"ward_counters_the_targeting_ability", "std-fra"},                // Koth's two triggers: ward counters the one that targeted
		{"eot_pump_past_int8", "std-fra"},                                 // Aerid Konstrari's pump past int8 stalls loudly
		{"attached_equipment_becomes_a_copy", "std-msh"},                  // CR 704.5p: Shuri makes an attached Equipment a copy of a creature
		{"unless_evaluated_amount", "std-msh"},                            // Doc Samson's mana can pay an unless cost
		{"room_alt_granted_prefix_prices_the_other_door", "std-dsk"},      // Unholy Annex showing door 0 cast as Ritual Chamber: the granted-mana prefix is priced at {3}{B}{B}, not the shown door's {2}{B}
		{"lotus_pair_pays_two_colours", "std-fra"},                        // two Lotus tokens, three mana of one colour each, pay {R}{G}: the plan search keeps the second unit for the colour the first leaves owed
		{"granted_mana_creature_is_no_ability_planner_unit", "std-tla"},   // Honest Work's Humble Merchant (Katara) is no unit of an ability's planner, so Flexible Waterbender's waterbend {3} falls to the window, which taps her before the Islands
		{"granted_types_any_land_census_position", "std-mkm"},             // Leyline of the Guildpact: Public Thoroughfare (Produced$ Any) is listed with the fixed sources, so the plan taps Plains, not the Thoroughfare
		{"delayed_return_skips_object_blinked_since", "std-fin"},          // CR 400.7: Random Encounter's end-step return finds Y'shtola Rhul already exiled and returned by her own trigger, a new object
		{"returned_creature_was_not_dealt_damage", "std-fin"},             // CR 400.7: Qutrub Forayer raised after combat damage killed it is dealt no damage this turn, so its destroy mode has no target
		{"echo_copied_equip_offers_its_own_source", "std-msh"},            // CR 301.5c, 707.10c: Echo's copy of Vibranium Energy Daggers' Equip never offers the animated Daggers itself
		{"departed_damage_source_keeps_last_known_controller", "std-msh"}, // CR 113.7a: Human Torch (owned by the opponent) dies, then deals damage; The Thing, Ben Grimm still sees a Hero its controller controlled
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
		{"mayplay_hybrid_witness_prices_the_rider", "std-mkm"},           // CR 601.2b: Outrageous Robbery's any-type cast of the exiled Trostani, Three Whispers ({G}{G/W}{W}, no pip ask): the witness prices the rider's cost, one generic per hybrid pip
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
		{"toadstool_ward_s222", true, false}, // CR 603.3b, 701.6a: three Hellkite activations at a Ward {2} Admirer with a Ward {1} Role; the ward trigger of an ability a sibling ward countered leaves without asking
		{"toadstool_ward_s170", true, false},
		{"sleep_cursed_ward_s180", true, false},
		{"toadstool_ward_s223", true, false},
		{"role_sba_leaves_with_same_pass_death", true, false},                 // CR 704.3, 603.10a: Cut In's new Role sends the old Wicked Role away in the pass Wicked Visitor dies, so Visitor sees the Role leave
		{"filter_fed_by_granted_mana", false, false},                          // CR 605.3b, 601.2g: Prophetic Prism's {1} is paid by the Food Night of the Sweets' Revenge lets tap for {G}, so Embereth Veteran is castable
		{"role_token_counts_as_attached_aura_cost", false, false},             // CR 601.2g, 602.2b: Faunsbane Troll's "Sacrifice an Aura attached to it" sees the Role token and the printed plan keeps it, so the granted Food mana funds the {1}
		{"act_paid_filter_second_colour", true, false},                        // CR 601.2g, 602.2b: Restless Fortress's {1}{W}{B} animation is offered when a free filter (Crystal Grotto) makes one pip and Rootrider Faun's paid filter the other
		{"woe_activation_sac_exile_return_asks_in_exile", false, false},       // CR 614.12, 610.3: Werefox Bodyguard sacrificed for its own activation returns Talion, the Kindly Lord, whose number is asked in exile before it enters
		{"legend_rule_and_saga_sacrifice_one_batch", true, false},             // CR 704.3, 603.10a: the Neva the legend rule removes still sees The Princess Takes Flight sacrificed in the same state-based check
		{"fell_horseman_returned_card_not_bottomed", false, false},            // CR 400.7: Edgewall Inn returns Fell Horseman to hand before its dies trigger resolves, so the trigger cannot put the new object on the bottom of the library
		{"hidden_pick_aura_entry_attach_choice", false, false},                // CR 303.4f/g: Fetch Quest's hidden pick takes Bestial Bloodline, which waits in the graveyard for its enchant choice while the spell is still on the stack
		{"restricted_each_leftover_not_for_adventure_creature", false, false}, // CR 106.6, 715.4: Troyan's {U} left over from Grove's Bounty floats restricted, so Elusive Otter ({U}, no {X}) is not cast from adventure exile with it
	})
}

package oracle

import "testing"

// std-msh's repros, on the same terms as repro_fixes_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"target_attacking_lki_after_lethal_damage", false, false},              // CR 608.2h: the enrage rider still sees the attacker's last battlefield status after lethal damage moved it to the graveyard, so the extra combat phase is granted
		{"sneak_paid_through_filter_land", true, false},                         // CR 702.190a, 605.3a: Elektra sneaked for {1}{B}{B} with a Swamp and Surveillance Room's {1},{T} any-colour filter beside two Plains: the filter's prefix search funds the sneak cast
		{"combo_filter_allocation_needs_first", false, false},                   // CR 601.2g, 605.3a: Baxter Building's {4}, {T} makes four mana in any combination; the {G} cast orders the allocation answers that name its pip first, as gorge's scriptOrderAnswers does, so the search finds the script within its 160 nodes (dirac tried all four W first)
		{"power_up_printed_witness_composes_cost_statics", false, false},        // CR 601.2f, 702.193b: Abomination's {5}{R/G}{R/G} Power-up beside Hulk is offered when five sources pay the printed cost less Hulk's {3}, the planner's witness
		{"hybrid_ability_plan_composes_cost_statics", false, false},             // CR 601.2f, 602.2b: Abomination's hybrid Power-up beside Hulk taps the lands of the plan for its printed cost less Hulk's {3}, not the manual window's
		{"baxter_filter_combo_need_first", false, false},                        // CR 605.3b: a combination filter's allocation asks try the combinations naming a colour the cost needs first, so the node budget reaches the funding that pays
		{"sorcery_play_waits_for_filter_prefix_trigger", false, false},          // CR 603.2c, 605.1b: Hydraulic Helper tapped to fund Surveillance Room's {1} fee queues Rewrite History's trigger once, which resolves before the sorcery-speed Shuri is cast
		{"powerup_cost_static_planner_prices_reduced_print_torch", true, false}, // CR 601.2f, 702.193b: Hulk, Gamma Goliath's {3} Power-up reduction composes into the printed cost the autopay planner prices, so Human Torch, Johnny Storm's {6}{R} is offered at {3}{R}
		{"filter_search_branches_on_treasure_funding", true, false},             // CR 605.3a: gorge's prefix search over Surveillance Room's and Baxter Building's fees also funds them with the Treasure, G first, and runs out of its 160 nodes, so The Unbeatable Squirrel Girl's {1}{G}{G}{G} stays unoffered
		{"powerup_cost_static_planner_prices_reduced_print", true, false},       // CR 601.2f, 702.193: Hulk, Gamma Goliath's {3} Power-up reduction composes into the autopay planner's printed cost, so Pet Avengers' {6}{G} is offered at {3}{G} from the sources
		{"power_up_witness_prices_cost_statics", true, false},                   // CR 601.2f: a Power-up offer's planner witness is the printed cost after the cost statics (Hulk, Gamma Goliath's {3} off White Tiger, Ava Ayala's {5}{G}), not the bare printed cost
		{"powerup_witness_composes_cost_statics", true, false},                  // CR 601.2f, 702.193b: gorge's autopay witness for a power-up prices the printed cost after the cost statics (Hulk, Gamma Goliath's {3}), so Quicksilver's {4}{R} is offered from four sources
		{"powerup_window_plan_composes_cost_statics", true, false},              // CR 601.2f, 702.193b: the activation's plan is armed by that same statics-composed witness, so the reduced {1} taps the Forest the plan picks, not the first source in the window
		{"filter_combo_allocates_needed_colours", false, false},                 // CR 601.2g-h, 605.3b: Baxter Building's {4},{T} combination filter pays Truck Toss's {2}{R}{R} once its allocation names the red the search needs
		{"power_up_planner_prices_cost_statics", true, false},                   // CR 601.2f, 602.2b, 702.193a: Hulk, Gamma Goliath's {3} off She-Hulk, Jade Defender's {4}{G}{G} Power-up is in the planner's printed cost, so five lands with three Forests offer it
		{"powerup_static_reduction_offer", true, false},                         // CR 601.2f, 602.2b, 702.193a: Hulk's {3} static lowers Viv Vision's power-up enough for six sources to pay it, though neither the pool nor the printed {7} is affordable
		{"put_counter_optional_departed_source_no_ask", true, false},            // CR 400.7: "you may put a counter on it" is not asked once its source has left the battlefield, since the counter could not land
		{"trigger_cost_bound_counts_filters_paid_ability", true, false},         // CR 118.3, 605.3a: Grim Reaper's {3}{B} with a Plains, a Swamp and Surveillance Room untapped is offered: gorge's potential bound counts the Room's {C} and its {1},{T} any-colour filter (the other sources fund the fee)
	})
}

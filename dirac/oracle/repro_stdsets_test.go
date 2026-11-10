package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Games from the Standard sets' synthetic pools (or, for a card no synthetic
// pool holds, the table built from the repro's own decks) that once
// diverged at the end of their script, on the autopay surface (the full one
// where a case says so): both engines now agree there, and for 200 decisions
// after.
func TestStdSetReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	// A row is {pool, name}, or {pool, name, full} for a game recorded on the full surface
	// (gorge's own payment asks); the others ran on autopay.
	const full = "full"
	for _, c := range [][]string{
		{"std-eoe", "restricted_grant_untap_trigger_cannot_fund_equip"},                 // Mm'menon's spell-only blue on Nanoform Sentinel, whose tap untaps a land, leaves Illvoi Light Jammer's Equip {3} unpayable: gorge's relaxed proof runs no prefix search
		{"std-eoe", "tap_another_source_pays_equip_beside_granted_mana"},                // Gene Pollinator and four lands pay Meltstrider's Gear's Equip {5} beside Mm'menon's spell-only grant: the grant cap keeps the play
		{"std-tla", "waterbend_min_taps_filter"},                                        // Barrels of Blasting Jelly adds nothing to what the window can pay
		{"std-tla", "granted_cap_restricted_source"},                                    // Honest Work beside Jasmine Dragon Tea Shop: the printed sources price Sokka
		{"std-tla", "altadd_mana_part_manual_window"},                                   // Redirect Lightning's {2} part folds into the cost after the offer's plan was armed, so the cast pays through the manual window in battlefield order, not the planner's census (CR 601.2b)
		{"std-tdm", "mayplay_altadd_mana_part_witness"},                                 // Caustic Exhale may-played from exile prices its {1} part beside the printed {B} at the offer, so the plan taps for both before the part is chosen (CR 118.9d, 601.2b)
		{"std-blb", "script_cap_skips_sufficient_plain_casts"},                          // Mind Spring and Mockingbird pay from the Islands: no prefix search spent before Pawpatch Recruit's offspring cast
		{"std-fdn", "kicked_cast_granted_mana_prefix"},                                  // Into the Roil kicked, UU from New Horizons' Mountain: the prefix prices the composed cost
		{"std-blb", "clone_returned_card_spent_zero_mockingbird", full},                 // Splash Portal returns Mockingbird, which was not cast: no mana spent, so it cannot copy Brave-Kin Duo
		{"std-blb", "clone_returned_card_spent_zero_two_birds", full},                   // the same under Finch Formation and Valley Floodcaller: only a creature of mana value 0 is offered
		{"std-ecl", "convoke_target_window_springleaf_drum"},                            // Protective Response convoked by two, the last {W} from a Springleaf Drum tapping a third creature: the target menu keeps its candidates
		{"std-ecl", "convoke_target_window_drum_three_picks"},                           // Appeal to Eirdu convoked by three Kithkin, the last generic from the Drum
		{"std-ecl", "convoke_target_window_bloom_tender", full},                         // Temporal Cleansing with no picks and a Bloom Tender, a source gorge's census cannot price: the cast is not reversed at the target
		{"std-ecl", "wretch_blight_dies"},                                               // a Retched Wretch that came back has no abilities: its second death, with -1/-1 counters again, triggers nothing
		{"std-ecl", "pyrrhic_strike_unpaid"},                                            // Pyrrhic Strike cast without its optional two-life cost: its charm chooses exactly one mode (CR 117.12, 702.34)
		{"std-ecl", "pyrrhic_strike_paid"},                                              // Pyrrhic Strike cast paying the two life: its charm chooses up to two modes (CR 117.12, 702.34)
		{"std-ecl", "kindle_flashback_behold_lowered_at_priority"},                      // Kindle the Inner Flame flashed back with its behold of three Elementals: gorge's planner declines a behold part, so the cast's {1}{R} is tapped at priority before the spell moves, and the behold is chosen after the target (CR 601.2c, 601.2h, 702.34a)
		{"std-ecl", "kindle_copy_end_step_sacrifice_echoed"},                            // a Kindle the Inner Flame copy of an Elemental beside Twinflame Travelers: the copy's own end-step sacrifice triggers an additional time (CR 603.2d, 707.2)
		{"std-fra", "countersculpt_beholds_a_jace"},                                     // Countersculpt's either-or additional cost paid by beholding a Jace planeswalker, not the {1}: the behold is chosen after the target (CR 601.2b, 601.2h)
		{"std-blb", "parting_gust_return_counter"},                                      // Parting Gust returns a creature under its owner's control: the owner puts its counter, so their Stocking the Pantry triggers
		{"std-blb", "calamitous_tide_leave_together", full},                             // Calamitous Tide bounces Dour Port-Mage with Plumecreed Mentor: one departure, so the Port-Mage sees the Mentor leave
		{"std-sos", "treasure_mana_token_ceases_quietly", full},                         // a Treasure sacrificed for mana ceases to exist in the graveyard: Garrison Excavator's cards-leave-your-graveyard trigger stays quiet
		{"std-tdm", "thunder_of_unity_saga_etb"},                                        // a chapter of Thunder of Unity with no other enters watcher out: the creature cast afterwards still drains
		{"std-tdm", "twobrid_window_taps_flexible_source"},                              // a twobrid cast's manual window taps the flexible source for its first owed colour, stranding the pip only that source could make: both engines reverse the cast and keep the tap and its mana (CR 601.2g-h, 733.1)
		{"std-trk", "beam_grant_lone_grantor"},                                          // Scotty the only creature, Open Communications in the graveyard: the grant has no creature to return
		{"std-tdm", "neriv_departed_source_entered"},                                    // Stadium Headliner sacrificed the turn it entered: Neriv still doubles
		{"std-blb", "copied_flashback_spell_was_not_cast"},                              // Kitsa copies a flashed-back Otterball Antics: the copy's token takes no counter
		{"std-fin", "cast_window_restricted_mana_short", full},                          // The Emperor of Palamecia's one mana does not reach Ice Magic's {4}: no target offered, no cast reversed after it
		{"std-blb", "hybrid_face_restricted_source", full},                              // a Village's restricted mana is the only source of a hybrid pip's colour: Cindering Cutthroat's pip ask offers it beside the other
		{"std-tdm", "dragonstorm_return_trigger_source_destroyed"},                      // Encroaching Dragonstorm destroyed with its return-to-hand trigger on the stack: the trigger finds a new object in the graveyard and moves nothing
		{"std-tdm", "dig_rest_stays_in_library_at_taken_cards_enter_ask"},               // United Battlefront takes Hollowmurk Siege and its choose-a-mode ask is pending: the six cards not taken are still in the library
		{"std-tdm", "dig_aura_enters_attached_to_its_one_host"},                         // United Battlefront puts Wingspan Stride onto the battlefield with Host of the Hereafter its only legal host: it enters enchanting it
		{"std-ecl", "dig_aura_two_hosts_asks_what_it_enchants"},                         // Ajani, Outland Chaperone's ultimate takes Evershrike's Gift with two creatures to enchant: the choice is asked before the Aura moves
		{"std-dsk", "eerie_resolutions_tally_across_trigger_lines"},                     // Victor's second Eerie resolution of the turn, one through an entering enchantment and one through a Room fully unlocked, is the opponent's discard, not another surveil
		{"std-fin", "target_dependent_cost_starting_town_white", full},                  // Fate of the Sun-Cryst aimed at a tapped creature costs {2}{W}: the planner's census counts Starting Town's white while a point of life is not lethal
		{"std-fin", "life_pain_land_beside_filter_pays_double_green", full},             // Summon: Titan's GG from Starting Town and Capital City: a paid-life land is no census gap beside the filter
		{"std-blb", "exile_triggered_source_left_battlefield"},                          // Gossip's Talent level 3 with the Colossified Galewind Moose killed by deathtouch as it dealt combat damage: it is in the graveyard, a new object the exile-and-return finds nothing of
		{"std-fra", "loyalty_cost_counters_trigger_put_counters"},                       // Ajani Unrelenting's [+1] beside an Inspired Tethermage: the loyalty counter put as the cost triggers it, with Ajani's own trigger ordered beside it
		{"std-tmt", "sneak_restricted_any_colour_beside_tap_extra"},                     // Leonardo, Big Brother sneaked for {W} from Turtle Lair's restricted any-colour mana with Groundchuck & Dirtbag adding {G} beside it: the lowered payment takes the wheel's any-colour pick
		{"std-tmt", "restricted_any_colour_pick_beside_plain_colour", full},             // Mikey & Leo paid {W}{W} from a Plains and a second land, Turtle Lair tapped for green beside the Plains' white: the green it leaves floating is restricted, so Cowabunga is not offered
		{"std-trc", "julian_graveyard_instant_stays_put"},                               // Julian Bashir's trigger aimed at an instant in the graveyard: it stays there (CR 304.4), so Vindicate finds no such permanent
		{"std-trc", "reg_barclay_face_down_copy_keeps_exceptions"},                      // Reg Barclay copies a face-down creature: the token is a 2/2 face-down copy that is still an Illusion artifact with haste
		{"std-trc", "station_trigger_lists_with_static_grants", full},                   // U.S.S. Prometheus at station 8 beside an older Supplementation Node: the Node's granted counter trigger lists first, as gorge orders the statics' grants
		{"std-trc", "uhura_commander_identity_mana_pays_nothing"},                       // Uhura's "you may pay {1}" with only Arcane Signet and Queen's Diamond untapped and no commander: neither funds it, so only the decline is offered
		{"std-trk", "trigger_cost_window_skips_restricted_source"},                      // Marooned's "you may pay {3}" with Isolinear Processor untapped: its artifact-only mana cannot pay the Aura's trigger, so the window does not tap it
		{"std-trk", "blastoneuron_does_not_see_itself_leave_the_graveyard"},             // Blastoneuron returned from the graveyard by U.S.S. Reliant: its leave-your-graveyard watcher looks back to a board without it (CR 603.10a), so only the enters trigger is put on the stack
		{"std-trk", "single_search_tapped_land_skips_enters_untapped"},                  // Genesis Device's second ability sacrifices one Island and puts Q Continuum onto the battlefield tapped under Syndicate Liquidators: the single card enters as a batch too, so its enters-untapped charm is not put on the stack beside the landfall trigger (CR 603.6a, 110.5b)
		{"std-trk", "genesis_device_tapped_land_skips_enters_untapped"},                 // Genesis Device puts Quark's Bar and four others onto the battlefield tapped: the land entered tapped, so its enters-untapped charm is not put on the stack (CR 603.6a, 110.5b)
		{"std-trk", "sweep_stops_at_shock_land_election"},                               // Deep Space 9 puts Breeding Pool, Plains and Orion onto the battlefield: the sweep stops at the pay-life election and the cards after it stay exiled until it is answered (CR 614.1c, 701.23)
		{"std-trk", "maco_trigger_graveyard_target_is_no_creature_you_control", full},   // Test of Courage exiling a Horta from a graveyard: MACO Strike Team watches creatures you control becoming the target, and a creature card in a graveyard is not one (CR 109.4, 110.2), so no Officer token is made
		{"std-trc", "arex_up_to_three_lands_ends_when_none_left"},                       // Lt. Arex's "up to three" land pick with one land in hand: the pick ends after it, no lone Done posed beside the kernel's priority
		{"std-trc", "sweep_counts_destroyed_token_copy"},                                // Join the Collective destroys nine creatures, one a token copy of Giant Space Amoeba: the copy is still in the graveyard until the next check (CR 111.7, 707.10a), so its nonartifact count makes eight Borg tokens, not seven
		{"std-trc", "doubled_colourless_tap_takes_the_plain_ability", full},             // Painful Truths paid with Spire of Industry's {C} doubled by Forsaken Monument: an any-colour ability makes no {C} (CR 105.1), so the lowering taps the plain ability and the cast is made, as the kernel makes it
		{"std-trc", "phased_out_face_down_permanent_stays_a_2_2"},                       // U.S.S. Defiant phases out a face-down Lightning Greaves: it stays face down (CR 110.5, 708.2), a nameless 2/2 creature, not its printed Equipment
		{"std-trc", "station_counters_skip_a_phased_out_spacecraft", full},              // Queen's Diamond's station ability resolves after U.S.S. Defiant phased it out: it can't be affected (CR 702.26b), so it gets no charge counters
		{"std-spm", "unrelaxable_census_searches_play_wide_source_cannot_fund", full},   // Doc Ock's Tentacles' equip {5} from three lands and two Supportive Parents activations, Interdimensional Web Watch untapped: Parents' cost is no once-only one, so gorge's census is unrelaxable, no relaxed proof drops the play the Web Watch's exile-cast mana cannot fund, and the prefix search pays it
		{"std-spm", "target_menu_keeps_candidates_an_exile_cast_source_may_fund", full}, // Grow Extra Arms cast from exile with only {G} floating, Interdimensional Web Watch untapped: gorge's untapped-source gate reads its exile-cast mana, so the menu keeps Gallant Citizen, and the cast window taps the Web Watch for the unreduced {1} (CR 106.6, 601.2c, 601.2g)
		{"std-spm", "prefix_trigger_if_you_do_cost_answers_yes_first", full},            // Molten Man's prefix taps Spider-Gwen, Free Spirit for Supportive Parents' white: her loot resolves before the cast, and gorge's search answers its "you may discard" election yes first, as gorge lists it, so a card is discarded and one drawn
		{"std-spm", "tap_two_trigger_stacks_before_the_play_is_priced", full},           // Kraven, Proud Predator not offered: the filter search's first Supportive Parents answer taps Spider-Gwen, Free Spirit, her loot trigger is stacked and asked, and the sorcery-speed cast waits for it (CR 307.1, 603.3), so gorge's search spends its 160 nodes without a payer node for Kraven
		{"std-spm", "tap_two_answer_skips_the_plays_own_tap_source", full},              // Guy in the Chair's Web Support, {2}{G},{T}, from an Island, a Forest and Supportive Parents' white: the Parents' "choose 2" answers that tap Guy are skipped, as gorge skips them (CR 602.2b), so the search reaches Lurking Lizards and Mechanical Mobster within its budget
		{"std-spm", "target_menu_keeps_candidates_a_tap_two_source_may_fund", full},     // Grow Extra Arms short of its unreduced {1}{G} with Supportive Parents live: the window census prices no tapXType cost, so the menu keeps every creature, not only the Spider the reduction needs (CR 601.2c, 601.2g)
		{"std-spm", "cast_window_taps_two_creatures_for_supportive_parents", full},      // Grow Extra Arms aimed at a non-Spider, its {1} from Supportive Parents in the cast window: gorge asks for the two creatures to tap, a pick at a time, and the cast is made
		{"std-dft", "run_over_second_target_after_vehicle", full},                       // Run Over aimed first at a crewed Vehicle costs {G}: its second target, an opponent's creature, is still asked
		{"std-msh", "returning_aura_chosen_by_its_owner"},                               // Wiccan, Rising Magician returns Super Speed at the end step under its owner's control: the owner chooses what it enchants
		{"std-tmt", "exile_any_grave_stolen_creature_dies"},                             // a stolen Paramecia Coloniex dies into its owner's graveyard: its controller may still exile it from there
		{"std-hob", "stir_up_trouble_game_ending_treasure_sacrifice"},                   // Stir Up Trouble with the one Treasure for its {B} beside The Sackville-Bagginses, whose drain ends the game: one route offers the cast
		{"std-hob", "target_gated_equip_reduction_autopay"},                             // Dwarven Mauler's "equip abilities that target this creature cost {2} less" is priced by the autopay planner too: Orcrist's {3} equip is offered from two untapped lands at {1} (CR 601.2f, 602.2b)
		{"std-eoe", "grant_on_tapx_source_keeps_its_own_gap_play"},                      // Mm'menon's spell-only grant sits on Gene Pollinator, whose own tapXType ability is the census gap that pays Lander Token's {2}: the kernel's grant search misses the tap-another gap and drops the activation gorge's prefix search prices (CR 605.1a, 601.2g)
		{"std-mkm", "ward_evidence_payment_triggers_collect_evidence"},                  // a ward {Collect evidence} paid with Cryptic Coat's graveyard beside an Evidence Examiner: the payment is a collect-evidence action, so the Examiner investigates
		{"std-blb", "clone_of_lord_dies_its_statics_end", full},                         // a Mockingbird that entered as Ygra dies in combat with it: Ygra is no Food artifact once the copy's static is gone (CR 613.1d)
		{"std-blb", "witness_restricted_source_pays_the_printed_pip"},                   // Mockingbird cast with an any-type rider and Lilypad Village the only blue source: its creature-only {U} pays the printed pip, so the witness taps it for blue instead of pricing the pip as generic (CR 106.6)
		{"std-blb", "dying_copy_mana_value_is_the_copied_cards", full},                  // a Mockingbird copying Plumecreed Escort dies beside Jackdaw Savior: "lesser mana value" reads the copy's 2 (CR 707.2, 608.2h), so Nettle Guard is a target
		{"std-tla", "first_filter_beside_granted_lands_funds_from_the_first_land"},      // Barrels of Blasting Jelly's {1} filter beside Great Divide Guide's any-colour lands: gorge's prefix search taps a Forest, pays the filter with its mana, then the Guide, so the kernel's grant search holds the filter among its candidates (CR 605.3a)
		{"std-tla", "activation_beside_live_grant_funds_from_the_grant"},                // Mai, Jaded Edge's Honest Work grant live beside Rumble Arena's filter: Agna Qel'a's activation prices without the grant in its filter prefix, so the prefix yields to the grant search, which taps Mai for the filter's fee first, as gorge's one search does (CR 605.3a)
		{"std-tla", "land_made_an_island_asks_its_own_colour_first", full},              // a Forest an Earthbend made an Island asks G, then U, then the Guide's any colour: the face's own ability precedes the type's intrinsic one (CR 305.6)
		{"std-tla", "bumi_up_to_lessons"},                                               // Bumi's charm chooses up to the number of Lesson cards in its controller's graveyard, possibly none (CR 117.12)
		{"std-tla", "bumi_lesson_count_filtered_by_matchup"},                            // the same count with one Lesson in the graveyard: the matchup keeps the subtype its filter names, so the trigger is put on the stack with one legal mode (CR 603.3c)
		{"std-woe", "trigger_cost_filter_window_lowest_colour", full},                   // Icewrought Sentry's {1}{U} paid with a Swamp and Scarecrow Guide's {1}-for-any-colour filter: the trigger cost's window has no cost to read, so the filter answers white
		{"std-woe", "granted_ability_counts_on_its_grantor", full},                      // Prophetic Prism sacrificed through the ability Food Fight grants it: the damage is 1 plus the Food Fights its controller controls, a count only Food Fight defines (CR 113.7a), so it deals 2, not the 0 the artifact's own table gives
		{"std-fdn", "joraga_grant_after_witness_protection_loss"},                       // Joraga Invocation cast after Witness Protection took a Helpful Hunter's abilities: the grant is a later layer-6 effect (CR 613.7), so the Hunter must be blocked with the others and the lone Reaver token is offered no other pair
		{"std-fin", "sacrificed_host_returns_aura_which_asks_what_it_enchants"},         // The Gold Saucer's cost sacrifices White Auracite, which sends Sleep Magic back: its controller's choice of what it enchants is asked inside the cost payment and the answer is kept, not dropped (CR 303.4f, 610.3)
		{"std-fin", "lethal_pain_flex_all_ranks_plain_source"},                          // CR 119.4: Starting Town's pay-1-life ability is lethal at 1 life, but the phase-2 key 5 reads the source's FlexAll (its {C} and every colour), so the plan taps the third Forest rather than the {C} land
		{"std-fra", "graveyard_type_count_skips_a_waiting_token"},                       // a Lotus token sacrificed to pay for Hungering Puppetbeast waits in the graveyard for the next check: it is no card, so Tarmogoyf does not count its artifact type (CR 111.7, 704.5d)
		{"std-otj", "saddled_copy_fires_the_copied_triggers", full},                     // a Peerless Ropemaster made a copy of Stubborn Burrowfiend by Assimilation Aegis, saddled: it has the copied card's become-saddled trigger (CR 707.2)
		{"std-msh", "negative_power_dig_looks_at_nothing"},                              // Jessica Jones with a negative power activates: the top X cards are none, not 254 (gorge effDig clamps DigNum to 0)
		{"std-mkm", "presumed_dead_returned_creature_is_suspected"},                     // Presumed Dead on Exit Specialist, which dies and is returned by the trigger: the suspect step still finds the creature its own resolution returned (menace and cannot block), unlike a trigger whose object came back before it resolved
		{"std-mkm", "destroyed_target_takes_no_counter_from_sibling_mode"},              // CR 400.7: Deadly Complication destroys Rune-Brand Juggler, then its counter mode finds no creature: the card returned later by It Doesn't Add Up enters without a +1/+1 counter
		{"std-msh", "graveyard_creature_count_skips_a_waiting_token", full},             // a Villain token that died waits in the graveyard for the next check: it is no card, so Killmonger, Scourge of Wakanda's two-creature-cards bonus stays off (CR 111.7, 704.5d)
		{"std-msh", "copy_ability_sacrificed_clue_is_still_an_artifact"},                // Scientist Supreme of A.I.M. copies the ability of a Clue sacrificed to pay for it: the token has ceased to exist but was an artifact when it left (CR 113.7a), so the ability is a target
		{"std-msh", "x_spell_mana_value_counts_x_for_cost_reduction"},                   // Photon Blast Barrage cast for X = 3 beside The Scarlet Witch: the spell on the stack has mana value 5 (CR 202.3e), so its {2} reduction applies and the X ask offers the values only the reduced cost can pay
		{"std-msh", "ant_man_activation_subcounter_x"},                                  // The Astonishing Ant-Man's activation removes an announced X up to its +1/+1 counters and makes that many Insects (CR 107.3, 118.3)
		{"std-dft", "basri_exert_cost"},                                                 // Basri, Tomorrow's Champion's {W}, {T}, Exert cost makes the token and leaves the exerted creature unable to untap at its controller's next untap step (CR 702.100a-b)
		{"std-fra", "sphinxs_approach_exile_stack_self"},                                // Sphinx's Approach's unless cost exiles itself from the stack and four cards named Sphinx's Approach from the graveyard, then searches a Sphinx creature (CR 118.12a)
		{"std-msh", "x_spell_cast_from_hand_counts_x_for_cost_reduction", full},         // the same cast from the hand with floating mana: the X ask offers 7 and 8, which only the reduced cost pays
		{"std-msh", "lowered_discard_cast_window_restricted_mana", full},                // Titania, Rugged Rumbler cast with a discard, its {B} pip's window tapping Villainous Hideout for Villain-only mana: the window pays, no planner tapping again after the discard (CR 601.2g-h, 106.6)
		{"std-msh", "power_up_used_then_becomes_copy_is_new_ability"},                   // a token Aerial Doombot that used its Power-up and is made a copy of the other Doombot by Shuri until end of turn: the copy's abilities are new, so it may power up again, and the used one returns at cleanup (CR 613.1a, 702.193a)
		{"std-msh", "power_up_copy_mana_cost_reduces", full},                            // a Super Suit made a copy of Aerial Doombot by Shuri on the turn it entered powers up for {5}{U} less the copy's {U}, not Super Suit's printed {1}{U} (CR 702.193b, 707.2)
		{"std-msh", "ruinous_crew_up_to_x"},                                             // The Ruinous Wrecking Crew's enters trigger chooses up to the X paid for it, possibly none (CR 117.12, 107.3)
		{"std-msh", "restricted_mana_spell_only_batch_does_not_pay_equip", full},        // Castle Doom's artifact-spell-only mana floating beside a Guidelight Optimizer, whose mana also activates abilities, does not pay Falcon's Wing Harness's equip: the two are different restrictions (CR 106.6)
		{"std-msh", "crewed_vehicle_that_died_is_no_creature_card"},                     // a Dependable Quinjet crewed by Undercover Skrull dies beside one creature card: it is an artifact card in the graveyard, so Skrull's two-creature-cards bonus stays off in the move's own event (CR 400.7, 613.1d)
		{"std-dft", "sacrifice_group_keeps_entrant_after_as_enters_choice"},             // Push the Limit returns Burner Rocket, Clamorous Ironclad and Lifecraft Engine, which asks for its creature type: the Engine joins the other two in the one end-step sacrifice (CR 603.7)
		{"std-trk", "last_player_wins_before_cant_lose_source_departs"},                 // a stolen Georgiou keeps its controller alive at 0 life; its owner loses and leaves, and the survivor wins at once (CR 104.2a, 800.4a) instead of losing to the life total as the card leaves
		{"std-trk", "kicker_sacrifice_treasure_spent_for_mana"},                         // Eject the Warp Core with a Treasure the only artifact and the only black source: the payment sacrifices it for {B} before the kicker's cost is paid, so the kicked cast is not offered (CR 601.2g-h)
		{"std-trk", "census_order_choice_source_beside_fixed_grant_shuttle"},            // Dr. Leonard "Bones" McCoy paid {1}{G}{G} beside Rok-Tahk's {G}{G} grant: a creature Class-F Shuttlecraft's any-colour mana sits at its battlefield position in gorge's census, so the generic is paid by the first Plains after it, not the first Plains (key 8)
		{"std-trk", "census_order_choice_source_beside_fixed_grant_phlox"},              // the same census order with Xenobotanist and Phlox's any-colour mana under Rok-Tahk's grant
		{"std-trk", "granted_mana_one_tap_cannot_fund_two_pips", full},                  // Warp Speed's {7}{U}{U} less three with Xenobotanist the only blue source beside Rok-Tahk's granted {G}{G}: one tap makes one blue, so the activation is not offered
		{"std-dsk", "finality_wurm_two_exile_marks_no_ask"},                             // Balustrade Wurm back with a finality counter takes two Scorching Dragonfire exile marks: the counter exiles it at the move, no order ask (CR 614.1, gorge finalityReplacementApplies)
		{"std-dsk", "token_x_power_read_by_enters_watcher"},                             // Unwilling Vessel dies with three counters: the Spirit token enters as a 3/3, so Enduring Innocence's power-2-or-less watcher stays quiet (CR 111.3)
		{"std-dsk", "exile_cost_cast_priced_on_printed_sources", full},                  // Abhorrent Oculus with six cards to exile and its blue only from Enduring Vitality's grant to Fear of Impostors: the planner prices the shape on the printed sources, so the cast is not offered
		{"std-dsk", "face_down_creature_loses_granted_mana", full},                      // a manifested Zimone under Unable to Scream, which came after Enduring Vitality's grant: it has no mana ability, so Tyvar's {3}{G}{G} is not offered with four sources (CR 613.1f, 613.7)
		{"std-dsk", "returned_enchantment_eerie"},                                       // CR 603.6a: Enduring Vitality returns from the graveyard while Diversion Specialist's dug card resolves, and Infernal Phantom's Eerie trigger on the returned enchantment is put on the stack
		{"std-msh", "hulk_enrage_blocker_died_not_attacking"},                           // a blocking The Incredible Hulk killed in combat: "if he's attacking" reads a creature card in the graveyard, which attacks nothing, so no extra combat follows (CR 506.4, 400.7)
		{"std-msh", "dfc_back_face_trigger_asks_optional_after_dying", full},            // The Sensational She-Hulk's damage trigger, put on the stack as she died: the line is printed on her back face, which the card in the graveyard no longer shows, and the may is still asked (CR 712.8a, 603.5)
		{"std-msh", "ward_alt_pays_with_restricted_nonspell_mana", full},                // Titania's ward paid with Hydraulic Helper's mana, which may not cast a nonartifact spell but pays a ward (CR 106.6)
		{"std-msh", "hulk_power_up_reducer", full},                                      // Hulk, Gamma Goliath's "power-up abilities of other creatures you control cost {3} less" lowers Brave Brawler's {4}{W} Power-up to {1}{W} (CR 601.2f, 602.2b, 702.193a)
		{"std-msh", "power_up_witness_composes_cost_statics"},                           // CR 601.2f: gorge's printed-cost power-up witness still composes cost statics, so Hercules is offered with two lands once Hulk takes {3} off his {4}{G}
		{"std-msh", "exile_cast_discard_alt_cost_keeps_whole_hand", full},               // Titania cast from exile through Moonstone's permission discards the one card in hand: the spell is not in the hand, so it does not count against it (CR 601.2b)
		{"std-msh", "resolved_limit_new_object_after_returning"},                        // Baron Strucker, whose "once each turn" connive trigger resolved before he died, comes back under Thunderbolts Conspiracy as a new object (CR 400.7): the Swordsman returning after him triggers it again
		{"std-msh", "modal_token_copy_exceptions_both_faces"},                           // Multiversal Incursion's token copy of Tony Stark, "except it isn't legendary", transforms: the token shows the other face with the same exception (CR 707.8a, 701.28)
		{"std-trk", "dig_shock_land_election_holds_entrants_checks"},                    // Branching Timeline puts Sacred Foundry and Syndicate Liquidators onto the battlefield: the Foundry's pay-life ask suspends the dig, and Liquidators' landfall for it goes on the stack only after the whole dig is done (CR 603.6a)
		{"std-trk", "dig_shock_land_election_trigger_order"},                            // the same dig with Hallowed Fountain asked before three more entrants: all their enters triggers are ordered together once the dig is done
		{"std-trk", "prefix_surveil_trigger_mills_the_card", full},                      // Deep Space 9 cast after tapping Jett Reno for Rok-Tahk's granted {G}{G}: gorge's prefix lets Jett's surveil resolve first and answers it with the empty top pile, so Adversary by Algorithm goes to the graveyard (CR 701.25a)
		{"std-trk", "beam_cost_returns_leave_graveyard_watcher_stalker"},                // Final Frontier cast by beam me up returning Hologram Stalker: the spell leaves the graveyard before the cost is paid (CR 601.2a), so Stalker's "whenever a card leaves your graveyard" triggers and goes on the stack though Stalker left (CR 603.3, 113.7a)
		{"std-trk", "beam_cost_returns_leave_graveyard_watcher_vash"},                   // the same with Vash, Freelance Archaeologist returned by the beam me up cost: its draw trigger is put on the stack
		{"std-trk", "charm_mode_sub_target_keeps_make_it_so"},                           // Make It So targeting a Treasure it then sacrifices for mana: the target player is still legal, so the spell resolves and that player draws two cards (CR 608.2b)
		{"std-trk", "late_entry_order_unicomplex_condition_fails"},                      // The Unicomplex played under two Warp Speeds with two other lands: its own enters-tapped body applied last at the re-ask taps nothing, so it stays untapped and its enters-untapped trigger is put on the stack (CR 616.1f)
		{"std-trk", "filter_search_funds_with_restricted_colourless"},                   // Starfleet Academy's {5} with Isolinear Processor untapped: gorge's prefix search tries the artifact-only {C} as funding and runs out of nodes, so Artificial Anomaly's sacrifice ability is not offered (CR 605.3a)
		{"std-trk", "static_cast_trigger_keeps_caster_priority"},                        // Test of Identity cast from Tasha Yar's exile on the opponent's turn with Overflow Submatrix out: its Static$ True cast trigger resolves off the stack and the caster keeps priority (CR 117.3c)
		{"std-dft", "repeat_each_targeted_mills_the_graveyard_counts"},                  // Riverchurn Monument's exhaust ability targets both players: each mills cards equal to their own graveyard count, the controller's mill first (gorge AliveFrom), and both engines agree on every one
		{"std-eoe", "repeat_each_targeted_mills_half_the_library"},                      // Singularity Rupture destroys all creatures, then its two target players each mill half their library rounded down, the controller first (gorge AliveFrom), and both engines agree on every one
		{"std-sos", "ark_of_hunger_mayplay_both_zones"},                                 // Ark of Hunger's tap mills a card and registers the remembered card's play permission over the graveyard and exile: the recorded game reaches the tap, and both engines agree through the grant and the cleanup
		{"std-spm", "parker_luck_repeat_each_loses_the_others_reveal_mv"},               // Parker Luck's end-step trigger: two players each reveal their top card, and each loses life equal to the OTHER player's revealed card's mana value (Swamp 0, Prowler, Clawed Thief 3), then both cards go to hand; both engines agree on every one
		{"std-dft", "exile_trigger_looks_back_from_battlefield_or_graveyard"},           // Ketramose warped and exiled at the end step sees itself leave: a ChangesZoneAll line with Origin$ Battlefield,Graveyard looks back like Origin$ Battlefield (CR 603.10a)
		{"std-msh", "improvise_cast_spends_artifact_only_mana"},                         // Arc Reactor improvised with Captain America's Shield beside Hydraulic Helper's artifact-only blue: the cast spends the restricted blue before the plain pool
		{"std-fin", "setpt_static_live_attached_controller"},                            // Aettir and Priwen gives the equipped creature base P/T equal to the Equipment controller's life total: the live layer-7b set is re-read as the life total changes, after the equip cost is paid
		{"std-fin", "flickered_bearer_equipment_unattached_at_reentry"},                 // Phoenix, Warden of Fire exiles itself and returns beside its twin, a legend ask pending: the Coral Swords it bore show unattached (CR 400.7), not on the returned Joshua
		{"std-dft", "artifact_mana_restricted_to_activated_abilities_boommobile"},       // Boommobile enters and adds four mana of one colour spendable only to activate abilities: they pay its exhaust ability and no spell, and the artifact's units are typed (CR 106.6, 605.1a)
		{"std-fin", "dig_imprint_reno_and_rude_plays_the_exiled_card"},                  // Reno and Rude connects, exiles the top card and sacrifices another permanent: the exiled card, imprinted on it, is cast from exile with mana of any type, then the imprint is cleared
		{"std-woe", "dig_imprint_rotisserie_elemental_plays_the_exiled_card"},           // Rotisserie Elemental connects and sacrifices itself: the top cards (as many as its skewer counters) are exiled and imprinted, played this turn, then the imprint is cleared
		{"custom", "dig_imprint_chandra_legacy_of_fire_plays_the_exiled_cards"},         // Chandra, Legacy of Fire removes loyalty counters from her permanents, exiles that many cards and imprints them: each is played from exile this turn, then the imprint is cleared (the remembered amount stays apart from the imprinted set)
		{"custom", "search_second_pick_shares_a_land_type"},                             // Myriad Landscape finds an Island, then a second Island: gorge's Submit refuses two basics with no common land type, so the second pick offers only the Islands, as the kernel offers them; played on its own decks' table because the tail partition no longer holds every card of the recorded decks
		{"std-msh", "noncombat_damage_plus_host_power"},                                 // Avengers Disassembled beside Hawkeye, Young Avenger (power 2): its 3 damage to each opposing creature becomes 5, to its controller's own creatures stays 3 (CR 614.1a)
		{"std-tdm", "three_counter_kinds_among_creatures_pump_the_veteran"},             // +1/+1, stun and a lifelink counter spread over one player's creatures: both Hundred-Battle Veterans get +2/+4 while three different kinds are among them, and both engines agree on every one
		{"std-tdm", "reverberating_summons_discard_hand"},                               // Reverberating Summons activates with six cards in hand: the whole hand is discarded in hand order, then the enchantment is sacrificed, and both engines agree through the resolution
		{"std-tdm", "omen_face_census_credits_granted_delve"},                           // Skimming Strike, Dirgur Island Dragon's Omen, offered off one floating {U} with Teval's granted delve paying its {1} (CR 702.66a, 720.3a)
		{"std-tmt", "graveyard_return_target_and_same_named_cards_tapped"},              // Rat King, Verminister's return resolves: its graveyard target and every other card there with its name enter tapped, both engines agreeing on every one
		{"std-msh", "ward_poison_counters_pay_and_decline"},                             // The Serpent Society targeted twice by the opponent: one ward election pays its five poison counters (the payer gets them), the other declines and counters the spell, both engines agreeing on every one (CR 702.21a, 122.1f)
		{"std-msh", "unless_draw_x_paid_opponent_draws"},                                // Armor Wars' chapter I with artifacts out: the controller pays by drawing a card for each, then each opponent draws one, both engines agreeing on the count and the order
		{"std-msh", "unless_draw_x_declined_and_zero_x_decline_only"},                   // Armor Wars' chapter I twice: once declined with artifacts out, once with none, where the draw is offered as a decline alone (an X of zero is no payment, CR 118.3)
		{"std-tla", "unless_damage_you_x_source_power_pay_and_decline"},                 // Combustion Man's attack trigger: the target's controller either takes damage equal to his power or the permanent is destroyed, both engines agreeing on each answer (CR 118.12, 120.3)
		{"std-tmt", "casey_jones_next_upkeep_discards_three_at_random"},                 // Casey Jones, Vigilante's enters trigger draws three and registers a delayed program: at its controller's next upkeep three cards are discarded at random (CR 603.7)
		{"std-woe", "feral_encounter_next_combat_power_damage"},                         // Feral Encounter's delayed program fires at the next beginning of combat this turn: a creature you control is targeted, the optional second target declined (CR 603.7, 115.1)
		{"custom", "graveyard_return_target_and_same_named_cards_to_hand"},              // Echoing Return resolves: its graveyard target and every other card there with its name go to the hand, both engines agreeing on every one; played on its own decks' table because the tail-53 pool no longer holds Teetering Peaks
		{"std-trc", "effect_trigger_damage_to_you_fires_for_the_owner"},                 // Right of Vengeance cast on the opponent's turn: the until-end-of-turn trigger fires for a creature's combat damage to its owner only
		{"std-dft", "effect_trigger_captures_the_parent_target"},                        // Captain Howler's discard trigger pumps a creature and the Effect link remembers that target: only its combat damage to a player draws
		{"custom", "return_animate_before_entry_replacement", full},                     // Good-Fortune Unicorn returned by Valkyrie's Call beside Giada, Font of Hope: the return's "it's an Angel" applies before the entry replacements read it, so Giada's extra counters count (CR 614.12)
		{"custom", "printed_delve_gurmag_angler", full},                                 // Gurmag Angler cast off Priest of Gix's floating black with two graveyard cards delved: the printed keyword pays through the kernel's existing delve ask (CR 702.66a)
		{"custom", "printed_delve_murderous_cut", full},                                 // Murderous Cut cast off Priest of Gix's floating black with graveyard cards delved: the same printed-keyword path
		{"custom", "cmd5_ancient_tomb_two_colourless_fund_a_spell", full},               // kfuzz cmd5 heuristic pair 10: Ancient Tomb's one activation adds {C}{C} (a pain-only source, 2 damage), so beside two Mountains it funds Archwing Dragon's {2}{R}{R} and the cast is offered (CR 106.1, 605.1a); played on its own decks' table because the cmd5 pool no longer holds the card
		{"std-fra", "rise_of_the_deathbringer_remembers_drawn"},                         // Rise of the Deathbringer cast in its Draw mode: it draws three, and the chained lose-life reads the remembered set (life 1, not 4, when the write is lost)
		{"std-sos", "transcendent_archaic_optional_draw_remembers", full},               // Transcendent Archaic's optional Converge draw accepted off Forests: it draws one, and the chained discard of two runs on the remembered condition
		{"custom", "spider_slayer_destroys_blocking_spider"},                            // a Spider-Slayer that deals combat damage to a blocking Giant Spider destroys it: the damaged creature is the DamageDone trigger's object, so the destroy is not a no-op
		{"custom", "cosmium_confluence_destroys_two_enchantments"},                      // Cosmium Confluence chooses its destroy mode twice and its search mode once: the two enchantments are the two instances' targets, the groups asked in printed order (CR 700.2d, 601.2c)
		{"custom", "moment_of_reckoning_destroys_twice"},                                // Moment of Reckoning chooses its destroy mode twice: each instance destroys its own target (CR 700.2d)
		{"custom", "season_of_loss_sacrifices_three_times"},                             // Season of Loss chooses its one-pawprint mode three times and its two-pawprint mode once: five pawprints, nothing left to add (CR 700.2i, 107.18)
		{"custom", "put_counter_choice_remembers_the_recipient_season_of_gathering"},    // Season of Gathering puts three +1/+1 counters on a creature you control, each pick remembered so it gains vigilance and trample until end of turn, then destroys every artifact or enchantment (CR 700.2i, 122.1)
		{"custom", "put_counter_choice_remembers_the_recipient_kaylas_command"},         // Kayla's Command chooses the token and counter modes: the +1/+1 counter goes on a creature you control, which is remembered and gains double strike until end of turn (CR 700.2, 122.1)
		{"custom", "search_remembers_the_searcher_prishes_wanderings"},                  // Prishe's Wanderings searches for a basic land and puts it onto the battlefield tapped: the search is remembered, so the +1/+1 counter trigger it gates goes on a creature you control (CR 701.23b, 603.12)
		{"custom", "cmd5_commander_bounce_waits_for_the_command_zone_choice", full},     // kfuzz cmd5 heuristic pair 3: Boomerang aimed at Lathliss, Dragon Queen, a commander: the owner's command-zone choice is posed while Boomerang is still on the stack, as the bounce instruction would move her, and Boomerang reaches the graveyard only after the answer (CR 608.2n, 903.9b)
		{"custom", "webslinged_enter_spiders_man_tokens", full},                         // Spiders-Man, Heroic Horde cast for its web-slinging cost off two Hulking Raptors' and a Burning-Tree Emissary's floating mana, a crewed Smuggler's Copter's pilot returned: its enters trigger gains 3 life and makes two Spider tokens (CR 702.188, 603.4); a plain cast would not
		{"custom", "cmd5_token_ids_reused_after_cease", full},                           // kfuzz cmd5 heuristic pair 3: Keen Engineering's thopters and the goblins make the game's 25th token with both 100-card decks holding 200 object ids: the new token takes the id of one that ceased to exist in an earlier turn, on both sides (CR 111.7, 704.5d); TestCmd5TokenIdsReusedNoStall checks it does not stall
		{"custom", "behold_resolution_cost_untaps_the_searched_land"},                   // Elven Passage's search finds a basic land and its unless-switched behold of an Elf pays with one of three candidates (battlefield Elves then hand Elves): nothing moves and the land untaps; both engines agree through the pick and past it (CR 701.4a)
		{"custom", "behold_trigger_cost_makes_a_treasure"},                              // Sarkhan, Dragon Ascendant's enters trigger offers its own behold cost of a Dragon: paying with one of two candidates makes the Treasure, and both engines agree through the pick and past it (CR 701.4a)
		{"custom", "end_blaze_chosen_card_played_next_turn", full},                      // End-Blaze Epiphany kills a Grizzly Bears, exiles that many cards and lets its controller play the chosen one on their next turn (the may-play of the chosen card to the end of the next turn)
		{"custom", "wondrous_wasp_target_loses_abilities"},                              // The Wondrous Wasp enters and taps a Llanowar Elves, which loses all abilities for as long as the Wasp stays on the battlefield (a host-bound loss of abilities)
		{"custom", "impractical_joke_damages_a_shielded_creature"},                      // Impractical Joke deals 3 to a Swooping Protector: damage can't be prevented this turn, so its SHIELD counter stays and the damage is dealt (CR 615.12, 122.1c)
		{"custom", "a_ready_to_rumble_damages_a_shielded_creature"},                     // A-Ready to Rumble's first mode deals 5 to a Swooping Protector past its SHIELD counter (CR 615.12)
		{"custom", "pyrewood_gearhulk_enters_damage_cant_be_prevented"},                 // Pyrewood Gearhulk enters: its trigger pumps the other creatures and makes damage unpreventable for the turn (an end-of-turn emblem)
		{"custom", "flaring_pain_resolves"},                                             // Flaring Pain resolves: damage can't be prevented this turn
		{"custom", "leyline_of_punishment_on_the_battlefield"},                          // Leyline of Punishment's printed statics: players can't gain life, damage can't be prevented
		{"custom", "cmd5_commander_death_choice_precedes_token_cease", full},            // kfuzz cmd5 heuristic pair 31: Giada, Font of Hope, a commander, blocks a Zombie token and both die in combat: the command-zone choice is posed before the check's other actions, so the token is still in the graveyard at the ask and ceases after the answer (CR 704.3, 704.6d, 903.9a)
		{"custom", "cmd5_hidden_pick_commander_to_command_zone", full},                  // kfuzz cmd5 heuristic pair 110: Temur Sabertooth returns Ghalta, Primal Hunger, a commander, and the owner sends it to the command zone instead: it leaves the battlefield, Rancor goes to the graveyard and its trigger is stacked (CR 903.9b)
		{"custom", "cmd5_hidden_pick_commander_to_hand", full},                          // kfuzz cmd5 heuristic pair 92: the same return answered to let it go: Ghalta reaches its owner's hand and the resolution goes on (CR 903.9b)
		{"custom", "belligerent_attack_plays_top_of_library"},                           // The Belligerent crewed and attacking: its trigger lets p0 look at and play the top card of their library for the turn, lands and spells, and the top card changes as each is played
		{"custom", "gwenom_attack_pays_life_for_top_of_library"},                        // Gwenom, Remorseless attacking: a spell cast from the top of the library is offered at the alternative cost only and pays life equal to its mana value in place of its mana cost (CR 118.9)
		{"custom", "flashback_hybrid_pips_and_discard"},                                 // Twinned Vision flashed back for {1}{U/R}{U/R} and a discard, thirteen times: each hybrid pip's colour is announced like a printed cost's, then the discard is picked (CR 107.4e, 601.2b, 702.34a)
		{"custom", "sneak_hybrid_pip_announced"},                                        // Foot Ninjas sneaked for {3}{W/B}: the hybrid pip's colour is announced like a printed cost's (CR 107.4e, 601.2b, 702.190a)
		{"std-woe", "restricted_each_source_no_colour_ask_beside_filter"},               // Troyan, Gutsy Explorer's {G}{U} is one activation with no colour ask: the filter search beside Crystal Grotto charges it one node, so Restless Fortress's animate is still found within the 160-node budget (CR 106.1, 605.3b)
		{"std-woe", "bargain_only_offer_with_restricted_source_in_pool"},                // Johann's Stopgap bargained for {1}{U} off a floating pool is offered whatever restricted sources the pool holds: Troyan in the decks no longer hides the discounted cast (CR 702.166, 601.2f)
		{"custom", "imprint_token_counters_iroh_tea_master_ally"},                       // Iroh, Tea Master gives a permanent away and makes the 1/1 Ally: its +1/+1 counters go on the imprinted token only, and the cleanup clears the imprint (the imprinted set apart from the remembered given-away permanent)
		{"custom", "imprint_exile_return_sirens_ruse"},                                  // Siren's Ruse exiles a creature you control and returns the imprinted card to the battlefield, both engines agreeing through the return and past it
		{"custom", "imprint_token_counters_summoners_sending_spirit"},                   // Summoner's Sending exiles a creature card from a graveyard and makes a Spirit that takes a +1/+1 counter off the imprinted token when the exiled card's mana value is 4 or more
	} {
		pool, name := c[0], c[1]
		if len(c) > 3 || len(c) == 3 && c[2] != full {
			t.Fatalf("row %v: want {pool, name} or {pool, name, %q}", c, full)
		}
		onFull := len(c) == 3
		t.Run(name, func(t *testing.T) {
			// A tail pool is a hash-ordered run of the lowered non-Standard cards, so its
			// membership moves whenever another card is lowered: a repro pinned to one
			// breaks on an unrelated change. Such a repro plays on its own cards.
			if strings.HasPrefix(pool, "tail-") {
				t.Fatalf("pool %q: a repro on a tail pool uses the pool \"custom\"", pool)
			}
			if div := replayStdRepro(t, reg, pool, name, onFull); div != nil {
				t.Fatalf("divergence at %d: %s %s %v", div.At, div.What, div.Field, div.Diff)
			}
		})
	}
}

// replayStdRepro replays std pool's repro name on the full surface or on
// autopay, and 200 decisions past its script: the divergence, or nil.
func replayStdRepro(t *testing.T, reg *cards.Registry, pool, name string, full bool) *Divergence {
	t.Helper()
	return replayStdReproResult(t, reg, pool, name, full).Div
}

// replayStdReproResult is replayStdRepro's whole lockstep result.
func replayStdReproResult(t *testing.T, reg *cards.Registry, pool, name string, full bool) Result {
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
	set := reproSet(t, pool, r)
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = !full
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

// TestTapXOwnAnswerMutation: with the answers that tap the play's own {T}
// source kept, the Supportive Parents search spends its nodes on them and
// Guy in the Chair's Web Support is not offered where gorge offers it.
func TestTapXOwnAnswerMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugNoTapXOwnSkip = true
	defer func() { dirac.DebugNoTapXOwnSkip = false }()
	if div := replayStdRepro(t, reg, "std-spm", "tap_two_answer_skips_the_plays_own_tap_source", true); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}

// TestFiltTrigSettleMutation: with a step's Taps trigger left unstacked, the
// filter search prices Kraven, Proud Predator under Spider-Gwen's loot and
// offers the cast gorge does not.
func TestFiltTrigSettleMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugFiltNoTrigSettle = true
	defer func() { dirac.DebugFiltNoTrigSettle = false }()
	if div := replayStdRepro(t, reg, "std-spm", "tap_two_trigger_stacks_before_the_play_is_priced", true); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}

// TestAskYesFirstMutation: with a yes-or-no election tried in the kernel's
// order, the prefix search declines Spider-Gwen's loot where gorge's discards
// and draws.
func TestAskYesFirstMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugAskNoYesFirst = true
	defer func() { dirac.DebugAskNoYesFirst = false }()
	if div := replayStdRepro(t, reg, "std-spm", "prefix_trigger_if_you_do_cost_answers_yes_first", true); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}

// TestRestrManaOpenMutation: with a restricted mana source left out of the
// window's untapped-source gate, Grow Extra Arms's menu drops the creature
// Interdimensional Web Watch's exile-cast mana may fund.
func TestRestrManaOpenMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugNoRestrManaOpen = true
	defer func() { dirac.DebugNoRestrManaOpen = false }()
	if div := replayStdRepro(t, reg, "std-spm", "target_menu_keeps_candidates_an_exile_cast_source_may_fund", true); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}

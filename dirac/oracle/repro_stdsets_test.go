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
		{"std-big", "worldwalker_helm_map_follows_the_first_of_two_foods"},              // Worldwalker Helm beside Bristlebud Farmer's two Foods: the Map is made once for the Token resolution, entering after the first Food (CR 614.1a, 616)
		{"std-big", "worldwalker_helm_copy_makes_its_map_before_the_copy"},              // Worldwalker Helm copying an artifact token: gorge makes the Map first, then the copy (CR 707.2, 111.5)
		{"std-big", "worldwalker_helm_five_adders_all_make_a_map_copy_a", full},         // five Worldwalker Helms copying a Map token: each adds its Map before the copy, five of them (the kernel's adder buffer held four)
		{"std-big", "worldwalker_helm_five_adders_all_make_a_map_copy_b", full},         // the same, the copy made by the other player's Helm
		{"std-eoe", "tapestry_warden_stations_with_toughness"},                          // Tapestry Warden makes a power<toughness creature station with its toughness: the 3/4 Warden puts four charge counters on Entropic Battlecruiser (CR 702.184a)
		{"std-eoe", "restricted_grant_untap_trigger_cannot_fund_equip"},                 // Mm'menon's spell-only blue on Nanoform Sentinel, whose tap untaps a land, leaves Illvoi Light Jammer's Equip {3} unpayable: gorge's relaxed proof runs no prefix search
		{"std-eoe", "tap_another_source_pays_equip_beside_granted_mana"},                // Gene Pollinator and four lands pay Meltstrider's Gear's Equip {5} beside Mm'menon's spell-only grant: the grant cap keeps the play
		{"std-tla", "waterbend_min_taps_filter"},                                        // Barrels of Blasting Jelly adds nothing to what the window can pay
		{"std-tla", "granted_cap_restricted_source"},                                    // Honest Work beside Jasmine Dragon Tea Shop: the printed sources price Sokka
		{"std-tla", "altadd_mana_part_manual_window"},                                   // Redirect Lightning's {2} part folds into the cost after the offer's plan was armed, so the cast pays through the manual window in battlefield order, not the planner's census (CR 601.2b)
		{"std-tdm", "mayplay_altadd_mana_part_witness"},                                 // Caustic Exhale may-played from exile prices its {1} part beside the printed {B} at the offer, so the plan taps for both before the part is chosen (CR 118.9d, 601.2b)
		{"std-tdm", "altadd_fallback_window_filter_lowest_colour"},                      // Caustic Exhale's {1} part chosen beside a payable behold: the fallback window answers Abzan Devotee's colour after its fee with the lowest colour, not the owed {B}, so the cast is stranded and reversed (CR 601.2g, 733.1)
		{"std-tdm", "altadd_fallback_window_stranded_reverses", full},                   // the same fallback window with a Sandsteppe Citadel's colour: the stranded payment reverses the cast, not "autopay found no plan" (CR 733.1)
		{"std-blb", "script_cap_skips_sufficient_plain_casts"},                          // Mind Spring and Mockingbird pay from the Islands: no prefix search spent before Pawpatch Recruit's offspring cast
		{"std-fdn", "kicked_cast_granted_mana_prefix"},                                  // Into the Roil kicked, UU from New Horizons' Mountain: the prefix prices the composed cost
		{"std-blb", "clone_returned_card_spent_zero_mockingbird", full},                 // Splash Portal returns Mockingbird, which was not cast: no mana spent, so it cannot copy Brave-Kin Duo
		{"std-blb", "clone_returned_card_spent_zero_two_birds", full},                   // the same under Finch Formation and Valley Floodcaller: only a creature of mana value 0 is offered
		{"std-ecl", "convoke_target_window_springleaf_drum"},                            // Protective Response convoked by two, the last {W} from a Springleaf Drum tapping a third creature: the target menu keeps its candidates
		{"std-ecl", "convoke_target_window_drum_three_picks"},                           // Appeal to Eirdu convoked by three Kithkin, the last generic from the Drum
		{"std-ecl", "convoke_target_window_bloom_tender", full},                         // Temporal Cleansing with no picks and a Bloom Tender, a source gorge's census cannot price: the cast is not reversed at the target
		{"std-ecl", "static_hexproof_own_colours_red_spell"},                            // Tam, Mindful First-Year's own-colours hexproof static: a red creature beside it is not an opponent's red spell target while Tam is, and both engines agree
		{"std-ecl", "wretch_blight_dies"},                                               // a Retched Wretch that came back has no abilities: its second death, with -1/-1 counters again, triggers nothing
		{"std-ecl", "pyrrhic_strike_unpaid"},                                            // Pyrrhic Strike cast without its optional two-life cost: its charm chooses exactly one mode (CR 117.12, 702.34)
		{"std-ecl", "pyrrhic_strike_paid"},                                              // Pyrrhic Strike cast paying the two life: its charm chooses up to two modes (CR 117.12, 702.34)
		{"std-ecl", "kindle_flashback_behold_lowered_at_priority"},                      // Kindle the Inner Flame flashed back with its behold of three Elementals: gorge's planner declines a behold part, so the cast's {1}{R} is tapped at priority before the spell moves, and the behold is chosen after the target (CR 601.2c, 601.2h, 702.34a)
		{"std-ecl", "kindle_copy_end_step_sacrifice_echoed"},                            // a Kindle the Inner Flame copy of an Elemental beside Twinflame Travelers: the copy's own end-step sacrifice triggers an additional time (CR 603.2d, 707.2)
		{"std-ecl", "hidden_pick_library_position_top", full},                           // Lluwen, Imperfect Naturalist puts a milled creature or land card on top of the library (LibraryPosition$ 0), not at the bottom: the hidden pick places what it moved
		{"std-fra", "countersculpt_beholds_a_jace"},                                     // Countersculpt's either-or additional cost paid by beholding a Jace planeswalker, not the {1}: the behold is chosen after the target (CR 601.2b, 601.2h)
		{"std-blb", "parting_gust_return_counter"},                                      // Parting Gust returns a creature under its owner's control: the owner puts its counter, so their Stocking the Pantry triggers
		{"std-blb", "calamitous_tide_leave_together", full},                             // Calamitous Tide bounces Dour Port-Mage with Plumecreed Mentor: one departure, so the Port-Mage sees the Mentor leave
		{"std-sos", "treasure_mana_token_ceases_quietly", full},                         // a Treasure sacrificed for mana ceases to exist in the graveyard: Garrison Excavator's cards-leave-your-graveyard trigger stays quiet
		{"std-tdm", "return_enters_with_hexproof_and_indestructible_counters"},          // Perennation returns a milled card with a hexproof and an indestructible counter: both are part of its entry, in that order (CR 122.6, 614.1c)
		{"std-tdm", "thunder_of_unity_saga_etb"},                                        // a chapter of Thunder of Unity with no other enters watcher out: the creature cast afterwards still drains
		{"std-tdm", "twobrid_window_taps_flexible_source"},                              // a twobrid cast's manual window taps the flexible source for its first owed colour, stranding the pip only that source could make: both engines reverse the cast and keep the tap and its mana (CR 601.2g-h, 733.1)
		{"std-trk", "beam_grant_lone_grantor"},                                          // Scotty the only creature, Open Communications in the graveyard: the grant has no creature to return
		{"std-tdm", "neriv_departed_source_entered"},                                    // Stadium Headliner sacrificed the turn it entered: Neriv still doubles
		{"std-blb", "copied_flashback_spell_was_not_cast"},                              // Kitsa copies a flashed-back Otterball Antics: the copy's token takes no counter
		{"std-fin", "cast_window_restricted_mana_short", full},                          // The Emperor of Palamecia's one mana does not reach Ice Magic's {4}: no target offered, no cast reversed after it
		{"std-blb", "hybrid_face_restricted_source", full},                              // a Village's restricted mana is the only source of a hybrid pip's colour: Cindering Cutthroat's pip ask offers it beside the other
		{"std-tdm", "dragonstorm_return_trigger_source_destroyed"},                      // Encroaching Dragonstorm destroyed with its return-to-hand trigger on the stack: the trigger finds a new object in the graveyard and moves nothing
		{"std-fra", "theorists_proxy_next_spell_survives_countersculpt"},                // Theorist's Proxy sacrificed, then another cast: the opponent's Countersculpt does not counter it (CR 101.2)
		{"std-tdm", "mistrise_village_next_spell_survives_dispelling_exhale", full},     // Mistrise Village's activation, then Highspire Bell-Ringer: the opponent's Dispelling Exhale does not counter it (CR 101.2)
		{"std-tdm", "dig_rest_stays_in_library_at_taken_cards_enter_ask"},               // United Battlefront takes Hollowmurk Siege and its choose-a-mode ask is pending: the six cards not taken are still in the library
		{"std-tdm", "dig_aura_enters_attached_to_its_one_host"},                         // United Battlefront puts Wingspan Stride onto the battlefield with Host of the Hereafter its only legal host: it enters enchanting it
		{"std-ecl", "dig_aura_two_hosts_asks_what_it_enchants"},                         // Ajani, Outland Chaperone's ultimate takes Evershrike's Gift with two creatures to enchant: the choice is asked before the Aura moves
		{"std-dsk", "eerie_resolutions_tally_across_trigger_lines"},                     // Victor's second Eerie resolution of the turn, one through an entering enchantment and one through a Room fully unlocked, is the opponent's discard, not another surveil
		{"std-fin", "target_dependent_cost_starting_town_white", full},                  // Fate of the Sun-Cryst aimed at a tapped creature costs {2}{W}: the planner's census counts Starting Town's white while a point of life is not lethal
		{"std-fin", "life_pain_land_beside_filter_pays_double_green", full},             // Summon: Titan's GG from Starting Town and Capital City: a paid-life land is no census gap beside the filter
		{"std-fin", "filter_negsub_five_subtypes_bounces_the_rest"},                     // Summon: Leviathan's first chapter negates five subtypes in one clause: the Leviathan stays, the Faerie returns to its owner's hand
		{"std-blb", "exile_triggered_source_left_battlefield"},                          // Gossip's Talent level 3 with the Colossified Galewind Moose killed by deathtouch as it dealt combat damage: it is in the graveyard, a new object the exile-and-return finds nothing of
		{"std-fra", "loyalty_cost_counters_trigger_put_counters"},                       // Ajani Unrelenting's [+1] beside an Inspired Tethermage: the loyalty counter put as the cost triggers it, with Ajani's own trigger ordered beside it
		{"std-tmt", "sneak_restricted_any_colour_beside_tap_extra"},                     // Leonardo, Big Brother sneaked for {W} from Turtle Lair's restricted any-colour mana with Groundchuck & Dirtbag adding {G} beside it: the lowered payment takes the wheel's any-colour pick
		{"std-tmt", "restricted_any_colour_pick_beside_plain_colour", full},             // Mikey & Leo paid {W}{W} from a Plains and a second land, Turtle Lair tapped for green beside the Plains' white: the green it leaves floating is restricted, so Cowabunga is not offered
		{"std-tmt", "filter_sacrifices_its_source_to_pay_a_spell", full},                // Omni-Cheese Pizza's {1}, {T}, sacrifice filter makes the {B} of a Squirrelanoids (a Plains funds its {1}): the Pizza leaves the battlefield as the activation is paid, a Super Shredder in play takes its counter (CR 602.2b, 605.1a)
		{"std-trc", "sulu_stations_a_spacecraft_with_tripled_power"},                    // Sulu, Heroic Helmsman stations U.S.S. Prometheus, Prototype with his power tripled: six charge counters (CR 702.184a)
		{"std-trc", "julian_graveyard_instant_stays_put"},                               // Julian Bashir's trigger aimed at an instant in the graveyard: it stays there (CR 304.4), so Vindicate finds no such permanent
		{"std-trc", "reg_barclay_face_down_copy_keeps_exceptions"},                      // Reg Barclay copies a face-down creature: the token is a 2/2 face-down copy that is still an Illusion artifact with haste
		{"std-trc", "station_trigger_lists_with_static_grants", full},                   // U.S.S. Prometheus at station 8 beside an older Supplementation Node: the Node's granted counter trigger lists first, as gorge orders the statics' grants
		{"std-trc", "uhura_commander_identity_mana_pays_nothing"},                       // Uhura's "you may pay {1}" with only Arcane Signet and Queen's Diamond untapped and no commander: neither funds it, so only the decline is offered
		{"std-trc", "uhura_commander_only_mana_pays_nothing"},                           // Uhura's "you may pay {1}" with Queen's Diamond (mana spent only to cast your commander) as the only untapped mana that is not a zero-counter or commander-identity source: its commander-only mana cannot pay a trigger's cost (CR 106.6), so only the decline is offered
		{"std-trk", "trigger_cost_window_skips_restricted_source"},                      // Marooned's "you may pay {3}" with Isolinear Processor untapped: its artifact-only mana cannot pay the Aura's trigger, so the window does not tap it
		{"std-trk", "blastoneuron_does_not_see_itself_leave_the_graveyard"},             // Blastoneuron returned from the graveyard by U.S.S. Reliant: its leave-your-graveyard watcher looks back to a board without it (CR 603.10a), so only the enters trigger is put on the stack
		{"std-trk", "single_search_tapped_land_skips_enters_untapped"},                  // Genesis Device's second ability sacrifices one Island and puts Q Continuum onto the battlefield tapped under Syndicate Liquidators: the single card enters as a batch too, so its enters-untapped charm is not put on the stack beside the landfall trigger (CR 603.6a, 110.5b)
		{"std-trk", "genesis_device_tapped_land_skips_enters_untapped"},                 // Genesis Device puts Quark's Bar and four others onto the battlefield tapped: the land entered tapped, so its enters-untapped charm is not put on the stack (CR 603.6a, 110.5b)
		{"std-trk", "sweep_stops_at_shock_land_election"},                               // Deep Space 9 puts Breeding Pool, Plains and Orion onto the battlefield: the sweep stops at the pay-life election and the cards after it stay exiled until it is answered (CR 614.1c, 701.23)
		{"std-trk", "maco_trigger_graveyard_target_is_no_creature_you_control", full},   // Test of Courage exiling a Horta from a graveyard: MACO Strike Team watches creatures you control becoming the target, and a creature card in a graveyard is not one (CR 109.4, 110.2), so no Officer token is made
		{"std-trc", "arex_up_to_three_lands_ends_when_none_left"},                       // Lt. Arex's "up to three" land pick with one land in hand: the pick ends after it, no lone Done posed beside the kernel's priority
		{"std-trc", "delayed_remembered_creature_bottomed_then_draw"},                   // U.S.S. Farragut puts a creature card from its owner's hand onto the battlefield, and at the next end step the delayed trigger over that one creature puts it on the bottom of its owner's library and the reflexive draw follows (CR 603.7, 603.12)
		{"std-trc", "sweep_counts_destroyed_token_copy"},                                // Join the Collective destroys nine creatures, one a token copy of Giant Space Amoeba: the copy is still in the graveyard until the next check (CR 111.7, 707.10a), so its nonartifact count makes eight Borg tokens, not seven
		{"std-trc", "guardian_returns_card_not_found_after_graveyard_exile"},            // The Guardian of Forever sacrifices a Starfleet Crew and Stonespeaker Crystal exiles the graveyard before the end step: the card is a new object the delayed return cannot find, so it stays in exile (CR 400.7)
		{"std-trc", "doubled_colourless_tap_takes_the_plain_ability", full},             // Painful Truths paid with Spire of Industry's {C} doubled by Forsaken Monument: an any-colour ability makes no {C} (CR 105.1), so the lowering taps the plain ability and the cast is made, as the kernel makes it
		{"std-trc", "phased_out_face_down_permanent_stays_a_2_2"},                       // U.S.S. Defiant phases out a face-down Lightning Greaves: it stays face down (CR 110.5, 708.2), a nameless 2/2 creature, not its printed Equipment
		{"std-trc", "station_counters_skip_a_phased_out_spacecraft", full},              // Queen's Diamond's station ability resolves after U.S.S. Defiant phased it out: it can't be affected (CR 702.26b), so it gets no charge counters
		{"std-spm", "unrelaxable_census_searches_play_wide_source_cannot_fund", full},   // Doc Ock's Tentacles' equip {5} from three lands and two Supportive Parents activations, Interdimensional Web Watch untapped: Parents' cost is no once-only one, so gorge's census is unrelaxable, no relaxed proof drops the play the Web Watch's exile-cast mana cannot fund, and the prefix search pays it
		{"std-spm", "target_menu_keeps_candidates_an_exile_cast_source_may_fund", full}, // Grow Extra Arms cast from exile with only {G} floating, Interdimensional Web Watch untapped: gorge's untapped-source gate reads its exile-cast mana, so the menu keeps Gallant Citizen, and the cast window taps the Web Watch for the unreduced {1} (CR 106.6, 601.2c, 601.2g)
		{"std-spm", "prefix_trigger_if_you_do_cost_answers_yes_first", full},            // Molten Man's prefix taps Spider-Gwen, Free Spirit for Supportive Parents' white: her loot resolves before the cast, and gorge's search answers its "you may discard" election yes first, as gorge lists it, so a card is discarded and one drawn
		{"custom", "sunken_citadel_activation_only_pair"},                               // Sunken Citadel's chosen {C}{C}, spendable only to activate abilities of land sources, pays Safe Haven's {2} activation: the kernel plans the activation-only larger alternative as gorge does (CR 106.6, 601.2g-h, 602.2b)
		{"custom", "steelswarm_operator_two_restricted_abilities"},                      // Steelswarm Operator's artifact-spell {U} pays Wayfarer's Bauble and its artifact-activation {U}{U} the Bauble's {2} activation: the kernel keeps both restricted abilities and plans the half the payment's class admits (CR 106.6, 601.2g-h, 602.2b)
		{"custom", "exile_this_and_two_others_from_graveyard"},                          // Say Its Name exiles itself unasked, then two other Say Its Names the player picks, from its graveyard; the search elects its zones (library, graveyard, hand) and finds Altanak (CR 118.3, 602.2b)
		{"custom", "mana_flare_dual_land_taps_for_one_more_of_its_pick"},                // Mana Flare beside Rugged Highlands: a land making R or G taps for one more mana of the colour it picked, to whoever tapped it, and gorge folds it into each production (CR 106.12a, 605.1b)
		{"custom", "graveyard_hand_search_for_one_named_card"},                          // Dark Supplicant sacrificing three Clerics searches its graveyard, hand and library for a card named Scion of Darkness (none in the deck): the zone election of the Graveyard,Hand pair, then nothing found (CR 701.23)
		{"custom", "tarnation_vista_chosen_mana_of_other_vistas_pays_the_fee", full},    // Four Tarnation Vistas beside a Porcelain Legionnaire: gorge's search taps three for their chosen {R} (each asks which ability, the chosen {T} listed first) and the fourth's each-colour {W}, its {1} paid from the floating {R}, pays Bartolomé del Presidio; the kernel's each-colour search plays the same prefix (CR 106.1, 605.1a)
		{"custom", "tarnation_vista_each_colour_paid_from_floating_mana", full},         // One Tarnation Vista with a Dark Ritual's {B}{B}{B} floating: the activation asks which ability only because the pool holds the fee, and the each-colour {W} pays Bartolomé del Presidio (CR 106.1, 605.1a)
		{"custom", "evendo_filter_pays_a_spell_with_a_green_fee", full},                 // Evendo, Waking Haven at 12 charge counters with {G} floating from Crumbling Vestige: gorge lists its Station-granted "{G}, {T}: Add {G} for each creature" once the pool pays the pip, and the window activates it for seven
		{"custom", "uthros_filter_pays_a_spell_with_a_blue_fee", full},                  // Uthros, Titanic Godcore the same with {U} floating: its granted "{U}, {T}: Add {U} for each artifact" makes four
		{"std-spm", "tap_two_trigger_stacks_before_the_play_is_priced", full},           // Kraven, Proud Predator not offered: the filter search's first Supportive Parents answer taps Spider-Gwen, Free Spirit, her loot trigger is stacked and asked, and the sorcery-speed cast waits for it (CR 307.1, 603.3), so gorge's search spends its 160 nodes without a payer node for Kraven
		{"std-spm", "tap_two_answer_skips_the_plays_own_tap_source", full},              // Guy in the Chair's Web Support, {2}{G},{T}, from an Island, a Forest and Supportive Parents' white: the Parents' "choose 2" answers that tap Guy are skipped, as gorge skips them (CR 602.2b), so the search reaches Lurking Lizards and Mechanical Mobster within its budget
		{"std-spm", "target_menu_keeps_candidates_a_tap_two_source_may_fund", full},     // Grow Extra Arms short of its unreduced {1}{G} with Supportive Parents live: the window census prices no tapXType cost, so the menu keeps every creature, not only the Spider the reduction needs (CR 601.2c, 601.2g)
		{"std-spm", "cast_window_taps_two_creatures_for_supportive_parents", full},      // Grow Extra Arms aimed at a non-Spider, its {1} from Supportive Parents in the cast window: gorge asks for the two creatures to tap, a pick at a time, and the cast is made
		{"std-spm", "two_pay_life_statics_one_alternative_each", full},                  // Gwenom, Remorseless's play-from-the-top Effect twice (Peter Parker's Camera copies the attack trigger): gorge lists one pay-life cast per Effect, AltCostIndex 1 and 2, and the kernel offers one each instead of stalling (CR 118.9)
		{"std-dft", "run_over_second_target_after_vehicle", full},                       // Run Over aimed first at a crewed Vehicle costs {G}: its second target, an opponent's creature, is still asked
		{"std-msh", "returning_aura_chosen_by_its_owner"},                               // Wiccan, Rising Magician returns Super Speed at the end step under its owner's control: the owner chooses what it enchants
		{"std-tmt", "exile_any_grave_stolen_creature_dies"},                             // a stolen Paramecia Coloniex dies into its owner's graveyard: its controller may still exile it from there
		{"std-hob", "stir_up_trouble_game_ending_treasure_sacrifice"},                   // Stir Up Trouble with the one Treasure for its {B} beside The Sackville-Bagginses, whose drain ends the game: one route offers the cast
		{"std-hob", "target_gated_equip_reduction_autopay"},                             // Dwarven Mauler's "equip abilities that target this creature cost {2} less" is priced by the autopay planner too: Orcrist's {3} equip is offered from two untapped lands at {1} (CR 601.2f, 602.2b)
		{"custom", "fallers_faithful_destroyed_undamaged_target_draws_two"},             // Faller's Faithful destroys an undamaged Grizzly Bears: the record the graveyard card keeps is empty, so its controller draws two (CR 400.7)
		{"custom", "soulbright_seeker_third_resolution_adds_mana"},                      // Soulbright Seeker's activated pump resolving a third time in one turn adds {R}{R}{R}{R}: the once-per-turn records count an activated ability's resolutions as a trigger's
		{"custom", "time_spiral_untap_pick_stack_differs"},                              // Time Spiral exiles itself at its Origin$ Stack ChangeZone, then asks the untap-up-to-six-lands pick: the spell is in exile and off the stack at the pick, as gorge has it (CR 608.2c, 608.2n)
		{"std-eoe", "grant_on_tapx_source_keeps_its_own_gap_play"},                      // Mm'menon's spell-only grant sits on Gene Pollinator, whose own tapXType ability is the census gap that pays Lander Token's {2}: the kernel's grant search misses the tap-another gap and drops the activation gorge's prefix search prices (CR 605.1a, 601.2g)
		{"std-mkm", "bite_down_evidence_collected_cast_reduces_cost", full},             // Bite Down on Crime cast collecting evidence costs {2} less (CR 601.2b, 601.2f), at the offer and at the charge
		{"std-trc", "return_to_dust_two_targets_in_own_main"},                           // Return to Dust cast in its controller's main phase takes up to two targets (CR 601.2c)
		{"std-trc", "return_to_dust_one_target_outside_own_main"},                       // Return to Dust cast outside its controller's main phase takes one target
		{"std-mkm", "ward_evidence_payment_triggers_collect_evidence"},                  // a ward {Collect evidence} paid with Cryptic Coat's graveyard beside an Evidence Examiner: the payment is a collect-evidence action, so the Examiner investigates
		{"std-blb", "clone_of_lord_dies_its_statics_end", full},                         // a Mockingbird that entered as Ygra dies in combat with it: Ygra is no Food artifact once the copy's static is gone (CR 613.1d)
		{"std-blb", "witness_restricted_source_pays_the_printed_pip"},                   // Mockingbird cast with an any-type rider and Lilypad Village the only blue source: its creature-only {U} pays the printed pip, so the witness taps it for blue instead of pricing the pip as generic (CR 106.6)
		{"std-blb", "relaxed_restricted_grant_proof_counts_filters"},                    // Lilypad Village's {U} surveil with Clement's creature-only grant and Three Tree Mascot's {1} filter: gorge's relaxed proof counts the filter, so the activation is searched and offered (CR 106.6, 605.3a)
		{"std-blb", "grant_search_budget_spent_before_filter_funds_activation"},         // Rockface Village's {R} pump beside Clement and Three Tree Scribe's creature-only grants and Hidden Grotto: gorge's one prefix search spends its 160 nodes on the grants and offers nothing (CR 605.3a)
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
		{"std-otj", "hollow_marauder_draws_for_a_small_discard"},                        // Hollow Marauder's enters trigger against an opponent holding lands: the opponent discards one, the draw count is the one targeted opponent, and both engines draw
		{"std-otj", "hollow_marauder_no_draw_for_a_big_discard"},                        // the same against an opponent whose discard has mana value 4 or more: the count less that discard is zero, and neither engine draws
		{"std-tmt", "madame_null_pays_life_for_counters"},                               // Madame Null, Power Broker pays life equal to an entering creature's power for that many +1/+1 counters on it: both engines offer the payment and agree on life and counters
		{"std-tdm", "reverberating_summons_discard_hand"},                               // Reverberating Summons activates with six cards in hand: the whole hand is discarded in hand order, then the enchantment is sacrificed, and both engines agree through the resolution
		{"std-tdm", "omen_face_census_credits_granted_delve"},                           // Skimming Strike, Dirgur Island Dragon's Omen, offered off one floating {U} with Teval's granted delve paying its {1} (CR 702.66a, 720.3a)
		{"std-tmt", "graveyard_return_target_and_same_named_cards_tapped"},              // Rat King, Verminister's return resolves: its graveyard target and every other card there with its name enter tapped, both engines agreeing on every one
		{"std-msh", "ward_poison_counters_pay_and_decline"},                             // The Serpent Society targeted twice by the opponent: one ward election pays its five poison counters (the payer gets them), the other declines and counters the spell, both engines agreeing on every one (CR 702.21a, 122.1f)
		{"std-msh", "unless_draw_x_paid_opponent_draws"},                                // Armor Wars' chapter I with artifacts out: the controller pays by drawing a card for each, then each opponent draws one, both engines agreeing on the count and the order
		{"std-msh", "unless_draw_x_declined_and_zero_x_decline_only"},                   // Armor Wars' chapter I twice: once declined with artifacts out, once with none, where the draw is offered as a decline alone (an X of zero is no payment, CR 118.3)
		{"std-tla", "unless_damage_you_x_source_power_pay_and_decline"},                 // Combustion Man's attack trigger: the target's controller either takes damage equal to his power or the permanent is destroyed, both engines agreeing on each answer (CR 118.12, 120.3)
		{"std-tmt", "casey_jones_next_upkeep_discards_three_at_random"},                 // Casey Jones, Vigilante's enters trigger draws three and registers a delayed program: at its controller's next upkeep three cards are discarded at random (CR 603.7)
		{"std-tmt", "turtles_forever_library_pick_chosen_by_opponent"},                  // Turtles Forever finds four legendary creatures of different names in the library (no sideboard in the pool's games), an opponent chooses two for the hand, and the other two are shuffled in (CR 701.23, 400.11)
		{"std-tmt", "north_wind_avatar_wish_finds_empty_sideboard"},                     // North Wind Avatar's cast trigger puts a card from outside the game into the hand: the pool's games carry no sideboard, so the search finds nothing and shuffles nothing (CR 400.11)
		{"std-tmt", "cloning_shredder_copies_the_card_exiled_with_it"},                  // The Cloning of Shredder's chapter I exiles a creature card and copies it from Remembered; its chapter II copies the same card through the exiled-with link (CopyPermanent Defined$ ExiledWith), so the game reaches two token copies
		{"std-woe", "feral_encounter_next_combat_power_damage"},                         // Feral Encounter's delayed program fires at the next beginning of combat this turn: a creature you control is targeted, the optional second target declined (CR 603.7, 115.1)
		{"std-otj", "fortune_end_combat_exiles_itself_and_its_saddler"},                 // Fortune, Loyal Steed attacks saddled: at end of combat the player picks the creature that saddled it, and Fortune and that creature are exiled and returned together, both engines agreeing on the pick and the board (CR 603.7, 400.7)
		{"std-otj", "calamity_copies_a_saddler_tapped_and_attacking"},                   // Calamity, Galloping Inferno attacks saddled by several nonlegendary creatures: the controller picks one twice, each copy enters tapped and attacking and is sacrificed at the end step (CR 603.7, 508.4, 707.2)
		{"std-otj", "calamity_no_nonlegendary_saddler_still_schedules_the_sacrifice"},   // the same with only a legendary saddler: nothing can be chosen, no copy is made, and both delayed sacrifices are still put on the stack at the end step and do nothing (CR 603.7)
		{"custom", "calix_copies_a_nonlegendary_enchantment"},                           // Calix, Guided by Fate connects with an enchantment out: the controller picks which nonlegendary enchantment to copy and both engines agree on the offered pick and the token (CR 707.2, 603.5)
		{"custom", "good_king_mog_copies_a_moogle_token"},                               // Summon: Good King Mog XII's chapter effect on a noncreature cast: the controller picks one of the non-Saga tokens to copy, both engines agreeing on the offered pick and the copy (CR 707.2)
		{"std-fin", "firion_copy_equip_free_then_sacrificed_at_upkeep"},                 // Firion, Wild Rose Warrior copies an Excalibur II: the token's {3} equip costs {2} less, its sacrifice waits for the next upkeep, and both engines agree on the offered equip, the price and the sacrifice (CR 603.7, 707.9b)
		{"custom", "graveyard_return_target_and_same_named_cards_to_hand"},              // Echoing Return resolves: its graveyard target and every other card there with its name go to the hand, both engines agreeing on every one; played on its own decks' table because the tail-53 pool no longer holds Teetering Peaks
		{"std-trc", "effect_trigger_damage_to_you_fires_for_the_owner"},                 // Right of Vengeance cast on the opponent's turn: the until-end-of-turn trigger fires for a creature's combat damage to its owner only
		{"std-dft", "effect_trigger_captures_the_parent_target"},                        // Captain Howler's discard trigger pumps a creature and the Effect link remembers that target: only its combat damage to a player draws
		{"custom", "return_animate_before_entry_replacement", full},                     // Good-Fortune Unicorn returned by Valkyrie's Call beside Giada, Font of Hope: the return's "it's an Angel" applies before the entry replacements read it, so Giada's extra counters count (CR 614.12)
		{"custom", "printed_delve_gurmag_angler", full},                                 // Gurmag Angler cast off Priest of Gix's floating black with two graveyard cards delved: the printed keyword pays through the kernel's existing delve ask (CR 702.66a)
		{"custom", "printed_delve_murderous_cut", full},                                 // Murderous Cut cast off Priest of Gix's floating black with graveyard cards delved: the same printed-keyword path
		{"custom", "cmd5_ancient_tomb_two_colourless_fund_a_spell", full},               // kfuzz cmd5 heuristic pair 10: Ancient Tomb's one activation adds {C}{C} (a pain-only source, 2 damage), so beside two Mountains it funds Archwing Dragon's {2}{R}{R} and the cast is offered (CR 106.1, 605.1a); played on its own decks' table because the cmd5 pool no longer holds the card
		{"std-fra", "rise_of_the_deathbringer_remembers_drawn"},                         // Rise of the Deathbringer cast in its Draw mode: it draws three, and the chained lose-life reads the remembered set (life 1, not 4, when the write is lost)
		{"std-sos", "transcendent_archaic_optional_draw_remembers", full},               // Transcendent Archaic's optional Converge draw accepted off Forests: it draws one, and the chained discard of two runs on the remembered condition
		{"std-sos", "restricted_fixed_colour_pair_unit_pays_two_mana"},                  // Tablet of Discovery's spell-only {R}{R} beside its {R} pays an Improvisation Capstone: the kernel plans the two-mana alternative as gorge does, not two Mountains (CR 106.6, 601.2g-h)
		{"custom", "spider_slayer_destroys_blocking_spider"},                            // a Spider-Slayer that deals combat damage to a blocking Giant Spider destroys it: the damaged creature is the DamageDone trigger's object, so the destroy is not a no-op
		{"custom", "repeateach_antics_roles_each_opposing_creature", full},              // Asinine Antics against Grizzly Bears: a Cursed Role is attached to each creature the opponent controls in one resolution (RepeatEach RepeatCards$ Creature.OppCtrl, ChangeZoneTable$)
		{"custom", "repeateach_witch_roles_each_rat", full},                             // Twisted Sewer-Witch beside a Muck Rats: the Rat token it makes and the Rat already out each get a Wicked Role (RepeatCards$ Creature.Rat+YouCtrl)
		{"custom", "repeateach_brutes_roles_each_target", full},                         // Become Brutes aimed at two creatures: each gains haste and gets a Monster Role (RepeatEach DefinedCards$ Targeted)
		{"custom", "radioactive_spider_searches_a_spider_hero"},                         // Radioactive Spider sacrificed for its search: only the Lady Spider and the Iron Spider (Spider Heroes) are offered, not the Radioactive Spiders still in the library
		{"custom", "lyra_effect_trigger_draws_on_combat_damage"},                        // Lyra's activated Effect, then she connects: the until-end-of-turn trigger draws two for her own combat damage, so the end-step trigger makes an Angel
		{"custom", "anti_venom_burn_becomes_counters"},                                  // Lightning Strike aimed at Anti-Venom, Horrifying Healer: the damage is prevented and he gets three +1/+1 counters instead (CR 615.1, 614.1a)
		{"custom", "wolverine_damage_heals_what_was_marked"},                            // damage to a damaged Wolverine, Fierce Fighter: what was already marked on him is healed, then the new damage is dealt (CR 614.1a)
		{"custom", "wolverine_double_blocked_heals_first_hit"},                          // Wolverine, Fierce Fighter double-blocked: each blocker's damage event heals the one before it, so only the last blocker's damage stays marked on him, and every event is still dealt (CR 614.1a, 510.2)
		{"custom", "wolverine_double_blocked_last_declared_survives"},                   // Wolverine, Fierce Fighter double-blocked by a Grizzly Bears and a Typhoid Rats declared against slot order: the blocker declared last is the event whose damage and deathtouch mark stay (CR 614.1a, 510.2)
		{"custom", "mkm_expedited_inheritance_past_16_permissions"},                     // kfuzz std-mkm heuristic pair 88: Expedited Inheritance exiles the cards of a long run of damaged deaths and each takes a play-from-exile permission, 40 live at the peak: the cold buffer holds the records past the 16 in the State, so the game plays on with no capacity stall
		{"custom", "cosmium_confluence_destroys_two_enchantments"},                      // Cosmium Confluence chooses its destroy mode twice and its search mode once: the two enchantments are the two instances' targets, the groups asked in printed order (CR 700.2d, 601.2c)
		{"custom", "moment_of_reckoning_destroys_twice"},                                // Moment of Reckoning chooses its destroy mode twice: each instance destroys its own target (CR 700.2d)
		{"custom", "cost_static_attacking_you_reads_the_caster"},                        // Red Alert! exiled from seat 1 by Vulcan Mind-Meld and cast by seat 0 while seat 0 attacks seat 1: the "creature attacking you" discount reads the caster, so one Plains cannot pay its {2}{R} (CR 601.2a, 601.2f)
		{"custom", "season_of_loss_sacrifices_three_times"},                             // Season of Loss chooses its one-pawprint mode three times and its two-pawprint mode once: five pawprints, nothing left to add (CR 700.2i, 107.18)
		{"custom", "put_counter_choice_remembers_the_recipient_season_of_gathering"},    // Season of Gathering puts three +1/+1 counters on a creature you control, each pick remembered so it gains vigilance and trample until end of turn, then destroys every artifact or enchantment (CR 700.2i, 122.1)
		{"custom", "put_counter_choice_remembers_the_recipient_kaylas_command"},         // Kayla's Command chooses the token and counter modes: the +1/+1 counter goes on a creature you control, which is remembered and gains double strike until end of turn (CR 700.2, 122.1)
		{"custom", "search_remembers_the_searcher_prishes_wanderings"},                  // Prishe's Wanderings searches for a basic land and puts it onto the battlefield tapped: the search is remembered, so the +1/+1 counter trigger it gates goes on a creature you control (CR 701.23b, 603.12)
		{"custom", "pump_all_remembers_the_pumped_day_of_black_sun", full},              // Day of Black Sun with X 0 remembers every creature of mana value 0 it sweeps: those lose their abilities and are destroyed, then the set is cleared
		{"custom", "destroy_remembers_lki_break_the_spell"},                             // Break the Spell destroys its caster's own enchantment: the destroyed card is remembered and was a permanent they controlled, so they draw
		{"custom", "destroy_remembers_lki_gloomwidows_feast"},                           // Gloomwidow's Feast destroys a blue flyer: the destroyed card is remembered and is blue, so the Spider token is made
		{"custom", "return_remembers_the_explorer_defossilize"},                         // Defossilize returns a Memnite killed by Disfigure: the returned card is remembered and explores twice (CR 701.44a, 400.7)
		{"custom", "return_remembers_lki_mana_value_reanimate"},                         // Reanimate returns a Walking Corpse killed by Disfigure: the returned card is remembered and its controller loses two life, its mana value
		{"custom", "prepared_face_copy_remembers_lki_mana_value"},                       // a prepared Grave Researcher casts its Reanimate copy on a Walking Corpse: the same remembered mana value is the life lost
		{"custom", "cmd5_commander_bounce_waits_for_the_command_zone_choice", full},     // kfuzz cmd5 heuristic pair 3: Boomerang aimed at Lathliss, Dragon Queen, a commander: the owner's command-zone choice is posed while Boomerang is still on the stack, as the bounce instruction would move her, and Boomerang reaches the graveyard only after the answer (CR 608.2n, 903.9b)
		{"custom", "webslinged_enter_spiders_man_tokens", full},                         // Spiders-Man, Heroic Horde cast for its web-slinging cost off two Hulking Raptors' and a Burning-Tree Emissary's floating mana, a crewed Smuggler's Copter's pilot returned: its enters trigger gains 3 life and makes two Spider tokens (CR 702.188, 603.4); a plain cast would not
		{"custom", "cmd5_token_ids_reused_after_cease", full},                           // kfuzz cmd5 heuristic pair 3: Keen Engineering's thopters and the goblins make the game's 25th token with both 100-card decks holding 200 object ids: the new token takes the id of one that ceased to exist in an earlier turn, on both sides (CR 111.7, 704.5d); TestCmd5TokenIdsReusedNoStall checks it does not stall
		{"custom", "cmd5_army_of_the_damned_past_224_objects", full},                    // kfuzz cmd5 heuristic pair 43: Wretched Ranks' Army of the Damned and Endless Ranks of the Dead make more than 224 objects, past the old target-byte bound
		{"custom", "behold_resolution_cost_untaps_the_searched_land"},                   // Elven Passage's search finds a basic land and its unless-switched behold of an Elf pays with one of three candidates (battlefield Elves then hand Elves): nothing moves and the land untaps; both engines agree through the pick and past it (CR 701.4a)
		{"custom", "behold_trigger_cost_makes_a_treasure"},                              // Sarkhan, Dragon Ascendant's enters trigger offers its own behold cost of a Dragon: paying with one of two candidates makes the Treasure, and both engines agree through the pick and past it (CR 701.4a)
		{"custom", "end_blaze_chosen_card_played_next_turn", full},                      // End-Blaze Epiphany kills a Grizzly Bears, exiles that many cards and lets its controller play the chosen one on their next turn (the may-play of the chosen card to the end of the next turn)
		{"custom", "wondrous_wasp_target_loses_abilities"},                              // The Wondrous Wasp enters and taps a Llanowar Elves, which loses all abilities for as long as the Wasp stays on the battlefield (a host-bound loss of abilities)
		{"custom", "impractical_joke_damages_a_shielded_creature"},                      // Impractical Joke deals 3 to a Swooping Protector: damage can't be prevented this turn, so its SHIELD counter stays and the damage is dealt (CR 615.12, 122.1c)
		{"custom", "a_ready_to_rumble_damages_a_shielded_creature"},                     // A-Ready to Rumble's first mode deals 5 to a Swooping Protector past its SHIELD counter (CR 615.12)
		{"custom", "pyrewood_gearhulk_enters_damage_cant_be_prevented"},                 // Pyrewood Gearhulk enters: its trigger pumps the other creatures and makes damage unpreventable for the turn (an end-of-turn emblem)
		{"custom", "flaring_pain_resolves"},                                             // Flaring Pain resolves: damage can't be prevented this turn
		{"custom", "leyline_of_punishment_on_the_battlefield"},                          // Leyline of Punishment's printed statics: players can't gain life, damage can't be prevented
		{"custom", "frenzied_baloth_combat_damage_to_a_shielded_creature"},              // Frenzied Baloth's combat damage to a blocking Swooping Protector: combat damage can't be prevented, so its SHIELD counter stays and the damage is dealt (CR 615.12, 122.1c)
		{"custom", "cmd5_commander_death_choice_precedes_token_cease", full},            // kfuzz cmd5 heuristic pair 31: Giada, Font of Hope, a commander, blocks a Zombie token and both die in combat: the command-zone choice is posed before the check's other actions, so the token is still in the graveyard at the ask and ceases after the answer (CR 704.3, 704.6d, 903.9a)
		{"custom", "cmd5_hidden_pick_commander_to_command_zone", full},                  // kfuzz cmd5 heuristic pair 110: Temur Sabertooth returns Ghalta, Primal Hunger, a commander, and the owner sends it to the command zone instead: it leaves the battlefield, Rancor goes to the graveyard and its trigger is stacked (CR 903.9b)
		{"custom", "cmd5_hidden_pick_commander_to_hand", full},                          // kfuzz cmd5 heuristic pair 92: the same return answered to let it go: Ghalta reaches its owner's hand and the resolution goes on (CR 903.9b)
		{"custom", "cmd5_x_witness_eats_only_target_at_payable_x", full},                // kfuzz cmd5 heuristic pair 646: Lotus Petal alone pays Repeal at X=0 and is the only mana-value-0 permanent: the witness eats the only target the payable X allows, so the cast is not offered (gorge witness_unreachable; CR 601.2c, 601.2g)
		{"custom", "belligerent_attack_plays_top_of_library"},                           // The Belligerent crewed and attacking: its trigger lets p0 look at and play the top card of their library for the turn, lands and spells, and the top card changes as each is played
		{"custom", "gwenom_attack_pays_life_for_top_of_library"},                        // Gwenom, Remorseless attacking: a spell cast from the top of the library is offered at the alternative cost only and pays life equal to its mana value in place of its mana cost (CR 118.9)
		{"custom", "flashback_hybrid_pips_and_discard"},                                 // Twinned Vision flashed back for {1}{U/R}{U/R} and a discard, thirteen times: each hybrid pip's colour is announced like a printed cost's, then the discard is picked (CR 107.4e, 601.2b, 702.34a)
		{"custom", "sneak_hybrid_pip_announced"},                                        // Foot Ninjas sneaked for {3}{W/B}: the hybrid pip's colour is announced like a printed cost's (CR 107.4e, 601.2b, 702.190a)
		{"custom", "dsk_kaito_fifth_emblem"},                                            // kfuzz std-dsk heuristic pair 587: Kaito, Bane of Nightmares' +1 makes an emblem each turn he survives, five or more in all: the kernel holds them and plays on, as gorge does
		{"std-woe", "restricted_each_source_no_colour_ask_beside_filter"},               // Troyan, Gutsy Explorer's {G}{U} is one activation with no colour ask: the filter search beside Crystal Grotto charges it one node, so Restless Fortress's animate is still found within the 160-node budget (CR 106.1, 605.3b)
		{"std-woe", "bargain_only_offer_with_restricted_source_in_pool"},                // Johann's Stopgap bargained for {1}{U} off a floating pool is offered whatever restricted sources the pool holds: Troyan in the decks no longer hides the discounted cast (CR 702.166, 601.2f)
		{"custom", "imprint_token_counters_iroh_tea_master_ally"},                       // Iroh, Tea Master gives a permanent away and makes the 1/1 Ally: its +1/+1 counters go on the imprinted token only, and the cleanup clears the imprint (the imprinted set apart from the remembered given-away permanent)
		{"custom", "as_if_haste_sick_creature_activates_its_tap_ability"},               // Shang-Chi, Master of Kung Fu out: a Prodigal Sorcerer pings the turn it entered, summoning sick, and still can't attack (CR 302.6)
		{"custom", "cant_be_suspected_enchanted_creature_suspects_itself"},              // Airtight Alibi on a Repeat Offender: its ETB clears the suspicion, and the Offender's own "otherwise, suspect it" then does nothing (CR 701.60)
		{"custom", "sweep_exiles_the_target_and_same_named_tokens_legions_to_ashes"},    // Legions to Ashes exiles an opponent's Soldier token and every other Soldier token that player controls: the target and the same-named tokens leave together, and both engines agree on every one (CR 608.2b); played on its own decks' table because BRO has no pool
		{"custom", "imprint_exile_return_sirens_ruse"},                                  // Siren's Ruse exiles a creature you control and returns the imprinted card to the battlefield, both engines agreeing through the return and past it
		{"custom", "imprint_token_counters_summoners_sending_spirit"},                   // Summoner's Sending exiles a creature card from a graveyard and makes a Spirit that takes a +1/+1 counter off the imprinted token when the exiled card's mana value is 4 or more
		{"custom", "warp_x_broodguard_elite"},                                           // Broodguard Elite warped for {X}{G} at several X, once with a floating {G} that pays it at X = 0: the X is announced like a printed one, and that cast taps nothing more (CR 601.2b, 702.185a)
		{"custom", "disguise_reduced_turn_up_fugitive_codebreaker"},                     // Fugitive Codebreaker turned face up from Priest of Urabrask's {R}{R}{R} with Lightning Bolts in its owner's graveyard: the disguise cost is reduced by {1} for each (CR 601.2f, 702.168c)
		{"custom", "effect_capture_trigger_flickers_the_dealer"},                        // Long River Lurker's Effect remembers its target: the creature is unblockable, and its combat damage triggers the optional flicker once, for that creature alone (CR 603.7, 611.2, 400.7)
		{"custom", "effect_capture_static_lets_a_defender_attack"},                      // The Pride of Hull Clade targets itself: the Effect lets it attack as though it lacked defender, and its one combat damage trigger draws (CR 702.3b, 611.2)
		{"custom", "scene_second_tapx_beside_granted_any_colour_pays_pride", full},      // Scene of the Crime's tap-another ability and a Plant token Insidious Roots lets tap for any colour pay The Pride of Hull Clade's {2}{U}{U} together, the lands the {2}: gorge offers the activation and so does the kernel (CR 605.3b, 601.2g)
		{"custom", "bilbos_gambit_gift_promised_players_cannot_cast"},                   // Bilbo's Gambit returns a Lightning Bolt to its owner's hand with the gift promised: players can't cast spells this turn, so the other seat's second Bolt is not offered (an end-of-turn emblem)
		{"custom", "calamitys_wake_players_cannot_cast_noncreature_spells"},             // Calamity's Wake resolves while the other seat holds a Lightning Bolt and an untapped Mountain: no noncreature spell can be cast this turn (an end-of-turn emblem)
		{"custom", "fra_fifth_one_colour_source"},                                       // Intrepid Stablemaster beside four other sources making all their mana in one colour: the fifth is priced (colour multisets per amount), not a capacity stall
		{"custom", "tmt_dynamic_mana_past_40"},                                          // Improvised Arsenal's mana amount grows past 40 as its copies and the enchanted creature grow: the autopay variants cover it, not a capacity stall
		{"std-dsk", "twitching_doll_mana_puts_a_nest_counter", full},                    // Twitching Doll taps for a red spell: its mana ability's rider puts a nest counter on it as the mana is added, with no stack and no decision (CR 605.3b)
		{"std-ecl", "lavaleaper_basic_land_taps_for_one_more"},                          // Lavaleaper beside Mountains on both sides: a basic land taps for one more mana of its type, to whoever tapped it, and gorge's planner folds it into the tap (CR 106.12a, 605.1b)
		{"std-otj", "roxanne_meteorite_taps_for_two_of_one_colour"},                     // Roxanne, Starfall Savant beside her Meteorite token on both sides: the token's {T}: Add one mana of any color brings one more mana of the colour it made, and gorge's planner folds it into each colour's alternative (CR 106.12a, 605.1b)
		{"custom", "speed_young_avenger_grants_blockable_only_by_haste"},                // Speed, Young Avenger casts a Lightning Bolt and pays {1}: its reflexive trigger makes a creature with haste blockable only by creatures with haste this turn (the targeted grant with no exile sweep)
		{"std-blb", "granted_creature_only_mana_pays_creature_spells", full},            // Clement, the Worrywort's creature-spell-only {G} or {U} on Frogs: the mana floats restricted and pays creature spells, never a noncreature spell (CR 106.6)
		{"std-trc", "granted_artifact_mana_pays_an_equip_activation"},                   // Ambulatory Reactor's {C} for artifact spells and abilities of artifact sources, tapped through a vehicle first in battlefield order: the activated half of the restriction rides with the mana, so Skullclamp's equip is paid from it (CR 106.6, 602.2)
		{"std-trc", "x_witness_counts_pain_colour_beside_a_grant", full},                // CR 107.3, 601.2f: Grand Crescendo's {X}{W}{W} lowered from Plains, Talisman of Dominance and Spire of Industry with Ambulatory Reactor's grant in the table: the witness counts Spire's paid-life white as gorge's census does, so X = 1 taps all three
		{"custom", "granted_artifact_only_mana_battery_bearer"},                         // Battery Bearer's {C} that can't be spent on a nonartifact spell, granted to creatures: it pays artifact spells and any ability (CR 106.6)
		{"custom", "granted_combo_gu_asks_green_first_thranduil", full},                 // Thranduil the Strategist's {G} or {U} on its Elves: the colour ask lists the card's own order, G then U
		{"custom", "imprint_mill_patient_naturalist_land_to_hand"},                      // Patient Naturalist mills three cards with a land among them: the imprinted land goes to the hand and no Treasure is made, both engines agreeing through the pick and past it
		{"custom", "imprint_mill_patient_naturalist_no_land_makes_a_treasure"},          // Patient Naturalist mills three cards with no land among them: the pick finds nothing and the Treasure is made
		{"custom", "departed_source_counters_after_slot_reuse"},                         // Jadzia Dax, Living Legacy's first-main trigger waits on the stack while Impossible Choice destroys her and its two Officer tokens take her permanent slot: the trigger still adds {G} for her four last counters, read from the departure snapshot (CR 608.2h)
		{"custom", "gogo_copies_ability_x_times", full},                                 // Gogo, Master of Mimicry paying X of two or more off the mana a small Blaze left floating, copying a Prodigal Sorcerer ping that many times: each copy is minted and its new target asked in turn (CR 707.10c); played on its own decks' table because Blaze and Prodigal Sorcerer are in no Standard pool
		{"std-lci", "dynamic_count_second_ability_pair"},                                // Itlimoc, Cradle of the Sun's {T}: Add {G} beside {T}: Add {G} for each creature you control (CR 106.1, 601.2g, 605.3a): gorge's planner takes the larger alternative of the one unit, so the kernel plans the count pair too
		{"std-lci", "count_pair_larger_alternative_beside_mana_grants"},                 // CR 106.1, 601.2g, 605.3a: with a continuous mana grant in the table every source takes the grant census (availGrant), which must still count Itlimoc's larger alternative ({G} per creature): four creatures and four lands pay Tendril of the Mycotyrant's {5}{G}{G}
		{"custom", "cmd5_search_second_pick_swamps_share_a_land_type", full},            // kfuzz cmd5 heuristic pair 42: Myriad Landscape finds a Swamp, then a second Swamp (CR 701.23, 205.3i); the matchup table keeps the basic land types the search compares
		{"custom", "cryoshatter_enchanted_attacker_tapped_is_destroyed"},                // Cryoshatter on a Grizzly Bears that attacks: the attack tap fires its AttachedBy Taps watcher and the Bears is destroyed (TriggeredCardLKICopy)
		{"custom", "cryoshatter_enchanted_vigilant_attacker_stays"},                     // Cryoshatter on a Brushstrider that attacks: vigilance, so it is not tapped and stays
		{"custom", "cryoshatter_on_mono_colour_dork_does_not_fund_a_cast"},              // Cryoshatter on an untapped Llanowar Elves: the tap fires its unvetted Taps watcher, so gorge's census never lists the Elves and its {G} funds no Grizzly Bears on the autopay surface
		{"custom", "cryoshatter_twice_on_dork_opponent_orders_the_triggers", full},      // CR 605.3a, 603.3b: two Cryoshatters on an untapped Llanowar Elves: its tap sets off two of the opponent's triggers, which the opponent orders before the Elves' controller gets priority; the {G} stays in the pool, so the full surface offers the Grizzly Bears
		{"std-lci", "free_cast_unpayable_tap_cost_returns_to_hand"},                     // CR 601.2b, 118.3, 733.1, 701.57a: a discover's free cast of Guardian of the Great Door with one untapped permanent left cannot pay its tap four: the cast is reversed and the card goes to hand
		{"custom", "static_untap_other_own_artifacts_unwinding_clock"},                  // Unwinding Clock untaps the Alpha Myr its controller attacked with during the other player's untap step (CR 502.3): a static over its controller's artifacts, not only itself
		{"custom", "static_can_attack_defender_own_creatures_ghalta"},                   // Ghalta the Immovable lets the Reito Sentinels its controller has attack as though they lacked defender (CR 702.3b)
		{"custom", "static_cant_attack_enchanted_by_own_aura_eriette"},                  // Eriette of the Charmed Apple: an opponent's creature enchanted by an Aura Eriette's controller controls is offered no attack at them (CR 508.1c)
		{"custom", "static_can_attack_defender_modified_guardians_of_oboro"},            // Guardians of Oboro under an Aura is a modified Defender creature: it attacks as though it lacked defender (CR 702.3b)
		{"custom", "static_untap_other_own_creatures_prop_room"},                        // Prop Room untaps the Savannah Lions its controller attacked with during the other player's untap step (CR 502.3)
		{"std-dsk", "graveexile_pick_count_is_the_trigger_amount"},                      // Nashi, Searcher in the Dark deals combat damage: it mills that many cards and takes a legendary or enchantment card from among them to hand, the pick's count the damage dealt
		{"std-tla", "iroh_grand_lotus_grave_flashback"},                                 // Iroh, Grand Lotus on the battlefield: on its controller's turn only, a graveyard Lightning Strike or Pillar Launch has flashback at its mana cost and a Lesson at {1}, and each is exiled after (CR 702.34a)
		{"custom", "pt_compare_hulkling_greater_power_or_toughness"},                    // CR 603.4: Hulkling, Burgeoning Bruiser (2/3) takes a +1/+1 counter when a creature of greater power (3/3) or greater toughness (2/4) enters under its controller, not a 0/3: the trigger filter and the resolution recheck compare against his own power and toughness
		{"custom", "pt_compare_amalia_explores_power_not_twenty"},                       // Amalia Benavides Aguirre explores when her controller gains life, and her power is not exactly 20, so the destroy-all is skipped
		{"custom", "exile_battlefield_cost_fear_of_abduction", full},                    // Fear of Abduction cast paying its additional cost by exiling a creature you control (a pick of the battlefield, not a sacrifice): the exile pick is posed after the mana is paid through the manual window, since gorge's planner declines an exile part (CR 118.3, 601.2h, 406.2); played on its own decks' table
		{"custom", "hand_grave_sweep_echo_of_eons"},                                     // Echo of Eons: each player's hand then graveyard goes into its owner's library (the hands first), both libraries shuffled in the order of the first card moved, then seven cards drawn (CR 701.24, 121.2)
		{"custom", "hand_grave_sweep_timetwister"},                                      // Timetwister: the same Hand,Graveyard sweep over every player
		{"custom", "hand_grave_sweep_time_reversal"},                                    // Time Reversal: the same sweep, then the spell exiles itself
		{"custom", "hand_grave_sweep_days_undoing"},                                     // Day's Undoing: the same sweep, then the turn ends if it is the caster's
		{"custom", "lilah_resolved_instant_is_exiled_and_plotted"},                      // Lilah, Undefeated Slickshot out, a Lightning Helix cast from hand resolves: it is exiled instead of going to the graveyard and becomes plotted (CR 614.1a, 702.170b)
		{"custom", "lilah_countered_instant_goes_to_the_graveyard"},                     // the same Helix countered by Counterspell goes to the graveyard: the replacement is Fizzle$ False, the resolution move alone
		{"custom", "lilah_plotted_instant_cast_again_goes_to_the_graveyard"},            // a plotted Helix cast again from exile on a later turn is not cast from hand, so Lilah leaves it to the graveyard
		{"custom", "nita_exiled_bolt_cast_this_turn_is_exiled_again"},                   // Nita, Forum Conciliator exiles an opponent's Lightning Bolt and it is cast this turn with any mana: the spell is one she does not own, and it is exiled instead of going back to the graveyard
		{"custom", "nita_exiled_bolt_countered_is_exiled_again"},                        // the same recast Bolt countered by Counterspell is exiled too: Nita's replacement names no resolution, so a counter is replaced as well
		{"custom", "ty_lee_target_stays_tapped_through_its_untap_step"},                 // Ty Lee, Chi Blocker taps a Wall of Wood as she enters: it stays tapped through its controller's untap steps while she stays under her controller's control (Duration$ AsLongAsControl, an Untap replacement scoped to the untap step)
		{"custom", "dungeon_geists_target_stays_tapped_through_its_untap_step"},         // Dungeon Geists's enters trigger is the same lock as Ty Lee's, a creature of any controller
		{"custom", "icefall_regent_target_stays_tapped_through_its_untap_step", full},   // Icefall Regent's enters trigger is Dungeon Geists's lock again, recorded on the full surface too; the autopay-surface game is battlefield_self_target_raise_does_not_decline_own_cast
		{"custom", "battlefield_self_target_raise_does_not_decline_own_cast"},           // Icefall Regent's battlefield self-target RaiseCost taxes spells that target the permanent, not the card's own cast from hand: on autopay both engines plan its {3}{U}{U} from the Islands (CR 113.6, 601.2f)
		{"custom", "merchant_raiders_target_stays_tapped_through_its_untap_step"},       // Merchant Raiders's enters trigger taps up to one target creature and locks it the same way
		{"custom", "time_of_ice_target_stays_tapped_through_its_untap_step"},            // Time of Ice's chapter locks the creature it tapped for as long as its controller controls the Saga
		{"custom", "ty_lee_target_untaps_by_an_effect"},                                 // the same Wall untaps when a Vitalize resolves on it: Ty Lee's lock holds only in the untap step
		{"custom", "ty_lee_stolen_by_act_of_treason_frees_the_target"},                  // an Act of Treason takes Ty Lee: the lock ends for good with the control change (CR 611.2b), so the Wall untaps in its next untap step
		{"custom", "ty_lee_dies_to_lightning_bolt_frees_the_target"},                    // Ty Lee dies to a Lightning Bolt: her lock is not live without her, so the Wall untaps in its next untap step
		{"custom", "spider_woman_target_stays_tapped_through_its_untap_step", full},     // Spider-Woman, Secret Agent taps a Wall of Wood as she enters: it can't become untapped for as long as she is on the battlefield under her controller
		{"custom", "spider_woman_target_cannot_be_untapped_by_an_effect", full},         // the same Wall stays tapped through a Vitalize that would untap it: Spider-Woman's lock replaces every untap, not the untap step's alone
		{"std-mkm", "agency_outfitter_grave_hand_election"},                             // Agency Outfitter's OriginAlternative$ Graveyard,Hand pair: both zones are elected and searched, DifferentNames$ keeps a second Magnifying Glass out, and no library was searched so nothing is shuffled
		{"custom", "animate_name_tenth_district_hero_becomes_mileva"},                   // Tenth District Hero's second ability, evidence collected from the Bolts in its graveyard: it becomes a legendary 5/5 named Mileva, the Stalwart, and other creatures its controller has are indestructible (CR 613.1c, 611.3a)
		{"custom", "animate_equip_the_irencrag_becomes_everflame"},                      // The Irencrag's trigger answered yes as a legendary creature enters: it becomes the legendary Equipment Everflame, Heroes' Legacy and loses its {T}: Add {C} (CR 613.1a, 613.1f)
		{"custom", "animate_equip_domesticated_watercourse_equips_a_creature"},          // Domesticated Watercourse's Animate keeps its abilities and grants it equip {2}: gorge offers the equip and the kernel hosts it on the animated permanent itself, which then attaches to a creature (CR 613.1f, 301.5)
		{"custom", "filter_each_skycloud_pays_white_and_blue", full},                    // Skycloud Expanse's {1},{T}: Add {W}{U}, an all-at-once filter and its card's only mana ability: a Plains funds the {1} and the Skycloud adds both colours, no colour ask, so Galina's Knight's {W}{U} is paid (CR 106.1, 605.3b); played on its own decks' table because the frozen std-trc pool predates the lowering
		{"std-ecl", "mutable_explorer_makes_a_tapped_mutavault"},                        // Mutable Explorer enters: its trigger copies the card named Mutavault, a tapped token (CopyPermanent DefinedName$, CR 707.2)
		{"custom", "generated_horizons_copies_a_forest_past_48_sources"},                // Generated Horizons' upkeep copy of the card named Forest; a cast past 48 sources taps the least ObjIDs, as gorge's uncapped census does (CR 707.2, 601.2g-h)
		{"custom", "warren_attack_charm_rabbit_token"},                                  // Warren Warleader attacking with Savannah Lions: its AttackersDeclared charm makes a Rabbit tapped and attacking, the defender the attack trigger's defending player (CR 508.4)
		{"custom", "sneaked_spell_tokens_enter_attacking"},                              // The Last Ronin's Technique cast for its sneak cost in the declare blockers step: its three Spirits enter tapped and attacking, gorge's combat fallback (CR 508.4, 702.190a)
		{"std-msh", "delay_spell_source_bound"},                                         // CR 603.7c: Loki Laufeyson's delayed SpellCast bound is its own power at the cast, re-read after the registration (a three-mana spell misses at power 2; a two-mana one fires), and both engines agree on every one
		{"custom", "delay_spell_source_departs"},                                        // CR 608.2h: Loki Laufeyson pumped to 5 by Titan's Strength registers his delayed SpellCast, then is sacrificed to Goblin Bombardment: the bound reads his last known power, so a Divination (mana value 3) is copied
		{"std-spm", "unless_addcounter_cheering_crowd", full},                           // Cheering Crowd's each-player first-main trigger pays its unless +1/+1 counter, and the body's mana follows the counter
		{"std-mkm", "unless_addcounter_sanctuary_wall", full},                           // Sanctuary Wall's tap ability pays the stun counter on its target, and the body puts the second stun counter on the Wall
		{"custom", "counted_proliferate_planeswalker_types", full},                      // Tam, the Possibility proliferates once per planeswalker type among its controller's planeswalkers (two Ajanis count once, CR 205.3j) and Tromell, Seymour's Butler once per nontoken creature that entered under you this turn: each a fresh proliferate ask, none at zero (CR 701.34a)
		{"custom", "tannuk_steadfast_second_hand_warp_grant", full},                     // Tannuk, Steadfast Second on the battlefield: an artifact or red creature card in hand is warp-cast at {2}{R}, exiled at the next end step and recast from exile on a later turn, the grant read live (CR 702.185a)
		{"std-otj", "lively_dirge_hidden_pick_total_cmc", full},                         // Lively Dirge's Spree return mode takes creature cards whose printed mana values total 4 or less from the graveyard into one hidden pick: the running sum limits the second card to a 1-drop
		{"std-tmt", "michelangelos_technique_dig_total_cmc"},                            // Michelangelo's Technique looks at eight and takes creature cards whose total mana value is 6 or less: the bound narrows the second pick as the first sums in
		{"std-dsk", "rip_spawn_hunter_dig_distinct_powers"},                             // Rip, Spawn Hunter's Dig takes creature and Vehicle cards with pairwise different powers: a second card of a power already taken is not offered
		{"custom", "ao_the_dawn_sky_dig_total_cmc", full},                               // Ao, the Dawn Sky's dies trigger takes nonland permanents with total mana value 4 or less: the bound rides the pick filter's head, not the instruction
		{"std-otj", "mayplay_spree_cast_filter_prefix_search", full},                    // Dance of the Tumbleweeds may-played from exile, its only green a Conduit Pylons filter: gorge prices a may-play cast at its printed cost and searches the filter prefix even though every Spree mode adds mana
		{"std-otj", "lively_dirge_second_pick_over_budget", full},                       // Lively Dirge's return mode after a first creature card: the remaining graveyard creatures whose mana value would carry the total past 4 are no longer offered, so the pick ends
		{"std-tmt", "michelangelos_technique_second_pick_over_budget", full},            // Michelangelo's Technique after its first take: no other creature card in the eight fits the remaining mana-value budget, so only Done is offered
		{"std-tmt", "manhole_missile_hand_to_bottom"},                                   // Manhole Missile's optional hand-to-library move with LibraryPosition$ -1 puts the picked card at the bottom and remembers it, so the gated draw happens (gorge handLibraryTail)
		{"std-dsk", "rip_spawn_hunter_second_pick_repeated_power", full},                // Rip, Spawn Hunter's Dig after a first take: a card whose power was already taken is not offered again, so only Done remains
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

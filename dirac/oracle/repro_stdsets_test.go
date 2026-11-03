package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Games from the Standard sets' synthetic pools that once diverged at the end of
// their script, on the autopay surface (the full one where a case says so): both
// engines now agree there, and for 200 decisions after.
func TestStdSetReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	// the games recorded on the full surface (gorge's own payment asks): the others ran on autopay
	fullSurface := map[string]bool{"calamitous_tide_leave_together": true, "treasure_mana_token_ceases_quietly": true, "cast_window_restricted_mana_short": true, "convoke_target_window_bloom_tender": true, "clone_returned_card_spent_zero_mockingbird": true, "clone_returned_card_spent_zero_two_birds": true, "hybrid_face_restricted_source": true, "target_dependent_cost_starting_town_white": true, "life_pain_land_beside_filter_pays_double_green": true, "run_over_second_target_after_vehicle": true, "restricted_any_colour_pick_beside_plain_colour": true}
	for _, c := range []struct{ pool, name string }{
		{"std-tla", "waterbend_min_taps_filter"},                          // Barrels of Blasting Jelly adds nothing to what the window can pay
		{"std-tla", "granted_cap_restricted_source"},                      // Honest Work beside Jasmine Dragon Tea Shop: the printed sources price Sokka
		{"std-blb", "script_cap_skips_sufficient_plain_casts"},            // Mind Spring and Mockingbird pay from the Islands: no prefix search spent before Pawpatch Recruit's offspring cast
		{"std-fdn", "kicked_cast_granted_mana_prefix"},                    // Into the Roil kicked, UU from New Horizons' Mountain: the prefix prices the composed cost
		{"std-blb", "clone_returned_card_spent_zero_mockingbird"},         // Splash Portal returns Mockingbird, which was not cast: no mana spent, so it cannot copy Brave-Kin Duo
		{"std-blb", "clone_returned_card_spent_zero_two_birds"},           // the same under Finch Formation and Valley Floodcaller: only a creature of mana value 0 is offered
		{"std-ecl", "convoke_target_window_springleaf_drum"},              // Protective Response convoked by two, the last {W} from a Springleaf Drum tapping a third creature: the target menu keeps its candidates
		{"std-ecl", "convoke_target_window_drum_three_picks"},             // Appeal to Eirdu convoked by three Kithkin, the last generic from the Drum
		{"std-ecl", "convoke_target_window_bloom_tender"},                 // Temporal Cleansing with no picks and a Bloom Tender, a source gorge's census cannot price: the cast is not reversed at the target
		{"std-ecl", "wretch_blight_dies"},                                 // a Retched Wretch that came back has no abilities: its second death, with -1/-1 counters again, triggers nothing
		{"std-blb", "parting_gust_return_counter"},                        // Parting Gust returns a creature under its owner's control: the owner puts its counter, so their Stocking the Pantry triggers
		{"std-blb", "calamitous_tide_leave_together"},                     // Calamitous Tide bounces Dour Port-Mage with Plumecreed Mentor: one departure, so the Port-Mage sees the Mentor leave
		{"std-sos", "treasure_mana_token_ceases_quietly"},                 // a Treasure sacrificed for mana ceases to exist in the graveyard: Garrison Excavator's cards-leave-your-graveyard trigger stays quiet
		{"std-tdm", "thunder_of_unity_saga_etb"},                          // a chapter of Thunder of Unity with no other enters watcher out: the creature cast afterwards still drains
		{"std-trk", "beam_grant_lone_grantor"},                            // Scotty the only creature, Open Communications in the graveyard: the grant has no creature to return
		{"std-tdm", "neriv_departed_source_entered"},                      // Stadium Headliner sacrificed the turn it entered: Neriv still doubles
		{"std-blb", "copied_flashback_spell_was_not_cast"},                // Kitsa copies a flashed-back Otterball Antics: the copy's token takes no counter
		{"std-fin", "cast_window_restricted_mana_short"},                  // The Emperor of Palamecia's one mana does not reach Ice Magic's {4}: no target offered, no cast reversed after it
		{"std-blb", "hybrid_face_restricted_source"},                      // a Village's restricted mana is the only source of a hybrid pip's colour: Cindering Cutthroat's pip ask offers it beside the other
		{"std-tdm", "dragonstorm_return_trigger_source_destroyed"},        // Encroaching Dragonstorm destroyed with its return-to-hand trigger on the stack: the trigger finds a new object in the graveyard and moves nothing
		{"std-tdm", "dig_rest_stays_in_library_at_taken_cards_enter_ask"}, // United Battlefront takes Hollowmurk Siege and its choose-a-mode ask is pending: the six cards not taken are still in the library
		{"std-tdm", "dig_aura_enters_attached_to_its_one_host"},           // United Battlefront puts Wingspan Stride onto the battlefield with Host of the Hereafter its only legal host: it enters enchanting it
		{"std-ecl", "dig_aura_two_hosts_asks_what_it_enchants"},           // Ajani, Outland Chaperone's ultimate takes Evershrike's Gift with two creatures to enchant: the choice is asked before the Aura moves
		{"std-dsk", "eerie_resolutions_tally_across_trigger_lines"},       // Victor's second Eerie resolution of the turn, one through an entering enchantment and one through a Room fully unlocked, is the opponent's discard, not another surveil
		{"std-fin", "target_dependent_cost_starting_town_white"},          // Fate of the Sun-Cryst aimed at a tapped creature costs {2}{W}: the planner's census counts Starting Town's white while a point of life is not lethal
		{"std-fin", "life_pain_land_beside_filter_pays_double_green"},     // Summon: Titan's GG from Starting Town and Capital City: a paid-life land is no census gap beside the filter
		{"std-blb", "exile_triggered_source_left_battlefield"},            // Gossip's Talent level 3 with the Colossified Galewind Moose killed by deathtouch as it dealt combat damage: it is in the graveyard, a new object the exile-and-return finds nothing of
		{"std-fra", "loyalty_cost_counters_trigger_put_counters"},         // Ajani Unrelenting's [+1] beside an Inspired Tethermage: the loyalty counter put as the cost triggers it, with Ajani's own trigger ordered beside it
		{"std-tmt", "sneak_restricted_any_colour_beside_tap_extra"},       // Leonardo, Big Brother sneaked for {W} from Turtle Lair's restricted any-colour mana with Groundchuck & Dirtbag adding {G} beside it: the lowered payment takes the wheel's any-colour pick
		{"std-tmt", "restricted_any_colour_pick_beside_plain_colour"},     // Mikey & Leo paid {W}{W} from a Plains and a second land, Turtle Lair tapped for green beside the Plains' white: the green it leaves floating is restricted, so Cowabunga is not offered
		{"std-dft", "run_over_second_target_after_vehicle"},               // Run Over aimed first at a crewed Vehicle costs {G}: its second target, an opponent's creature, is still asked
		{"std-msh", "returning_aura_chosen_by_its_owner"},                 // Wiccan, Rising Magician returns Super Speed at the end step under its owner's control: the owner chooses what it enchants
		{"std-tmt", "exile_any_grave_stolen_creature_dies"},               // a stolen Paramecia Coloniex dies into its owner's graveyard: its controller may still exile it from there
		{"std-hob", "stir_up_trouble_game_ending_treasure_sacrifice"},     // Stir Up Trouble with the one Treasure for its {B} beside The Sackville-Bagginses, whose drain ends the game: one route offers the cast
	} {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
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
			set := cardsettest.Synth(t, c.pool)
			tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.Config.GorgeAutopaySurface = !fullSurface[c.name]
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
		})
	}
}

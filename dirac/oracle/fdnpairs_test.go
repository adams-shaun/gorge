package oracle

import (
	"path/filepath"
	"testing"
)

// fdnRegressPairs are census pairings (seed 1, the kfuzz census's
// -features -autopay-surface configuration) that
// diverge without their fix and play to the end in lockstep with it,
// against the gorge the go.work points at (develop d6e00379f). Each was
// re-verified there by reverting its fix on the current tree. The four
// replacement-card pairs are coverage pairs (the card plays to the end),
// not revert-verified.
var fdnRegressPairs = []struct {
	pair int
	why  string
}{
	{1547, "ward—pay N life with less life: gorge still asks pay/decline (effWard), paying fails, the ward counters (60b5c97)"},
	{4061, "the same, the ward trigger's stack (60b5c97)"},
	{2014, "Etali's free casts never offer a spell whose mandatory targets cannot all be chosen (CR 601.2c); it stays exiled and the trigger finishes (af473c5, kernel followed 2026-10-18)"},
	{4595, "a multi-object slot does not fizzle while any of its objects is legal (CR 608.2b; b3199bf)"},
	{4596, "Krenko tokens past MaxObj: the kernel stalls board instead of recycling a ceased token's id (gorge never reuses ids)"},
	{22, "a plain cast gorge's planner never prices is dropped from the autopay surface when only the potential planner scripts it (71405d1)"},
	{64, "the same (71405d1)"},
	{17950, "an each-player sacrifice is one leave batch across the players: High-Society Hunter (sacrificed to Blasphemous Edict) draws for every other nontoken creature (71de742)"},
	{551, "the same batch, its trigger order (71de742)"},
	{32, "two Garruk's Uprisings grant a hand creature nothing (CR 109.2)"},
	{2514, "Vivien Reid's emblem reaches no hand or graveyard creature (AffectedZone$ Battlefield)"},
	{4121, "a lost-ability anthem (Empyrean Eagle under Imprisoned in the Moon) grants nothing off the battlefield (f894ddf)"},
	{2518, "Banner of Kinship's dynamic +N/+N is read off the battlefield (f894ddf)"},
	{1300, "a token dying with Fake Your Own Death's granted trigger: the stack band is named by the ceased token (gorge abilityName), not the granting card (f894ddf)"},
	{2923, "the same with another token (f894ddf)"},
	{659, "Etali casts the opponent's card: the stack card view is keyed by its caster, not its owner (f894ddf)"},
	{853, "the same (f894ddf)"},
	{26, "the derived type line covers a subtype change: Infernal Vessel's returned Demon (b00f50f)"},
	{42, "the same, Mild-Mannered Librarian become a Werewolf (subtypes set; b00f50f)"},
	{16, "the same, Witness Protection's Citizen on a creature with no card-type change (b00f50f)"},
	{3, "the same, Skyknight Squire a Knight in addition to its other types (b00f50f)"},
	{47, "the same, Kellan become a Detective (b00f50f)"},
	{283, "Twinflame Tyrant doubles the damage its controller's sources deal to the opponent and the opponent's permanents, combat included (coverage)"},
	{1649, "Crystal Barricade prevents the noncombat damage dealt to its controller's other creatures (coverage)"},
	{19608, "Crystal Barricade gives its controller hexproof: the opponent's spells and abilities can't target that player (coverage)"},
	{7774, "Doubling Season doubles the tokens and the counters an effect makes for its controller (coverage)"},
	{29722, "Progenitus put into a graveyard from anywhere goes onto the library top and the library is shuffled (coverage)"},
	{6559, "ward—pay 7 life with 3 life: gorge still asks pay/decline (effWard), paying fails, the ward counters (Sire of Seven Deaths vs Bigfin Bouncer)"},
	{12692, "Etali's free casts offer a spell with no legal target (gorge effects/play.go filters only CastProhibited); casting it is refused, the card stays exiled and the trigger finishes"},
	{51798, "the same with three scripted casts (Burglar Rat, Goldvein Pick, Macabre Waltz)"},
	{210, "two Garruk's Uprisings grant a hand creature nothing (CR 109.2; pair 32)"},
	{275, "Vivien Reid's emblem reaches no hand or graveyard creature (pair 2514)"},
	{708, "a token dying with Fake Your Own Death's granted trigger: the stack band is named by the ceased token (gorge abilityName), not the granting card (develop pairs 1300, 1880, 2923)"},
	{1650, "the same with another token"},
	{1836, "the same with another token"},
	{170, "Secluded Courtyard played as a land: gorge asks its creature type while the land is still in the hand, the land play completing after the answer; later in the game Rogue's Passage's {4} activation taps four basics and not the untapped Courtyard, which gorge ranks as a six-type five-colour source whatever the payment admits (kernel restrPlanPC)"},
	{23266, "Fiend Artisan cast from a graveyard through a permission: gorge lowers the potential play's witness at the cast, then asks each {B/G} pip's colour (pay_B / pay_G)"},
	{56861, "Bloom Tender's each-colour mana is a gap in gorge's planner census: Mild-Mannered Librarian's activation is offered through the scripted prefix that taps Bloom Tender first"},
	{1107, "a ward {2} paid in gorge's mana window with Llanowar Elves and then Bloom Tender (the window lists every mana ability in battlefield order; Tender is no autopay source)"},
	{7489, "Secluded Courtyard's restricted any-colour ability is a census alternative a cast its chosen type admits plans with (gorge, 85fddcb68), no prefix-search gap: Healer's Hawk through Tinybones (its permission's any-type mana, which the planner never prices) is priced from its printed {W} (kernel restrPriced, restrCan)"},
	{66352, "a land played from another player's card (Secluded Courtyard through Tinybones) asks its as-enters type while still in exile: gorge changed its control at the play, so the projection names the player playing it (kernel projObj, preAsk)"},
	{34253, "Giada's restricted {W} from the ward window floats beside the pool (gorge's RestrictedMana): Hungry Ghoul's {1} cannot spend it"},
	{35332, "two Twinflame Tyrants, two asked combat hits: the blocked Gorehorn Raider and its blocker, dealt lethal damage before the second ask, stay on the battlefield until it is answered (CR 704.3)"},
	{36931, "the same, with a Twinflame Tyrant blocker: the replacements re-collected after each pick"},
	{93841, "the same with two Twinflame Tyrants: the asked hit takes the defending player to 0 and the next attacker's damage is still dealt (CR 704.3)"},
	{4337, "the Empyrean Eagle in hand stays 2/3 under a static whose source lost its abilities (Witness Protection)"},
	{5517, "Reassembling Skeleton's graveyard activation: gorge reaches it only by a scripted potential play, so it is not in gorge's net list (kernel.FeatPlannedActivation prices a graveyard ability without granted mana sources)"},
	{4601, "Quilled Greatwurm's counter-removal graveyard cast is no planned cast (kernel.FeatMayPlayUntyped leaves out StRemove6Counters)"},
	{29936, "Thousand-Year Storm copies six instants and sorceries; each copy's controller is asked a new target as it resolves (inherited target first), and the active player gets priority after it (CR 117.3b). Pair 6042 played seven copies until the f0f22ed merge changed its game; it makes none now; since gorge's mandatory search shuffle changed its game the copy-priority knob no longer changes it (13415 carries that coverage)"},
	{9476, "Sphinx's Tutelage repeats its mill while two milled nonland cards share a colour (six repeats; gorge effRepeat's gate re-checked after each pass, capped by the library size)"},
	{9295, "Akroma's Memorial grants protection from black and red: combat damage from a black or red creature to its controller's creatures is prevented (CR 702.16e, gorge emit's protectedFrom), only that source's (repl_gen.go protColFrom)"},
	{472, "a sacrificed token in the graveyard while the next player chooses its sacrifice (Liliana, Dreadhorde General's -4) is not listed: gorge's view lists no Ephemeral object (kernel.FeatZoneList; revert-verified on gorge)"},
	{480, "a creature under Imprisoned in the Moon is a land, so the view shows its printed P/T and its three +1/+1 counters move nothing (CR 208.3, kernel.FeatPT)"},
	{4016, "the same, another Imprisoned in the Moon pair (FeatPT)"},
	{9747, "a fourth until-end-of-turn grant of one keyword (Sower of Chaos's can't-block x4) changes the object's content hash, so the view lists it four times (kernel ObjHash reads kwx; revert-verified)"},
	{1358, "Archmage of Runes / Mocking Sprite reduce a flashback cast in the planner and pool gates: the flashback Think Twice is in gorge's net list (1541b01; revert-verified on gorge)"},
	{4899, "the same, Mocking Sprite with Think Twice and Revenge of the Rats (1541b01)"},
	{14353, "a Nightmare copy (Abyssal Harvester) of Mild-Mannered Librarian become a Werewolf is a Werewolf only: the copy's extra subtype is layer 1, under the subtype-setting override (3ec714e; diverged before it on gorge)"},
	{20135, "a sacrificed token copy of a card (a Strix Lookout token) in the graveyard while the next player chooses its Liliana, Dreadhorde General sacrifice is not listed: the kernel's view skips it by the token flag, not the card's type"},
	{56839, "the same, another token copy (FeatZoneList ofToken)"},
	{51720, "Soulstone Sanctuary's permanent Animate activated 33 times lists vigilance 33 times: the keyword repeats are per-bit counts (kernel.FeatKwReps), no longer capped at 32 levels"},
	{45914, "Imprisoned in the Moon's granted {C} is a census gap: gorge searches the first four gap plays in its walk order, the graveyard casts in graveyard order with Think Twice's flashback among Muldrotha's casts (kernel grantCap sorts by walkKey)"},
	{76303, "the same, Treetop Snarespinner activations in battlefield entry order"},
	{22668, "Empyrean Eagle's \"other creatures you control with flying get +1/+1\" and Akroma's Memorial's flying reach no Fiendish Panda in hand (CR 109.2)"},
	{11658, "Liliana, Dreadhorde General's -9: the opponent's per-type pick is gorge's choose:card over the permanents (kernel.ChGenChooseEach encodes like ChGenChooseCard: okCard, gorge's id-order key)"},
	{2433, "the same pick's list position: gorge's ChooseCard walks the objects in id order (kernel.ChGenChooseEach's key)"},
	{60288, "a free cast from the hand through Omniscience is gorge's printed hand-walk arm with the waived cost as its alternative cost (mode \"\", AltCostIndex 1), listed before the pass, not a may-play cast (CR 118.9)"},
	{91381, "a granted activation (Fishing Pole's bait ability on Lightshell Duo) is gorge's granted sweep after every printed ability, and a counter kind removed to none stays listed at 0 on the Fishing Pole (CR 613.1f, seeCtr)"},
	{13415, "Thousand-Year Storm copies; each copy's new-target ask, then the active player gets priority (coverage)"},
	{73, "Chandra, Flameshaper's -4 divides its 8 damage as it is activated: gorge's CR 601.2d \"divide\" ask comes after the targets and before the loyalty cost is paid, each target at least 1, and the resolution deals the announced shares (gorge; kernel bsDivide)"},
	{222, "the same, another Chandra -4 division"},
	{1294, "the same, another Chandra -4 division"},
	{176, "Sun-Blessed Healer's lifelink damage to three blockers is one life-gain event after every damage event of the pass: one Ajani's Pridemate trigger (CR 702.15e; gorge; kernel combatEventsGen)"},
}

func TestFDNRegressPairs(t *testing.T) {
	t.Parallel()

	c, _ := testFDNPool(t)
	pool, err := ReadPool(filepath.Join(filepath.Dir(c.DecksDir), "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: true}
	for _, rp := range fdnRegressPairs {
		ps := PairingsFrom(pool, rp.pair, 1, 1)
		if len(ps) != 1 || ps[0].Index != rp.pair {
			t.Fatalf("pair %d: no such pairing", rp.pair)
		}
		p := ps[0]
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			t.Fatalf("pair %d: %v", rp.pair, err)
		}
		k, err := SetSide(rawSet("fdn"), tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatalf("pair %d: %v", rp.pair, err)
		}
		fp, err := FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		var fs FeatStats
		rc := RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100, Check: FeatCheck(fp, &fs)}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("pair %d (%s): diverged at decision %d: %s %s %v", rp.pair, rp.why, d.At, d.What, d.Field, d.Diff)
		}
	}
}

// fdnRulesPairs are census pairings that play to the end in lockstep
// rules-only (no FeatCheck): their fix is the canonical grammar, and the
// feature side's half is still open (named per pair).
var fdnRulesPairs = []struct {
	pair int
	why  string
}{}

func TestFDNRegressPairsRules(t *testing.T) {
	t.Parallel()

	c, _ := testFDNPool(t)
	pool, err := ReadPool(filepath.Join(filepath.Dir(c.DecksDir), "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: true}
	for _, rp := range fdnRulesPairs {
		ps := PairingsFrom(pool, rp.pair, 1, 1)
		if len(ps) != 1 || ps[0].Index != rp.pair {
			t.Fatalf("pair %d: no such pairing", rp.pair)
		}
		p := ps[0]
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			t.Fatalf("pair %d: %v", rp.pair, err)
		}
		k, err := SetSide(rawSet("fdn"), tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatalf("pair %d: %v", rp.pair, err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("pair %d (%s): diverged at decision %d: %s %s %v", rp.pair, rp.why, d.At, d.What, d.Field, d.Diff)
		}
	}
}

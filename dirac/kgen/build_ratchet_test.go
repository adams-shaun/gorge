package kgen

import (
	"os"
	"regexp"
	"testing"
	"time"
)

// buildAcceptancePin is the build-level acceptance ratchet over ALL.txt:
// the cards that both lower and BUILD (counter kinds, cost shapes, filter
// compares are build refusals the lowering ratchet, acceptancePins, does
// not see), as `kgen -pool x -card-list ALL.txt` builds them, in process
// and writing nothing. Equality, like acceptancePins: a fall is a
// regression, a rise is re-pinned in the same commit.
// the Attacks + Defined$ TriggeredAttacker refusal for broad
// ValidCard filters lowers the build count (see acceptancePins).
// +15 from TriggeredCard$CastTotalManaSpent (Tf2CastSpent), which
// offsets the fall.
// +19 from trig:CommitCrime, Count$CommittedCrimeThisTurn
// and CheckDefinedPlayer committedCrimeThisTurn.
// +18 from the other:Defined$ grammar sweep.
// +22 for K:Plot, then -7 for the PutCounter triggered-power
// refusal (see acceptancePins).
// +6 from the TriggeredAttackerLKICopy condition lift (the five
// Ordeals and Viv Vision).
// +24 from lifting the broad-Attacks refusal.
// +10 from the Waterbend<N> activation cost (CR 701.67).
// +1 from CtrDread (Grasping Shadows).
// +1 from lifting Zimone, Infinite Analyst.
// +8 from lifting the PutCounter triggered-power refusal (see
// acceptancePins).
// +3 (Impatience, Predatory Advantage, Relic Runner).
// +5 from kop.CondAnd (two static gates on one static).
// +44 from Prepare (CR 722).
// +25 from printed K:Cascade (CardDef.Cascade); the same 25
// the lowering ratchet counts.
// +16 from printed K:Improvise (CR 702.126a).
// +11 from two-door Rooms (see acceptancePins).
// +5 for K:Demonstrate (the four printed Techniques and
// Creative Technique; see acceptancePins).
// +1 for Laboratory Drudge (see acceptancePins).
// +9 from printed K:Casualty:<N> (the same nine SNC cards
// listed in acceptancePins).
// +102 from hybrid pips printed outside WUBRG order; the
// same cards the lowering ratchet counts.
// +16 from the Battle/Siege lift (CR 310): the 15 MOM Sieges
// below plus Invasion of Theros, which builds only in the full corpus (its
// ChangeType$ Aura,Demigod,God reads Demigod as a subtype the 37-battle
// probe pool lacks). The MOM list itself is +15 (see acceptancePins).
// +13 from printed K:Sneak (CR 702.190a), +12 TMT and +1 MSH.
// +55 (ALL) from lifting multi-pick charms (CharmNum$/
// MinCharmNum$: Cryptic Command, Ojutai's Command, Primal Command, Urza's
// Command, Return from the Wilds, Craftwork Crusher, ...), then -19 from
// refusing an each-player discard with a choice (Liliana of the Veil,
// Rankle's Prank, ...); DSK -1. The review's non-mode-target refusal
// changes no pin.
//
// +19 (ALL) from the same cards, now that the
// kernel asks each player for the choice instead of resolving a no-op.
//
// +11 from the Omen lift (CR 720); Bloomvine Regent's face
// and Scavenger Regent's front still refuse at build, so 11 of the 13 TDM
// Omen cards build.
//
// +13 from the one-shot delayed SpellCast registration
// (CR 603.7; the cards are listed at acceptancePins' ALL pin); on the
// base one more TDM card builds, so the lift is +13.
//
// A: +4 from the four out-of-order gated Verges.
// B: +1 from Stature, Size Shifter's {X} power-up.
//
// +28 on the merged base from Token AttachedTo$ (24 Standard WOE,
// 4 ALL-only); the lowering ratchet counts the same 28.
//
// (value-heads) +36 from the value heads, modifiers and
// !HasCounters (the cards are listed at acceptancePins' ALL pin).
// (ninjutsu) +26 from the K:Ninjutsu hand ability (the
// cards are listed at acceptancePins' ALL pin).
// +30 for the combat-state filter words and the
// TriggeredDefender damage (see acceptancePins).
//
// (filter-props) +25 from Outlaw, MonoColor, !with<Keyword>,
// withMostLife and descended (listed at acceptancePins' ALL pin).
// (flashback-costs) +4 from non-mana flashback costs.
// +33 from the painland rider (painRider: the painlands, Elves
// of Deep Shadow and the other one-damage pain sources).
// (token-attacking) +19 for TokenAttacking$ (see
// (remembered-persist) +6 from the enters/leaves remembered pair.
// (Optional$) +19 for the mill / attach "you may" (OpMayAsk;
// see acceptancePins).
// +32 from a Continuous static's granted Ward:N (splitWardGrant).
// (durations) +13 for the host-bound durations and
// without<Keyword> (see acceptancePins).
// (Condition$ Evolve) +2 (Tf2Evolve; see acceptancePins).
// +1 (Malcolm, Alluring Scoundrel: CtrChorus and the guarded
// graveyard Play).
// (Imprint$, Exclude$) +11 (Reckless Impulse, the Gates, Thriving lands).
// (MinMaxBlocker, card-type ChooseType) +4 (see acceptancePins).
// (IsPresent$) +1 (Ascendant Packleader; see acceptancePins).
// (resolution costs) +9 (see acceptancePins).
// +16 from printed Phyrexian pips (Gut Shot, Mutagenic Growth,
// Dismember, Vault Skirge, Mental Misstep and kin).
// (ChangeNum$/Mandatory$/Reorder$) +9 (Brainstorm and kin, Lembas).
// (embalm) +10 (see acceptancePins).
// (ThisDefinedAndTgts$, Duration$) +6 (Floodpits Drowner, Deep-Cavern Bat and kin).
// (Chain Lightning) +5 (see acceptancePins).
// +5 from the bare library look (PeekAndReveal NoReveal$ without
// RememberPeeked$: Mishra's Bauble and kin) on OpLookAck C=1.
// (Part A) +76 over develop from the nine Count$ value shapes (see acceptancePins).
// (Sheoldred, Dusk Rose Reliquary) +2 (see acceptancePins).
// (Maelstrom Pulse) +1 (see acceptancePins).
// (RememberLKI$, UnlessType$) +11 (Gleeful Demolition, the Thirst cycle and kin).
// (UntapType$) +11 (Snap and the untap-lands kin).
// +51 from the remember/scope params (ChangeZoneAll
// RememberChanged$, ForgetOtherRemembered$, RememberLKI$, RememberControlled$,
// RememberDamaged$, PumpAll ValidTgts$, Discard UnlessType$; listed at
// acceptancePins' ALL pin).
// (step 5's UnlessType$ is OpDiscardUnless after the develop merge)
// +12 (an absent Defined$ / ValidTypes$ as-enters ChooseType and
// AddType$ ChosenType: Chronicle of Victory, Dawn-Blessed Pennant, Adaptive Automaton and kin).
// (MvFromGraveyard) +1 (Promise of Tomorrow un-refused; 19117 -> 19118).
// +7 (ChangeZone ForgetChanged$ True + DefinedDesc$); re-pinned 19149 -> 19153 (+4) after the rebase onto develop, 19153 -> 19194 after merging develop, then 19194 -> 19344 after merging develop (223874ccd)
// +7 (a printed graveyard ability's source-independent Activation$ Delirium / Threshold / IsPresent$ PresentZone$: Balustrade Wurm, Resurrected Cultist, Uchbenbak and kin; 19395 -> 19402; ratchet DSK 191 -> 193, LCI 215 -> 216).
// +3 (printed K:Hexproof:<Colour>: Knight of Grace, Knight of Malice and one more carrier; 19524 -> 19527).
// +7 (RememberedLKI$CardPower / $CardToughness over a Sacrifice RememberSacrificed$ set: Tribute to Hunger, Twisted Justice, Xathrid Demon and kin; 19864 -> 19871. Reign of the Pit and Soul Separator stay refused: their read is a token override).
// +1 (Melek, Reforged Researcher: OnlyFirstSpell$ with ValidSpell$; 19927 -> 19928; ratchet MKM 204 -> 205).
// +1 (Zero, Incorporeal Sensate: an Effect MayPlay beside a RaiseCost$ with no Activator$, the owner pays; 21443 -> 21444; ratchet TRK 276 -> 277).
// +1 (The City on the Edge of Forever: Phases AllValid$ and S:Mode$ CantPhaseIn; 21444 -> 21445; ratchet TRK 277 -> 278).
// +1 (Brazen Bluff: a Pump carries the spell target, the optional exile and the free noncreature cast; 21445 -> 21446; ratchet TRK 278 -> 279).
// +1 (Ensign Pavel Chekov: RepeatEach Opponent over ChooseCard in exile with ImprintChosen$, then MayPlay over the chosen; 21446 -> 21447; ratchet TRK 279 -> 280).
// +1 (Brad Boimler, Eager Ensign: the tap-triggered until-end-of-turn AddCounter replacement; 21447 -> 21448; ratchet TRK 280 -> 281).
// +1 (Daniels, Temporal Agent: PutCardToLibFromGrave<1/-1/CARDNAME> self cost; 21448 -> 21449; ratchet TRK 281 -> 282).
// +3 (Khan, Vengeful Heart; Misthollow Griffin; Eternal Scourge: MayPlay from exile; 21449 -> 21452; ratchet TRK 282 -> 283).
// +1 (Infinite Diversity: X free copies of an exiled instant or sorcery; 21452 -> 21453; ratchet TRK 283 -> 284).
// +3 (Kira Nerys; The Holodeck; Gabriel Lorca: land's TRK cards, Holo-Engineer already counted; 21454 -> 21457; ratchet TRK 285 -> 288).
// +1 (First Contact: the static SpellCast StoreSVar of Count$YouCastThisGame read by an EQ1 ConditionCheckSVar; 21457 -> 21458; ratchet TRK 288 -> 289).
// +1 (Lwaxana Troi, Charming Guest: the keep-tapped untap election and the tap-bound Effect-delivered Panharmonicon; land; merged 21458 -> 21459; ratchet TRK 289 -> 290).
// +1 (Dr. McCoy, Surly Surgeon: Destroy replacement removes +1/+1 counters and heals damage; 21459 -> 21460; ratchet TRK 290 -> 291).
// +2 (Gowron, Eager for Battle: Sacrificed$Valid Card.Klingon; Shilgengar, Sire of Famine: Card.Angel; 21460 -> 21462; ratchet TRK 291 -> 292).
// +1 (Nog, Ambitious Cadet: the Officer Animate grants a graveyard land permission and an extra land play; 21462 -> 21463; ratchet TRK 292 -> 293).
// +1 (Stray from History: a charm mode's hand move reads the owner from that mode's own player slot; 21463 -> 21464; ratchet TRK 293 -> 294).
// +1 (Kai Winn, Faithless Leader: the as-enters nonland card name and the opponent cast/activation trigger over it; 21464 -> 21465; ratchet TRK 294 -> 295).
// +4 (Captain Benjamin Sisko: a Play's mana-value bound names any one-letter SVar and Valid$ carries YouOwn; land; merged 21465 -> 21469; ratchet TRK 295 -> 296).
// +1 (Dominion Saboteur: the remembered-origin counter rider of an ETB copy; merged 21470 -> 21471; ratchet TRK 297 -> 298).
// +1 (The Inner Light: an Effect's remembered target and the damage amount reach its DamageDone emblem trigger; merged 21470 -> 21471; ratchet TRK 297 -> 298).
// +4 (a library search's WithCountersType$ entry rider: Captain Jean-Luc Picard, Magmatic Hellkite, Neoform and Wakanda Forever!; merged 21472 -> 21476; ratchet TDM 243 -> 244, TRK 299 -> 300).
// +4 (H.M.S. Bounty: Count$YourExtraTurns beside Defined$ You on AddTurn; land; merged 21476 -> 21480, TRK 300 -> 301).
// +2 (Those Old Scientists, Turn the Earth: a multi-target graveyard shuffle's per-owner move lowers to OpMayShuffleMove; merged 21472 -> 21474; ratchet TRK 299 -> 300).
// +1 (Q, Stacking the Deck: the exile-granted copy-and-cast ability of a card exiled from the top of the library; merged 21472 -> 21473; ratchet TRK 299 -> 300).
// +2 (U.S.S. Enterprise-D, Galaxy-Class and Axgard Artisan: CounterAddedOnce FirstTime$ True's per-source once-a-turn latch; 21486 -> 21488; ratchet MOM 196 -> 197, TRK 305 -> 306).
// +3 (Dream Harvest; Improvisation Capstone: RememberRevealed$+MinTotalCMC$ DigUntil, ECL/SOS +1; Black Widow, Super Spy: ImprintFound$ alone, MSH +1; ALL build-level 21496 -> 21499).
// +1 (Sanar, Innovative First-Year: the DigUntil RememberRevealed$ library reveal and its five per-colour picks; ALL build-level 21523 -> 21524; ratchet ECL 232 -> 233).
// +1 (Teval, Arbiter of Virtue: a granted Delve on the stack lowers and the cast announces its graveyard exiles; merged ALL 21523 -> 21524; ratchet TDM 244 -> 245).
// +3 (Cemetery Recruitment, Vengeful Rebirth, Tamiyo, Inquisitive Student: RememberTargets$ on a targeted Graveyard->Hand return; ALL build-level 21526 -> 21529).
// +3 (Wilt-Leaf Liege, Obstinate Baloth, Loxodon Smiter: the opponent-caused discard-to-battlefield replacement, lowered on land, absent here before the merge; ALL build-level 21529 -> 21532).
// +1 (Vizier of the Menagerie: the ManaConvert static lowering for creature spells; ALL build-level 21532 -> 21533).
// +2 (the WISH counter kind, kop.CtrWish: Wishclaw Talisman and Ring of Three Wishes lower; ALL build-level 21532 -> 21534).
// +1 (Carnelian Orb of Dragonkind: the AddsKeywords$ Haste mana rider lowers; ALL build-level 21534 -> 21535).
// +6 (the printed K:Prevent damage keywords: Fog Bank and kin; ALL build-level 21535 -> 21541).
// +2 (Sorcerous Spyglass: the as-enters look and any-card name choice and CantBeActivated over the chosen name; Phyrexian Revoker, the same class over the nonland name; land; merged ALL build-level 21541 -> 21543).
// +1 (Vizier of the Menagerie: the ManaConvert static lowering for creature spells; land; merged ALL build-level 21543 -> 21544).
// +1 (Ramos, Dragon Engine: the all-at-once counter-removal mana ability, kop.CtrEachManaAb; land; merged ALL build-level 21544 -> 21545).
// +2 (Animate Duration$ UntilUntaps, kop.Override.WhileTapped: Hedge Whisperer and The Blackstaff of Waterdeep lower; land; merged ALL build-level 21545 -> 21547).
// +1 (Vannifar, Evolved Enigma: the cloak-from-hand shape lowers; land; merged ALL build-level 21547 -> 21548).
// +1 (Mazemind Tome: the counter-threshold state trigger, Tf4StateCtr; land; merged ALL build-level 21548 -> 21549).
// +3 (a variable CollectEvidence cost and evidence beside the triggered-card exile: Incinerator of the Guilty, Urgent Necropsy, Lamplight Phoenix lower; merged ALL build-level 21549 -> 21552).
// +1 (Pyromancer's Goggles: the TriggersWhenSpent$ deferred mana source's copy rider lowers; this branch; merged ALL build-level 21556 -> 21557).
// +1 (Officious Interrogation: the per-target RaiseCost Cost$ W U, kop.StRaisePerTarget; land; merged ALL build-level 21557 -> 21558).
// +5 (the Effect MustAttack attack duty, kop.ExtAttacksIfAble: Boiling Blood, Heckling Fiends, Incite, Rage Nimbus, Shipwreck Singer lower; ALL build-level 21564 -> 21569).
// +4 (No Witnesses: the withMostTypeCreature investigate and the counted Num$ arm lower, with land's other merged lowerings; on land, its line lost in land's rebase re-pins; this branch; merged ALL build-level 21565 -> 21569).
// +1 (Molten Exhale: a may-flash extra that is a behold, CardDef.MayFlashBehold; ALL build-level 21569 -> 21570).
// +6 (the ExtTurnFaceUp effect turn-up, Urgent Necropsy's collect-evidence X, the each-target Aura-copy enchant ask and the DamageAll planeswalker/battle clause lower, with land's re-measure after its rebase; land; merged ALL build-level 21573 -> 21579).
// +1 (Doomsday Excruciator: a RepeatEach player body's library count and blind face-down Dig; on land, its line lost in land's rebase re-pins; this branch; merged ALL build-level 21584 -> 21585).
// +1 (Worlds Within Worlds: a RepeatEach player body's per-player hand move; on land, its line lost in land's rebase re-pins; this branch; merged ALL build-level 21585 -> 21586).
// +1 (Painter's Studio // Defaced Gallery: the unlock trigger's DB$ Effect statics read the door's own SVar table; on land, its line lost in land's rebase re-pins; this branch; merged ALL build-level 21595 -> 21596).
// +1 (White Lotus Hideout: a {T} {C} ability beside a restricted any-colour one and a {1},{T} filter; this branch; ALL build-level 21599 -> 21600).
// +2 (Baxter Building's four-mana combination filter and Haunted Screen's life-paid three-colour ability lower beside their first mana ability; msh-second-mana-ability; merged ALL build-level 21630 -> 21632).
// +3 (the Taps watchers over your own and the equipped creature lower: Hawkeye's Bow (EquippedBy), Judge of Currents and Magda, Brazen Outlaw (YouCtrl); the AttachedBy aura family still refuses; this branch; merged ALL build-level 21636 -> 21639).
// +4 (No Witnesses: the withMostTypeCreature investigate and the counted Num$ arm lower; on land, its line lost in land's rebase re-pins; this branch; merged ALL build-level 21565 -> 21569).
// +6 (the ExtTurnFaceUp effect turn-up, Urgent Necropsy's collect-evidence X, the each-target Aura-copy enchant ask and the DamageAll planeswalker/battle clause lower, with land's re-measure after its rebase; land; merged ALL build-level 21573 -> 21579).
// +1 (Doomsday Excruciator: a RepeatEach player body's library count and blind face-down Dig; on land, its line lost in land's rebase re-pins; this branch; merged ALL build-level 21584 -> 21585).
// +1 (Worlds Within Worlds: a RepeatEach player body's per-player hand move; on land, its line lost in land's rebase re-pins; this branch; merged ALL build-level 21585 -> 21586).
// +2 (Waterbending Lesson, The Unagi of Kyoshi Island: an UnlessCost$ Waterbend<N> and Ward:Waterbend<N>; this branch; merged TLA 246 -> 248, ALL 21595 -> 21597).
// +2 (the merge with land: this branch's UnlessCost$/Ward Waterbend<N> beside land's OptionalCost Waterbend<N> spell, whose two TLA cards land had already lowered; merged TLA 248 -> 250, ALL build-level 21638 -> 21640).
// +3 (the merge with land: land's Taps watchers over your own and the equipped creature beside this branch's UnlessCost$/Ward Waterbend<N>; merged MSH 231 -> 232, ALL build-level 21640 -> 21643).
// +2 (the merge with land: land's re-measure after its rebase (a TDM card and one more) beside this branch's Waterbend unless/ward cards, each side's two disjoint; merged ALL build-level 21643 -> 21645).
// +1 (the merge with land: land's Yue, the Moon Spirit free noncreature cast beside this branch's UnlessCost$/Ward Waterbend<N> cards, both sides' unrefusal of Airbender Ascension taken once; merged TLA 250 -> 251, ALL build-level 21645 -> 21646).
// +1 (the merge with land: this branch's anyRFiltMana lowers White Lotus Hideout's three-ability source, which land's refuseMultiMana refuses; merged TLA 251 -> 252, ALL build-level 21646 -> 21647).
// +8 (the same eight cards' Effect Mode$ MustBlock programs build, one per card: ALL build-level 21647 -> 21655).
// +4 (the filter word TargetedPlayerOwn (the one-token OwnedBy Targeted) lowers:
// Bulwark, Jon Irenicus, the Exile, Nicol Bolas, the Ravager, Search Warrant; the merge with land,
// beside this branch's Hulk Power-up ReduceCost arm; ALL build-level 21655 -> 21659).
// +1 (Hulk, Gamma Goliath's Power-up-only ReduceCost arm; ALL build-level 21659 -> 21660).
// +1 (Ark of Hunger: the remembered-card MayPlay over Graveyard,Exile; ALL build-level 21664 -> 21665).
// +1 (Parker Luck: the RepeatEach Targeted body reads the other player's revealed card's mana value; ALL build-level 21665 -> 21666).
// +10 (ValidSpell$ Activated.Equip ReduceCost, the target-gated equip price: Auriok Steelshaper, Bladehold War-Whip, Bureau Headmaster, Cloud, Planet's Champion, Dwarven Mauler, Fervent Champion, Ghostfire Blade, Helitrooper, Nahiri, Storm of Stone, Éowyn, Lady of Rohan; ALL build-level 21666 -> 21676).
// +5 (the merge with land: this branch's lowers beside land's Activated.Equip ReduceCost ten, disjoint; merged ALL build-level 21676 -> 21681).
// +1 (Myriad Landscape: the library search's ShareLandType$ is read; ALL build-level 21682 -> 21683).
// +1 (Aettir and Priwen: an attached static's live set P/T is read from its controller; ALL build-level 21681 -> 21682).
// +1 (Hundred-Battle Veteran: the self-pump gate counts the different counter kinds among your creatures; ALL build-level 21682 -> 21683).
// +1 (Hawkeye, Young Avenger: the damage-plus operand reads the replacement host's power; ALL build-level 21683 -> 21684).
// +1 (Hermitic Herbalist: a restricted Combo of any colours beside {T}: any colour is a wide census gap; ALL build-level 21706 -> 21707).
// +2 (the merge with land: this branch's Strategic Betrayal Origin$ Battlefield,Graveyard
// ChangeZoneAll sweep builds beside land's Binding Negotiation game-wide Exile->Graveyard
// hidden pick; merged ALL build-level 21707 -> 21709).
// +15 (the activation-cost tokens listed at ratchet_test.go's ALL pin: Exert<N/NICKNAME>,
// SubCounter<X/KIND> and ExileFromStack<1/Card.Self>; ALL build-level 21709 -> 21724).
// +5 (the merge with land: this branch's +5 beside land's +26, disjoint; merged ALL build-level 21784 -> 21789).
// +1 (Boommobile: kop.SpmActivated; the merge with land: land's 21792 + 1 = 21793).
// +6 (a static's granted Exalted: the cards listed at ratchet_test.go's ALL pin; ALL build-level 21873 -> 21879).
// +3 (a filter whose activation cost sacrifices its source: the cards listed at ratchet_test.go's ALL pin; ALL build-level 21930 -> 21933).
// +1 (the merge with land: this branch's Tam, Mindful First-Year static Hexproof:CardColors lower beside land's Amalia Benavides Aguirre / Hulkling / Burgeoning Bruiser lowers, disjoint; merged ALL build-level 21989 -> 21990).
// +1 (Agency Outfitter: the OriginAlternative$ Graveyard,Hand pair builds; ALL build-level 21990 -> 21991).
// Pin history: git log -p dirac/kgen/build_ratchet_test.go; KGEN_REPIN=1 rewrites it.
var buildAcceptancePin = [2]int{21991, 34442} // {built, cards}

func TestCorpusBuildAcceptance(t *testing.T) {
	cardsDir := ratchetCards(t)
	start := time.Now()
	p, err := loadCardList(testSetlistPath(t, "ALL"), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	cv := computeCoverage(p, lw)
	t.Logf("ALL build-level: %d of %d build (%v)", cv.lowered, cv.cards, time.Since(start))
	got := [2]int{cv.lowered, cv.cards}
	if got != buildAcceptancePin && os.Getenv("KGEN_REPIN") == "1" {
		row := regexp.MustCompile(`(?m)^(var buildAcceptancePin = \[2\]int\{)\d+, \d+(\})`)
		msg, err := repinFile("build_ratchet_test.go", row, "ALL build-level", buildAcceptancePin, got, os.Getenv("KGEN_REPIN_FALL"))
		if err != nil {
			t.Fatal(err)
		}
		t.Log(msg)
		return
	}
	if got != buildAcceptancePin {
		t.Errorf("ALL build-level: %d of %d, pinned %d of %d: a fall is a build regression; a rise is re-pinned in dirac/kgen/build_ratchet_test.go in the same commit",
			got[0], got[1], buildAcceptancePin[0], buildAcceptancePin[1])
	}
}

package kgen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The acceptance ratchets: kgen lowers every
// registered card list in dirac/cardset/setlists and the accepted count
// (cards that lower, whether or not refused by name) must EQUAL the pin. A fall
// is a regression; a rise is re-pinned in the same commit, so the pins are
// always the live counts.
//
// The count is LOWER + BUILD (checkAcceptance runs buildPool): a card the LIR
// pass accepts but `generate` refuses (counter kinds, cost shapes, filter
// compares) is not accepted here, so the pins equal what can be emitted.
// The earlier LIR-only pins were higher (ALL 17340 vs 16588 built).
var acceptancePins = map[string][2]int{ // list -> {accepted, cards}
	// +1 (Brad Boimler, Eager Ensign: the tap-triggered until-end-of-turn AddCounter replacement; TRK 280 -> 281).
	// +1 (Daniels, Temporal Agent: PutCardToLibFromGrave<1/-1/CARDNAME> self cost; TRK 281 -> 282).
	// +3 (Khan, Vengeful Heart; Misthollow Griffin; Eternal Scourge: MayPlay from exile; TRK 282 -> 283, ALL 21449 -> 21452).
	// +1 (Infinite Diversity: X free copies of an exiled instant or sorcery; TRK 283 -> 284, ALL 21452 -> 21453).
	// +3 (Kira Nerys; The Holodeck; Gabriel Lorca: land's TRK cards, Holo-Engineer being the same change already counted; TRK 285 -> 288, ALL 21454 -> 21457).
	// +1 (First Contact: the first-spell-cast rider; TRK 288 -> 289, ALL 21457 -> 21458).
	// +1 (Lwaxana Troi, Charming Guest: the keep-tapped untap election and the tap-bound Effect-delivered Panharmonicon; land; merged TRK 289 -> 290, ALL 21458 -> 21459).
	// +1 (Dr. McCoy, Surly Surgeon: Destroy replacement removes +1/+1 counters and heals damage; TRK 290 -> 291, ALL 21459 -> 21460).
	// +2 (Gowron, Eager for Battle: Sacrificed$Valid Card.Klingon; Shilgengar, Sire of Famine: Card.Angel; TRK 291 -> 292, ALL 21460 -> 21462).
	// +1 (Nog, Ambitious Cadet: the Officer Animate grants a graveyard land permission and an extra land play; TRK 292 -> 293, ALL 21462 -> 21463).
	// +1 (Stray from History: a charm mode's hand move reads the owner from that mode's own player slot; TRK 293 -> 294, ALL 21463 -> 21464).
	// +1 (Kai Winn, Faithless Leader: the as-enters nonland card name and the opponent cast/activation trigger over it; TRK 294 -> 295, ALL 21464 -> 21465).
	// +4 (Captain Benjamin Sisko: a Play's mana-value bound names any one-letter SVar and Valid$ carries YouOwn; land; merged TRK 295 -> 296, ALL 21465 -> 21469).
	// +1 (Dominion Saboteur: the remembered-origin counter rider of an ETB copy; merged TRK 297 -> 298, ALL 21470 -> 21471).
	// +1 (The Inner Light: an Effect's remembered target and the damage amount reach its DamageDone emblem trigger; merged TRK 297 -> 298, ALL 21470 -> 21471).
	// +4 (a library search's WithCountersType$ entry rider: Captain Jean-Luc Picard, Magmatic Hellkite, Neoform and Wakanda Forever!; merged ALL 21472 -> 21476; TDM 243 -> 244, TRK 299 -> 300).
	// +4 (H.M.S. Bounty: Count$YourExtraTurns beside Defined$ You on AddTurn; land; merged ALL 21476 -> 21480, TRK 300 -> 301).
	// +2 (Those Old Scientists, Turn the Earth: a multi-target graveyard shuffle's per-owner move lowers to OpMayShuffleMove; merged TRK 299 -> 300, ALL 21472 -> 21474).
	// +1 (Q, Stacking the Deck: the exile-granted copy-and-cast ability of a card exiled from the top of the library; merged TRK 299 -> 300, ALL 21472 -> 21473).
	// +2 (U.S.S. Enterprise-D, Galaxy-Class and Axgard Artisan: CounterAddedOnce FirstTime$ True's per-source once-a-turn latch; MOM 196 -> 197, TRK 305 -> 306, ALL 21486 -> 21488).
	// +3 (Dream Harvest; Improvisation Capstone: RememberRevealed$+MinTotalCMC$ DigUntil, ECL/SOS +1; Black Widow, Super Spy: ImprintFound$ alone, MSH +1; ALL 21496 -> 21499).
	// +1 (Sanar, Innovative First-Year: the DigUntil RememberRevealed$ library reveal and its five per-colour picks; ECL 232 -> 233, ALL 21523 -> 21524).
	// +1 (Teval, Arbiter of Virtue: a granted Delve on the stack lowers and the cast announces its graveyard exiles; merged ALL 21523 -> 21524, TDM 244 -> 245).
	// +3 (Cemetery Recruitment, Vengeful Rebirth, Tamiyo, Inquisitive Student: RememberTargets$ on a targeted Graveyard->Hand return; FDN 507 -> 508, ALL 21526 -> 21529).
	// +3 (Wilt-Leaf Liege, Obstinate Baloth, Loxodon Smiter: the opponent-caused discard-to-battlefield replacement, lowered on land, absent here before the merge; FDN 508 -> 509, ALL 21529 -> 21532).
	// +1 (Vizier of the Menagerie: the ManaConvert static lowering for creature spells; FDN 509 -> 510, ALL 21532 -> 21533).
	// +2 (the WISH counter kind, kop.CtrWish: Wishclaw Talisman and Ring of Three Wishes lower; FDN 509 -> 510, ALL 21532 -> 21534).
	// +1 (Carnelian Orb of Dragonkind: the AddsKeywords$ Haste mana rider lowers; FDN 510 -> 511, ALL 21534 -> 21535).
	// +6 (the printed K:Prevent damage keywords: Fog Bank and kin; FDN 511 -> 512, ALL 21535 -> 21541).
	// +2 (Sorcerous Spyglass: the as-enters look and any-card name choice and CantBeActivated over the chosen name; Phyrexian Revoker, the same class over the nonland name; land; merged FDN 512 -> 513, LCI 250 -> 251, ALL 21541 -> 21543).
	// +1 (Vizier of the Menagerie: the ManaConvert static lowering for creature spells; land; merged FDN 513 -> 514, ALL 21543 -> 21544).
	// +1 (Ramos, Dragon Engine: the all-at-once counter-removal mana ability, kop.CtrEachManaAb; land; merged FDN 514 -> 515, ALL 21544 -> 21545).
	// +2 (Animate Duration$ UntilUntaps, kop.Override.WhileTapped: Hedge Whisperer and The Blackstaff of Waterdeep lower; land; merged MKM 231 -> 232, ALL 21545 -> 21547).
	// +1 (Vannifar, Evolved Enigma: the cloak-from-hand shape lowers; land; merged MKM 232 -> 233, ALL 21547 -> 21548).
	// +1 (Mazemind Tome: the counter-threshold state trigger, Tf4StateCtr; land; merged FDN 515 -> 516, ALL 21548 -> 21549).
	// +3 (a variable CollectEvidence cost and evidence beside the triggered-card exile: Incinerator of the Guilty, Urgent Necropsy, Lamplight Phoenix lower; merged MKM 233 -> 236, ALL 21549 -> 21552).
	// +1 (Pyromancer's Goggles: the TriggersWhenSpent$ deferred mana source's copy rider lowers; this branch; merged FDN 516 -> 517, ALL 21556 -> 21557).
	// +1 (Officious Interrogation: the per-target RaiseCost Cost$ W U, kop.StRaisePerTarget; land; merged MKM 237 -> 238, ALL 21557 -> 21558).
	// +5 (the Effect MustAttack attack duty, kop.ExtAttacksIfAble: Boiling Blood, Heckling Fiends, Incite, Rage Nimbus, Shipwreck Singer lower; ALL 21564 -> 21569).
	// +4 (No Witnesses' withMostTypeCreature investigate and the counted Num$ arm lower; on land, its line lost in land's rebase re-pins; this branch; merged MKM 234 -> 235, ALL 21565 -> 21569).
	// +1 (Molten Exhale: a may-flash extra that is a behold, CardDef.MayFlashBehold; merged TDM 245 -> 246, ALL 21569 -> 21570).
	// +6 (the ExtTurnFaceUp effect turn-up, Urgent Necropsy's collect-evidence X, the each-target Aura-copy enchant ask and the DamageAll planeswalker/battle clause lower, with land's re-measure after its rebase; land; merged ALL 21573 -> 21579).
	// +1 (Doomsday Excruciator: a RepeatEach player body's DigNum$ Count$ValidLibrary Card.RememberedPlayerCtrl/Minus.6 and its blind face-down exile; on land, its line lost in land's rebase re-pins; this branch; merged DSK 236 -> 237, ALL 21584 -> 21585).
	// +1 (Worlds Within Worlds: a RepeatEach player body's hand move DefinedPlayer$ Remembered up to Count$ValidHand Creature.RememberedPlayerCtrl, and the remembered set's return from exile as the Return walk; on land, its line lost in land's rebase re-pins; this branch; merged MSH 226 -> 227, ALL 21585 -> 21586).
	// +1 (Painter's Studio // Defaced Gallery: the unlock trigger's DB$ Effect statics read the door's own SVar table; on land, its line lost in land's rebase re-pins; this branch; merged DSK 238 -> 239, ALL 21595 -> 21596).
	// +1 (White Lotus Hideout: a {T} {C} ability beside a restricted any-colour one and a {1},{T} filter; this branch; TLA 246 -> 247, ALL 21599 -> 21600).
	// +2 (Baxter Building's four-mana combination filter and Haunted Screen's life-paid three-colour ability lower beside their first mana ability; msh-second-mana-ability; merged DSK 241 -> 242, MSH 228 -> 229, ALL 21630 -> 21632).
	// +3 (the Taps watchers over your own and the equipped creature lower: Hawkeye's Bow (EquippedBy), Judge of Currents and Magda, Brazen Outlaw (YouCtrl); the AttachedBy aura family still refuses; this branch; merged MSH 230 -> 231, ALL 21636 -> 21639).
	// +4 (No Witnesses' withMostTypeCreature investigate and the counted Num$ arm lower; on land, its line lost in land's rebase re-pins; this branch; merged MKM 234 -> 235, ALL 21565 -> 21569).
	// +6 (the ExtTurnFaceUp effect turn-up, Urgent Necropsy's collect-evidence X, the each-target Aura-copy enchant ask and the DamageAll planeswalker/battle clause lower, with land's re-measure after its rebase; land; merged ALL 21573 -> 21579).
	// +1 (Doomsday Excruciator: a RepeatEach player body's DigNum$ Count$ValidLibrary Card.RememberedPlayerCtrl/Minus.6 and its blind face-down exile; on land, its line lost in land's rebase re-pins; this branch; merged DSK 236 -> 237, ALL 21584 -> 21585).
	// +1 (Worlds Within Worlds: a RepeatEach player body's hand move DefinedPlayer$ Remembered up to Count$ValidHand Creature.RememberedPlayerCtrl, and the remembered set's return from exile as the Return walk; on land, its line lost in land's rebase re-pins; this branch; merged MSH 226 -> 227, ALL 21585 -> 21586).
	// +2 (Waterbending Lesson, The Unagi of Kyoshi Island: an UnlessCost$ Waterbend<N> and Ward:Waterbend<N>; this branch; merged TLA 246 -> 248, ALL 21595 -> 21597).
	// +2 (the merge with land: this branch's UnlessCost$/Ward Waterbend<N> beside land's OptionalCost Waterbend<N> spell, whose two TLA cards land had already lowered; merged TLA 248 -> 250, ALL 21638 -> 21640).
	// +3 (the merge with land: land's Taps watchers over your own and the equipped creature beside this branch's UnlessCost$/Ward Waterbend<N>; merged MSH 231 -> 232, ALL 21640 -> 21643).
	// +2 (the merge with land: land's re-measure after its rebase (a TDM card and one more) beside this branch's Waterbend unless/ward cards, each side's two disjoint; merged TDM 248 -> 249, ALL 21643 -> 21645).
	// +1 (the merge with land: land's Yue, the Moon Spirit free noncreature cast beside this branch's UnlessCost$/Ward Waterbend<N> cards, both sides' unrefusal of Airbender Ascension taken once; merged TLA 250 -> 251, ALL 21645 -> 21646).
	// +1 (the merge with land: this branch's anyRFiltMana lowers White Lotus Hideout's three-ability source, which land's refuseMultiMana refuses; merged TLA 251 -> 252, ALL 21646 -> 21647).
	// +8 (this branch lowers the Effect Mode$ MustBlock static beside the pair's MustAttack: Hustle // Bustle,
	// Courtly Provocateur, Culvert Ambusher, Culling Mark, Peema Aether-Seer, Provoke, Shessra, Death's Whisper
	// and Timely Interference; MKM +2, DMU +1, ALL +8).
	// +4 (the filter word TargetedPlayerOwn (the one-token OwnedBy Targeted) lowers:
	// Bulwark, Jon Irenicus, the Exile, Nicol Bolas, the Ravager, Search Warrant; the merge with land,
	// beside this branch's Hulk Power-up ReduceCost arm; ALL 21655 -> 21659).
	// +1 (Hulk, Gamma Goliath: the Power-up-only ReduceCost arm,
	// ValidSpell$ Activated.PowerUp; MSH 232 -> 233, ALL 21659 -> 21660).
	// +1 (Ark of Hunger: the remembered-card MayPlay over Graveyard,Exile; SOS 248 -> 249, ALL 21664 -> 21665).
	// +1 (Parker Luck: the RepeatEach Targeted body reads the other player's revealed card's mana value; SPM 160 -> 161, ALL 21665 -> 21666).
	// +10 (ValidSpell$ Activated.Equip ReduceCost, the target-gated equip price: Auriok Steelshaper, Bladehold War-Whip, Bureau Headmaster, Cloud, Planet's Champion, Dwarven Mauler, Fervent Champion, Ghostfire Blade, Helitrooper, Nahiri, Storm of Stone, Éowyn, Lady of Rohan; FIN 266 -> 267, HOB 152 -> 153, ONE 195 -> 196, ALL 21666 -> 21676).
	// +5 (the merge with land: this branch's lowers beside land's Activated.Equip ReduceCost ten, disjoint; merged ECL 233 -> 234, MSH 233 -> 234, TLA 252 -> 253, ALL 21676 -> 21681).
	// +1 (Myriad Landscape: the library search's ShareLandType$ is read; ALL 21682 -> 21683).
	// +1 (Aettir and Priwen: an attached static's live set P/T is read from its controller; FIN 267 -> 268, ALL 21681 -> 21682).
	// +1 (Hundred-Battle Veteran: the self-pump gate counts the different counter kinds among your creatures; TDM 249 -> 250, ALL 21682 -> 21683).
	// +1 (Hawkeye, Young Avenger: the damage-plus operand reads the replacement host's power; MSH 234 -> 235, ALL 21683 -> 21684).
	// +1 (Hermitic Herbalist: a restricted Combo of any colours, amount 2, beside {T}: any colour is a wide census gap; TLA 253 -> 254, ALL 21706 -> 21707).
	// +2 (the merge with land: this branch's Origin$ Battlefield,Graveyard ChangeZoneAll sweep
	// (Strategic Betrayal; TDM 250 -> 251) beside land's game-wide Exile->Graveyard hidden pick
	// (Binding Negotiation; OTJ 241 -> 242); merged ALL 21707 -> 21709).
	// +15 (the activation-cost tokens: Exert<N/NICKNAME> as an announced ability cost,
	// SubCounter<X/KIND> announced up to the source's counters, and
	// ExileFromStack<1/Card.Self> as an unless part: Angel of Condemnation, Auntie Blyte,
	// Bad Influence, Basri, Tomorrow's Champion, Chamber Sentry, Cruel Sadist,
	// Dragon's Approach, Fervent Paincaster, Geistflame Reservoir, Hope Tender,
	// Infused Arrows, Pride Sovereign, Sphinx's Approach, Steward of Solidarity,
	// Talon of Pain, The Astonishing Ant-Man; ALL 21709 -> 21724, DFT 248 -> 249,
	// FRA 245 -> 246, MSH 235 -> 236).
	// +9 (the merge with land: land's +9 beside the +15 above; merged ALL 21709 -> 21733, FRA 246 -> 247 measured).
	// +5 (the merge with land: this branch's +5 beside land's +26, disjoint; merged ALL 21784 -> 21789 measured).
	// +1 (Boommobile: an artifact's resolving mana restricted to activated abilities, kop.SpmActivated; the merge with land: land's 21792 + 1 = 21793, DFT 250 -> 251).
	// +6 (a static's granted Exalted lowers as a latent Attacks trigger: First Sliver's Chosen, Merchant of Truth, Order of Sacred Dusk, Sublime Archangel, Xenk, Paladin Unbroken, Zarda, the Power Princess; ALL 21873 -> 21879).
	// +3 (a filter whose activation cost sacrifices its source lowers, kop.FilterManaAb: Omni-Cheese Pizza (TMT 174 -> 175), Chromatic Star, Golden Egg; ALL 21930 -> 21933).
	// +3 (a battlefield power == a constant and a toughness > a value lower: Amalia Benavides Aguirre (LCI 255 -> 256), Hulkling, Burgeoning Bruiser (MSH 240 -> 241), Savage Swipe's powerEQ2 condition; ALL 21945 -> 21948).
	// +1 (the merge with land: this branch's Tam, Mindful First-Year static Hexproof:CardColors lower beside land's Amalia Benavides Aguirre / Hulkling / Burgeoning Bruiser lowers, disjoint; merged ECL 238 -> 239, ALL 21989 -> 21990).
	// +2 (the merge with land: this branch's Hollow Marauder lowers OTJ 244 -> 245 and Madame Null lowers TMT 176 -> 177 beside land's; merged ALL 22036 -> 22038).
	// +8 (a reflected TapsForMana body over every land or Mountain,Forest,Plains lowers, priced on the pick: Mana Flare, Heartbeat of Spring, Dictate of Karametra, Zhur-Taa Ancient, Nikya of the Old Ways, Keeper of Progenitus, Mirari's Wake, Zendikar Resurgent; ALL 22097 -> 22105).
	// +3 (lt-low-tappower-station: Tapestry Warden, Sulu, Heroic Helmsman and Stoic Star-Captain lower their TapPowerValue lines to StatTapPower statics; ALL 22092 -> 22095).
	// +3 (the merge with land: this branch's three TapPowerValue statics beside land's 22114; merged ALL 22114 -> 22117).
	// +3 (an Effect's trigger bound to a remembered set: The Eagles Are Coming!, Mysterio, Master of Illusion, T'Pring, Never Parted; ALL 22145 -> 22148, HOB 158 -> 159, SPM 168 -> 169, TRC 297 -> 298).
	// +2 (the optional copy-instead CreateToken replacement of an Equipment or Aura gated on no token of yours having entered this turn, kop.ReplCopyFirstToken: Mirrormind Crown, Moonlit Meditation; ECL 242 -> 243, EOE 248 -> 249, ALL 22149 -> 22151).
	// +1 (ConditionActivationLimit$ lowers to kop.ExActivations on a card whose one non-mana activated ability it gates: Dragon Whelp; DMU 194 -> 195, ALL 22176 -> 22177).
	// +1 (Fiend Hunter's Defined$ DirectRemembered leave return lowers as the remembered-persist pair; ALL 22176 -> 22177).
	// +1 (the merge with land: Count$TotalDamageReceivedThisTurn lowered: Tangled Colony beside land's 22180; ALL 22180 -> 22181, WOE 261 -> 262).
	// +5 (Dig NoLooking$ and Manifest Defined$ Remembered lower: Telling Time, Moment of Truth, Expressive Iteration, Write into Being, Hamza, Might of the Yathan; ALL 22345 -> 22350, MOM 199 -> 200).
	// +5 (the hasManaAbility filter property and DrainMana: Power Sink, Moonsilver
	// Key, Mana Short, Midnight Arsonist and Raggadragga, Goreguts Boss; ALL
	// 22345 -> 22350).
	// +31 (K:Partner with lowers: the pair marker and the targeted player's own-library search for the named partner (lower_partnerwith.go): Alisaie Leveilleur, Alphinaud Leveilleur, Blaring Captain, Blaring Recruiter, Brallin, Skyshark Rider, Cazur, Ruthless Stalker, Chakram Retriever, Chakram Slinger, Evie Frye, Impetuous Protege, Jenny Flint, Kamber, the Plunderer, Krav, the Unredeemed (refused by name: dirac/support/refused), Laurine, the Diversion, Ley Weaver, Lore Weaver, Merry, Warden of Isengard, Nikara, Lair Scavenger, Pippin, Warden of Isengard, Proud Mentor, Rocksteady, Mutant Marauder, Sam, Loyal Attendant, Shabraz, the Skyshark, Silvar, Devourer of the Free, Soulblade Corrupter, Soulblade Renewer, Timin, Youthful Geist, Toothy, Imaginary Friend, Trynn, Champion of Freedom, Ukkima, Stalking Shadow, Virtus the Veiled; ALL 22381 -> 22412; merged ALL 22706 -> 22737).
	// +12 from the single-colour storage-battery mana abilities (the 11 storage lands and City of Shadows; the build ratchet counts the same +12): merged ALL 22884 -> 22896.
	// +29 (the monarch, CR 725: api BecomeMonarch, Mode$ BecomeMonarch, Count$Monarch, CheckDefinedPlayer$ You|Opponent.isMonarch and a monarch-gated Effect lower: Aragorn, King of Gondor, Chief Magistrate of Mercadia, Court of Ardenvale, Court of Bounty, Court of Cunning, Court of Embereth, Court of Garenbrig, Court of Grace, Court of Ire, Crimson Fleet Commodore, Custodi Lich, Faramir, Steward of Gondor, Feast of Succession, Garrulous Sycophant, Grave Venerations, Keeper of Keys, Knights of the Black Rose, Marchesa's Decree, Nakia, Wakandan Operative, Oath of Eorl, Palace Sentinels, Queen Marchesa, Regal Sliver, Skyline Despot, Staunch Throneguard, Thorn of the Black Rose, Throne Warden, Throne of the High City, Éomer, King of Rohan; ALL 22381 -> 22410).
	// +30 (the merge with land: the monarch lowering above against land's corpus; merged ALL 22868 -> 22898).
	// +16 (the merge with land: land's later work after f486bfb466, the zone-scoped PlayerCount extreme and Pump/UntilYourNextTurn lowering at 0d7bfabead, DSK 250 -> 251; merged ALL 22898 -> 22914 measured).
	// +16 (the merge with land: this branch's CR 702.43 Multikicker and Count$TimesKicked lowering against the merged corpus; merged ALL 22929 -> 22945 measured).
	// +16 (the merge with land at 065ba12671b7: the same CR 702.43 Multikicker and Count$TimesKicked lowering against land's later corpus; merged ALL 22971 -> 22987 measured).
	// Pin history: git log -p dirac/kgen/ratchet_test.go; a commit lists the cards it moves.
	// +12 (the merge with land: the storage-battery abilities above against land's corpus; merged ALL 22971 -> 22983 measured).
	// +16 (the merge with land at bcd3c03ad118: the same CR 702.43 Multikicker and Count$TimesKicked lowering against land's later corpus; merged ALL 23052 -> 23068 measured).
	// +24 (mid-resolution ChooseNumber lowers: ChooseNumberMid, with Count$ChosenNumber and the dynamic PayEnergy<X> unless cost; an activated {X} energy cost stays refused rather than losing its amount; merged onto land c70201715c: ALL 23076 -> 23100, WOE 263 -> 264 measured).
	// -1 (Gilded Light refuses: its until-end-of-turn player shroud lowered to a keyword grant that lands on no permanent; ALL 23105 -> 23104 measured).
	// +16 (the merge with land at 1acc5919d9: this branch's CR 702.43 Multikicker and Count$TimesKicked lowering beside land's later work; merged ALL 23115 -> 23131, WOE stays 264 measured).
	// +14 (the merge with land at 6014a11d54: this branch's Extort lowering, the printed K:Extort expansion and the granted trigger, beside land's 23177; merged ALL 23177 -> 23191 measured).
	"ALL": {23204, 34442},
	"BIG": {27, 30},
	"BLB": {261, 278},
	"BRO": {218, 280},
	"DFT": {255, 276},
	"DMU": {197, 270},
	"DSK": {251, 276},
	"ECL": {244, 273},
	"EOE": {250, 266},
	"FDN": {517, 517},
	"FIN": {276, 312},
	"FRA": {255, 269},
	"HOB": {159, 172},
	"LCI": {260, 290},
	"MKM": {251, 279},
	"MOM": {201, 296},
	"MSH": {246, 276},
	"NEO": {201, 287},
	"ONE": {201, 271},
	"OTJ": {251, 276},
	"SNC": {187, 266},
	"SOS": {257, 270},
	"SPM": {169, 192},
	"TDM": {256, 276},
	"TLA": {261, 285},
	"TMT": {183, 194},
	"TRC": {301, 337},
	"TRK": {311, 311},
	"WOE": {264, 281},
}

// testSetlistDir is setlistDir relative to the test's working directory
// (dirac/kgen), not the module root the -census-std flag runs from.
const testSetlistDir = "../../" + setlistDir

func ratchetCards(t *testing.T) string {
	t.Helper()
	cardsDir := os.Getenv("KGEN_CARDS")
	if cardsDir == "" {
		var err error
		if cardsDir, err = cardset.CorpusDir(); err != nil {
			t.Skipf("SKIP: acceptance ratchet NOT RUN: %v; link the corpus as .cards here", err)
		}
	}
	if _, err := os.Stat(cardsDir); err != nil {
		t.Skipf("SKIP: acceptance ratchet NOT RUN: corpus %s missing (%v); link it as .cards here", cardsDir, err)
	}
	return cardsDir
}

func checkAcceptance(t *testing.T, cardsDir, list string) {
	t.Helper()
	pin, ok := acceptancePins[list]
	if !ok {
		t.Fatalf("%s.txt has no pin in acceptancePins", list)
	}
	p, err := loadCardList(testSetlistPath(t, list), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	// Lower AND build: buildPool appends the build-level refusals (counter
	// kinds, cost shapes, filter compares) to each card, so the pin equals
	// what `generate` can emit, not just what the LIR pass accepts.
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatalf("%s: build: %v", list, err)
	}
	cv := computeCoverage(p, lw)
	t.Logf("%s: %d of %d accepted", list, cv.lowered, cv.cards)
	got := [2]int{cv.lowered, cv.cards}
	if got != pin && os.Getenv("KGEN_REPIN") == "1" {
		repinMu.Lock()
		defer repinMu.Unlock()
		msg, err := repinFile("ratchet_test.go", repinRow(list), list, pin, got, os.Getenv("KGEN_REPIN_FALL"))
		if err != nil {
			t.Fatal(err)
		}
		t.Log(msg)
		return
	}
	if got != pin {
		t.Errorf("%s: accepted %d of %d, pinned %d of %d: a fall is a lowering regression; a rise is re-pinned in dirac/kgen/ratchet_test.go in the same commit",
			list, got[0], got[1], pin[0], pin[1])
	}
}

// TestCorpusAcceptance is the corpus ratchet (the generated ALL list, ~3 s).
func TestCorpusAcceptance(t *testing.T) {
	checkAcceptance(t, ratchetCards(t), "ALL")
}

// TestSetAcceptance is the per-set ratchet, one subtest per list.
func TestSetAcceptance(t *testing.T) {
	cardsDir := ratchetCards(t)
	ents, err := os.ReadDir(testSetlistDir)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range ents {
		list, ok := strings.CutSuffix(e.Name(), ".txt")
		if !ok {
			continue
		}
		seen++
		t.Run(list, func(t *testing.T) { checkAcceptance(t, cardsDir, list) })
	}
	if seen != len(acceptancePins)-1 {
		t.Errorf("%d set lists, %d set pins: a list and its pin are added together", seen, len(acceptancePins)-1)
	}
}

var repinMu sync.Mutex

// repinRow matches one `"<LIST>": {a, b},` pin row; group 1 is everything up
// to the numbers, so only the numeric literals are replaced.
func repinRow(list string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^(\t"` + regexp.QuoteMeta(list) + `": *\{)\d+, *\d+(\},)`)
}

// repinSource rewrites the row in src to {got[0], got[1]}; comments and order
// are untouched. Groups 1 and 2 of row are what surrounds the numbers.
func repinSource(src string, row *regexp.Regexp, got [2]int) (string, bool) {
	if !row.MatchString(src) {
		return src, false
	}
	return row.ReplaceAllString(src, fmt.Sprintf("${1}%d, %d${2}", got[0], got[1])), true
}

// repinFile moves list's pin in path from pin to got and returns the line to
// print. A lower accepted count is refused, leaving the file alone, unless
// fallReason says why it is acceptable.
func repinFile(path string, row *regexp.Regexp, list string, pin, got [2]int, fallReason string) (string, error) {
	msg := fmt.Sprintf("%s: repinned to %d of %d", list, got[0], got[1])
	if got[0] < pin[0] {
		if fallReason == "" {
			return "", fmt.Errorf("%s: accepted fell from %d to %d: a fall is a regression; fix it, or set KGEN_REPIN_FALL=<reason> to accept it",
				list, pin[0], got[0])
		}
		msg += fmt.Sprintf(", accepted fell from %d: %s", pin[0], fallReason)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	out, ok := repinSource(string(b), row, got)
	if !ok {
		return "", fmt.Errorf("no pin row for %s in %s", list, path)
	}
	return msg, os.WriteFile(path, []byte(out), 0o644)
}

func TestRepinRewritesOnlyNumbers(t *testing.T) {
	src := "var acceptancePins = map[string][2]int{ // list\n\t// Pin history: x\n\t\"ALL\": {1, 2},\n\t// note\n\t\"BLB\": {3, 4},\n}\n"
	out, ok := repinSource(src, repinRow("BLB"), [2]int{30, 40})
	if !ok {
		t.Fatal("row not found")
	}
	want := strings.Replace(src, "{3, 4}", "{30, 40}", 1)
	if out != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
	if _, ok := repinSource(src, repinRow("XXX"), [2]int{1, 1}); ok {
		t.Error("missing list reported found")
	}
}

// fakePinFile is a pin file holding one list, FAKE, at {10, 20}.
func fakePinFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pins")
	if err := os.WriteFile(path, []byte("\t\"FAKE\": {10, 20},\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func wantFakePin(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "\t\"FAKE\": "+want+",\n" {
		t.Errorf("pin file holds %q, want the row at %s", got, want)
	}
}

func TestRepinRefusesFall(t *testing.T) {
	path := fakePinFile(t)
	_, err := repinFile(path, repinRow("FAKE"), "FAKE", [2]int{10, 20}, [2]int{9, 20}, "")
	if err == nil {
		t.Fatal("a fall with no reason was accepted")
	}
	if !strings.Contains(err.Error(), "FAKE") {
		t.Errorf("error does not name the list: %v", err)
	}
	wantFakePin(t, path, "{10, 20}")
}

func TestRepinFallWithReason(t *testing.T) {
	path := fakePinFile(t)
	msg, err := repinFile(path, repinRow("FAKE"), "FAKE", [2]int{10, 20}, [2]int{9, 20}, "a refusal that closes a divergence")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "a refusal that closes a divergence") {
		t.Errorf("message does not print the reason: %q", msg)
	}
	wantFakePin(t, path, "{9, 20}")
}

func TestRepinTakesARise(t *testing.T) {
	path := fakePinFile(t)
	if _, err := repinFile(path, repinRow("FAKE"), "FAKE", [2]int{10, 20}, [2]int{11, 21}, ""); err != nil {
		t.Fatal(err)
	}
	wantFakePin(t, path, "{11, 21}")
}

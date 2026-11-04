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
// +1 (Nog, Ambitious Cadet: the Officer Animate grants a graveyard land permission and an extra land play; 21459 -> 21460; ratchet TRK 290 -> 291).
// +1 (Dominion Saboteur: the remembered-origin counter rider of an ETB copy; merged 21460 -> 21461; ratchet TRK 291 -> 292).
// Pin history: git log -p dirac/kgen/build_ratchet_test.go; KGEN_REPIN=1 rewrites it.
var buildAcceptancePin = [2]int{21461, 34442} // {built, cards}

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

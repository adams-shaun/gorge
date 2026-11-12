package oracle

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// projchars.go is the wider projection: each
// battlefield object's derived types, subtypes, colours and keywords,
// compared beside its P/T. The gorge side maps gorge's derived strings
// through the matchup's kernel vocabulary (charVocab, set by RealKernel);
// with no kernel vocabulary (gorge against gorge) the fields stay zero.

// CharsProj is one battlefield object's derived characteristics.
type CharsProj struct {
	Types    uint16      // kop.TypeMask, TToken excluded
	Subtypes kop.SubMask // over the kernel table's Subtypes
	AllTypes bool        // every creature type (an all-creature-types grant)
	Colors   uint8       // kop.ColorMask WUBRG
	Keywords uint32      // kop.KwMask
}

// kwStaticBits are kernel keyword bits gorge models as a static ability
// (CantBlockBy, CantBlock, PreventDamage), never as a keyword in its derived
// list: they are compared through behaviour (the attack/block options and
// combat damage), not here. Both sides mask them out.
const kwStaticBits = kop.KwUnblockable | kop.KwCantBlock | kop.KwWither |
	kop.KwPreventCombatDamageTo | kop.KwPreventCombatDamageFrom

// charVocab is the kernel table's vocabulary the gorge side maps into.
type charVocab struct {
	sub map[string]int // subtype name -> Table.Subtypes index
	// creat are the vocabulary's creature types (gorge
	// effects.CreatureTypeWordList): an all-creature-types object has each
	// (CR 702.73a), which gorge's derived type words may not list (a
	// changeling token, kxpaid's Springleaf Parade)
	creat kop.SubMask
}

func newCharVocab(tbl *kop.Table) *charVocab {
	v := &charVocab{sub: map[string]int{}}
	for i, n := range tbl.Subtypes {
		if i < kop.MaxSubtypes {
			v.sub[n] = i
		}
	}
	for _, w := range effects.CreatureTypeWordList() {
		if i, ok := v.sub[w]; ok {
			v.creat = v.creat.With(i)
		}
	}
	return v
}

// gorgeTypeBits are gorge's type words with a kop.TypeMask bit.
var gorgeTypeBits = map[string]kop.TypeMask{
	"Land": kop.TLand, "Creature": kop.TCreature, "Artifact": kop.TArtifact, "Enchantment": kop.TEnchantment,
	"Planeswalker": kop.TPlaneswalker, "Instant": kop.TInstant, "Sorcery": kop.TSorcery, "Battle": kop.TBattle,
	"Kindred": kop.TKindred, "Tribal": kop.TKindred, "Aura": kop.TAura, "Equipment": kop.TEquipment,
	"Basic": kop.TBasic, "Legendary": kop.TLegendary,
}

// gorgeKwExact are gorge keyword spellings with a kop.KwMask bit (kgen's
// kwBits plus feat.KeywordNames' granted spellings).
var gorgeKwExact = map[string]kop.KwMask{
	"Flying": kop.KwFlying, "Reach": kop.KwReach, "First Strike": kop.KwFirstStrike, "Double Strike": kop.KwDoubleStrike,
	"Deathtouch": kop.KwDeathtouch, "Lifelink": kop.KwLifelink, "Trample": kop.KwTrample, "Vigilance": kop.KwVigilance,
	"Haste": kop.KwHaste, "Menace": kop.KwMenace, "Defender": kop.KwDefender, "Flash": kop.KwFlash,
	"Hexproof": kop.KwHexproof, "Indestructible": kop.KwIndestructible, "Prowess": kop.KwProwess,
	"Protection from everything": kop.KwProtectionAll, "Shroud": kop.KwShroud,
	"Unblockable": kop.KwUnblockable, "CantBlock": kop.KwCantBlock,
	"CARDNAME can't block.": kop.KwCantBlock,
	// Decayed (CR 702.147a): kgen lowers it as KwCantBlock plus the
	// Attacks trigger over the end-of-combat sacrifice (lower_decayed.go)
	"Decayed":                      kop.KwCantBlock,
	"HIDDEN CARDNAME can't block.": kop.KwCantBlock,
	// an Animate HiddenKeywords$ grant; the kernel's
	// KwCantBlock plus the override record's CantAttack (masked like
	// every kwStaticBits keyword: compared through the combat options)
	"CARDNAME can't attack or block.": kop.KwCantBlock,
	// a Pump KW$ grant (Alchemist's Vial): the same kernel pair
	"HIDDEN CARDNAME can't attack or block.": kop.KwCantBlock,
	"Hexproof:Instant":                       kop.KwHexproofFromInstants, "HexproofFromInstants": kop.KwHexproofFromInstants,
	"Protection from black": kop.KwProtBlack, "ProtectionFromBlack": kop.KwProtBlack,
	"Protection from red": kop.KwProtRed, "ProtectionFromRed": kop.KwProtRed,
	"Prevent all combat damage that would be dealt to CARDNAME.":              kop.KwPreventCombatDamageTo,
	"Prevent all combat damage that would be dealt to and dealt by CARDNAME.": kop.KwPreventCombatDamageTo | kop.KwPreventCombatDamageFrom,
	"PreventCombatDamageTo": kop.KwPreventCombatDamageTo, "PreventCombatDamageFrom": kop.KwPreventCombatDamageFrom,
	"FlashbackOwnCost": kop.KwFlashbackOwnCost,
	"Fear":             kop.KwFear, "Shadow": kop.KwShadow,
}

// gorgeKwPrefix are parameterised gorge keywords ("Ward:2") with a bit.
var gorgeKwPrefix = []struct {
	p string
	m kop.KwMask
}{
	{"Ward:", kop.KwWard}, {"Kicker:", kop.KwKicker}, {"Flashback:", kop.KwFlashback}, {"Flashback", kop.KwFlashback},
}

// gorgeKwElsewhere are gorge keywords the kernel models as something other
// than a KwMask bit (named, never silently dropped): the subtype set
// (Changeling), a CardDef field (Offspring, Gift, Landwalk, Enchant, Equip)
// or an ability. Only the prefix before ':' (or the whole word) is listed.
var gorgeKwElsewhere = map[string]string{
	"Changeling":               "the subtype set (kgen's changelingSubs; CharsProj.Subtypes)",
	"Beam me up":               "CardDef.BeamMeUp (a graveyard cast for AltCost with a return; kw_beammeup.go)",
	"Harmonize":                "CardDef.Harmonize (a graveyard cast for AltCost; kw_harmonize.go)",
	"Mayhem":                   "CardDef.Mayhem (a graveyard cast for AltCost if discarded this turn; kw_mayhem.go)",
	"Offspring":                "CardDef.Offspring (an optional additional cost and its enters trigger)",
	"Storm":                    "CardDef.Storm (the printed keyword's native cast trigger, dirac/kw_storm.go; a StatStackKw grant, static_stackkeyword.go)",
	"Mobilize":                 "gorge kwMobilize's Attacks trigger: a remembering Token plus a group DelayedTrigger (kgen lowers the expansion's nodes; OpDelayedProgram DpGroup, ExtSacDelayed)",
	"Devoid":                   "a colourless face in every zone (kgen lower.go faceHasKeyword Devoid; CharsProj.Colors)",
	"Equip":                    "CardDef.EquipCost (an activated ability)",
	"Enchant":                  "CardDef.Enchant (the Aura's target spec)",
	"etbCounter":               "an enters-with-counters replacement (ability)",
	"Sunburst":                 "an enters-with-counters replacement (ability; gorge kwSunburst's Count$Converge counters)",
	"ETBReplacement":           "an as-enters replacement (ability)",
	"MayEffectFromOpeningHand": "CardDef.Leyline",
	"Crew":                     "an activated ability with a CostTapPower cost (gorge kwCrew's expansion; C6)",
	"Saddle":                   "an activated ability with a CostTapPower cost and OpSaddle (gorge kwSaddle's expansion; C6)",
	"Station":                  "an activated ability flagged AfStation (gorge rules/station.go; kcounters)",
	"A deck can have any number of cards named CARDNAME.": "a deckbuilding rule, no game characteristic",
	"Partner":            "a deckbuilding rule (CR 702.124, a commander pair), no game characteristic; the kernel seats the pair from Config.Commanders",
	"Doctor's companion": "a deckbuilding rule (CR 702.124, a commander pair), no game characteristic; the kernel seats the pair from Config.Commanders",
	"Landwalk":           "CardDef.Landwalk, or a Continuous static's Static.Landwalk grant (CR 702.14; compared through the block options)",
	"Horsemanship":       "CardDef.Evasion (compared through the block options)",
	"Intimidate":         "CardDef.Evasion (compared through the block options)",
	"Wither":             "CardDef.Wither (compared through the -1/-1 counters)",
	"Undying":            "gorge kwUndying's dies trigger (kgen expandedKeywords; compared through the zone moves and the +1/+1 counters)",
	"Exalted":            "gorge's expanded attacks-alone trigger (kgen expandedKeywords, Tf2Alone; compared through the pump on the stack and P/T)",
	"Prevent all damage that would be dealt to CARDNAME.": "CardDef.PreventDmgTo (CR 615.1; compared through the damage behaviour)",
	"CantAttackAlone":        "CardDef.AloneAtk (CR 506.5; compared through the attack options and Done, combat_alone.go)",
	"CantBlockAlone":         "CardDef.AloneBlk (CR 506.5; compared through the block options and Done, combat_alone.go)",
	"CantAttackOrBlockAlone": "CardDef.AloneAtk and CardDef.AloneBlk (CR 506.5; compared through both declaration option lists and Done, combat_alone.go)",
	"Warp":                   "CardDef.HasWarp (a hand alt cost and the exile recast; compared through the cast options)",
	"Plot":                   "CardDef.HasPlot (a hand special action and the exile free cast; compared through the cast options)",
	"Bestow":                 "CardDef.HasBestow (a hand alt cost, the Aura spell and the attached type switch; compared through the cast options, the types and the attachments)",
	"Sneak":                  "CardDef.HasSneak (a declare-blockers hand alt cost; compared through the cast options and the entry's tapped and attacking flags)",
	"MayFlashCost":           "CardDef.HasMayFlash (an instant-timing hand cast for the mana cost plus the extra; compared through the cast options; kw_mayflash.go)",
	"Web-slinging":           "CardDef.HasWebSling (a hand alt cost plus returning a tapped creature; compared through the cast options and the return; kw_websling.go)",
	"Dash":                   "CardDef.HasDash (a hand alt cost, haste and the end-step return to hand; compared through the cast options, the stack and the battlefield; kw_dash.go)",
	"Evoke":                  "CardDef.HasEvoke (a hand alt cost and the sacrifice trigger queued as it enters; compared through the cast options and the stack; kw_evoke.go)",
	"Start your engines":     "CardDef.Engines (CR 702.179a; compared through the speed projection)",
	"Bargain":                "CardDef.Bargain (an optional additional cost, no battlefield characteristic; compared through the cast options; kw_bargain.go)",
	"Infect":                 "CardDef.Infect (compared through the counters and poison)",
	"Toxic":                  "CardDef.Toxic (compared through poison)",
	"Ravenous":               "an enters-with-counters replacement and an enters trigger reading the paid X (gorge's Ravenous expansion; kxpaid)",
	"You may choose not to untap CARDNAME during your untap step.": "ReplMayNotUntap (the untap-step election, ChGenKeepTapped; compared through the choose options and the tapped flag, repl_untapchoice.go)",
	"HIDDEN This card doesn't untap during your next untap step.":  "the one-shot perm flag pfSkipUntap (compared through the untap step)",
	"Skulk":                             "CardDef.Evasion EvSkulk (printed only)",
	"Affinity":                          "a ReduceCost static reading the artifact count (kgen expandedKeywords: gorge kwAffinity's expansion; no kernel card sets the KwAffinity bit, so the printed keyword is no battlefield characteristic; typesall: Sojourner's Companion)",
	"Cycling":                           "a hand activated ability (CostDiscardSelf, AfCycling; gorge kwCycling's expansion), no battlefield characteristic",
	"AlternateAdditionalCost":           "CardDef.SpellCost / AltAddCost (a cast-time additional cost, no battlefield characteristic; Dusk Rose Reliquary)",
	"Impending":                         "CardDef.Impending (a hand alt cost, the entry time counters and the not-a-creature switch, compared through the cast options and the types)",
	"Unearth":                           "a graveyard activated ability (Return of itself then OpUnearth: haste, the end-step exile and the exile-instead promise; gorge kwUnearth's expansion), no battlefield characteristic",
	"Training":                          "an Attacks trigger with Trigger.N TrigNTraining (gorge kwTraining's expansion), no battlefield characteristic",
	"Firebending":                       "an Attacks trigger whose AddMana has B = 1 (State.fbR; gorge kwFirebending's expansion, CR 702.189a), no battlefield characteristic",
	"Melee":                             "an Attacks Self trigger pumping its holder: a printed one from gorge kwMelee's expansion, a static's granted one kop.Ability.MeleeGrant (kw_melee.go; CR 702.121a), no battlefield characteristic",
	"Increment":                         "a SpellCast trigger with Tf2Increment (gorge kwIncrement's expansion), no battlefield characteristic",
	"Craft":                             "a battlefield activated ability (the ExileCtrlOrGrave material and Exile<1/CARDNAME> costs, then the transformed Return; gorge kwCraft's expansion), no battlefield characteristic",
	"Embalm":                            "a graveyard activated ability (CostExileGrave of itself, OpCopyAsTokenEx with a white / Zombie / no-mana-cost exception; gorge kwEmbalm's expansion), no battlefield characteristic",
	"TypeCycling":                       "a hand activated ability (CostDiscardSelf, AfCycling, OpSearchEx; gorge kwTypeCycling's expansion), no battlefield characteristic",
	"Class":                             "the Class level activators and the ClassBand-gated abilities (CtrLevel, kw_class.go), no battlefield characteristic",
	"Chapter":                           "the chapter abilities (TrigCounterAddedOnce on LORE, kw_saga.go), no battlefield characteristic",
	"Convoke":                           "CardDef.Convoke (a cast-time payment, no battlefield characteristic; kw_convoke.go)",
	"Improvise":                         "CardDef.Improvise (a cast-time payment, no battlefield characteristic; kw_convoke.go)",
	"Delve":                             "CardDef.Delve (a cast-time payment, no battlefield characteristic; kw_delve.go)",
	"Morph":                             "CardDef.MorphCost/MorphFamily (the face-down cast and turn-up special action)",
	"Megamorph":                         "CardDef.MorphCost/MorphFamily (the face-down cast and turn-up special action)",
	"Disguise":                          "CardDef.MorphCost/MorphFamily (the face-down cast and turn-up special action)",
	"Cascade":                           "CardDef.Cascade (a native cast trigger; compared through the free-cast election and the exile moves)",
	"Ninjutsu":                          "a hand activated ability (AfNinjutsu, CostReturn, OpNinjutsu; gorge kwNinjutsu's expansion), no battlefield characteristic",
	"Protection":                        "CardDef.ProtFrom (a printed K:Protection:<spec> quality; compared through the target, block and damage options)",
	"Living Weapon":                     "an enters trigger making a 0/0 black Phyrexian Germ and attaching the Equipment to it (gorge's Living Weapon expansion; phyact: Lashwrithe), no battlefield characteristic",
	"Daybound":                          "CardDef.Daybound (the day/night designation and the faces it turns, kw_daynight.go; compared through the face and the day/night projection)",
	"Nightbound":                        "CardDef.Nightbound (as Daybound)",
	"AllMustBlock":                      "CardDef.Lure (CR 509.1c; compared through the block options and Done, combat_lure.go)",
	"MustBlock":                         "CardDef.MustBeBlocked (CR 509.1c; compared through the block options and Done, combat_mustbeblocked.go)",
	"CARDNAME must be blocked if able.": "the arena mark mechMustBlocked an Animate's HiddenKeywords$ sets (ExtMustBeBlocked; compared through the block options and Done)",
	"HIDDEN CARDNAME must be blocked if able.": "the arena mark mechMustBlocked a Pump / PumpAll KW$ sets (ExtMustBeBlocked; compared through the block options and Done)",
	"Hideaway":          "its ETB trigger (OpExt ExtHideaway; the keyword has no other rules effect)",
	"Backup":            "gorge kwBackup's expansion: an ETB trigger putting a +1/+1 counter and granting the attack trigger (kgen lowers the expansion's nodes; animgb_test.go), no battlefield characteristic",
	"Job select":        "gorge kwJobSelect's expansion: an enters trigger making a 1/1 Hero token and attaching the Equipment to it (kgen lowers the expansion's nodes; compared through the stack, the token and the attachment), no battlefield characteristic",
	"Battle cry":        "gorge kwBattleCry's expansion: an Attacks trigger pumping the other attackers (kgen lowers the expansion's nodes; compared through the stack and the attackers' power), no battlefield characteristic",
	"Cumulative upkeep": "gorge kwCumulativeUpkeep's expansion: an upkeep trigger putting an age counter and charging the generic cost once per counter, else sacrificing (ExtCumulative, dirac/trig_cumulative.go; compared through the age counters, the pay and decline options and the sacrifice), no battlefield characteristic",
	"Annihilator":       "gorge kwAnnihilator's expansion: an Attacks trigger making the defending player sacrifice N permanents (kgen lowers the expansion's node, Annihilator$ N as the Sacrifice count; compared through the stack and the sacrifice choice), no battlefield characteristic",
	"Gift":              "CardDef.Gift (a cast-time promise and its GiftAbility; compared through the gift election and the stack; kw_gift.go), no battlefield characteristic",
}

// gorgeKw maps one gorge keyword to its kernel bit. ok is false for a
// keyword with no kernel bit and no listed elsewhere-model: a projection
// error.
func gorgeKw(s string) (kop.KwMask, bool) {
	if b, ok := gorgeKwExact[s]; ok {
		return b, true
	}
	for _, e := range gorgeKwPrefix {
		if strings.HasPrefix(s, e.p) {
			return e.m, true
		}
	}
	if q, ok := strings.CutPrefix(s, "Hexproof:"); ok {
		switch q {
		case "white", "blue", "black", "red", "green", "White", "Blue", "Black", "Red", "Green":
			// hexproof from a chosen colour (Skrelv; genState.colG,
			// compared through the target options), or a printed
			// K:Hexproof:<Colour> (CardDef.HexColors)
			return 0, true
		}
	}
	head := s
	if i := strings.IndexByte(head, ':'); i >= 0 {
		head = head[:i]
	}
	if _, ok := gorgeKwElsewhere[head]; ok {
		return 0, true
	}
	return 0, false
}

// gorgeChars is battlefield object oid's CharsProj from gorge's derived
// characteristics; err names the first keyword with no kernel bit.
func gorgeChars(e *rules.Engine, v *charVocab, oid state.ObjID) (CharsProj, string) {
	d := e.Derived(oid)
	// Printed protection from a quality is CardDef.ProtColors/ProtFrom, not
	// a KwMask bit (the kernel sets only KwProtBlack/KwProtRed, and only for
	// granted protection): drop the face's printed spellings so a card like
	// Ureni, the Song Unending ("protection from white and from black") is
	// compared through the target, block and damage options. Protection from
	// everything stays a keyword bit.
	var printed map[string]bool
	if o := e.G.Obj(oid); o != nil && o.Face() != nil {
		printed = make(map[string]bool, len(o.Face().Keywords))
		for _, k := range o.Face().Keywords {
			printed[k] = true
		}
	}
	kws := d.Keywords
	if len(printed) > 0 {
		kws = make([]string, 0, len(d.Keywords))
		for _, k := range d.Keywords {
			if printed[k] && strings.HasPrefix(k, "Protection from ") && k != "Protection from everything" {
				continue
			}
			kws = append(kws, k)
		}
	}
	c, kw := charsOf(v, d.Types, d.AllCreatureTypes, d.Colors, kws)
	if kw != "" {
		return c, fmt.Sprintf("gorge keyword %q (on %s) has no kernel bit", kw, cardName(e.G.Obj(oid).Card))
	}
	return c, ""
}

// charsOf maps gorge's derived type words, all-creature-types mark, colour
// letters and keywords through v; bad is the first keyword with no kernel
// bit ("" when none).
func charsOf(v *charVocab, types []string, all bool, colors string, kws []string) (c CharsProj, bad string) {
	var t kop.TypeMask
	for _, w := range types {
		if b, ok := gorgeTypeBits[w]; ok {
			t |= b
		}
		if i, ok := v.sub[w]; ok {
			c.Subtypes = c.Subtypes.With(i)
		}
	}
	c.Types = uint16(t)
	c.AllTypes = all
	if all {
		c.Subtypes = c.Subtypes.Or(v.creat)
	}
	for _, r := range colors {
		switch r {
		case 'W':
			c.Colors |= uint8(kop.ColW)
		case 'U':
			c.Colors |= uint8(kop.ColU)
		case 'B':
			c.Colors |= uint8(kop.ColB)
		case 'R':
			c.Colors |= uint8(kop.ColR)
		case 'G':
			c.Colors |= uint8(kop.ColG)
		}
	}
	for _, k := range kws {
		b, ok := gorgeKw(k)
		if !ok && bad == "" {
			bad = k
		}
		c.Keywords |= uint32(b &^ kwStaticBits)
	}
	return c, bad
}

package kgen

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// ignoredParams are descriptive, AI-hint or tag parameters with no rules
// meaning: lowering never reads them and their presence never refuses.
var ignoredParams = map[string]bool{
	"SpellDescription": true, "StackDescription": true, "TgtPrompt": true, "PrecostDesc": true,
	"AILogic": true, "AITgts": true, "AIPreference": true, "IsCurse": true, "ChangeTypeDesc": true,
	"DiscardValidDesc": true, "ValidDescription": true, "TriggerDescription": true, "Description": true,
	"KeywordLine": true, "Secondary": true, "Image": true, "ChoicePrompt": true, "AIPowerScale": true,
	"AIPreferenceOverride": true, "Ultimate": true, "MayPlayText": true, "Keyword": true,
	"ConditionDescription": true, "SelectPrompt": true,
	"GiftDescription": true, "CostDesc": true, "ValidTgtsDesc": true,
	"SacMessage":         true, // Forge SacrificeEffect's prompt text; gorge never reads it
	"DefinedDesc":        true, // ChangeZone caption text ("that card"); gorge lists it in params.DescriptiveKeys (descriptive.go), never read
	"AIPhyrexianPayment": true, // Forge's AI hint for paying a Phyrexian pip with life; gorge never reads it
	"ChangeValidDesc":    true, // Dig/ChangeZone caption text ("nonland"); gorge lists it in params.DescriptiveKeys (descriptive.go), never read
	"IfDesc":             true, // caption text; gorge lists it in params.DescriptiveKeys (descriptive.go), never read
	"NoPutDesc":          true, // caption text; gorge lists it in params.DescriptiveKeys (descriptive.go), never read
	"PrimaryPrompt":      true, // caption text; gorge lists it in params.DescriptiveKeys (descriptive.go), never read
	"AINoRecursiveCheck": true, // Forge AI hint: only in gorge's accepted-key lists (params/mana.go, params/dealdamage.go, changezone_params.go), never read
	"AIXMax":             true, // Forge AI hint for the X it announces: only in gorge's accepted-key lists (changezone_params.go, params/*.go), never read
	"UnlessAI":           true, // Forge AI hint (CopySpellAbilityAi / the unless-cost AI: whether the AI pays or copies); gorge never reads it (engine/rules/paramcensus_test.go records it as an AI hint)
}

// params tracks which parameters of one IR node the lowering read, so an
// unread rules parameter refuses instead of being silently dropped.
type params struct {
	m    map[string]string
	used map[string]bool
}

func newParams(m map[string]string) *params { return &params{m: m, used: map[string]bool{}} }

func (p *params) get(k string) (string, bool) {
	v, ok := p.m[k]
	if ok {
		p.used[k] = true
	}
	return v, ok
}

func (p *params) str(k string) string { v, _ := p.get(k); return strings.TrimSpace(v) }

func (p *params) is(k, want string) bool { return p.str(k) == want }

// sortedParamKeys returns a params map's keys in sorted order. A refusal that
// names the first unlisted key must iterate this, not the map, so the named
// key is the same on every run.
func sortedParamKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// consume marks keys read whose value the caller has checked some other way.
func (p *params) consume(ks ...string) {
	for _, k := range ks {
		if _, ok := p.m[k]; ok {
			p.used[k] = true
		}
	}
}

func (p *params) unread() []string {
	var out []string
	for k := range p.m {
		if !p.used[k] && !ignoredParams[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// errRefuse is a lowering refusal raised inside a node.
type errRefuse struct{ reason string }

func (e errRefuse) Error() string { return e.reason }

func refuse(format string, a ...any) error { return errRefuse{fmt.Sprintf(format, a...)} }

// lowerCtx lowers one card (or token).
type lowerCtx struct {
	capRem   bool  // lowering the body of a capturing emblem's DamageDealtOnce trigger: a Remembered read before the body writes its own set is the captured object, the damage source (lower_effectcaptured.go)
	effCount bool  // lowering the body of an Effect trigger that remembers a count: Remembered$Amount reads the frozen trigger amount (lower_effectremember.go)
	pairCap  *lRef // the capture slot the pair lowering already resolved for the emblem (lower_effectcaptured.go)

	// permOrGraveCard: lowering a ValidTgts$ ChangeZone over Origin$ Battlefield,Graveyard (lower_originmulti.go)
	permOrGraveCard bool
	// graveExileCard: lowering a ValidTgts$ ChangeZone over Origin$ Graveyard,Exile (lower_graveexiletgt.go)
	graveExileCard bool

	// deferTrig: the TriggersWhenSpent$ rider the face's deferred mana ability queued (lower_deferredmana.go)
	deferTrig *cards.Trigger
	// fuseAlt: lowering a fused split spell, the right half lowerFace folds in after the left half's lines (lower_fuse.go)
	fuseAlt *cards.Face

	firstCastSVar  string    // the SVar a static SpellCast StoreSVar of Count$YouCastThisGame writes (lower_firstcast.go)
	firstCastTrig  int       // that trigger's index + 1: it lowers to nothing
	stormTrig      int       // the K:Storm expansion trigger's index + 1: it lowers to nothing (lower_storm.go)
	imprintMox     bool      // the face is the imprint-mox shape: its two static bookkeeping triggers lower to nothing (lower_imprintmox.go)
	bluffPlay      bool      // lowering the Play behind a nothing-remembered gate: ValidSA$ Spell.nonCreature+YouOwn may lower (lower_zone.go, Brazen Bluff; Yue's hand population, playOwnHand)
	animAll        bool      // lowering an AnimateAll: animateRemoveKeywords accepts a bare RemoveKeywords$ (gorge effAnimateAll reads it)
	speedRepl      bool      // lowering a max-speed static's granted replacement: damagePlus reads ValidTarget$ Player.Opponent (lower_maxspeedrepl.go)
	replEnter      bool      // inside an ETB replacement's ReplaceWith chain
	breachStage    int       // Breach the Multiverse's imprint chain: 1 the ChangeZone of Imprinted is next, 2 the Cleanup (lower_repeatgravechoice.go)
	exileLandStage int       // Ensign Pavel Chekov's imprint chain: 1 the Imprinted Effect is next, 2 the Cleanup (lower_repeatexilechoice.go)
	blind          *blindDig // a blind face-down Dig whose Effect lowers next (lower_blinddig.go)
	grantInner     []lStatic // a station self grant's inner statics, emitted after the outer static (lower_stationgrant.go)
	station        string    // K:Station's parameter (lower_station.go)
	beholdOK       bool      // the cost being read is a spell's AlternateAdditionalCost first part or a MayFlashCost extra: Behold<N/Type> may lower (kernel act_behold.go)
	unblockedOK    bool      // the filter being read is K:Ninjutsu's Return cost: "unblocked" may lower (lower_ninjutsu.go)
	exwSynth       bool      // the ExiledWithSource filter is synthesised for a Defined$ ExiledWith (the card's text does not name the word)
	batchPoolOK    bool      // the trigger being lowered is a ChangesZoneAll exile line whose body picks among its batch members (lower_trigzonebatch.go kayaBatchBody)
	wasCastOK      bool      // the filter being read is a battlefield-entering trigger's ValidCard(s)$: wasCast may lower
	cantCastProv   bool      // the filter being read is a CantBeCast ValidCard$: wasCastFromTheirHand may lower (kernel matchSpellCardOffer)
	spellProvOK    bool      // the filter being read is a SpellCast ValidCard$, a spell's own ConditionPresent$ or a mana ability's Spell. RestrictValid$: the cast-provenance words may lower
	chosenNumOK    bool      // the filter being read is a battlefield SpellCast ValidCard$ of a card that chooses a number as it enters: a "Chosen" bound may lower (lower_choosenumber.go)
	exwOK          bool      // the filter being read is a resolution-time mover's ChangeType$: ExiledWithSource may lower
	exwStatic      bool      // lowering a Mode$ Continuous static: ExiledWithSource reads the static's own source
	spellBaseOK    bool      // the filter being read is a ReduceCost ValidTarget$: a Spell head may lower (lower_trigger.go)
	trigBodyCost   bool      // the cost parser is reading a printed trigger's own body Cost$ (PayLife<X> by a value, lowerCtx.effect)
	trigOwnCost    bool      // payInstr: the cost is a printed trigger's own Cost$ (first instruction of its body)
	trigXMax       *lValue   // payInstr: the XMax$ cap of a trigger's own announced {X} cost (Shanna); nil = none read
	trigRemX       bool      // the ImmediateTrigger's RememberSVarAmount$ X is the announced X (lower_zone.go immediateTrigger)
	trigXPaid      bool      // the body's Count$xPaid is that announced X (set once the PayManaX cost lowered)
	xPaidAb        *lAbility // the ability whose own cost carries {X}: a bare Amount$ X is its paid X (gorge count.go raw "X" -> Ctx.X)
	wbOK           bool      // costParts: a Waterbend<N> part may lower (an UnlessCost$ read by unless(); payInstr gates the shape)
	unlessCost     bool      // payInstr: the cost is an UnlessCost$ of a non-delayed body
	unlessParse    bool      // costHand: the tokens are an UnlessCost$ (ExileFromStack)
	wardCost       bool      // payInstr: the cost is a Ward keyword's UnlessCost$
	unlessSubs     []usGate  // UnlessResolveSubs$ gates awaiting their chain level's end (lower_unlesssubs.go)
	reflexMulti    bool      // lowering a NumTimes reflexive latent: a multi-pick charm may lower on it (lower_numtimes.go)
	numTimesK      int       // lowering a NumTimes latent for the announced count k: Count$TriggerRememberAmount is k (lower_numtimes.go)
	remLKMVN       int       // RememberedLKI$CardManaCost reads admitted (value_remlkimv.go)
	remLKN         int       // RememberedLKI$ P/T reads lc.value produced: refuseRemLKIPT refuses unless each one sits in an ability program (value_remlki.go)
	flipSide       uint8     // lowering a FlipCoin branch: 1 the win block, 2 the lose block; a bare Wins/Losses read is that flip's 1/0 (lower_dice.go flipCoin)
	flipTallyAb    *lAbility // the ability whose FlipCoin consumed RememberNumber$: its Count$RememberedNumber is the unlowered flip tally, refused (value.go)
	p              *pool
	name           string
	card           *cards.Card
	face           *cards.Face
	out            *lCard
	ab             *lAbility // the ability being lowered
	mode           int       // the charm mode being lowered (-1 none)
	chainT         []int     // target slots declared by the current SA chain, in order
	remTgt         int       // target slots a Pump RememberObjects$ ThisTargetedCard remembered (bit per slot; lower_remembered.go)
	condDone       bool      // the node's Condition*$ was lowered into its op (ChangeSpeed): nodeCondition skips it once
	atRes          bool      // the node being lowered declares its targets mid-resolution
	depth          int       // chain nesting (1: the ability's own chain)
	head           bool      // the node being lowered heads its chain
	inGroup        bool      // lowering a Mobilize group program's body (SacrificeDelayed is legal)
	czForgetSet    bool      // a ChangeZoneAll ForgetOtherRemembered$ ran in this ability: Remembered is a many-object set (lower_changezoneall.go)
	chains         int       // chain entries so far (every-blocker census: did a node's body get read?)
	guardNode      string    // the top-level node guard is lowering (every-blocker census names extra refusals by it)
	curNode        *cards.SA // the SA node being lowered (its Sub is the rest of the chain)
	condDefined    bool      // defined() is reading a ConditionDefined$ (lower_triggerattackers.go)
	own            ownTgt    // the slot curNode declared itself (lower_owntarget.go)
	chainRoot      *cards.SA // the root of the outermost chain being lowered (chain, depth 1)
	fbGranted      bool      // lowering a granted Firebending N trigger (lower_firegrant.go): manaEffect needs no ElementalBend marker
	mlTrigger      bool      // lowering a Melee trigger, printed or granted (lower_meleegrant.go): its pump reads cards.MeleePumpCount
	fbMarker       *cards.SA // the Firebending expansion's DB$ ElementalBend node manaEffect vetted (elementalBend lowers only it)
	// digImprinted: this chain's DigUntil imprinted its found card and
	// remembered every revealed card, Jodah's shape (lower_digimprint.go): the
	// Play of the Imprinted card and the random-order bottom of the
	// Remembered exile read it
	digImprinted bool
	// playLibrary: this chain's Play casts from the remembered cards in the
	// library (Cosmic Cube; lower_playlibrary.go): the random-order bottom of
	// the rest reads it
	playLibrary bool
	// digImprintFoundMode: this chain's DigUntil imprinted its found cards
	// (ImprintFound$ alone, Black Widow, Super Spy; lower_digimprint.go): the
	// chain's optional PutCounter, the Effect's RememberObjects$ Imprinted
	// and the Cleanup's ClearImprinted$ read it
	digImprintFoundMode bool
	// digRememberedLibrary: this chain's DigUntil remembered every revealed
	// card and left them in the library (RememberRevealed$ with
	// RevealedDestination$ Library, Sanar; lower_digimprint.go): the chain's
	// ChooseFromDefined$ Remembered.<colour> picks read it
	digRememberedLibrary bool
	// digImprintChosen: the chain's imprint is the kernel's chosen set (set by
	// digImprintFound and digChosenLibraryPick): the Effect's
	// RememberObjects$ Imprinted, its MayPlay and the Cleanup's
	// ClearImprinted$ read it
	digImprintChosen bool
	// digImprintRoot: the chain root whose Dig imprinted the resolution's
	// imprinted set (Dig Imprint$ on a permanent's ability, Reno and Rude,
	// Rotisserie Elemental; lower_digimprintset.go): only that chain's
	// Effect, Defined$ Imprinted and Cleanup read it
	digImprintRoot *cards.SA
	known          func(string) bool
	// granted > 0 while an ability or trigger another object is granted
	// (an AddAbility$/AddTrigger$ SVar, an emblem's trigger) is lowered
	granted int
	// grantExile while the activated ability an Animate Abilities$ grants an
	// exiled card is lowered (lower_exilegrant.go): the one place an
	// ExileFromHand<1/Card> cost and a paid Play of Card.Self lower
	grantExile bool
	// altCostHand while a spell's own AlternativeCost static's Cost$ is
	// lowered: an ExileFromHand<1/Spec> part keeps its filter (lower_trigger.go)
	altCostHand bool
	// grantAttached while the trigger an Equipment's or Aura's static grants
	// its attached permanent is lowered (AffectedDefined$ Equipped/Enchanted)
	grantAttached bool
	// repeatPl > 0 while a RepeatEach-over-players body is lowered: its
	// Defined$ Player.IsRemembered is the repeated player (lower_repeatplayers.go)
	repeatPl int
	// repeatCard > 0 while a RepeatEach-over-cards body is lowered: its
	// Token AttachedTo$ Remembered is the repeated card (lower_repeatcards.go)
	repeatCard int
	// repeatOpp: that body repeats over the controller's opponents
	repeatOpp bool
	// repeatAllPl: that body repeats over every player (RepeatPlayers$ Player)
	repeatAllPl bool
	// joinForcesName is the accumulator SVar the current top-level node's
	// chain writes (lower_storesvar.go joinForcesStore), joinForcesReset once
	// its trailing write of 0 has been lowered; joinForcesNames every
	// accumulator any node of the card wrote
	joinForcesName  string
	joinForcesReset bool
	joinForcesNames map[string]bool
	// repeatCheckSVar is the SVar an enclosing gated Repeat's
	// RepeatCheckSVar$ names, while that body is lowered; repeatCheckAb is
	// its ability, so a StoreSVar$ after the loop (the reset) still matches
	// (lower_storesvar.go)
	repeatCheckSVar string
	repeatCheckAb   *lAbility
	// repeatStoredGuard: the body stored the gate SVar true under the
	// library-empty condition; repeat folds that stop into the gate
	repeatStoredGuard bool
	// repeatImprintBody: lowering the body of a RepeatDefined$ Imprinted
	// loop; a Dig Imprint$ on the spell may open the set when its reads sit
	// in the same ability (lower_digparams.go)
	repeatImprintBody bool
	// rollName is the ResultSVar$ of a RollDice lowered in ability rollAb:
	// a later bare amount of that name (not a card SVar) is the roll
	// (lower_combatstatics.go)
	rollName string
	rollAb   *lAbility
	// excess is the name of the ExcessSVar$ a DealDamage lowered in the
	// current chain publishes (gorge's resolution-scoped Excess SVar): a
	// later bare amount or ConditionCheckSVar of that name is the excess
	// damage (Violent Echoes). Cleared at each ability's chain.
	excess string
	// cloneRepl while an ETB copy replacement's body is lowered (its
	// Choices$ bound may read the cast's mana spent)
	cloneRepl bool
	// replSelf while a self ETB replacement's body is lowered
	// (its Count$Converge reads the entering spell)
	replSelf bool
	// replOptional: the ETBReplacement keyword line ends ":Optional"
	replOptional bool
	// trigBody while a triggered ability's body (not its header or
	// intervening-if condition) is lowered (kxpaid: Count$xPaid binding)
	trigBody bool
	// atkPresent while an Attacks trigger's own IsPresent$ condition is
	// lowered (Siege Dragon's Wall.DefenderCtrl; gorge
	// cr-trigcond-defenderctrl binds the event's defending player at
	// queue time, and the resolution check reads the captured one)
	atkPresent bool
	// attachedTgtOK while a node with no target slot of its own lowers a
	// Choices$ filter that may read AttachedTo Targeted (lower_filterreferents.go)
	attachedTgtOK bool
	// spawnDmg while an ImmediateTrigger body spawned by Card.Self's combat
	// damage to a player lowers: ControlledBy Spawner>TriggeredTarget is the
	// opponent (lower_filterwords.go)
	spawnDmg bool
	// spawnAtk while an ImmediateTrigger body spawned by Card.Self's Attacks
	// trigger lowers: ControlledBy Spawner>TriggeredDefendingPlayer is the
	// attack's defending player (lower_filterwords.go)
	spawnAtk bool
	// actCost while a printed activated ability's own Cost$ is lowered
	// (RemoveAnyCounter<N/Kind/Filter>, kop.CostSubAmong)
	actCost bool
	// escCost while a printed escape cost is lowered (lower_escape.go): its
	// Exile<N/Spec> battlefield part may stand beside the ExileFromGrave part
	// (CardDef.EscBfCost is a Cost of its own)
	escCost bool
	// lifeXSpell while a spell's own Cost$ is lowered: PayLife<X> (an
	// announced life part) may lower on a spell or an
	// activated ability (actCost) and nowhere else
	lifeXSpell bool
	// prevColor while a mid-resolution ChooseColor waits for its
	// prevent-the-chosen-colour Effect (Prismatic Strands)
	prevColor bool
	// colGrant while a mid-resolution ChooseColor's chosen-colour grants
	// (Skrelv: the Pump DefinedKW$ ChosenColor, the CantBlockBy Effect, the
	// ClearChosenColor Cleanup) are still to be lowered
	colGrant bool
	// clAt is 1 + the index of a CounterLose instruction whose following
	// Effect (the host-bound loss of abilities) is still to be read.
	clAt   int
	colTgt lRef // the chosen-colour Pump's target
	// targetedDefined is the node's Defined$ Targeted.<qualifier> while that
	// node is lowered (graveexile): only this exact value is accepted by defined(),
	// and node() guards the node on the target matching the qualifier.
	targetedDefined string
	// chosenOppAb is the ability whose one-opponent ChoosePlayer is lowered
	// ChosenPlayer reads the opponent in that ability only.
	chosenOppAb *lAbility
	// remOppLinks are the chain links after a Pump RememberObjects$
	// TargetedController whose Defined$ Remembered reads the one opponent
	// (lower_remembered.go pumpRememberOppController).
	remOppLinks []*cards.SA
}

// lowerCard lowers a card; a refusal is recorded in out.Refusals (the card
// is unsupported if any is).
func lowerCard(p *pool, name string, c *cards.Card, token bool) *lCard {
	out := &lCard{Name: name, Token: token}
	lc := &lowerCtx{p: p, name: name, card: c, out: out, mode: -1, known: p.knownSubtype}
	if len(c.Faces) != 1 || c.AlternateMode != "" {
		before := len(out.Refusals)
		if !lc.lowerFaces(c) { // lower_faces.go
			if len(out.Refusals) == before {
				lc.refuseNode("card", "multi-face card (%d faces, %q)", len(c.Faces), c.AlternateMode)
			}
			return out
		}
	} else {
		lc.face = c.Faces[0]
	}
	if token {
		out.printed = lc.face.Name
	}
	lc.lowerFace()
	lc.exwCheck() // lower_exw.go
	return out
}

// lowerFace lowers lc.face's record and abilities into lc.out.
func (lc *lowerCtx) lowerFace() {
	for _, body := range lc.face.SVars {
		if b, ok := strings.CutPrefix(body, "Count$"); ok && strings.EqualFold(strings.TrimSpace(b), "Converge") {
			// gorge faceWantsConverge: the planner declines it too
			// ("shape:mana_spent_reader")
			lc.out.SpendRead = true
		}
		if strings.Contains(body, "Count$CastTotalManaSpent") {
			// gorge faceWantsCastSpend: the cast planner declines the
			// face ("shape:mana_spent_reader")
			lc.out.SpendRead = true
		}
	}
	for _, k := range lc.face.Keywords {
		if cards.KeywordHead(k) == "Sunburst" {
			// gorge faceWantsConverge reads the printed keyword too: the
			// planner declines the cast as for a Count$Converge body
			lc.out.SpendRead = true
		}
	}
	lc.lowerRecord()
	// the face's printed mana abilities' produced colours (CardDef.
	// ProduceCol; lower_reflectedmana.go producibleMask): a Produce-mode
	// reflected ability's read
	lc.out.ProduceCol = producibleMask(lc.face)
	lc.out.Identity = kop.ColorMask(lc.card.ColourIdentity())
	lc.scanStorm() // lower_storm.go: K:Storm's expanded trigger, before the lines lower
	// lower_imprintmox.go: the imprint-mox shape, before the lines lower
	lc.imprintMox = lc.imprintMoxShape()
	lc.lowerKeywords()
	lc.lowerLines(true, false)
	if lc.fuseAlt != nil {
		lc.lowerFusedHalf() // lower_fuse.go
	}
	lc.lowerTail()
}

// lowerLines lowers lc.face's ability, trigger, static and replacement
// lines into lc.out; origin records each ability's gorge face index (a
// Room state def's other door has none: lower_rooms.go); perm skips the
// spell lines (a Room state def is a battlefield-only def).
func (lc *lowerCtx) lowerLines(origin, perm bool) {
	out := lc.out
	lc.scanFirstCast() // lower_firstcast.go
	for i, a := range lc.face.Abilities {
		if perm && a.Kind == "SP" {
			continue
		}
		n := len(out.Abilities)
		lc.guard(fmt.Sprintf("A[%d] %s$ %s", i, a.Kind, a.API), func() error {
			entered := lc.chains
			err := lc.lowerAbility(a)
			if err != nil {
				lc.probeBody(a, entered)
			}
			return err
		})
		if len(out.Abilities) > n && origin {
			out.Abilities[n].Origin, out.Abilities[n].OriginOK = i, true
		}
	}
	for i, t := range lc.face.Triggers {
		t := t
		n := len(out.Abilities)
		if lc.firstCastTrig == i+1 {
			continue // the stamp: dirac marks the spell when it is cast (lower_firstcast.go)
		}
		if lc.stormTrig == i+1 {
			continue // K:Storm's expansion: CardDef.Storm and the kernel's native trigger own it (lower_storm.go)
		}
		if lc.imprintMox && i > 0 {
			continue // the imprint mox's static Cleanup / ForgetImprinted$ bookkeeping: the kernel's exiled-with record (lower_imprintmox.go)
		}
		if t.Mode == "TapsForMana" {
			lc.guard(fmt.Sprintf("T[%d] Mode=%s", i, t.Mode), func() error { return lc.tapExtraStatic(t) })
			continue
		}
		lc.guard(fmt.Sprintf("T[%d] Mode=%s", i, t.Mode), func() error {
			entered := lc.chains
			err := lc.lowerTriggered(t, "")
			if err != nil {
				lc.probeBody(t.Effect, entered)
			}
			return err
		})
		if len(out.Abilities) > n && origin {
			out.Abilities[n].Origin, out.Abilities[n].OriginOK = 0x80|i, true
		}
	}
	if t := lc.deferTrig; t != nil {
		lc.deferTrig = nil
		lc.guard("T:TriggersWhenSpent", func() error { return lc.lowerTriggered(*t, "") }) // lower_deferredmana.go
	}
	for i, s := range lc.face.Statics {
		s := s
		lc.guard(fmt.Sprintf("S[%d] Mode=%s", i, s.Mode), func() error { return lc.lowerStatic(s) })
	}
	for i, r := range lc.face.Repls {
		r := r
		lc.guard(fmt.Sprintf("R[%d] Event=%s", i, r.Event), func() error {
			if r.Event == "Moved" && lc.out.Engines {
				// gorge's speed start (checkSpeedStart, emit.go) misses a
				// Start your engines! permanent whose entry rides a Moved
				// replacement (Gas Guzzler's "enters tapped"): the kernel
				// starts speed, gorge does not
				return refuse("Start your engines! beside a Moved replacement")
			}
			return lc.lowerRepl(r)
		})
	}
}

// lowerTail runs the card-level passes after the lines are lowered.
func (lc *lowerCtx) lowerTail() {
	if lc.station != "" {
		lc.guard("K:Station:"+lc.station, func() error { return lc.lowerStation(lc.station) })
	}
	lc.lowerGift()
	lc.refuseCrossAbilityRemembered()
	lc.refuseCrossAbilityImprinted()
	lc.refuseRememberedPower() // value_countheads.go
	lc.refuseMultiMana()
	lc.refuseCtxRememberedCount()
	lc.refuseRemLKIPT()                // value_remlki.go
	lc.refuseRemLKIMV()                // value_remlkimv.go
	lc.refuseRememberAmountObjects()   // lower_remembered.go
	lc.refuseRememberSearcherObjects() // lower_remembersearcher.go
	lc.numberOnceAbilities()
	lc.refusePlotCombos()
	lc.refuseHarmonizeCombos()
	lc.refuseReboundCombos() // lower_rebound.go
	lc.refuseMayhemCombos()
	lc.refuseEscapeCombos()
	lc.lowerRetrace()
	lc.refuseBeamCombos()
	lc.refuseSneakCombos()
	lc.refuseBuybackCombos() // lower_buyback.go
	lc.refuseWebSlingCombos()
	lc.refuseMayFlashCombos()
	lc.refuseEvokeCombos()
	lc.refuseDashCombos()          // lower_dash.go
	lc.refuseMultikickerCombos()   // K:Multikicker beside another cast route
	lc.refuseSuspendCombos()       // lower_suspend.go
	lc.refuseForetellCombos()      // lower_foretell.go
	lc.refuseSweepTgtPlayerMulti() // lower_sweeptargetplayer.go
	lc.lowerImpending()
	lc.lowerOverload() // lower_overload.go
}

// lowerImpending appends Impending's end-step tick after the printed lines
// (gorge grants it at entry, after the printed triggers) and refuses the
// keyword beside another cast route: the kernel's impended cast is one
// plain alternative mana cost of a single-face permanent card.
func (lc *lowerCtx) lowerImpending() {
	out := lc.out
	if out.Impending == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil ||
		out.Convoke || out.Improvise || len(lc.card.Faces) > 1 || out.Types&uint16(kop.TCreature) == 0 {
		lc.refuseNode("K:Impending", "Impending beside another cast route, or not a single-face creature card")
		return
	}
	lc.guard("K:Impending", lc.impendingTick)
}

// refuseMultikickerCombos refuses Multikicker beside any other cast cost
// route: the kernel models the multikicker count as the one extra cost of a
// kicked cast, and a second route (or a plain Kicker, whose count reads
// differently) would compete for that cast. The keyword itself is lowered
// by lowerKeywords.
func (lc *lowerCtx) refuseMultikickerCombos() {
	out := lc.out
	if out.Multikicker == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil ||
		out.AltAddCost != nil || out.Sneak != nil || out.Impending != nil || out.Bestow != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Offspring != nil || out.Gift || out.MorphCost != nil ||
		out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree ||
		out.Convoke || out.Improvise || len(lc.card.Faces) > 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Multikicker", "Multikicker beside another cast route or additional cost")
	}
}

// refusePlotCombos refuses the plot keyword beside any other cast cost
// route (the kernel's plot action pays one plain mana cost), on an {X}
// spell (its price has no X) or on a multi-face card (the plot action has
// no face route). The keyword itself is lowered by lowerKeywords; a card
// refused here keeps only its static record.
func (lc *lowerCtx) refusePlotCombos() {
	out := lc.out
	if out.Plot == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise {
		lc.refuseNode("K:Plot", "Plot beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Plot", "Plot on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Plot", "Plot on a multi-face card")
	}
}

// refuseHarmonizeCombos refuses the harmonize keyword beside any other cast
// route (its announcement is the convoke pick stage: Convoke or Improvise
// would pose it twice) or a flashback cost read after it. The keyword itself
// is lowered by lowerKeywords.
func (lc *lowerCtx) refuseHarmonizeCombos() {
	out := lc.out
	if out.Harmonize == nil {
		return
	}
	if out.Convoke || out.Improvise {
		lc.refuseNode("K:Harmonize", "Harmonize beside Convoke or Improvise")
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Sneak != nil || out.Impending != nil || out.Bestow != nil ||
		out.SpellCost != nil || out.AltAddCost != nil || out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Casualty != 0 || out.Cost.X != out.Harmonize.X {
		// a printed {X} with an X-less harmonize cost (or the reverse) reads
		// an X the graveyard cast never announces
		lc.refuseNode("K:Harmonize", "Harmonize beside another cast route or additional cost")
	}
	if out.Flashback != nil {
		lc.refuseNode("K:Harmonize", "Harmonize beside Flashback")
	}
}

// refuseMayhemCombos refuses the mayhem keyword beside any other cast route:
// Mayhem, Flashback and Harmonize all claim CardDef.AltCost, and the kernel's
// mayhem cast is one plain alternative mana cost (dirac/kw_mayhem.go).
func (lc *lowerCtx) refuseMayhemCombos() {
	out := lc.out
	if out.Mayhem == nil {
		return
	}
	if out.Flashback != nil || out.Harmonize != nil {
		lc.refuseNode("K:Mayhem", "Mayhem beside Flashback or Harmonize (all claim CardDef.AltCost)")
	}
	if out.Convoke || out.Improvise || out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Sneak != nil || out.Impending != nil ||
		out.Bestow != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Offspring != nil || out.Gift || out.MorphCost != nil ||
		out.Casualty != 0 || out.Cost.X {
		lc.refuseNode("K:Mayhem", "Mayhem beside another cast route or additional cost")
	}
}

// lowerRetrace refuses the retrace keyword beside another cast route and
// builds its additional cost. Retrace does not claim CardDef.AltCost (the
// printed base stays; only the discard folds on, dirac/kw_retrace.go), but
// the kernel canon reads ONE mode-2 graveyard cast kind per card
// (canon/translate.go): Flashback, Harmonize, Mayhem and Beam me up would
// all take the mode-2 target byte 0. The other routes and costs are ones the
// retrace offer walk does not price (dirac kw_retrace.go offerRetraceCasts
// prices the printed base and the discard alone): Convoke, Improvise and
// Delve fold credits into the base (gorge offerCastable), {X} needs the
// cast's xAsk, and the rest carry their own pick or mana cost on the cast.
func (lc *lowerCtx) lowerRetrace() {
	out := lc.out
	if !out.Retrace {
		return
	}
	if out.Flashback != nil || out.Harmonize != nil || out.Mayhem != nil || out.BeamMeUp != nil {
		lc.refuseNode("K:Retrace", "Retrace beside a mode-2 graveyard-cast keyword (canon/translate.go reads one)")
	}
	if out.Convoke || out.Improvise || out.Delve || out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Sneak != nil || out.Impending != nil ||
		out.Bestow != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Offspring != nil || out.Gift || out.MorphCost != nil ||
		out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.Madness != nil || out.Cost.X {
		lc.refuseNode("K:Retrace", "Retrace beside another cast route, additional cost or an {X} cost (the offer walk prices the printed base and the discard alone)")
	}
	// The discard part is built directly (lc.cost's costParts refuses a
	// filtered discard outside an activated ability): a bare Discard<1/Land>
	// is the whole cost, no other part can ride beside it.
	c, err := lc.costHand("Discard<1/Land>")
	if err != nil || c.DiscardF == nil || c.DiscardN != 1 ||
		c.Mana.Generic != 0 || c.Mana.Pips != [5]int{} || c.Mana.C != 0 || c.Mana.X || c.Mana.XX ||
		len(c.Mana.Hybrid) != 0 || len(c.Mana.Twobrid) != 0 || len(c.Mana.Phyrexian) != 0 || c.Mana.XColor != 0 {
		lc.refuseNode("K:Retrace", "retrace additional cost %q", "Discard<1/Land>")
		return
	}
	out.RetraceCost = &c
}

// sneakLowered gates the K:Sneak arm of lowerKeywords.
const sneakLowered = true

// refuseSneakCombos refuses the sneak keyword beside any other cast cost
// route (the kernel's sneak cast pays one plain mana cost plus the return),
// on an {X} spell or on a multi-face card, the refusePlotCombos shape.
func (lc *lowerCtx) refuseSneakCombos() {
	out := lc.out
	if out.Sneak == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Offspring != nil || out.Gift || out.MorphCost != nil {
		lc.refuseNode("K:Sneak", "Sneak beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Sneak", "Sneak on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Sneak", "Sneak on a multi-face card")
	}
}

// refuseWebSlingCombos refuses the web-slinging keyword beside any other
// cast cost route (the kernel's web-slinging cast pays one plain mana cost
// plus the return), on an {X} spell or on a multi-face card, the
// refuseSneakCombos shape.
func (lc *lowerCtx) refuseWebSlingCombos() {
	out := lc.out
	if out.WebSling == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree {
		lc.refuseNode("K:Web-slinging", "Web-slinging beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Web-slinging", "Web-slinging on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Web-slinging", "Web-slinging on a multi-face card")
	}
}

// refuseMayFlashCombos refuses the may-flash keyword beside any other cast
// cost route (the kernel's may-flash cast pays the plain mana cost plus the
// extra and nothing else), on an {X} spell, on a multi-face card, and on a
// spell with its own timing restriction (gorge's mayflashTimingOK still reads
// activationPhasesOK), the refuseWebSlingCombos shape.
func (lc *lowerCtx) refuseMayFlashCombos() {
	out := lc.out
	if out.MayFlash == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil || out.Evoke != nil || out.Dash != nil {
		lc.refuseNode("K:MayFlashCost", "MayFlashCost beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:MayFlashCost", "MayFlashCost on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:MayFlashCost", "MayFlashCost on a multi-face card")
	}
}

func (lc *lowerCtx) refuseNode(node, format string, a ...any) {
	lc.out.Refusals = append(lc.out.Refusals, refusal{Card: lc.name, Node: node, Reason: fmt.Sprintf(format, a...)})
}

// guard runs one top-level node's lowering, recording a refusal on error.
func (lc *lowerCtx) guard(node string, fn func() error) {
	saved, savedNode := lc.ab, lc.guardNode
	lc.guardNode = node
	defer func() { lc.guardNode = savedNode }()
	lc.excess = ""
	lc.joinForcesName, lc.joinForcesReset = "", false
	if err := fn(); err != nil {
		lc.refuseNode(node, "%s", err.Error())
	}
	lc.ab = saved
	lc.mode = -1
	lc.chainT = nil
	lc.remTgt = 0
	lc.permOrGraveCard = false
	lc.graveExileCard = false
}

// lowerRecord fills the static card record.
func (lc *lowerCtx) lowerRecord() {
	f := lc.face
	out := lc.out
	mc, err := parsePrintedManaCost(f.ManaCost)
	if err != nil {
		lc.refuseNode("ManaCost", "%v", err)
	}
	out.Cost = mc
	out.CostEmpty = strings.TrimSpace(f.ManaCost) == ""
	out.AtkFace = faceObservesAttack(f) // lower_meleegrant.go
	for _, t := range f.Types {
		if b, ok := typeBits[t]; ok && t != "Card" && t != "Permanent" {
			out.Types |= b
			continue
		}
		if t == "World" {
			out.World = true // CR 704.5k (kernel zone_worldrule.go)
		}
		out.Subtypes = append(out.Subtypes, t)
	}
	out.NoAbilities = effects.FaceHasNoAbilities(f, out.Types&tLand != 0)
	out.HasManaAbility = effects.FaceHasManaAbility(f)
	out.Colors = colorsOf(f)
	if faceHasKeyword(f, "Devoid") {
		// CR 702.114a: gorge effects.faceColorMask / cards.printedColourMask
		// read a Devoid face as colourless in every zone
		out.Colors = ""
	}
	out.NoPT = f.PT == ""
	if f.PT != "" {
		_, _, ok := strings.Cut(f.PT, "/")
		if !ok {
			lc.refuseNode("PT", "PT %q", f.PT)
		} else {
			// A star reads 0 and the fixed term stands once the CDA is lost (CR 208.2a): a
			// "1+*" toughness is 1, as gorge reads the face (Cosmogoyf with no abilities is 0/1).
			out.PTStar = f.CharacteristicDefining()
			out.P, out.T = f.Power(), f.Toughness()
		}
	}
	if f.Loyalty != "" {
		out.Loyalty, _ = strconv.Atoi(f.Loyalty)
	}
	if f.Defense != "" {
		out.Defense, _ = strconv.Atoi(strings.TrimSpace(f.Defense)) // battle (lower_battle.go)
	}
}

// colorsOf is the face's colours as a "WUBRG" subset string: the Color
// field when present (tokens), else the mana cost's coloured symbols.
func colorsOf(f *cards.Face) string {
	set := map[byte]bool{}
	src := f.Colors
	if src == "" {
		src = f.ManaCost
	} else {
		lower := strings.ToLower(src)
		for i, n := range []string{"white", "blue", "black", "red", "green"} {
			if strings.Contains(lower, n) {
				set["WUBRG"[i]] = true
			}
		}
		src = ""
	}
	for _, tok := range strings.Fields(src) {
		if tok == "no" || tok == "cost" {
			continue
		}
		for i := 0; i < len(tok); i++ {
			if strings.IndexByte("WUBRG", tok[i]) >= 0 {
				set[tok[i]] = true
			}
		}
	}
	var b strings.Builder
	for i := 0; i < 5; i++ {
		if set["WUBRG"[i]] {
			b.WriteByte("WUBRG"[i])
		}
	}
	return b.String()
}

// parseManaCost parses a printed cost ("2 R", "X G", "BG BG", "no cost").
func parseManaCost(s string) (manaCost, error) { return parseManaCostOpt(s, false) }

// parsePrintedManaCost is parseManaCost for a card's printed mana cost, the
// one place a monocoloured hybrid symbol ({2/W}, Forge "2W", CR 107.4e) is
// read (lower_twobrid.go): every other cost keeps refusing it.
func parsePrintedManaCost(s string) (manaCost, error) { return parseManaCostOpt(s, true) }

// parseAbilityManaCost is parseManaCost for an activated ability's cost,
// which also reads a Phyrexian symbol ({W/P}, Forge "WP").
func parseAbilityManaCost(s string) (manaCost, error) { return parseManaCostSyms(s, false, true) }

// xColorLetter parses Forge's XColor$ value -- colour words ("Black") or a
// bare WUBRG letter ("R") -- as the one colour the {X} must be paid with.
// It reports ok=false for a value naming no colour or more than one (Soul
// Burn's "Black,Red"): the kernel models only a single-colour XColor$.
func xColorLetter(raw string) (byte, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 1 {
		c := raw[0]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		if strings.IndexByte("WUBRG", c) >= 0 {
			return c, true
		}
	}
	set, ok := colorWordSet(raw)
	if !ok || len(set) != 1 {
		return 0, false
	}
	return set[0], true
}

// xColor applies XColor$ to a cost's mana part: every announced {X} becomes
// that colour's pip (kop.ManaCost.XPip) instead of generic mana (CR 107.3,
// 601.2b). A colour the kernel cannot express as one pip, or an XColor$ on a
// cost with no {X}, refuses by name rather than lowering something else.
func (lc *lowerCtx) xColor(m *manaCost, raw string) error {
	letter, ok := xColorLetter(raw)
	if !ok {
		return refuse("XColor$ %q: only a single colour is lowered (a black-and-red X is not modelled)", raw)
	}
	if !m.X && !m.XX && !m.XXX {
		return refuse("XColor$ %q without an {X} in the cost", raw)
	}
	m.XColor = letter
	return nil
}

func parseManaCostOpt(s string, twobrid bool) (manaCost, error) {
	return parseManaCostSyms(s, twobrid, twobrid)
}

// parseManaCostSyms parses a cost; twobrid admits {2/W}, phyrexian {W/P}.
func parseManaCostSyms(s string, twobrid, phyrexian bool) (manaCost, error) {
	var m manaCost
	s = strings.TrimSpace(s)
	if s == "" || s == "no cost" {
		m.None = true
		return m, nil
	}
	for _, t := range strings.Fields(s) {
		if n, err := strconv.Atoi(t); err == nil {
			m.Generic += n
			continue
		}
		if t == "X" {
			if m.XXX {
				return m, fmt.Errorf("XXXX cost")
			}
			if m.XX {
				// {X}{X}{X}: X announced once, paid three times
				m.XXX = true
				continue
			}
			if m.X {
				// {X}{X}: X announced once, paid twice (the kernel's
				// ManaCost.X counts the symbols)
				m.XX = true
				continue
			}
			m.X = true
			continue
		}
		if t == "C" {
			m.C++
			continue
		}
		if i := strings.IndexByte("WUBRG", t[0]); len(t) == 1 && i >= 0 {
			m.Pips[i]++
			continue
		}
		if len(t) == 2 && strings.IndexByte("WUBRG", t[0]) >= 0 && strings.IndexByte("WUBRG", t[1]) >= 0 {
			m.Hybrid = append(m.Hybrid, t)
			continue
		}
		if twobrid && len(t) == 2 && t[0] == '2' && strings.IndexByte("WUBRG", t[1]) >= 0 {
			m.Twobrid = append(m.Twobrid, t[1])
			continue
		}
		if phyrexian && len(t) == 2 && t[1] == 'P' && strings.IndexByte("WUBRG", t[0]) >= 0 {
			// a Phyrexian pip ({W/P}, CR 107.4f): printed costs only
			m.Phyrexian = append(m.Phyrexian, t[0])
			continue
		}
		return m, fmt.Errorf("mana symbol %q", t)
	}
	return m, nil
}

// simpleKeywords are the keyword heads the kernel reads as KeywordMask bits.
var simpleKeywords = map[string]bool{
	"Flying": true, "First Strike": true, "Double Strike": true, "Deathtouch": true, "Lifelink": true,
	"Vigilance": true, "Trample": true, "Reach": true, "Haste": true, "Defender": true, "Menace": true,
	"Hexproof": true, "Indestructible": true, "Protection from everything": true, "Shroud": true,
	"Fear": true, "Shadow": true, "Prowess": true,
}

// expandedKeywords are keyword heads gorge's engine/cards expands into
// ordinary T:/A:/S:/R: lines (tagged Keyword$), lowered from those lines.
var expandedKeywords = map[string]bool{
	"Equip": true, "Ward": true, "Exalted": true, "Prowess": true, "Affinity": true, "Enchant": true,
	"etbCounter": true, "ETBReplacement": true,
	// Sunburst (CR 702.47): gorge kwSunburst expands it to a Moved
	// replacement putting Count$Converge P1P1 (a creature) or CHARGE
	// counters on the entering permanent
	"Sunburst": true,
	// Class (CR 716): gorge kwClass appends each level's activator
	// (AB$ ClassLevelUp) and its granted ability (ClassBand$ N) to the face
	"Class": true,
	// heads gorge's cards/kw_*.go expanders turn wholly into ordinary
	// T:/A:/R: lines (addKeywordTrigger / an appended AB$ / a Moved
	// replacement), with no other rules-level read of the printed keyword
	// (Afflict's is the granted-keyword path; Persist's a with<K> filter
	// word). Cipher is not here: its expansion is an SVar the engine splices
	// into the resolution, which kgen never sees.
	"Undying": true, "Persist": true, "Backup": true, "Fabricate": true, "Living Weapon": true,
	"Job select": true, "For Mirrodin": true, "Transmute": true, "Outlast": true, "Battle cry": true,
	"Afterlife": true, "Afflict": true, "Ravenous": true,
	// Demonstrate (CR 702.144a): gorge kwDemonstrate (cards/kw_demonstrate.go)
	// appends a "when you cast this spell" trigger whose body is the
	// __kwDemonstrate SVar, DB$ Demonstrate. (gorge's comment there cites
	// 702.152, which is Blitz in this edition.)
	"Demonstrate": true,
	// Hideaway (CR 702.75a): gorge kwHideaway's ETB trigger, DB$ Hideaway
	// (lower_hideaway.go)
	"Hideaway": true,
	// Annihilator (CR 702.86a): gorge kwAnnihilator's Attacks Self trigger,
	// DB$ Sacrifice of the defending player's permanents (Annihilator$ N)
	"Annihilator": true,
	// Cumulative upkeep (CR 702.24a): gorge kwCumulativeUpkeep expands it into
	// an upkeep trigger whose body is DB$ CumulativeUpkeep (lower_cumulative.go)
	"Cumulative upkeep": true,
	// Fading (CR 702.32a): gorge kwFading's EnterWithCounters FADE replacement
	// and its upkeep trigger, a Sacrifice of the source when it has no fade
	// counter chained ahead of the RemoveCounter (every op already lowers)
	"Fading": true,
	// Echo (CR 702.30a): gorge kwEcho's upkeep trigger, gated by Echo$ True,
	// whose body is DB$ Echo (lower_echo.go)
	"Echo": true,
}

// protColourLetters maps a "Protection from <q>" colour quality to its
// colours (gorge protecColourLetter; "each color" is any coloured source).
var protColourLetters = map[string]string{"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G",
	"each color": "WUBRG"}

// hexColourLetters maps a "Hexproof:<q>" colour quality (any case, as
// gorge's colour table reads it) to its WUBRG letter. Any other quality
// stays refused.
var hexColourLetters = map[string]string{"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G"}

// mergeColours unions two WUBRG letter sets in WUBRG order.
func mergeColours(a, b string) string {
	var out strings.Builder
	for _, r := range "WUBRG" {
		if strings.ContainsRune(a, r) || strings.ContainsRune(b, r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// plainQuality accepts a protection quality filter of object
// characteristics only (no controller, owner, source-relative, comparison or
// state terms), which reads the same on a spell and a permanent.
func plainQuality(f *lFilter) error {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return fmt.Errorf("quality %s", f)
	}
	for _, c := range f.Clauses {
		if c.CmpStat == cmpMonoColor && c.Types == 0 && c.Ctrl == 0 && c.Owner == 0 && c.Flags == 0 && c.Subtype == "" && c.NotSubtype == "" && c.Colors == "" && c.NotColors == "" {
			// gorge sourceHasQuality's monocolored class (Guardian of the
			// Guildpact): exactly one colour of the source
			continue
		}
		if c.Ctrl != 0 || c.Owner != 0 || c.Flags != 0 || c.CmpOp != 0 || c.CmpStat != 0 || c.CmpX != "" || c.CmpCounter != "" {
			return fmt.Errorf("quality %s", f)
		}
	}
	return nil
}

// mayNotUntapKW is the keyword line of a permanent whose controller may
// choose not to untap it (gorge engine/rules/untap.go hasUntapStepChoice).
const mayNotUntapKW = "You may choose not to untap CARDNAME during your untap step."

func (lc *lowerCtx) lowerKeywords() {
	for _, k := range lc.face.Keywords {
		head := cards.KeywordHead(k)
		param := ""
		if j := strings.IndexByte(k, ':'); j >= 0 {
			param = strings.TrimSpace(k[j+1:])
		}
		node := "K:" + k
		switch {
		case param == "" && lc.lowerPreventKeyword(head):
			// the printed K:Prevent sentence: CardDef.Prevent* (lower_preventkw.go),
			// consumed with its minted R: lines
		case head == "Hexproof" && hexColourLetters[strings.ToLower(param)] != "":
			lc.out.HexColors = mergeColours(lc.out.HexColors, hexColourLetters[strings.ToLower(param)])
		case head == "Hexproof" && param == "Instant":
			lc.out.Kw = append(lc.out.Kw, "HexproofFromInstants")
		case simpleKeywords[head] && param == "":
			lc.out.Kw = append(lc.out.Kw, head)
		case head == "Flash" && param == "":
			lc.out.Flash = true
		case head == "Decayed" && param == "":
			lc.lowerDecayed() // lower_decayed.go
		case head == "Infect" && param == "":
			// CR 702.90: gorge damage riders read HasKeyword(source, "Infect")
			lc.out.Infect = true
		case head == "Toxic":
			// CR 702.164: gorge ToxicValue sums every Toxic:<N> instance
			n, err := strconv.Atoi(param)
			if err != nil || n <= 0 || n > 20 {
				lc.refuseNode(node, "toxic %q", param)
				continue
			}
			lc.out.Toxic += n
		case head == "Cascade" && param == "":
			// CR 702.85a/c: each instance triggers separately; a granted
			// Cascade stays refused (lower_effect.go)
			lc.out.Cascade++
		case head == "Devoid" && param == "":
			// lowerRecord reads it (the face is colourless)
		case strings.TrimSpace(k) == mayNotUntapKW:
			// CR 502.3: gorge hasUntapStepChoice reads this bare line; the
			// kernel's untap step asks the controller (repl_untapchoice.go)
			lc.out.Repls = append(lc.out.Repls, lRepl{Kind: "MayNotUntap"})
		case head == "Changeling" && param == "":
			lc.out.Changeling = true
		case strings.HasPrefix(k, "Protection from ") && protColourLetters[strings.TrimPrefix(k, "Protection from ")] != "":
			// gorge rules/protection.go protectionQuality: one quality per
			// keyword; a colour word (or "each color": any coloured source)
			lc.out.ProtColors = mergeColours(lc.out.ProtColors, protColourLetters[strings.TrimPrefix(k, "Protection from ")])
		case head == "Protection" && param != "":
			// gorge protectionQuality: K:Protection:<spec>:<text>, the spec a
			// filter over the source (sourceHasQuality's matchesSpec); only
			// plain type / subtype / colour terms
			spec, _, _ := strings.Cut(param, ":")
			spec = strings.TrimSpace(spec)
			var f *lFilter
			var err error
			if strings.EqualFold(spec, "Permanent.ThisTurnCast") {
				// gorge engine/rules/protection.go:259-260 special-cases this
				// quality: o.Zone == Battlefield && permanentCastThisTurn(o.ID).
				// The kernel has the same two facts: s.g.cast (set in enterGen
				// only when the entering permanent is the resolving spell,
				// turn_gen.go:34-37) and s.g.zoneTurn (reset each turn at
				// turn_gen.go:247, set on entry at turn_gen.go:41). cast &&
				// zoneTurn is "became a permanent by resolving a spell cast this
				// turn"; matchCardGen refuses F2CastByYou off the battlefield,
				// matching gorge's Zone == Battlefield test.
				f = &lFilter{Clauses: []lClause{{Types: tPermanent, Flags: fCastByYou | fEnteredThisTurn}}}
			} else {
				f, err = lc.filter(spec)
				if err == nil {
					err = plainQuality(f)
				}
			}
			if err != nil {
				lc.refuseNode(node, "protection %q: %v", param, err)
				continue
			}
			if lc.out.ProtFrom == nil {
				lc.out.ProtFrom = f
			} else {
				lc.out.ProtFrom = &lFilter{Clauses: append(append([]lClause(nil), lc.out.ProtFrom.Clauses...), f.Clauses...)}
			}
		case head == "Start your engines" && param == "":
			// CR 702.179a: the speed start is the kernel's post-placement scan
			// (dirac/kw_speed.go); the keyword's other text is the
			// card's own lines
			lc.out.Engines = true
		case head == "Wither" && param == "":
			// CR 702.80: gorge damage riders read HasKeyword(source, "Wither")
			lc.out.Wither = true
		case cards.AloneKeywordOf(head) == cards.AloneKeywordCantAttackAlone && param == "":
			// CR 506.5: gorge combat.DeclaresAlone over the derived hidden
			// keywords (Option.AloneRestricted); kernel combat_alone.go
			lc.out.AloneAtk = true
		case cards.AloneKeywordOf(head) == cards.AloneKeywordCantBlockAlone && param == "":
			// CR 506.5's block-only sibling: the lone block is illegal, the
			// lone attack stays legal (kernel combat_alone.go).
			lc.out.AloneBlk = true
		case cards.AloneKeywordOf(head) == cards.AloneKeywordCantAttackOrBlockAlone && param == "":
			// CR 506.5's compound: BOTH the lone attack and the lone block
			// are illegal (kernel combat_alone.go). Gorge's
			// HiddenKeywordFlags carries the same two flags.
			lc.out.AloneAtk = true
			lc.out.AloneBlk = true
		case (head == "Horsemanship" || head == "Intimidate" || head == "Skulk") && param == "":
			// gorge rules/combat/block.go CanBlock: attacker-keyed blocking
			// restrictions over the derived keywords (printed only)
			lc.out.Evasion |= evasionBit(head)
		case head == "Landwalk":
			// gorge rules/combat/block.go LandwalkSpec: the parameter up to
			// a second colon is a filter over the defender's lands
			spec, _, _ := strings.Cut(param, ":")
			spec = strings.TrimSpace(spec)
			if spec == "" || lc.out.Landwalk != nil {
				lc.refuseNode(node, "landwalk %q", param)
				continue
			}
			f, err := lc.filter(spec)
			if err != nil {
				lc.refuseNode(node, "landwalk %q: %v", param, err)
				continue
			}
			lc.out.Landwalk = f
		case head == "Unearth":
			// CR 702.84a: gorge kwUnearth expands it into one graveyard
			// AB$ ChangeZone (Unearth$ True), lowered from that line
			// (lower_zone.go changeZone)
			if !lc.hasKeywordLine(k) {
				lc.refuseNode(node, "keyword %q without its ability", head)
			}
		case head == "Myriad" && param == "":
			// CR 702.116a: gorge kwMyriad expands it into an Attacks Self
			// trigger (Myriad$ True, DB$ Myriad), lowered from that line
			// (lower_myriad.go)
			if !faceHasMyriadTrigger(lc.face) {
				lc.refuseNode(node, "keyword %q without its trigger", head)
			}
		case head == "Training":
			// CR 702.149a: gorge kwTraining expands it into an Attacks
			// trigger (Training$ True), lowered from that line
			found := false
			for _, t := range lc.face.Triggers {
				found = found || t.Params["Training"] == "True"
			}
			if !found {
				lc.refuseNode(node, "keyword %q without its trigger", head)
			}
		case head == "Evolve":
			// CR 702.100a: gorge kwEvolve expands it into an ETB trigger
			// (Evolve$ True) whose body puts a +1/+1 counter on Self,
			// lowered from that line (readEvolveCondition)
			found := false
			for _, t := range lc.face.Triggers {
				found = found || t.Params["Evolve"] == "True"
			}
			if !found {
				lc.refuseNode(node, "keyword %q without its trigger", head)
			}
		case head == "Firebending":
			// CR 702.189a: gorge kwFirebending expands it into an Attacks
			// Self trigger whose body is DB$ Mana (Produced$ R) with
			// PersistentUntilEndOfCombat$ True, lowered from that line
			// (manaEffect). Not in expandedKeywords: the
			// Pump/static grants of the keyword stay refused.
			found := false
			for _, t := range lc.face.Triggers {
				if t.Params["Keyword"] != "Firebending" || t.Mode != "Attacks" || t.Params["ValidCard"] != "Card.Self" {
					continue
				}
				body := cards.ResolveSVar(lc.face.SVars, t.Params["Execute"])
				found = found || body != nil && body.API == "Mana" && body.Params["PersistentUntilEndOfCombat"] == "True"
			}
			if !found {
				lc.refuseNode(node, "keyword %q without its trigger", head)
			}
		case head == "Increment":
			// CR 702.xx (Secrets of Strixhaven): gorge kwIncrement expands it
			// into a SpellCast trigger (Increment$ True) whose body counters
			// Self, lowered from that line
			found := false
			for _, t := range lc.face.Triggers {
				found = found || t.Params["Increment"] == "True"
			}
			if !found {
				lc.refuseNode(node, "keyword %q without its trigger", head)
			}
		case head == "Craft":
			// CR 702.167a: gorge kwCraft expands it into one battlefield
			// AB$ ChangeZone (Cost$ <mana> ExileCtrlOrGrave<..> Exile<1/CARDNAME>,
			// Origin$ Exile, Transformed$ True), lowered from that line
			if !lc.hasKeywordLine(k) {
				lc.refuseNode(node, "keyword %q without its ability", head)
			}
		case head == "Embalm":
			// CR 702.128a: gorge expands it into a graveyard ability (AB$
			// CopyPermanent | Cost$ <cost> ExileFromGrave<1/CARDNAME> |
			// SorcerySpeed$, RemoveCost$, SetColor$ White, AddTypes$
			// Zombie), lowered from that line
			if !lc.hasKeywordLine(k) {
				lc.refuseNode(node, "keyword %q without its ability", head)
			}
		case head == "Encore":
			// CR 702.141a: gorge kwEncore expands it into one graveyard
			// AB$ Encore (Cost$ <cost> ExileFromGrave<1/CARDNAME>,
			// SorcerySpeed$ True), lowered from that line (lower_encore.go)
			if !lc.hasKeywordLine(k) {
				lc.refuseNode(node, "keyword %q without its ability", head)
			}
		case head == "Cycling" || head == "TypeCycling":
			// gorge expands it into a hand ability (AB$ Draw / AB$ ChangeZone
			// | Cost$ <cost> Discard<1/CARDNAME> | ActivationZone$ Hand,
			// cards/kw_cycling.go), lowered from that line; a cost the
			// expander cannot read leaves no line and refuses here.
			if !lc.hasKeywordLine(k) {
				lc.refuseNode(node, "keyword %q without its ability", head)
			}
		case head == "Ninjutsu":
			// CR 702.49: gorge kwNinjutsu expands the keyword into one hand
			// AB$ ChangeZone line (lower_ninjutsu.go), lowered from that line
			if !lc.hasKeywordLine(k) {
				lc.refuseNode(node, "keyword %q without its ability", head)
			}
		case head == "Crew" || head == "Saddle":
			// gorge kwCrew / kwSaddle expand the keyword into one AB$ line
			// (its tapXType<Any/...+withTotalPowerGE<N>> cost, lower_tappower.go)
			if !lc.hasKeywordLine(k) {
				lc.refuseNode(node, "keyword %q without its ability", head)
			}
		case head == "Chapter":
			// CR 714 (kw:Chapter): each chapter is a self trigger on the
			// lore counter, gated at its chapter number (kw_saga.go)
			lc.lowerChapters(param)
		case head == "Read ahead":
			// CR 714.3b: gorge implements it since (the lore-count
			// choice on entry; chapters below it do not trigger that turn),
			// but the kernel has no entry choice for it yet; refuse rather
			// than silently drop a keyword with rules meaning
			lc.refuseNode(node, "read ahead (CR 714.3b): the kernel has no lore-count entry choice")
		case head == "Station":
			// rules-side in gorge (rules/station.go); appended after the
			// face's own abilities (lowerFace, lower_station.go)
			lc.station = param
		case head == "Melee":
			// CR 702.121a: gorge kwMelee expands it into an Attacks Self
			// trigger (Keyword$ Melee) pumping Self by cards.MeleePumpCount,
			// lowered from that line (mlTrigger). gorge's granted walk
			// (rules/melee.go checkGrantedAttackPumpTriggers) discounts the
			// printed line only while it exists, so a grant on top of it
			// is its own instance (CR 702.121b), as the kernel lists it
			// (lower_meleegrant.go), and the only one on a face-down or
			// ability-less holder (CR 708.2, 613.7).
			if !faceHasMeleeTrigger(lc.face) {
				lc.refuseNode(node, "keyword %q without its trigger", head)
			}
		case head == "Extort" && param == "":
			// CR 702.101a: gorge kwExtort expands the keyword into a
			// SpellCast trigger (DB$ Extort), lowered from that line
			// (lower_effect.go's Extort arm). The keyword itself has no
			// CardDef bit; a face whose expansion is missing would lose
			// the trigger silently.
			if !faceHasExtortTrigger(lc.face) {
				lc.refuseNode(node, "keyword %q without its trigger", head)
			}
		case head == "Partner with":
			// CR 702.124j: the commander-pair marker (no game characteristic)
			// and gorge kwPartnerWith's enters trigger, lowered from that
			// line (lower_partnerwith.go)
			if !lc.partnerWithTrigger() {
				lc.refuseNode(node, "keyword %q without its trigger", head)
			}
		case head == "Doctor's companion":
			// CR 702.124m: a deck-construction marker (the card may be one
			// of two commanders beside a Doctor, gorge deck.IsPartnerPair's
			// doctorCompanionPair). It has no battlefield meaning, gorge
			// expands it into no ability, and the adapter's commander gate
			// reads the raw face keyword (dirac/adapter checkCommander
			// -> deck.IsPartnerPair on the deck's own cards), so lowering
			// nothing is exact.
		case expandedKeywords[head]:
			// Lowered from the expansion's own lines.
		case head == "Kicker":
			if kc, ok := lc.kickerNonMana(param); ok {
				// a one-part Sac<1/..> or Return<1/..> kicker
				// (gorge kickerCost is a full Cost, folded into the kicked
				// cast's cost); the zero-mana Kicker keeps the kicked mode
				lc.out.Kicker, lc.out.KickerCost = &manaCost{None: true}, kc
				continue
			}
			mc, err := parseManaCost(param)
			if err != nil || strings.Contains(param, ":") {
				lc.refuseNode(node, "kicker cost %q", param)
				continue
			}
			lc.out.Kicker = &mc
		case head == "Multikicker":
			// CR 702.43: the per-payment additional cost, paid any number of
			// times as the spell is cast. The kernel's kicked cast asks the
			// count before X and the targets (multikick, cast.go), so only a
			// plain mana parameter on a non-{X} spell is modelled; a non-mana,
			// {X} parameter or {X} base cost refuses (the count bound would be
			// read before the X announcement).
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.Contains(param, ":") || mc.None || mc.X {
				lc.refuseNode(node, "multikicker cost %q", param)
				continue
			}
			if lc.out.Cost.X {
				lc.refuseNode(node, "multikicker on an {X} spell")
				continue
			}
			lc.out.Multikicker = &mc
		case head == "Morph" || head == "Megamorph" || head == "Disguise":
			// CR 702.37/702.168/702.169: the keyword parameter is the
			// turn-face-up cost. The kernel's turn-up action spends only
			// mana (an {X} is announced first), so non-mana parameters refuse.
			// A printed generic reduction rides the second colon field
			// (lower_morphreduce.go).
			cs, red, rok := lc.morphReduceParam(param)
			mc, err := parseManaCost(cs)
			if !rok || err != nil || cs == "" || strings.Contains(cs, ":") || mc.None || red.Kind != vNone && (mc.X || mc.Generic == 0) {
				lc.refuseNode(node, "%s cost %q", strings.ToLower(head), param)
				continue
			}
			if lc.out.MorphCost != nil {
				lc.refuseNode(node, "second face-down keyword")
				continue
			}
			lc.out.MorphCost, lc.out.MorphReduce = &mc, red
			switch head {
			case "Morph":
				lc.out.MorphFamily = kop.MorphMorph
			case "Megamorph":
				lc.out.MorphFamily = kop.MorphMegamorph
			default:
				lc.out.MorphFamily = kop.MorphDisguise
			}
		case head == "Mobilize":
			// CR 702.181a: the expansion's Attacks trigger (kw_mobilize.go)
			// is its own node and lowers on its own: Token + group
			// DelayedTrigger (delayedTrigger's RememberChain$ False shape)
		case head == "Offspring":
			// CR 702.175a: an optional additional mana cost, offered as its
			// own cast option (gorge's "(offspring)" mode, the Buyback
			// shape); the expansion's enter trigger mints the 1/1 copy
			// (copyPermanent's Count$OffspringPaid)
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.Contains(param, ":") {
				lc.refuseNode(node, "offspring cost %q", param)
				continue
			}
			lc.out.Offspring = &mc
		case head == "Buyback":
			// CR 702.27a: an optional additional cost, offered as its own cast
			// option (gorge's "(buyback)" mode, the Offspring shape); the
			// resolving spell returns to hand (lower_buyback.go)
			if !lc.lowerBuyback(param) {
				lc.refuseNode(node, "buyback cost %q", param)
			}
		case head == "Casualty":
			// CR 702.153a: the optional creature sacrifice and the copy
			// trigger are the kernel's casualty cast (kw_casualty.go). The
			// fixed form only: Casualty:X (Ob Nixilis, the Adversary) and
			// its NonLegendary$ / SetLoyalty$ riders are refused.
			n, err := strconv.Atoi(param)
			switch {
			case param == "X" || strings.HasPrefix(param, "X:"):
				lc.refuseNode(node, "variable casualty (Casualty:X, its riders %q) is not lowered", param)
			case strings.Contains(param, ":"):
				lc.refuseNode(node, "casualty with riders %q (only a fixed Casualty:<N> is lowered)", param)
			case err != nil || n < 1 || n > 255:
				lc.refuseNode(node, "casualty amount %q", param)
			default:
				lc.out.Casualty = n
			}
		case head == "Bargain" && param == "":
			// CR 702.166: the artifact/enchantment/token sacrifice is the
			// kernel's bargained cast (kw_bargain.go); the card's
			// CardDef.Bargain picks the row and the paid flag rides sfKicked
			lc.out.Bargain = true
		case head == "Storm" && param == "":
			// CR 702.40a: gorge kwStorm (cards/kw_storm.go) expands it into
			// one SpellCast Card.Self trigger whose body is the __kwStorm
			// SVar, DB$ CopySpellAbility | Defined$ TriggeredSpellAbility |
			// Amount$ TriggerCount$CurrentStormCount/Minus1 | MayChooseTarget$
			// True (lower_storm.go). The line is never lowered: its amount is
			// the fire-time storm count, which the kernel reads off its own
			// turn history, so the trigger is the kernel-native abStorm
			// (dirac/kw_storm.go) and CardDef.Storm picks it; a copy of a
			// permanent spell resolves into a token (genPrep.anyPermCopy, CR
			// 707.10f). One native trigger per cast: a second instance refuses.
			if lc.stormTrig == 0 {
				lc.refuseNode(node, "keyword %q without its trigger", head)
				continue
			}
			if lc.out.Storm {
				lc.refuseNode(node, "second Storm keyword")
				continue
			}
			lc.out.Storm = true
		case head == "Teamwork":
			// CR 702.194a: the optional creature-tap cost is the kernel's
			// teamworked cast (kw_teamwork.go); N is the power the
			// tapped creatures must total. The paid flag rides sfKicked.
			n, err := strconv.Atoi(param)
			switch {
			case err != nil || n < 1 || n > 15:
				lc.refuseNode(node, "teamwork amount %q", param)
			case lc.out.Teamwork != 0:
				lc.refuseNode(node, "two Teamwork keywords")
			default:
				lc.out.Teamwork = n
			}
		case head == "Gift" && param == "":
			// CR 702.174: the body is the GiftAbility SVar (lowerGift)
			if cards.ResolveSVar(lc.face.SVars, "GiftAbility") == nil {
				lc.refuseNode(node, "gift without a GiftAbility")
				continue
			}
			lc.out.Gift = true
		case head == "Flashback" && strings.Contains(param, "<"):
			// a flashback cost with a sacrifice or tap-creatures part
			// (gorge flashbackCost: ParseCost of the keyword parameter,
			// paid through the ordinary cost stages; lower_fbcost.go)
			if err := lc.flashbackCost(param); err != nil {
				lc.refuseNode(node, "flashback cost %q: %v", param, err)
			}
		case head == "Flashback":
			mc, err := parseManaCost(param)
			if err != nil || strings.Contains(param, ":") {
				lc.refuseNode(node, "flashback cost %q", param)
				continue
			}
			lc.out.Flashback = &mc
		case head == "Harmonize":
			// CR 702.180a: a graveyard cast for the printed cost, one
			// creature tapped for its power (dirac/kw_harmonize.go).
			// An {X} cost announces X before the tap pick reads the total
			// (gorge xAsk after convokeAsk); {X}{X}, a rider or a non-mana
			// part is not read exactly.
			mc, err := parseManaCost(param)
			if err != nil || mc.XX || mc.XXX || mc.None || strings.ContainsAny(param, ":<") {
				lc.refuseNode(node, "harmonize cost %q", param)
				continue
			}
			if lc.out.Flashback != nil {
				lc.refuseNode(node, "Harmonize beside Flashback")
				continue
			}
			lc.out.Harmonize = &mc
		case head == "Beam me up":
			lc.beamMeUp(node, param) // lower_beammeup.go
		case head == "Escape":
			lc.escapeCost(node, param) // lower_escape.go (CR 702.138a; dirac/kw_escape.go)
		case head == "Mayhem":
			// CR 702.187a: a graveyard cast for the colon parameter, an
			// alternative mana cost, if the card was discarded this turn
			// (dirac/kw_mayhem.go). A bare keyword (the play-a-land
			// shape), an {X} cost, a rider or CardManaCost is not read exactly.
			mc, err := parseManaCost(param)
			if err != nil || mc.X || mc.None || strings.ContainsAny(param, ":<") || strings.Contains(param, "CardManaCost") {
				lc.refuseNode(node, "mayhem cost %q", param)
				continue
			}
			lc.out.Mayhem = &mc
		case head == "Retrace" && param == "":
			// CR 702.81a: a graveyard cast paying the PRINTED mana cost
			// plus the additional cost of discarding a land card
			// (dirac/kw_retrace.go). Retrace is no cost substitution: the
			// printed base stays, only the discard folds on, so it rides
			// mode bit 2 beside the mode-2 graveyard-cast keywords rather
			// than claiming CardDef.AltCost. lowerRetrace refuses the
			// collisions and builds the discard cost.
			lc.out.Retrace = true
		case head == "Spree" && param == "":
			// CR 702.172a: a modal spell whose modes carry ModeCost$
			// additional costs (lower_modecost.go reads them)
			if lc.out.Spree {
				lc.refuseNode(node, "Spree with Tiered (one mode-cost keyword)")
				continue
			}
			lc.out.Spree = true
		case head == "Tiered" && param == "":
			// CR 702.183a: Tiered is Spree's one-mode sibling and gorge
			// registers the two heads as one mechanism (rules/spree.go).
			// A Tiered card prints no CharmNum$/MinCharmNum$, so charmCounts
			// gives the exactly-one-mode pick. ModeCost holds each mode's
			// additional mana cost, as for Spree.
			if lc.out.Spree {
				lc.refuseNode(node, "Tiered with Spree (one mode-cost keyword)")
				continue
			}
			lc.out.Spree = true
		case head == "Convoke" && param == "":
			// CR 702.51a: printed only (gorge hasCastConvoke reads derived
			// keywords, but kgen refuses every grant). An {X} mana cost is
			// refused: xOpen absorption and xAsk pricing are not ported.
			if lc.out.Cost.X {
				lc.refuseNode(node, "Convoke with {X}")
				continue
			}
			if lc.out.Improvise {
				lc.refuseNode(node, "Convoke with Improvise")
				continue
			}
			lc.out.Convoke = true
		case head == "Improvise" && param == "":
			// CR 702.126a: printed only (gorge hasCastImprovise reads derived
			// keywords, but kgen refuses every grant). An {X} mana cost is
			// refused as for Convoke; both keywords on one card are refused
			// so an artifact creature is never a candidate for both.
			if lc.out.Cost.X {
				lc.refuseNode(node, "Improvise with {X}")
				continue
			}
			if lc.out.Convoke {
				lc.refuseNode(node, "Convoke with Improvise")
				continue
			}
			lc.out.Improvise = true
		case head == "Delve" && param == "":
			// CR 702.66a: the cast exiles graveyard cards toward its
			// generic cost (dirac/kw_delve.go); a granted Delve reads the
			// same kernel path.
			lc.out.Delve = true
		case head == "Rebound" && param == "":
			lc.lowerRebound(node) // lower_rebound.go
		case head == "Madness":
			// CR 702.35a-b: a discard replacement into exile plus a
			// keyword trigger whose resolution offers the cast for this
			// cost (dirac/kw_madness.go). X, hybrid and a
			// ":" rider (a granted form) refuse.
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.Contains(param, ":") || mc.X || len(mc.Hybrid) > 0 || len(mc.Twobrid) > 0 {
				lc.refuseNode(node, "madness cost %q", param)
				continue
			}
			lc.out.Madness = &mc
		case head == "Warp":
			// CR 702.185a: an alternative hand cost. The recast from exile
			// pays the mana cost. An {X} in the warp cost is announced like
			// a printed one (CR 601.2b): the cast's X ask prices the
			// replaced cost (kernel cast.go buildCostPreRaw).
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.Contains(param, ":") || mc.XX || mc.XXX {
				lc.refuseNode(node, "warp cost %q", param)
				continue
			}
			lc.out.Warp = &mc
		case head == "Impending":
			// CR 702.176a (gorge kw_impending.go): K:Impending:<N>:<cost>, an
			// alternative hand cost (the "impended" cast) entering with N
			// time counters, not a creature while one remains; the end-step
			// removal is synthesized as the card's own trigger
			ns, cost, ok := strings.Cut(param, ":")
			n, err := strconv.Atoi(strings.TrimSpace(ns))
			mc, err2 := parseManaCost(cost)
			if !ok || err != nil || err2 != nil || n < 1 || n > 255 || mc.X || lc.out.Impending != nil {
				lc.refuseNode(node, "impending %q", param)
				continue
			}
			lc.out.Impending, lc.out.ImpendingN = &mc, n // the tick and the combos: lowerTail's lowerImpending
		case head == "Sneak" && sneakLowered:
			// CR 702.190a: an alternative hand cost paid in the caster's own
			// declare-blockers step, plus the mandatory return of an
			// unblocked attacker (dirac/kw_sneak.go). X, CardManaCost, a ":"
			// rider (the granted form) and any non-mana part are refused;
			// a hybrid pip is announced like a printed cost's.
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.Contains(param, ":") || mc.X || mc.None {
				lc.refuseNode(node, "sneak cost %q", param)
				continue
			}
			if lc.out.Sneak != nil {
				lc.refuseNode(node, "Sneak beside an alternative or additional cost")
				continue
			}
			lc.out.Sneak = &mc
		case head == "Web-slinging":
			// CR 702.188a: an alternative hand cost under the plain cast's
			// timing, plus the mandatory return of a tapped creature
			// (dirac/kw_websling.go). X, hybrid, CardManaCost, a
			// ":" or "<" rider (the granted form) and any non-mana part are
			// refused.
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.X || mc.None || len(mc.Hybrid) != 0 {
				lc.refuseNode(node, "web-slinging cost %q", param)
				continue
			}
			if lc.out.WebSling != nil {
				lc.refuseNode(node, "Web-slinging beside an alternative or additional cost")
				continue
			}
			lc.out.WebSling = &mc
		case head == "MayFlashCost":
			// CR 702.8 (gorge rules/mayflash.go): "you may cast this as
			// though it had flash if you pay <cost> more". The instant-timing
			// cast pays the mana cost plus this (dirac/kw_mayflash.go).
			// X, hybrid, CardManaCost and any other non-mana part
			// (tapXType<..>) are refused. A lone Behold<1/Type> is the
			// extra (Molten Exhale): no mana, the behold folded into the
			// cast's cost-pick stage (CardDef.MayFlashBehold).
			if strings.HasPrefix(param, "Behold<") {
				lc.beholdOK = true
				bc, err := lc.cost(param)
				lc.beholdOK = false
				rest := bc
				rest.BeholdN, rest.BeholdF = 0, nil
				if err != nil || bc.BeholdN != 1 || !rest.pureMana() || !bc.Mana.None {
					lc.refuseNode(node, "may-flash cost %q", param)
					continue
				}
				if lc.out.MayFlash != nil {
					lc.refuseNode(node, "two MayFlashCost keywords")
					continue
				}
				lc.out.MayFlash, lc.out.MayFlashBehold = &manaCost{}, bc.BeholdF
				continue
			}
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.X || mc.None || len(mc.Hybrid) != 0 || len(mc.Twobrid) != 0 || len(mc.Phyrexian) != 0 {
				lc.refuseNode(node, "may-flash cost %q", param)
				continue
			}
			if lc.out.MayFlash != nil {
				lc.refuseNode(node, "two MayFlashCost keywords")
				continue
			}
			lc.out.MayFlash = &mc
		case head == "Dash":
			// CR 702.109a: an alternative hand cost; the permanent gains
			// haste and returns to its owner's hand at the next end step
			// (dirac/kw_dash.go). Hybrid pips are announced as for any cost;
			// X, CardManaCost, a ":" or "<" rider and any non-mana part are
			// refused.
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.X || mc.None {
				lc.refuseNode(node, "dash cost %q", param)
				continue
			}
			if lc.out.Dash != nil {
				lc.refuseNode(node, "two Dash keywords")
				continue
			}
			lc.out.Dash = &mc
		case head == "Evoke":
			// CR 702.74a: an alternative hand cost; the permanent is
			// sacrificed as it enters (dirac/kw_evoke.go). Hybrid
			// pips are announced as for any cost; X, CardManaCost, a ":" or
			// "<" rider (the granted form) and any non-mana part are
			// refused.
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.X || mc.None {
				lc.refuseNode(node, "evoke cost %q", param)
				continue
			}
			if lc.out.Evoke != nil {
				lc.refuseNode(node, "two Evoke keywords")
				continue
			}
			lc.out.Evoke = &mc
		case head == "Overload":
			// CR 702.96a: an alternative hand cost; if paid, every
			// "target" in the spell's text reads "each" (dirac/kw_overload.go).
			// X and hybrid pips are announced as for a printed cost; a
			// CardManaCost, a ":" or "<" rider and any non-mana part are
			// refused.
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.None {
				lc.refuseNode(node, "overload cost %q", param)
				continue
			}
			if lc.out.Overload != nil {
				lc.refuseNode(node, "two Overload keywords")
				continue
			}
			lc.out.Overload = &mc
		case head == "Bestow":
			// CR 702.103a: an alternative hand cost; the spell is an Aura
			// with enchant creature (gorge rules/bestow.go bestowCost and
			// bestowedAttachSA; kernel kw_bestow.go)
			if err := lc.bestowKeyword(param); err != nil {
				lc.refuseNode(node, "%v", err)
				continue
			}
		case head == "Plot":
			// CR 702.170a: the sorcery-speed special action pays this plain
			// mana cost and exiles the card with the plotted designation.
			// X, hybrid and any non-mana part have no fixed price to plan
			// (dirac/kw_plot.go).
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.Contains(param, ":") || mc.X || mc.None || len(mc.Hybrid) != 0 {
				lc.refuseNode(node, "plot cost %q", param)
				continue
			}
			lc.out.Plot = &mc
		case head == "Suspend":
			// CR 702.62a: a hand special action paying this plain mana cost
			// and exiling the card with N time counters (dirac/kw_suspend.go).
			// An X time (Aeon Chronicler's), X, hybrid and any non-mana part
			// have no fixed price or count to plan.
			ns, cs, ok := strings.Cut(param, ":")
			n, nerr := strconv.Atoi(strings.TrimSpace(ns))
			mc, err := parseManaCost(cs)
			if !ok || nerr != nil || n < 1 || n > 20 || err != nil || strings.TrimSpace(cs) == "" || strings.ContainsAny(cs, ":<") || mc.X || mc.None || len(mc.Hybrid) != 0 {
				lc.refuseNode(node, "suspend %q", param)
				continue
			}
			if lc.out.Suspend != nil {
				lc.refuseNode(node, "two Suspend keywords")
				continue
			}
			lc.out.Suspend, lc.out.SuspendN = &mc, n
		case head == "Foretell":
			// CR 702.143a: the {2} special action is fixed; the keyword's
			// cost prices the later cast from exile (dirac/kw_foretell.go).
			// X, hybrid, a ":" or "<" rider and any non-mana part have no
			// fixed price to plan.
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.X || mc.None || len(mc.Hybrid) != 0 {
				lc.refuseNode(node, "foretell cost %q", param)
				continue
			}
			if lc.out.Foretell != nil {
				lc.refuseNode(node, "two Foretell keywords")
				continue
			}
			lc.out.Foretell = &mc
		case head == "Paradigm":
			// SOS Paradigm (gorge rules/paradigm.go): a resolved
			// spell is exiled, then cast free from exile in each of its
			// owner's first main phases. gorge's spellRestZone reads the
			// spell's face, so only a single-face instant/sorcery lowers.
			if param != "" || lc.out.Types&(tInstant|tSorcery) == 0 || lc.out.Types&^(tInstant|tSorcery) != 0 || len(lc.card.Faces) > 1 {
				lc.refuseNode(node, "Paradigm on a non-instant/sorcery or multi-face card")
				continue
			}
			lc.out.Paradigm = true
		case head == "AlternateAdditionalCost":
			// "Sac<1/Creature>:3 B": pay either the first or the second.
			a, b, ok := strings.Cut(param, ":")
			if !ok && lc.out.SpellCost == nil {
				// one part (Dusk Rose Reliquary's "Sac<1/Creature;Artifact/
				// creature or artifact>"): gorge altAddAsk folds a lone
				// part into the cost unasked, an ordinary additional cost;
				// its ";" alternation is ParseCost's "," (cost/parse.go)
				c, err := lc.cost(strings.ReplaceAll(a, ";", ","))
				rest := c
				rest.Sac, rest.SacN = nil, 0
				if err != nil || c.Sac == nil || c.SacN != 1 || !rest.pureMana() || c.Mana.Generic != 0 || c.Mana.Pips != [5]int{} || c.Mana.X || len(c.Mana.Hybrid) != 0 {
					lc.refuseNode(node, "alternate additional cost %q", param)
					continue
				}
				lc.out.SpellCost, lc.out.AltAddOne = &c, true
				continue
			}
			if !ok {
				lc.refuseNode(node, "alternate additional cost %q", param)
				continue
			}
			lc.beholdOK = true // the spell's first part only (kernel act_behold.go)
			c1, err1 := lc.cost(a)
			lc.beholdOK = false
			c2, err2 := lc.cost(b)
			if c1.BeholdN != 0 {
				// "Behold<1/Kithkin>:2": the behold is the whole of part one
				rest := c1
				rest.BeholdN, rest.BeholdF = 0, nil
				if err1 == nil && (lc.out.BeholdExile || c1.BeholdN != 1 || !rest.pureMana() || !c1.Mana.None) {
					err1 = refuse("behold cost %q", a)
				}
			}
			if err1 != nil || err2 != nil || c1.Energy != 0 || c2.Energy != 0 || c1.Return != nil || c2.Return != nil {
				lc.refuseNode(node, "alternate additional cost %q", param)
				continue
			}
			lc.out.SpellCost = &c1
			lc.out.AltAddCost = &c2
		case head == "MayEffectFromOpeningHand" && param == "FromHand":
			body := lc.face.SVars["FromHand"]
			sa := cards.ResolveSVar(lc.face.SVars, "FromHand")
			if sa == nil {
				lc.refuseNode(node, "opening-hand effect %q", body)
				continue
			}
			fp := newParams(sa.Params)
			if sa.API != "ChangeZone" || fp.str("Defined") != "Self" || fp.str("Origin") != "Hand" || fp.str("Destination") != "Battlefield" || len(fp.unread()) != 0 {
				lc.refuseNode(node, "opening-hand effect %q", body)
				continue
			}
			lc.out.Leyline = true
		case head == "MayEffectFromOpeningHand":
			lc.refuseNode(node, "%s", refuseOpeningGated(param)) // (lower_gemstone.go)
		case strings.HasPrefix(k, "A deck can have any number of cards named"):
			// Deck construction only.
		case head == "Partner" && deckOnlyPartner(param):
			// CR 702.124a: deck construction only (lower_partner.go)
		case k == "Daybound" || k == "Nightbound":
			// CR 702.145 (gorge rules/daynight.go, cr-daybound-nightbound
			// and -transform): the day/night designation and the faces it
			// turns (kernel kw_daynight.go)
			if k == "Daybound" {
				lc.out.Daybound = true
			} else {
				lc.out.Nightbound = true
			}
		case cards.HiddenKeywordOf(k) == cards.HiddenKeywordAllMustBlock:
			// Lure (CR 509.1c; gorge kw:AllMustBlock, cr-lure-all-must-block
			// 617daacfb): every creature able to block it must.
			// HiddenKeywordOf canonicalises both the keyword and the old
			// sentence spelling, so the next alias needs no new case.
			lc.out.Lure = true
		case cards.HiddenKeywordOf(k) == cards.HiddenKeywordMustBlock:
			// CR 509.1c: "CARDNAME must be blocked if able." (gorge canonicalises
			// the sentence to the MustBlock head; the attacker-oriented
			// requirement, one blocker satisfies it)
			lc.out.MustBeBlocked = true
		case head == "Fuse" && param == "" && lc.out.FaceMode == kop.FaceSplit && fuseCard(lc.card):
			// CR 702.102a: the layout carries it (CardDef.FuseDef, the
			// fused spell's entry: lower_fuse.go); a half lowers as itself
		case head == "Aftermath" && param == "" && lc.out.FaceMode == kop.FaceSplit && lc.out.Face == 1:
			// CR 702.127a: a split card's right half cast only from its
			// owner's graveyard, then exiled as it leaves the stack (gorge
			// legal.go aftermathAlternateFace; dirac/face_aftermath.go).
			// castableFaceLayout refuses the keyword on the left half.
			lc.out.Aftermath = true
		case head == "Miracle":
			// CR 702.94a: the first draw of a turn offers the reveal; the
			// revealed card's trigger is on the stack, and its resolution
			// offers the cast from hand for this cost (dirac/kw_miracle.go).
			// X, hybrid and a ":" rider or granted spelling (Aminatou's
			// ManaCost form) refuse.
			mc, err := parseManaCost(param)
			if err != nil || param == "" || strings.Contains(param, ":") || mc.X || len(mc.Hybrid) > 0 || len(mc.Twobrid) > 0 {
				lc.refuseNode(node, "miracle cost %q", param)
				continue
			}
			lc.out.Miracle = &mc
		case head == "Compleated":
			// CR 702.150, as gorge implements it (the kw:Compleated
			// approximation): the life paid for the printed Phyrexian pips
			// lowers the entry loyalty (gorge events entry_counters'
			// CompleatedLifePaid, cast_commit's FlagCompleated CastInfo).
			// The kernel pays the pips' life in the payment window and
			// places the entry's CtrLoyalty from the printed Loyalty alone
			// (zones.go), so the life-paid amount would not reach the
			// entry: a life-pip cast would enter Jace at full loyalty
			// where gorge enters it two loyalty lower per pip.
			lc.refuseNode(node, "keyword %q: the pips' life does not reach the entry loyalty", head)
		default:
			lc.refuseNode(node, "keyword %q", head)
		}
	}
}

// faceHasKeyword reports whether the face carries the parameterless keyword
// head (gorge Face.HasKeywordRef).
func faceHasKeyword(f *cards.Face, head string) bool {
	for _, k := range f.Keywords {
		if cards.KeywordHead(k) == head {
			return true
		}
	}
	return false
}

// hasKeywordLine reports whether the face has the ability a keyword line
// expanded into (its KeywordLine$ is the line).
func (lc *lowerCtx) hasKeywordLine(k string) bool {
	for _, sa := range lc.face.Abilities {
		if sa != nil && sa.Params["KeywordLine"] == k {
			return true
		}
	}
	return false
}

// cost lowers a Cost$ string. Discarding the source (Discard<1/CARDNAME>)
// is a hand ability's cost only (costHand).
func (lc *lowerCtx) cost(s string) (lCost, error) {
	c, err := lc.costHand(s)
	if err != nil {
		return c, err
	}
	if err := discardHandCost(&c); err != nil {
		return c, err
	}
	return c, lc.costParts(c)
}

// discardHandCost rewrites Discard<N/Hand> (the whole hand) to the
// DiscardHand table byte: an activated ability's own cost pays it with the
// other non-mana picks, in hand order, before the source's own sacrifice,
// asking no choice (gorge pay.DiscardCandidates' whole-hand spec; an empty
// hand pays with nothing). Random stays an unless-cost selection method
// (gorge d66db41d8), and every other spec is refused by name.
func discardHandCost(c *lCost) error {
	switch c.DiscardSpec {
	case "":
		return nil
	case "Hand":
		c.DiscardHand, c.DiscardSpec, c.DiscardN = true, "", 0
		return nil
	default:
		return refuse("discard cost %q outside an unless cost", c.DiscardSpec)
	}
}

// costUnless is cost for an UnlessCost$: it also reads the Random and Hand
// discard selection methods gorge pays (pay/unlessflow.go AdvanceUnless).
func (lc *lowerCtx) costUnless(s string) (lCost, error) {
	lc.unlessParse = true
	c, err := lc.costHand(s)
	lc.unlessParse = false
	if err != nil {
		return c, err
	}
	return c, lc.costParts(c)
}

// costParts refuses the cost parts only an activated or hand ability may pay.
func (lc *lowerCtx) costParts(c lCost) error {
	if c.DiscardSelf {
		return refuse("discard cost %q outside a hand ability", "Discard<1/CARDNAME>")
	}
	if c.DiscardF != nil {
		// a filtered discard is a battlefield activated ability's own cost
		// (the candidates are that controller's hand cards; CR 118.3)
		return refuse("filtered discard cost outside an activated ability")
	}
	if c.DiscardHand && (lc.ab == nil || lc.ab.Kind != akActivated) {
		return refuse("discard-hand cost outside an activated ability")
	}
	if c.ReturnSelf {
		return refuse("return-self cost outside an activated ability")
	}
	if c.ExileSelf {
		return refuse("exile-self cost outside an activated ability")
	}
	if c.PutLibSelf {
		return refuse("put-this-card-on-bottom cost outside a graveyard activated ability")
	}
	if c.Untap {
		return refuse("untap-self cost outside an activated ability")
	}
	if c.TapPowN != 0 {
		return refuse("tap-any-number cost outside an activated ability")
	}
	if (c.Waterbend != 0 || c.WaterbendX) && !lc.wbOK {
		return refuse("waterbend cost outside an activated ability's own cost")
	}
	if c.ExileHandN != 0 {
		// ExileFromHand<N/Card> is a printed trigger's own optional cost only
		// (The Wrath of Khan's chapter II; gorge's triggered-cost window). A
		// spell, activated or unless cost keeps refusing it.
		if lc.ab == nil || lc.ab.Kind != akTriggered || len(lc.ab.Prog) != 0 {
			return refuse("ExileFromHand cost outside a trigger's own cost")
		}
	}
	if c.ExileTrigNew {
		// ExileAnyGrave<1/Card.TriggeredNewCard> is a printed dies
		// trigger's own optional cost only (Sandbender Scavengers; gorge's
		// triggered-cost window). An unless, spell or activated cost keeps
		// refusing it.
		if lc.ab == nil || lc.ab.Kind != akTriggered || len(lc.ab.Prog) != 0 {
			return refuse("ExileAnyGrave cost outside a trigger's own cost")
		}
	}
	return nil
}

// exileGraveSecond lowers a second ExileFromGrave part after the first. Only
// ExileFromGrave<1/CARDNAME> followed by ExileFromGrave<N/Spec> of the other
// cards lowers (kop.Cost.ExileGSelf: gorge exAsk settles the self part
// unasked, then asks the others); the spec must exclude the source, a third
// part, the self part second, another head or a bound X stay refused.
func (lc *lowerCtx) exileGraveSecond(c *lCost, head string, parts []string, n int, tok string) error {
	if !lc.actCost || !c.exileGCard || c.ExileGSelf || head != "ExileFromGrave" || parts[1] == "CARDNAME" || c.ExileGBf || c.ExileHand {
		return refuse("second exile cost part %q", tok)
	}
	f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
	if err != nil {
		return err
	}
	for _, cl := range f.Clauses {
		if cl.CmpX != "" || cl.Flags&fOther == 0 {
			return refuse("second exile cost part %q: the spec must exclude the source (Other)", tok)
		}
	}
	c.ExileGSelf, c.exileGCard, c.ExileG, c.ExileGN = true, false, f, n
	return nil
}

// exileGSelfGate refuses the self-and-others graveyard exile anywhere but a
// graveyard activated ability's cost.
func exileGSelfGate(ab *lAbility, cost lCost) error {
	if cost.ExileGSelf && (ab.Kind != akActivated || ab.Zone != zGraveyard) {
		return refuse("exile this card and others cost outside a graveyard activated ability")
	}
	return nil
}

// putLibSelfCost parses PutCardToLibFromGrave<1/-1/CARDNAME[/description]>:
// put the source card from its owner's graveyard on the bottom of that
// library as the cost (CR 118.3, 602.2b; Daniels, Temporal Agent). Only the
// self spec at -1; other positions, other specs and the Hand/Battlefield
// heads stay refused by name.
func putLibSelfCost(c *lCost, parts []string, n int, tok string) error {
	if len(parts) < 3 || len(parts) > 4 || n != 1 || parts[1] != "-1" || parts[2] != "CARDNAME" || c.PutLibSelf {
		return refuse("put-card-to-library cost %q", tok)
	}
	c.PutLibSelf = true
	return nil
}

// putLibSelfGate refuses a put-this-card-on-bottom cost anywhere but a
// graveyard activated ability whose only other cost is mana.
func putLibSelfGate(ab *lAbility, cost lCost) error {
	if !cost.PutLibSelf {
		return nil
	}
	rest := cost
	rest.PutLibSelf = false
	if ab.Kind != akActivated || ab.Zone != zGraveyard || !rest.pureMana() {
		return refuse("put-this-card-on-bottom cost outside a graveyard activated ability's own cost")
	}
	return nil
}

// energyPayCost parses PayEnergy<X> / PayEnergy<SVar>: the "pay one or more
// {E}" resolution unless cost (Aether Refinery, Localized Destruction, Pia
// Nalaar, Rampaging Aetherhood, Territorial Aetherkite, Vault 112). The amount
// is read when the cost is offered, like PayLife<X>'s LifeBy. An activated
// ability's printed cost is the constant kop.CostPayEnergy, so a dynamic one
// there would lose its amount: refuse it.
func (lc *lowerCtx) energyPayCost(c *lCost, parts []string, tok string) error {
	if !lc.unlessParse {
		return refuse("PayEnergy<%s> outside an unless cost", parts[0])
	}
	if len(parts) != 1 || c.Energy != 0 || c.EnergyBy != nil {
		return refuse("energy cost %q", tok)
	}
	v, err := lc.value(parts[0])
	if err != nil {
		return err
	}
	c.EnergyBy = &v
	return nil
}

// costHand is cost, also reading Discard<1/CARDNAME> (DiscardSelf).
func (lc *lowerCtx) costHand(s string) (lCost, error) {
	var c lCost
	var mana []string
	for _, t := range costTokens(strings.TrimSpace(s)) {
		head, arg, hasArg := strings.Cut(t, "<")
		arg = strings.TrimSuffix(arg, ">")
		if !hasArg {
			switch {
			case t == "Mandatory":
				// Forge's cost prefix (rules/cost/parse.go skips the same
				// word): the unless machinery decides pay or decline.
				continue
			case t == "T":
				c.Tap = true
				continue
			case t == "Q" && !c.Untap:
				c.Untap = true // gorge cost.Untap
				continue
			case t == "Forage" && c.Forage == nil:
				// gorge forageAsk's sacrifice candidates: Food.YouCtrl
				f, err := lc.filter("Food.YouCtrl")
				if err != nil {
					return c, err
				}
				c.Forage = f
				continue
			case isManaSym(t):
				mana = append(mana, t)
				continue
			case len(t) == 2 && t[1] == 'P' && strings.IndexByte("WUBRG", t[0]) >= 0:
				mana = append(mana, t) // a Phyrexian pip
				continue
			}
			if strings.HasPrefix(t, "XMin") {
				if err := xminToken(&c, t); err != nil {
					return c, err
				}
				continue
			}
			return c, refuse("cost token %q", t)
		}
		parts := strings.Split(arg, "/")
		if head == "Sac" && parts[0] == "X" {
			// the third part is prompt text, which may hold a slash
			// ("any number of artifacts and/or creatures")
			parts = strings.SplitN(arg, "/", 3)
			// Sac<X/Spec> (lower_xminsacx.go): the announced count
			if err := lc.sacXCost(&c, parts, t); err != nil {
				return c, err
			}
			continue
		}
		if head == "CollectEvidence" {
			// CollectEvidence<N> (CR 701.59a): exile graveyard cards with
			// total mana value N or greater (gorge cost.parse's Evidence
			// part). A variable <X> or a named amount is not modelled.
			if len(parts) != 1 || c.EvidenceN != 0 || c.EvidenceX {
				return c, refuse("evidence cost %q", t)
			}
			if parts[0] == "X" {
				// CollectEvidence<X>: the announced X (gorge costCarriesEvidenceX);
				// only a trigger's own cost lowers it (lower_effect.go payInstr)
				c.EvidenceX = true
				continue
			}
			n, err := strconv.Atoi(parts[0])
			if err != nil {
				return c, refuse("evidence cost %q: a named amount", t)
			}
			if n < 1 {
				return c, refuse("evidence cost %q", t)
			}
			c.EvidenceN = n
			continue
		}
		if head == "Blight" {
			// Blight<N> (CR 701.68): N -1/-1 counters on a creature you
			// control, chosen as a cost (gorge cost.parse's Blight part).
			// A variable Blight<X> is not modelled.
			if len(parts) != 1 {
				return c, refuse("blight cost %q", t)
			}
			n, err := strconv.Atoi(parts[0])
			if err != nil {
				return c, refuse("blight cost %q: a variable amount", t)
			}
			if n < 1 {
				return c, refuse("blight cost %q", t)
			}
			c.BlightN += n
			continue
		}
		if head == "Behold" {
			// Behold<N/Type> (CR 701.4a): reveal or choose N permanents or
			// hand cards of the type, as a spell's additional cost. Only
			// an AlternateAdditionalCost's first part, a MayFlashCost
			// extra, an OptionalCost part and a flashback cost read it
			// (beholdOK); each caller bounds N and the parts beside it.
			if !(lc.beholdOK || lc.unlessParse) || len(parts) != 2 || c.BeholdN != 0 {
				return c, refuse("behold cost %q", t)
			}
			n, err := strconv.Atoi(parts[0])
			if err != nil || n < 1 {
				return c, refuse("behold cost %q: not a fixed count", t)
			}
			if strings.Contains(parts[1], "ChosenType") {
				// the type a ChooseCreatureType cost part names
				return c, refuse("behold cost %q: of a type chosen as a cost", t)
			}
			if !effects.CreatureTypeWords(parts[1]) && !slices.Contains(effects.TypeChoiceLabels("Planeswalker", "", ""), parts[1]) {
				// a creature or planeswalker subtype (Countersculpt's Jace)
				return c, refuse("behold cost %q: not a creature or planeswalker type", t)
			}
			f, err := lc.filter(parts[1])
			if err != nil {
				return c, err
			}
			c.BeholdN, c.BeholdF = n, f
			continue
		}
		if head == "Waterbend" {
			// Waterbend<N> (CR 701.67): N generic in the cost may be paid by
			// tapping artifacts/creatures instead. Waterbend<X> (an
			// activated ability's own cost, Katara, Water Tribe's Hope) adds
			// one {X} the picks may pay (gorge cost.parse's c.X++ /
			// WaterbendX); one waterbend part per cost.
			if len(parts) != 1 {
				return c, refuse("waterbend cost %q", t)
			}
			if parts[0] == "X" {
				if !lc.actCost {
					// an unless, ward or optional cost's Waterbend<X> has no
					// announced X the kernel reads
					return c, refuse("waterbend cost %q: a variable amount outside an activated ability's own cost", t)
				}
				if c.WaterbendX || c.Waterbend > 0 {
					return c, refuse("waterbend cost %q: a second waterbend part", t)
				}
				c.WaterbendX = true
				mana = append(mana, "X")
				continue
			}
			if c.WaterbendX {
				return c, refuse("waterbend cost %q: a second waterbend part", t)
			}
			n, err := strconv.Atoi(parts[0])
			if err != nil {
				return c, refuse("waterbend cost %q: a variable amount", t)
			}
			if n < 1 {
				return c, refuse("waterbend cost %q", t)
			}
			c.Waterbend += n
			mana = append(mana, strconv.Itoa(n))
			continue
		}
		if head == "Exert" {
			// Exert<1/NICKNAME> (CR 702.100a): the source is exerted as the
			// ability's cost. It taps nothing and removes nothing; the
			// payment stamps the one-shot no-untap mark. Only a printed
			// activated ability's own cost, and only the source.
			if len(parts) < 2 || len(parts) > 3 || parts[0] != "1" || c.Exert || !lc.actCost {
				return c, refuse("exert cost %q", t)
			}
			if !strings.EqualFold(parts[1], "NICKNAME") && !strings.EqualFold(parts[1], "CARDNAME") {
				return c, refuse("exert cost %q: not the source", t)
			}
			c.Exert = true
			continue
		}
		if head == "tapXType" && parts[0] == "Any" {
			// tap any number with a total-power floor (Crew, Saddle;
			// lower_tappower.go): an activated ability's whole cost only
			if err := lc.tapPowerCost(&c, parts, t); err != nil {
				return c, err
			}
			continue
		}
		if head == "SubCounter" && len(parts) == 2 && parts[0] == "X" && parts[1] == "LOYALTY" {
			// a planeswalker's [-X] (CR 606.4): X announced up to the loyalty
			// it has, that many loyalty counters removed (gorge
			// rules xAsk / cast_subcounter.go); alone among the counter parts
			if c.Loyalty != 0 || c.LoyaltyX || c.SubCtr != 0 || c.SubAmongN != 0 || c.AddCtr != 0 {
				return c, refuse("counter cost %q: more than one counter part", t)
			}
			if c.LifeX {
				return c, refuse("life cost PayLife<X> beside a second announced-X part")
			}
			c.LoyaltyX = true
			continue
		}
		if head == "SubCounter" && len(parts) >= 2 && parts[0] == "X" && !strings.EqualFold(parts[1], "LOYALTY") {
			// SubCounter<X/KIND> (CR 107.3, 118.3): the activation announces
			// X up to the counters of that kind the source has, then removes
			// that many (gorge cost.parse's announced part, cast_subcounter.go).
			// Only the source form; a filtered target is another payment.
			if len(parts) > 3 || c.SubCtrX || c.SubCtr != 0 || c.SubAmongN != 0 || c.Loyalty != 0 || c.LoyaltyX || c.AddCtr != 0 || c.LifeX {
				return c, refuse("counter cost %q", t)
			}
			if strings.EqualFold(parts[1], "Any") {
				return c, refuse("counter cost %q: a wildcard kind", t)
			}
			if len(parts) == 3 && !strings.EqualFold(parts[2], "CARDNAME") && !strings.EqualFold(parts[2], "NICKNAME") {
				return c, refuse("counter cost %q: a filtered target", t)
			}
			if !lc.actCost && (lc.ab == nil || lc.ab.Kind != akMana) {
				return c, refuse("counter cost %q outside an activated ability's own cost", t)
			}
			c.SubCtrX, c.SubCtrXKnd = true, parts[1]
			continue
		}
		if head == "tapXType" && parts[0] == "X" {
			// tapXType<X/Spec>: tap any number, X the count (lower_tapxvar.go)
			if err := lc.tapXVarCost(&c, parts, t); err != nil {
				return c, err
			}
			continue
		}
		if head == "PayEnergy" && (parts[0] == "X" || lc.face.SVars[parts[0]] != "") {
			if err := lc.energyPayCost(&c, parts, t); err != nil {
				return c, err
			}
			continue
		}
		n, err := strconv.Atoi(parts[0])
		if err != nil && head != "PayLife" {
			return c, refuse("cost token %q", t)
		}
		switch head {
		case "Sac":
			if len(parts) < 2 {
				return c, refuse("cost token %q", t)
			}
			if parts[1] == "NICKNAME" {
				// gorge pay.SacrificeMatchSpec (engine/rules/quiet_facts.go
				// sacrificeMatchSpec) reads Forge's NICKNAME spec as CARDNAME
				// -- the source itself, which is also Forge
				// (CostPart.payCostFromSource) and XMage
				// (SacrificeSourceCost). Measured at this pin: gorge offers
				// both activations (Syr Ginger, the Meal Ender and Adric,
				// Mathematical Genius, engine/rules nickname_sac_test.go), so
				// the same reading as CARDNAME.
				c.SacSelf = true
				continue
			}
			if parts[1] == "CARDNAME" {
				c.SacSelf = true
				continue
			}
			if parts[1] == "OriginalHost" {
				// the granting Equipment or Aura (gorge bindGrantedCostReferents:
				// the one candidate is the grantor, settled unasked)
				if n != 1 || c.SacHost {
					return c, refuse("cost token %q", t)
				}
				c.SacHost = true
				continue
			}
			// gorge cost.parse folds a cost spec's ";" alternation to ","
			if c.SacX || c.SacExile {
				return c, refuse("sacrifice cost %q beside a second sacrifice part", t)
			}
			f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
			if err != nil {
				return c, err
			}
			c.Sac, c.SacN = f, n
		case "ExileFromGrave", "ExileCtrlOrGrave":
			// <N/Spec> or <N/Spec/description>: the description is the
			// prompt text only. ExileCtrlOrGrave (Craft's material, gorge
			// cost.parse's ZoneSet battlefield|graveyard part)
			// draws from the payer's other permanents too.
			if len(parts) < 2 || len(parts) > 3 || n <= 0 {
				return c, refuse("exile cost %q", t)
			}
			if c.ExileG != nil {
				// the one two-part shape: ExileFromGrave<1/CARDNAME> then
				// ExileFromGrave<N/Spec> of the other cards (Say Its Name)
				if err := lc.exileGraveSecond(&c, head, parts, n, t); err != nil {
					return c, err
				}
				continue
			}
			if head == "ExileCtrlOrGrave" {
				if parts[1] == "CARDNAME" {
					return c, refuse("exile cost %q", t)
				}
				c.ExileGBf = true
			}
			spec := parts[1]
			if spec == "CARDNAME" {
				if n != 1 {
					return c, refuse("exile cost %q", t)
				}
				spec = "Card.Self"
			}
			// gorge cost.parse folds a cost spec's ";" alternation to ","
			// (Creature;Artifact: either type)
			f, err := lc.filter(strings.ReplaceAll(spec, ";", ","))
			if err != nil {
				return c, err
			}
			for _, cl := range f.Clauses {
				if cl.CmpX != "" {
					// gorge binds an announced X into the spec (exAsk's
					// SpecContext); not modelled
					return c, refuse("exile cost %q: a variable bound", t)
				}
			}
			c.ExileG, c.ExileGN = f, n
			c.exileGCard = parts[1] == "CARDNAME" && head == "ExileFromGrave"
		case "ExileFromHand":
			// ExileFromHand<1/Card[/description]> in the two shapes gorge
			// lowers: the exile-granted ability's cost (lc.grantExile;
			// lower_exilegrant.go) exiles the ExileG cards from the payer's
			// hand, and a printed trigger's own cost (costParts gates the
			// place; cost_exilefromhand.go) keeps its own count. A filtered
			// spec, ExileFromHand<1/All> and CARDNAME stay refused.
			if lc.altCostHand {
				if err := lc.altExileHandCost(&c, parts, n, t); err != nil {
					return c, err
				}
				continue
			}
			if len(parts) < 2 || len(parts) > 3 || n <= 0 || parts[1] != "Card" {
				return c, refuse("exile cost %q", t)
			}
			if lc.grantExile {
				if n != 1 || c.ExileG != nil || c.ExileHandN != 0 {
					return c, refuse("cost token %q", t)
				}
				f, err := lc.filter("Card")
				if err != nil {
					return c, err
				}
				c.ExileG, c.ExileGN, c.ExileHand = f, n, true
			} else {
				if c.ExileHandN != 0 || c.ExileG != nil {
					return c, refuse("exile cost %q", t)
				}
				c.ExileHandN = n
			}
		case "PutCardToLibFromGrave":
			if err := putLibSelfCost(&c, parts, n, t); err != nil {
				return c, err
			}
		case "ExileAnyGrave":
			// ExileAnyGrave<N/Spec> (gorge cost.parse): exile N cards from
			// any graveyard. Only <1/Card.TriggeredNewCard>, optionally with
			// a description part, lowers: a printed dies trigger's own "you
			// may exile it" cost, where the bare TriggeredNewCard property
			// matches exactly the card the triggering event captured (gorge
			// filters.go, bound through the triggered-cost window's trigger
			// context). costParts keeps it out of
			// every non-trigger cost; payInstr reads the shape.
			if n != 1 || len(parts) < 2 || len(parts) > 3 || parts[1] != "Card.TriggeredNewCard" || c.ExileTrigNew {
				return c, refuse("cost token %q", t)
			}
			c.ExileTrigNew = true
		case "Discard":
			// One discard part per cost: lCost carries a single N/Spec, so a
			// second token would silently overwrite the first (Foil's
			// alternative cost is the corpus's only multi-discard line and
			// refuses on its Island spec already).
			if c.DiscardN > 0 || c.DiscardSelf || c.DiscardF != nil {
				return c, refuse("discard cost %q: more than one discard part", t)
			}
			if (len(parts) == 2 || len(parts) == 3) && parts[1] == "CARDNAME" && n == 1 {
				c.DiscardSelf = true // cycling (CR 702.29a): see costHand
				continue
			}
			// Discard<N/Spec[/Description]> (gorge cost.nonManaCost): the
			// third part is the reminder text only. Card is any N cards;
			// Random and Hand are the unless-cost selection methods
			// and are refused outside one.
			// Gorge's triggered-cost window ignores the count for a Hand
			// discard (it pays the whole hand whatever the token spells;
			// the corpus writes the ignored count as 0), so Discard<0/Hand>
			// reads as Discard<1/Hand>. Every other n < 1 still refuses.
			if len(parts) == 2 && parts[1] == "Hand" && n == 0 {
				n = 1
			}
			if len(parts) < 2 || len(parts) > 3 || n < 1 {
				return c, refuse("discard cost %q", t)
			}
			switch parts[1] {
			case "Card":
				c.DiscardSpec = ""
			case "Random", "Hand":
				c.DiscardSpec = parts[1]
			default:
				// A filtered discard (CR 118.3, 601.2h; gorge cost
				// nonManaCost's spec over pay.DiscardCandidates'
				// MatchesSpecFrom): Solitary Cell's Card.Legendary,
				// grandeur's Card.named<self>. The filter split keeps a
				// comma inside a named<Name> argument (splitAlternatives).
				f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
				if err != nil {
					return c, err
				}
				c.DiscardF = f
			}
			c.DiscardN = n
		case "AddCounter", "SubCounter", "RemoveAnyCounter":
			// Counter costs share gorge's reading (cost.parse): N, the kind
			// ("Any" is the wildcard), and an optional target field. A
			// CARDNAME/NICKNAME target (or none, for SubCounter) is the
			// source; any other target is a filter. A fourth field is the
			// reminder text only. AddCounter has no target field: a third
			// field is a different gorge payment (Sanctuary Wall) and stays
			// refused.
			if len(parts) < 2 || len(parts) > 4 {
				return c, refuse("counter cost %q", t)
			}
			kind := parts[1]
			if kind == "LOYALTY" && head != "RemoveAnyCounter" {
				if len(parts) != 2 {
					return c, refuse("counter cost %q", t)
				}
				if head == "AddCounter" {
					c.Loyalty = n
				} else {
					c.Loyalty = -n
				}
				continue
			}
			if head == "AddCounter" {
				// AddCounter<N/KIND> is a source-anchored cost. An
				// UnlessCost$ may also spell the recipient: CARDNAME/NICKNAME
				// is the source again, Card.targetedBy the parent's single
				// target. Any other third field is a different payment and
				// stays refused; a fourth field is reminder text only.
				if c.AddCtr != 0 || len(parts) < 2 || len(parts) > 4 {
					return c, refuse("counter cost %q", t)
				}
				if len(parts) == 2 {
					c.AddCtr, c.AddCtrKnd = n, kind
					continue
				}
				if !lc.unlessParse {
					return c, refuse("counter cost %q outside an unless cost", t)
				}
				switch {
				case strings.EqualFold(parts[2], "CARDNAME"), strings.EqualFold(parts[2], "NICKNAME"):
					c.AddCtrRef = lRef{Kind: rSelf}
				case strings.EqualFold(parts[2], "Card.targetedBy"):
					if len(lc.parentChain()) != 1 {
						return c, refuse("counter cost %q: targetedBy without one parent target", t)
					}
					slot := lc.parentLastTarget()
					if slot < 0 || slot > 2 {
						return c, refuse("counter cost %q: targetedBy target slot %d", t, slot)
					}
					c.AddCtrRef = lRef{Kind: rTarget, N: slot}
				default:
					return c, refuse("counter cost %q: AddCounter target %q", t, parts[2])
				}
				c.AddCtr, c.AddCtrKnd = n, kind
				continue
			}
			if c.SubCtr != 0 || c.SubAmongN != 0 {
				return c, refuse("counter cost %q: more than one counter-removal part", t)
			}
			src := len(parts) == 2
			if len(parts) >= 3 {
				if strings.EqualFold(parts[2], "CARDNAME") || strings.EqualFold(parts[2], "NICKNAME") {
					src = true
				}
			}
			if src && head == "SubCounter" && strings.EqualFold(kind, "Any") {
				// SubCounter<1/Any> names no source and no filter: the
				// corpus's wildcard removals always spell one out.
				return c, refuse("counter cost %q: Any without a source or filter", t)
			}
			if src {
				if kind == "Any" {
					// a source-anchored wildcard shares Cost.N/Filter with
					// CostSubAmong (Filter 0 names the source)
					c.SubAmongN, c.SubAmongKnd, c.SubAmongF = n, "Any", nil
				} else {
					c.SubCtr, c.SubCtrKnd = n, kind
				}
				continue
			}
			f, err := lc.filter(strings.ReplaceAll(parts[2], ";", ","))
			if err != nil {
				return c, err
			}
			c.SubAmongN, c.SubAmongKnd, c.SubAmongF = n, kind, f
		case "Return":
			// Return<N/Spec[/description]> (gorge cost.parse returnCost)
			if len(parts) < 2 || len(parts) > 3 || n < 1 || c.ReturnSelf || c.Return != nil {
				return c, refuse("return cost %q", t)
			}
			if parts[1] == "CARDNAME" {
				if n != 1 {
					return c, refuse("return cost %q", t)
				}
				c.ReturnSelf = true
				continue
			}
			f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
			if err != nil {
				return c, err
			}
			c.Return, c.ReturnN = f, n
		case "ExileFromStack":
			// ExileFromStack<1/Card.Self> (gorge cost.parse's stack part):
			// the resolving spell exiles itself, as an unless cost beside its
			// graveyard exile (payInstr pairs them). An activated cost's other
			// stack specs stay refused.
			if !lc.unlessParse || n != 1 || len(parts) < 2 || len(parts) > 3 || parts[1] != "Card.Self" || c.ExileStackSelf {
				return c, refuse("cost token %q", t)
			}
			c.ExileStackSelf = true
		case "Exile":
			// Exile<1/CARDNAME[/description]> (gorge exAsk takes the
			// source from the battlefield); any other Exile<> refuses
			if len(parts) < 2 || len(parts) > 3 || n < 1 || c.ExileSelf {
				return c, refuse("cost token %q", t)
			}
			switch parts[1] {
			case "CARDNAME", "NICKNAME":
				if n != 1 {
					return c, refuse("cost token %q", t)
				}
				c.ExileSelf = true
			case "OriginalHost":
				// Exile<1/OriginalHost> (The Dominion Bracelet): the granting
				// Equipment's exile, the sacrifice-host part with the exile
				// destination (gorge bindGrantedCostReferents binds it)
				if n != 1 || c.SacHost || !lc.actCost {
					return c, refuse("cost token %q", t)
				}
				c.SacHost, c.HostExile = true, true
			default:
				// Exile<N/Spec> from the battlefield: the Sac<N/Spec> pick
				// with the exile destination (kop.Cost.ExileBf)
				if !lc.actCost && !lc.lifeXSpell {
					return c, refuse("exile cost %q outside an activated ability's or a spell's own cost", t)
				}
				if c.Sac != nil || c.SacX {
					return c, refuse("exile cost %q beside a sacrifice part", t)
				}
				f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
				if err != nil {
					return c, err
				}
				c.Sac, c.SacN, c.SacExile = f, n, true
			}
		case "PayEnergy":
			// gorge cost.Energy: a constant N; PayEnergy<X> refuses
			if len(parts) != 1 || n < 1 || c.Energy != 0 {
				return c, refuse("energy cost %q", t)
			}
			c.Energy = n
		case "PayLife":
			if arg == "X" {
				// gorge Cost.LifeX: an announced life part paying the
				// stack entry's X (kernel cost_paylifex.go); only the
				// Count$xPaid body is read (pay.FixLifeXCost folds any other
				// into a fixed amount), on a spell or an activated ability
				if x, ok := lc.face.SVars["X"]; ok && strings.TrimSpace(x) != "Count$xPaid" {
					// A printed trigger's own "pay life equal to its power": gorge
					// folds the body into a fixed amount as the cost window opens
					// (evalTriggerCostFixedX), so the kernel reads the same value as
					// the payment is offered. Only the power of the card an enters
					// trigger fires for lowers (the shape played against gorge), and the
					// opponents dealt combat damage this turn on a postcombat-main Phase
					// trigger (Tymna the Weaver); every other body or trigger refuses
					// by name.
					if lc.trigBodyCost && lc.ab.Trigger != nil && triggerCostXShape(lc.ab.Trigger, strings.TrimSpace(x)) &&
						c.PayLife == 0 && !c.LifeX && c.LifeBy == nil && !c.LoyaltyX {
						v, verr := lc.svarValue(x, 0)
						if verr != nil {
							return c, verr
						}
						c.LifeBy = &v
						continue
					}
					return c, refuse("life cost PayLife<X> with SVar X %q", x)
				}
				if !lc.lifeXSpell && !lc.actCost {
					return c, refuse("life cost PayLife<X> on a %s ability", "non-spell, non-activated")
				}
				if c.LifeX || c.PayLife != 0 || c.LoyaltyX {
					return c, refuse("life cost PayLife<X> beside a second life or announced-X part")
				}
				c.LifeX = true
				continue
			}
			n, err := strconv.Atoi(arg)
			if err != nil || c.LifeX {
				return c, refuse("life cost %q", t)
			}
			c.PayLife = n
		case "tapXType":
			if len(parts) < 2 {
				return c, refuse("tap cost %q", t)
			}
			if parts[1] == "OriginalHost" {
				c.TapHost = true
				continue
			}
			f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
			if err != nil {
				return c, err
			}
			// tapXType taps untapped permanents you control matching the
			// spec (gorge TapCostCandidates: CostCandidates over the spec);
			// a subtype-only spec is any permanent of the subtype (a Gate,
			// a Vehicle), not only a creature
			for i := range f.Clauses {
				f.Clauses[i].Ctrl = whoYou
				f.Clauses[i].Flags |= fUntapped
			}
			c.TapN, c.TapF = n, f
		case "Draw":
			// Draw<N/You> (gorge cost.parse nonManaCost; paid in the
			// triggered-cost window by drawing, cumulative.go)
			if len(parts) != 2 || parts[1] != "You" || n < 1 || c.DrawN != 0 {
				return c, refuse("cost token %q", t)
			}
			c.DrawN = n
		default:
			return c, refuse("cost token %q", t)
		}
	}
	if (c.EvidenceN > 0 || c.EvidenceX) && (c.Forage != nil || c.ExileG != nil) || c.EvidenceX && c.ExileTrigNew {
		// gorge reserves a graveyard component's picks before the evidence
		// stage (cumulative.go evidenceReserveOrder): only the dies trigger's
		// own triggered-card exile is modelled (Lamplight Phoenix,
		// payInstr's ExileTrigNew + evidence pair)
		return c, refuse("evidence cost beside a graveyard-reading part")
	}
	if c.Forage != nil || c.ExileG != nil || c.DiscardF != nil {
		// a forage / exile / filtered-discard part shares Cost.N and
		// Cost.Filter: alone among the counted parts (dirac/kgen/build.go
		// cost)
		parts := 0
		for _, on := range []bool{c.Forage != nil, c.ExileG != nil, c.Sac != nil && !(lc.escCost && c.SacExile), c.DiscardN > 0, c.Loyalty != 0, c.SubCtr > 0, c.SubAmongN > 0, c.PayLife > 0 && !lc.altLifeShared(&c), c.TapN > 0, c.EvidenceN > 0} {
			if on {
				parts++
			}
		}
		if parts > 1 {
			return c, refuse("cost %q: a forage / exile part beside another counted part", s)
		}
	}
	if c.BeholdN > 0 && (c.BlightN != 0 || c.Sac != nil || c.DiscardN > 0 || c.Forage != nil || c.ExileG != nil || c.Tap || c.PayLife > 0) {
		return c, refuse("behold cost %q beside another part", s)
	}
	if c.ExileStackSelf && c.ExileG == nil {
		return c, refuse("ExileFromStack self without a graveyard exile")
	}
	if c.BlightN > 0 {
		parts := 0
		for _, on := range []bool{c.BlightN > 0, c.Forage != nil, c.ExileG != nil, c.Sac != nil, c.DiscardN > 0, c.Loyalty != 0, c.SubCtr > 0, c.SubAmongN > 0, c.PayLife > 0, c.TapN > 0, c.Waterbend > 0 || c.WaterbendX, c.Energy > 0, c.Return != nil, c.AddCtr > 0, c.EvidenceN > 0} {
			if on {
				parts++
			}
		}
		if parts > 1 {
			return c, refuse("blight cost beside another counted part")
		}
	}
	if len(mana) > 0 {
		mc, err := parseAbilityManaCost(strings.Join(mana, " "))
		if err != nil {
			return c, refuse("%v", err)
		}
		c.Mana = mc
	} else {
		c.Mana.None = true
	}
	if err := lc.announcedXCheck(&c, s); err != nil {
		return c, err
	}
	return c, nil
}

func isManaSym(t string) bool {
	if t == "X" || t == "C" {
		return true
	}
	if _, err := strconv.Atoi(t); err == nil {
		return true
	}
	for i := 0; i < len(t); i++ {
		if strings.IndexByte("WUBRG", t[i]) < 0 {
			return false
		}
	}
	return len(t) <= 2
}

// filter parses a Valid expression against the pool's subtypes.
func (lc *lowerCtx) filter(expr string) (*lFilter, error) {
	expr = lc.triggeredTargetCtrl(expr)
	expr, err0 := lc.spawnerTargetCtrl(expr) // lower_filterwords.go
	if err0 != nil {
		return nil, err0
	}
	expr, err0 = lc.spawnerDefendingCtrl(expr) // lower_filterwords.go
	if err0 != nil {
		return nil, err0
	}
	expr = expandBasicLandType(expr)
	if err := lc.referentGate(expr); err != nil {
		return nil, err
	}
	if err := lc.imprintedFilterGate(expr); err != nil { // lower_imprintzone.go
		return nil, err
	}
	f, err := parseFilter(expr, lc.known, lc.p.isTypeWord)
	if err != nil {
		return nil, refuse("filter %q: %v", expr, err)
	}
	if strings.Contains(expr, "attackingYou") {
		lc.twoSeat("attackingYou read as an attacker an opponent controls")
	}
	for i := range f.Clauses {
		c := &f.Clauses[i]
		if c.Flags&fNamedAsSource != 0 {
			// every named<X> is a literal clause (gorge wordNamed ->
			// sharesName reads the NAME, not the source; CR 201.2):
			// build.go resolves Name against the pool, Pred = PredNamed.
			// The source's own printed name is no exception: a
			// source-relative read follows the source's current name (a
			// layer-3 rename) and matches nothing once the source has left
			// the battlefield (Hare Apparent killed in response to its own
			// trigger), where gorge's literal name still matches
			c.Flags &^= fNamedAsSource
		}
		if c.CmpX != "" {
			v, err := lc.value(c.CmpX)
			if err != nil {
				return nil, err
			}
			if v.Kind == vX && lc.ab != nil && lc.ab.Cost.LoyaltyX {
				// a mana-value bound by a planeswalker's [-X] (Tezzeret the
				// Seeker, Sorin, Vengeful Bloodlord): no lockstep reads it yet
				return nil, refuse("filter %q: bound %s reads a loyalty [-X]", expr, c.CmpX)
			}
			c.CmpV = &v
		}
		if c.CmpStat == cmpWasCast && lc.spellProvOK && !lc.wasCastOK {
			// the same words read on a spell on the stack
			if c.CmpN&wasCastSpentZero != 0 {
				return nil, refuse("filter %q: CastSa Spell.ManaSpent EQ0 on a spell", expr)
			}
			if c.CmpN&wasCastWebSling != 0 {
				return nil, refuse("filter %q: webSlinged on a spell", expr)
			}
			if c.CmpN&^wasCastNot != wasCastAny {
				return nil, refuse("filter %q: wasCastFromGraveyard on a spell", expr)
			}
			n := spellCast
			if c.CmpN&wasCastNot != 0 {
				n = spellNot
			}
			c.CmpStat, c.CmpN = cmpWasCastSpell, n
		}
	}
	for _, c := range f.Clauses {
		if c.Flags&fBargained != 0 && !lc.out.Bargain {
			// the word is a Bargain carrier's own provenance (CR 702.166;
			// F2Kicked on the permanent); on any other card it would read
			// the kicker/offspring mark, so refuse it
			return nil, refuse("filter %q: bargained without Bargain", expr)
		}
		if c.Flags&fTeamwork != 0 && lc.out.Teamwork == 0 {
			// the same provenance bit on a Teamwork carrier only
			return nil, refuse("filter %q: Teamwork without Teamwork", expr)
		}
		if c.Unblocked && !lc.unblockedOK {
			// F2Unblocked is lockstep-proven only as K:Ninjutsu's Return
			// cost; a static, an effect's filter or a
			// target stays refused
			return nil, refuse("filter %q: unblocked outside a ninjutsu Return cost", expr)
		}
		if c.CmpStat == cmpExiledWith || c.Flags&fExiledWith != 0 {
			// the kernel reads the link against the matcher's source: the
			// resolving entry's, or a Continuous static's own object. A
			// target, a cost or a granted ability has none
			if lc.granted > 0 || !(lc.exwOK && lc.ab != nil || lc.exwStatic) {
				return nil, refuse("filter %q: ExiledWithSource outside a resolution-time mover or a continuous static", expr)
			}
			lc.out.ExiledWith = true
			if !lc.exwSynth {
				lc.out.ExwWord = true
			}
		}
		if c.Ctrl == whoHitBySrc {
			// the record is the card's own objects' hits: the filter's source
			// must be the card (a granted ability's is another card)
			if lc.granted > 0 || lc.ab == nil {
				return nil, refuse("filter %q: wasDealtCombatDamageThisTurnBySource outside the card's own ability", expr)
			}
			lc.out.CombatHitRec = true
		}
		if c.CmpStat == cmpWasCast && !lc.wasCastOK {
			if c.CmpN&wasCastWebSling != 0 {
				return nil, refuse("filter %q: webSlinged outside a battlefield-entering trigger's ValidCard$", expr)
			}
			// the kernel reads the cast only while the entering spell is
			// still resolving (kernel enterCast); a later read has no spell
			return nil, refuse("filter %q: wasCast outside a battlefield-entering trigger's ValidCard$", expr)
		}
		if c.TheirHand && !lc.cantCastProv {
			return nil, refuse("filter %q: wasCastFromTheirHand outside a CantBeCast ValidCard$", expr)
		}
		if c.CmpStat == cmpWasCastSpell && !lc.spellProvOK {
			// the kernel finds the cast only on a spell's own stack entry
			// (spellEntryOf): a SpellCast trigger or a spell's own condition
			return nil, refuse("filter %q: a cast-provenance word outside a SpellCast ValidCard$ or a spell's ConditionPresent$", expr)
		}
		if c.SpellBase && !lc.spellBaseOK {
			// gorge's Spell base is an object on the stack; only a
			// target-gated ReduceCost reads it (build_ab: Filter2 in
			// ZoneStack)
			return nil, refuse("filter %q: a Spell head outside a ReduceCost ValidTarget$", expr)
		}
		if (c.Ctrl == whoDefending || c.Owner == whoDefending) && !lc.atkPresent && !lc.spawnAtk && (!lc.trigBody || lc.ab == nil || lc.ab.Trigger == nil || lc.ab.Trigger.Event != "Attacks") {
			return nil, refuse("ControlledBy TriggeredDefendingPlayer outside an Attacks trigger's battlefield filter")
		}
		if c.CmpX != "" && (c.CmpV == nil || c.CmpV.Kind != vChosenNumber) {
			if _, ok := lc.face.SVars[c.CmpX]; !ok {
				return nil, refuse("filter %q: bound %s names no SVar", expr, c.CmpX)
			}
		}
	}
	for i := range f.Clauses {
		if f.Clauses[i].Owner == whoTgtCtrl {
			// the referent is the current link's target, a card off the
			// battlefield: gorge binds an object target's controller only
			// (a player target binds nothing), so the slot names no player
			t := lc.lastTarget()
			if t < 0 || !offFieldCardSlot(lc.ab.Targets[t].Zone) {
				return nil, refuse("filter %q: OwnedBy TargetedController without an earlier off-battlefield card target", expr)
			}
			f.Clauses[i].CtrlTgt = uint8(t + 1)
			continue
		}
		if f.Clauses[i].Ctrl != whoTgtOrCtrl {
			continue
		}
		// the referent is the current link's target -- the last slot the
		// chain declared before this filter. A filter read before any slot
		// has none, and gorge binds none.
		t := lc.lastTarget()
		if t < 0 {
			return nil, refuse("filter %q: ControlledBy TargetedOrController without an earlier target", expr)
		}
		f.Clauses[i].CtrlTgt = uint8(t + 1)
	}
	if err := lc.checkEachPlCtrl(expr, f); err != nil {
		return nil, err // lower_repeatplayers.go
	}
	return f, nil
}

// lowerAbility lowers one printed SP/AB line.
func (lc *lowerCtx) lowerAbility(sa *cards.SA) error {
	ab := &lAbility{Index: len(lc.out.Abilities)}
	lc.ab = ab
	lc.chainT = nil
	lc.xPaidAb = nil
	// a chosen-colour chain never crosses an ability boundary (the kernel's
	// res.chosen lives for one resolution): a chain that ends without the
	// ClearChosenColor$ Cleanup (Puca's Eye's ETB)
	// must not leak colGrant into the next ability's Pump/Animate reading
	lc.colGrant = false
	lc.colTgt = lRef{}
	lc.remTgt = 0
	ps := newParams(sa.Params)
	switch sa.Kind {
	case "SP":
		ab.Kind = akSpell
		// XColor$ is the colour the {X} must be paid with (Forge's
		// SpellAbility.getXColor; Consume Spirit's "Spend only black mana
		// on X"). It rides the spell ability, beside the printed cost.
		if xc, ok := ps.get("XColor"); ok {
			if err := lc.xColor(&lc.out.Cost, xc); err != nil {
				return err
			}
		}
		if lc.out.Cost.X || lc.out.Cost.XX || lc.out.Cost.XXX {
			lc.xPaidAb = ab
		}
		if ip, ok := ps.get("IsPresent"); ok {
			// "cast this spell only if <present>" (gorge spellTimingPre's
			// IsPresent$ cast gate, cr-spell-ispresent-cast-gate feb134b15;
			// Confront the Assault): the kernel withholds the cast
			// (castForbidden) unless the count of the controller's view holds
			for _, k := range [...]string{"PresentZone", "PresentPlayer", "PresentDefined"} {
				if _, ok := ps.get(k); ok {
					return refuse("spell IsPresent %s", k)
				}
			}
			f, err := lc.filter(ip)
			if err != nil {
				return err
			}
			c := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
			if pc, ok := ps.get("PresentCompare"); ok {
				op, v, err := lc.cmpSpec(pc)
				if err != nil {
					return err
				}
				c.Op, c.A = op, v
			}
			ab.ActCond = c
		}
		if c, ok := ps.get("Cost"); ok {
			lc.lifeXSpell = true
			cost, err := lc.cost(c)
			lc.lifeXSpell = false
			if err != nil {
				return err
			}
			if cost.Energy != 0 {
				return refuse("energy cost on a spell")
			}
			if cost.Return != nil {
				return refuse("return cost on a spell")
			}
			if cost.EvidenceX {
				// CollectEvidence<X>, X the targets' total mana value (Urgent
				// Necropsy; gorge cast_targets.go evidenceAmount): the
				// kernel's bsEvidence stage reads the chosen targets
				// (cost_collectevidencex.go)
				if x := strings.TrimSpace(lc.face.SVars["X"]); x != "AllTargeted$CardManaCost" || lc.out.Types&(tInstant|tSorcery) == 0 {
					return refuse("evidence cost %q: a named amount", "CollectEvidence<X>")
				}
				ab.EvidenceTargets = true
				cost.EvidenceX = false
			}
			// The printed mana cost rides on the card record; the SP
			// Cost$ adds the non-mana additional costs.
			if !cost.Mana.None && cost.Mana.String() != lc.out.Cost.String() {
				return refuse("spell Cost$ %q differs from the printed cost", c)
			}
			cost.Mana = manaCost{None: true}
			lc.out.SpellCost = &cost
		}
		if sa.API == "PermanentCreature" {
			// The permanent spell itself: only its additional cost matters.
			ps.consume("AdditionalDesc") // display text of a self discount (Dargo's "costs {2} less ...")
			if left := ps.unread(); len(left) > 0 {
				return refuse("unread params %v", left)
			}
			return nil
		}
		if lc.face.Types != nil && (lc.out.Types&tPermanent) != 0 && sa.API != "Attach" {
			return refuse("spell ability on a permanent card")
		}
	case "AB":
		ab.Kind = akActivated
		if sa.API == "Mana" || sa.API == "ManaReflected" {
			ab.Kind = akMana // ManaReflected: lower_reflectedmana.go
		}
		if ab.Kind == akMana && (strings.TrimSpace(sa.Params["ValidTgts"]) != "" || strings.TrimSpace(sa.Params["TargetType"]) != "") {
			return refuse("AB$ Mana with a target: gorge resolves it on the stack and adds the mana to the target (not a CR 605.1a mana ability), and an artifact source's mana is typed (pay.PlanPoolOK)")
		}
		c, ok := ps.get("Cost")
		if !ok {
			return refuse("activated ability without Cost$")
		}
		if err := handManaRefusal(ab.Kind, c); err != nil {
			return err
		}
		lc.unblockedOK = strings.TrimSpace(sa.Params["Keyword"]) == "Ninjutsu"
		lc.actCost = ab.Kind == akActivated
		cost, err := lc.costHand(c)
		lc.unblockedOK, lc.actCost = false, false
		if err != nil {
			return err
		}
		if xc, ok := ps.get("XColor"); ok {
			if err := lc.xColor(&cost.Mana, xc); err != nil {
				return err
			}
		}
		if cost.Mana.X || cost.Mana.XX || cost.Mana.XXX {
			lc.xPaidAb = ab
		}
		if err := xminParam(&cost, ps, ab.Kind == akActivated); err != nil {
			return err
		}
		if err := discardHandCost(&cost); err != nil {
			return err
		}
		if cost.DiscardHand && ab.Kind != akActivated && !discardHandManaShape(cost, ps) {
			return refuse("discard-hand cost on a mana ability beside a part other than sacrificing the source, or without InstantSpeed$ True")
		}
		if cost.TapPowN != 0 && (ab.Kind != akActivated || !tapPowerAlone(cost)) {
			return refuse("tap-any-number cost beside another part or on a mana ability")
		}
		if cost.TapPowN != 0 {
			// gorge's keyword expansions mint Crew as AB$ Animate and
			// Saddle as AB$ AlterAttribute, both with Keyword$ (graveexile).
			switch sa.Params["Keyword"] {
			case "Crew":
				cost.TapPowCrew = true
			case "Saddle":
				cost.TapPowSaddle = true
			}
		}
		ab.Cost = cost
		if cost.Return != nil {
			// gorge returnAsk (cost objects at payment): the only counted
			// part of its cost (Cost.N / Cost.Filter), not on a mana ability
			if ab.Kind == akMana {
				return refuse("mana ability with a return cost")
			}
			if cost.Sac != nil || cost.SacSelf || cost.DiscardN > 0 || cost.DiscardSelf || cost.Loyalty != 0 || cost.SubCtr > 0 ||
				cost.PayLife > 0 || cost.TapN > 0 || cost.TapHost || cost.SacHost || cost.Forage != nil || cost.ExileG != nil || cost.Energy != 0 || cost.ReturnSelf {
				return refuse("return cost beside another non-mana part")
			}
		}
		if cost.ReturnSelf {
			if ab.Kind == akMana {
				return refuse("mana ability with a return-self cost")
			}
			// gorge pays returns before the sacrifices: refuse beside any
			// other non-mana part but {T} (the kernel orders only these)
			if cost.Sac != nil || cost.SacSelf || cost.DiscardN > 0 || cost.DiscardSelf || cost.Loyalty != 0 || cost.SubCtr > 0 ||
				cost.PayLife > 0 || cost.TapN > 0 || cost.TapHost || cost.SacHost || cost.Forage != nil || cost.ExileG != nil || cost.Energy != 0 {
				return refuse("return-self cost beside another non-mana part")
			}
		}
		if cost.ExileSelf {
			if ab.Kind == akMana {
				return refuse("mana ability with an exile-self cost")
			}
			if cost.Sac != nil || cost.SacSelf || cost.DiscardN > 0 || cost.DiscardSelf || cost.Loyalty != 0 || cost.SubCtr > 0 ||
				cost.PayLife > 0 || cost.TapN > 0 || cost.TapHost || cost.SacHost || cost.Forage != nil || cost.ExileG != nil && !cost.ExileGBf || cost.Energy != 0 ||
				cost.ReturnSelf || cost.Return != nil {
				// (Craft's ExileCtrlOrGrave material beside its Exile<1/CARDNAME>:
				// the material picks are paid first, then the source, gorge
				// exAsk's part order)
				return refuse("exile-self cost beside another non-mana part")
			}
		}
		if cost.Waterbend != 0 || cost.WaterbendX {
			// Waterbend is an activated ability's own cost (CR 701.67a); the
			// kernel's offer credit and pick record cannot express a spell,
			// trigger, mana-ability or non-battlefield activation. A {T} in
			// the same cost excludes the source from the waterbend picks
			// (gorge castasks.go SourceTaps; kernel waterbendOptions).
			if ab.Kind != akActivated {
				return refuse("waterbend cost on a mana ability")
			}
			if ab.Zone != 0 && ab.Zone != zBattlefield {
				return refuse("waterbend cost outside a battlefield ability")
			}
			if cost.WaterbendX {
				// Waterbend<X> is the cost's one {X} and nothing else: a
				// printed mana symbol beside it would let the pre-fold pick
				// subtraction and the folded X's credit pay the same pick
				m := cost.Mana
				if !m.X || m.XX || m.Generic != 0 || m.Pips != [5]int{} || m.C != 0 || len(m.Hybrid) != 0 || len(m.Twobrid) != 0 || len(m.Phyrexian) != 0 {
					return refuse("waterbend cost Waterbend<X> beside printed mana")
				}
			} else if cost.Mana.X || cost.Mana.XX {
				return refuse("waterbend cost with an {X} in the mana cost")
			}
			if cost.Sac != nil || cost.SacSelf || cost.DiscardN > 0 || cost.DiscardSelf || cost.Loyalty != 0 || cost.SubCtr > 0 ||
				cost.PayLife > 0 || cost.TapN > 0 || cost.TapHost || cost.SacHost || cost.TapPowN != 0 || cost.Forage != nil || cost.ExileG != nil ||
				cost.Energy != 0 || cost.ReturnSelf || cost.ExileSelf || cost.Untap || cost.Return != nil || cost.DrawN > 0 {
				return refuse("waterbend cost beside another non-mana part")
			}
		}
		if ab.Kind == akMana && cost.Energy != 0 {
			// gorge pays it in the mana-activation walk; kernel mana
			// abilities pay only {T} / sacrifice-self
			return refuse("mana ability with an energy cost")
		}
		if ab.Kind == akMana && (cost.Forage != nil || cost.ExileG != nil) {
			// gorge pays these through the mana-activation walk
			// (AskManaForage); the kernel's mana abilities have no cost picks
			return refuse("mana ability with a forage / exile-from-graveyard cost")
		}
		if cost.EvidenceN > 0 && (ab.Kind != akActivated || ab.Zone != 0 && ab.Zone != zBattlefield) {
			// a mana ability would pay it in gorge's mana-activation walk
			// (Cryptex); a hand / graveyard ability is not modelled
			return refuse("evidence cost on a mana ability or outside a battlefield ability")
		}
		if ps.is("Planeswalker", "True") {
			ab.Loyalty = true
			ab.Sorcery = true
		}
		if v, ok := ps.get("SorcerySpeed"); ok && v == "True" {
			ab.Sorcery = true
		}
		if z, ok := ps.get("ActivationZone"); ok {
			ab.Zone = zoneNames[z]
			if ab.Zone == 0 {
				return refuse("activation zone %q", z)
			}
		}
		if err := putLibSelfGate(ab, cost); err != nil {
			return err
		}
		if err := exileGSelfGate(ab, cost); err != nil {
			return err
		}
		if cost.DiscardSelf && (ab.Zone != zHand || ab.Kind != akActivated || cost.DiscardN > 0) {
			return refuse("discard cost %q outside a hand ability", "Discard<1/CARDNAME>")
		}
		if cost.DiscardF != nil && (ab.Kind != akActivated || ab.Zone != zNone && ab.Zone != zBattlefield) {
			// the candidates are the activating controller's hand cards, and
			// only a battlefield activation excludes its source this way
			// (gorge pay.DiscardCandidates' casting flag; CR 118.3)
			return refuse("filtered discard cost outside a battlefield activated ability")
		}
		if ab.Zone == zHand && strings.TrimSpace(sa.Params["Keyword"]) == "Ninjutsu" {
			if err := lc.ninjutsuAbility(ab, ps, cost); err != nil {
				return err
			}
		}
		if ab.Zone == zHand && !cost.DiscardSelf && !ab.Ninjutsu {
			if !lc.putSelfAbility(sa.API, ab, ps, cost) { // lower_ninjutsu.go
				return refuse("hand ability without discarding itself")
			}
		}
		if kw := strings.TrimSpace(sa.Params["Keyword"]); ab.Zone == zHand && (kw == "Cycling" || kw == "TypeCycling") {
			ab.Cycling = true // gorge cyclingKeyword
		}
		if cost.ExileSelf && ab.Zone != 0 && ab.Zone != zBattlefield {
			return refuse("exile-self cost outside a battlefield ability")
		}
		if (cost.SacExile || cost.HostExile) && ab.Zone != 0 && ab.Zone != zBattlefield {
			return refuse("exile cost from the battlefield outside a battlefield ability")
		}
		if cost.Untap && (ab.Kind != akActivated || ab.Zone != 0 && ab.Zone != zBattlefield || cost.Tap) {
			// a mana ability's {Q} would ride autopay, which never untaps
			return refuse("untap-self cost outside an activated battlefield ability")
		}
		if v, ok := ps.get("GameActivationLimit"); ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return refuse("GameActivationLimit %q", v)
			}
			ab.Limit = n
		}
		if v, ok := ps.get("Exhaust"); ok {
			// CR 702.177a: activate only once (gorge activationLimitBlocked:
			// the GameActivationLimit$ counter, one per object stint)
			if v != "True" || ab.Limit != 0 {
				return refuse("Exhaust$ %q", v)
			}
			ab.Limit = 1
			ab.Exhaust = true
		}
		if v, ok := ps.get("CantCopy"); ok {
			// gorge SpellCopyAllowed reads "True" or "true"; the kernel's
			// ExtCopyAbility reads the table byte of the copied entry
			if v != "True" && v != "true" {
				return refuse("CantCopy$ %q", v)
			}
			ab.CantCopy = true
		}
		if v, ok := ps.get("PowerUp"); ok {
			// CR 702.193a: activate only once per object stint; CR 702.193b
			// reduces the cost by the permanent's mana cost on its entry turn.
			// gorge reads the reduction from the face's mana cost, so a cost
			// with a non-mana part is outside the modelled shape.
			if v != "True" || ab.Limit != 0 || ab.Kind != akActivated ||
				(ab.Zone != 0 && ab.Zone != zBattlefield) || !cost.pureMana() {
				return refuse("PowerUp$ %q", v)
			}
			// A single {X} lowers: the reduction never
			// touches X, which is announced first (CR 601.2b) and then
			// priced against the reduced cost (gorge xAsk). {X}{X} and
			// {X} beside a hybrid symbol have no lockstep carrier.
			if cost.Mana.XX {
				return refuse("power-up with {X}{X} cost")
			}
			if cost.Mana.X && len(cost.Mana.Hybrid) != 0 {
				return refuse("power-up with {X} and a hybrid symbol")
			}
			ab.Limit = 1
			ab.PowerUp = true
		}
		if v, ok := ps.get("ReduceCost"); ok {
			// An activated ability's own ReduceCost$ (Channel: "costs {1}
			// less to activate for each legendary creature you control";
			// gorge legal_activation.go ownReduceCost): the generic part is
			// reduced by the value, floored at zero, at the offer gate, the
			// planner witness and the charge alike. Only a printed hand or
			// battlefield ability whose value reads no target (the offer and
			// the charge then read one amount) is modelled
			// (lower_ownreduce.go).
			if err := lc.ownReduce(ab, cost, v); err != nil {
				return err
			}
		}
		if v, ok := ps.get("ActivationLimit"); ok {
			if v != "1" {
				return refuse("ActivationLimit %q", v)
			}
			ab.TurnOnce = true
		}
		if cost.AddCtr > 0 {
			// a counter put on the source as a cost (CR 118.3): the
			// -0/-1 mana-ability shape stays as it was (Wall of Roots; the
			// kernel's script gap, mana_countercost.go), and any other
			// kind may pay a non-mana ability alone among the counted
			// parts (Jessica Jones, Private Eye's AddCounter<1/STUN>)
			if cost.AddCtrKnd == "M0M1" {
				rest := cost
				rest.AddCtr, rest.AddCtrKnd = 0, ""
				if ab.Kind != akMana || !ab.TurnOnce || !rest.pureMana() || rest.Mana.Generic != 0 || rest.Mana.X || len(rest.Mana.Hybrid) != 0 || rest.Mana.Pips != [5]int{} {
					return refuse("counter-placing cost outside a once-each-turn mana ability")
				}
			} else if ab.Kind == akMana {
				return refuse("counter-placing cost outside a non-mana ability")
			} else if cost.Sac != nil || cost.DiscardN > 0 || cost.Loyalty != 0 || cost.SubCtr > 0 || cost.SubAmongN > 0 || cost.PayLife > 0 || cost.TapN > 0 || cost.TapPowN > 0 || cost.Forage != nil || cost.ExileG != nil || cost.Energy > 0 || cost.Return != nil || cost.DrawN > 0 || cost.Waterbend > 0 || cost.WaterbendX || cost.BlightN > 0 || cost.EvidenceN > 0 {
				return refuse("counter-placing cost beside another counted part")
			}
		}
		if v, ok := ps.get("Activation"); ok {
			switch v {
			case "Threshold":
				ab.ActCond = lCond{Kind: cThreshold}
			case "Delirium":
				ab.ActCond = deliriumCond() // gorge activationConditionOK
			case "Metalcraft":
				// "activate only if you control three or more artifacts"
				// (CR 207.2c; gorge activationConditionOK's metalcraft arm)
				if sa.API != "Mana" {
					return refuse("Activation %q outside a mana ability", v)
				}
				c, err := lc.bareCondition(ps, "Metalcraft")
				if err != nil {
					return err
				}
				ab.ActCond = c
			case "Solved":
				// "activate only if this Case is solved" (CR 719.3b; gorge
				// activationConditionOKSolved: the source's Solved flag)
				f, err := lc.filter("Card.Self+IsSolved")
				if err != nil {
					return err
				}
				ab.ActCond = lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
			default:
				return refuse("Activation %q", v)
			}
		}
		if cs, ok := ps.get("CheckSVar"); ok {
			if ab.ActCond.Kind != cNone {
				return refuse("two activation conditions")
			}
			if sa.API == "Mana" {
				return refuse("CheckSVar$ on a mana ability")
			}
			v, err := lc.value(cs)
			if err != nil {
				return err
			}
			c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
			if cmp, ok := ps.get("SVarCompare"); ok {
				op, b, err := lc.cmpSpec(cmp)
				if err != nil {
					return err
				}
				c.Op, c.B = op, b
			}
			ab.ActCond = c
		}
		if ip, ok := ps.get("IsPresent"); ok && sa.API != "ClassLevelUp" {
			// "activate only if <a permanent matching> is on the
			// battlefield" (gorge activation restriction IsPresent$, CR
			// 602.5b; Kitsa's Card.powerGE3+Self)
			if ab.ActCond.Kind != cNone {
				return refuse("two activation conditions")
			}
			if sa.API == "Mana" {
				// a gated mana ability: the kernel reads it as a
				// planner/window variant of the source (mana_gate.go)
				for _, k := range [...]string{"PresentZone", "PresentPlayer", "PresentDefined"} {
					if _, ok := ps.get(k); ok {
						return refuse("gated mana ability %s", k)
					}
				}
			}
			if _, ok := ps.get("PresentPlayer"); ok {
				return refuse("activation IsPresent PresentPlayer")
			}
			if _, ok := ps.get("PresentDefined"); ok {
				return refuse("activation IsPresent PresentDefined")
			}
			f, err := lc.filter(ip)
			if err != nil {
				return err
			}
			c := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
			if pc, ok := ps.get("PresentCompare"); ok {
				op, v, err := lc.cmpSpec(pc)
				if err != nil {
					return err
				}
				c.Op, c.A = op, v
			}
			if z, ok := ps.get("PresentZone"); ok {
				// gorge legal_activation.go: the named zones' counts summed
				if c, err = presentZoneCond(c, z, ip, lc); err != nil {
					return err
				}
			}
			ab.ActCond = c
		}
		if v, ok := ps.get("PlayerTurn"); ok {
			// "activate only during your turn" (gorge activationPhasesOK:
			// PlayerTurn$ True withholds the offer unless the activator is
			// the active player)
			// beside a CheckSVar$ / IsPresent$ gate (Essence Anchor) both are
			// independent offer-time reads gorge requires (activationPhasesOK
			// and sVarGateOK), so their conjunction is exact
			if v != "True" || sa.API == "Mana" {
				return refuse("activation PlayerTurn$ %q beside another condition", v)
			}
			ab.ActCond = andCond(ab.ActCond, lCond{Kind: cYourTurn})
		}
	default:
		return refuse("ability kind %s", sa.Kind)
	}
	lc.out.Abilities = append(lc.out.Abilities, ab)
	if ab.Ninjutsu || ab.PutSelf {
		return lc.ninjutsuBody(ps, ab.PutSelf) // lower_ninjutsu.go
	}
	if sa.API == "Mana" && ab.Kind != akSpell {
		return lc.lowerMana(sa, ps, ab)
	}
	if sa.API == "ManaReflected" && ab.Kind == akMana {
		return lc.lowerManaReflected(sa, ps, ab)
	}
	if ab.Kind == akSpell && lc.out.Gift {
		// CR 702.174b / gorge rules stack.go: a promised gift on an instant
		// or sorcery is given before the spell's other effects, the
		// GiftAbility body spliced at the head of the spell's chain
		if err := lc.giftHead(); err != nil {
			return err
		}
	}
	return lc.chain(sa, ps)
}

// giftHead emits the promised gift ahead of a spell's own effects: the
// GiveGift marker (gorge emits events.GiveGift just before the body) and
// the GiftAbility body, both skipped when no gift was promised.
func (lc *lowerCtx) giftHead() error {
	gift := cards.ResolveSVar(lc.face.SVars, "GiftAbility")
	return lc.guardBlock(lCond{Kind: cPromisedGift}, func() error {
		lc.emit(lInstr{Op: "GiveGift"})
		n := len(lc.ab.Targets)
		if err := lc.chain(gift, nil); err != nil {
			return fmt.Errorf("GiftAbility: %w", err)
		}
		if len(lc.ab.Targets) != n {
			return refuse("GiftAbility with a target")
		}
		return nil
	})
}

// lowerGift lowers a permanent's gift (CR 702.174c, gorge rules
// altcast.go altCostEnter): "when this permanent enters, if the gift was
// promised, the opponent gets the gift" -- a triggered ability queued after
// the card's own enter triggers, its gift given as it is put on the stack.
// An instant's or sorcery's gift is spliced into its spell (giftHead).
func (lc *lowerCtx) lowerGift() {
	if !lc.out.Gift || lc.out.Types&tPermanent == 0 {
		return
	}
	gift := cards.ResolveSVar(lc.face.SVars, "GiftAbility")
	ab := &lAbility{Kind: akTriggered, Index: len(lc.out.Abilities), Name: "Gift"}
	ab.Trigger = &lTrigger{Event: "ETB", Origin: zAny, Dest: zBattlefield,
		F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}, Cond: lCond{Kind: cPromisedGift}}
	lc.out.Abilities = append(lc.out.Abilities, ab)
	lc.guard("K:Gift GiftAbility", func() error {
		lc.ab, lc.chainT, lc.mode = ab, nil, -1
		if err := lc.chain(gift, nil); err != nil {
			return err
		}
		if len(ab.Targets) != 0 {
			return refuse("GiftAbility with a target")
		}
		return nil
	})
}

// lowerChapters lowers K:Chapter:<count>:<svar1>,... (CR 714, gorge
// cards.SagaChapters): one printed battlefield trigger per chapter, on the
// lore counter, gated at its chapter number. The count is the Saga's final
// chapter (CardDef.SagaN); the CR 714.4 SBA reads it. A chapter name that
// does not resolve refuses the card.
func (lc *lowerCtx) lowerChapters(param string) {
	countStr, list, _ := strings.Cut(param, ":")
	n, err := strconv.Atoi(strings.TrimSpace(countStr))
	if err != nil || n <= 0 || n > 255 {
		lc.refuseNode("K:Chapter:"+param, "chapter count %q", countStr)
		return
	}
	var names []string
	for nm := range strings.SplitSeq(list, ",") {
		if nm = strings.TrimSpace(nm); nm != "" {
			names = append(names, nm)
		}
	}
	if len(names) == 0 {
		lc.refuseNode("K:Chapter:"+param, "chapter list is empty")
		return
	}
	lc.out.SagaN = n
	for i, name := range names {
		if i >= n {
			break // gorge resolves the first count names only
		}
		sa := cards.ResolveSVar(lc.face.SVars, name)
		if sa == nil {
			lc.refuseNode("K:Chapter:"+param, "chapter %d SVar %q", i+1, name)
			return
		}
		ab := &lAbility{Kind: akTriggered, Index: len(lc.out.Abilities), Name: "Chapter"}
		ab.Trigger = &lTrigger{Event: "CounterAddedOnce", Step: "LORE",
			F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}}
		ab.Gate = &lGate{Ctr: "LORE", Level: i + 1}
		lc.out.Abilities = append(lc.out.Abilities, ab)
		lc.guard(fmt.Sprintf("K:Chapter:%d:%s", i+1, name), func() error {
			lc.ab, lc.chainT, lc.mode = ab, nil, -1
			if err := lc.chain(sa, nil); err != nil {
				return err
			}
			patchIfPaidSkips(ab) // lower_chapterifpaid.go
			return nil
		})
	}
}

// chain lowers an SA and its Sub chain into the current ability's program.
// ps carries the head node's params (already partly read by the caller).
func (lc *lowerCtx) chain(sa *cards.SA, ps *params) error {
	lc.chains++
	lc.depth++
	prevHead, prevRoot := lc.head, lc.chainRoot
	if lc.depth == 1 {
		lc.chainRoot = sa
	}
	defer func() { lc.depth--; lc.head, lc.chainRoot = prevHead, prevRoot }()
	usBase := len(lc.unlessSubs)
	var blockers []error // every-blocker census only: this chain's refusals, in order
	for node := sa; node != nil; node = node.Sub {
		lc.head = node == sa
		lc.curNode = node
		p := ps
		if node != sa || p == nil {
			p = newParams(node.Params)
		}
		p.consume("SubAbility")
		atkGuard, aerr := lc.lowerAttackLeg(node, p) // lower_attackleg.go
		if aerr != nil {
			return aerr
		}
		at, nt := len(lc.ab.Prog), len(lc.ab.Targets)
		lc.atRes = node != sa && lc.mode < 0 && (lc.ab.Kind == akSpell || lc.ab.Kind == akActivated) &&
			sa.API != "Charm" && sa.API != "CopySpellAbility" && subTargetAtResolution(node)
		err := lc.node(node, p)
		lc.atRes = false
		if err == nil {
			err = lc.checkTriggerAttackers(at) // lower_triggerattackers.go
		}
		if err != nil {
			err = fmt.Errorf("%s$ %s: %w", node.Kind, node.API, err)
			if lc.p != nil && lc.p.allBlockers {
				// every-blocker census: keep reading the Sub chain, noting
				// what the failed node left unread
				lc.noteUnreadable(p, node)
				blockers = append(blockers, err)
				continue
			}
			return err
		}
		for i := nt; i < len(lc.ab.Targets); i++ {
			if t := &lc.ab.Targets[i]; t.SubDecl && t.Divided > 0 {
				// a copy re-asks this link's declaration (CR 707.10c, gorge
				// copyChainDeclarations); only the shapes the kernel's copy
				// census and bounds read exactly lower
				return refuse("%s$ %s: a chain link's copy target declaration with a divided slot", node.Kind, node.API)
			}
			if !lc.ab.Targets[i].AtRes {
				continue
			}
			// the mid-resolution ask goes in front of the node's
			// instructions (inside its condition guard)
			pos := at
			if at < len(lc.ab.Prog) && lc.ab.Prog[at].Op == "SkipIf" && lc.ab.Prog[at].Skip == len(lc.ab.Prog)-at-1 {
				lc.ab.Prog[at].Skip++
				pos = at + 1
			}
			lc.ab.Prog = append(lc.ab.Prog, lInstr{})
			copy(lc.ab.Prog[pos+1:], lc.ab.Prog[pos:])
			lc.ab.Prog[pos] = lInstr{Op: "ChooseTargets", C: i}
		}
		if atkGuard >= 0 {
			lc.ab.Prog[atkGuard].Skip = len(lc.ab.Prog) - atkGuard - 1
		}
		if left := p.unread(); len(left) > 0 {
			err := refuse("%s$ %s: unread params %v", node.Kind, node.API, left)
			if lc.p != nil && lc.p.allBlockers {
				blockers = append(blockers, err)
				continue
			}
			return err
		}
		if err := lc.unionUses(at); err != nil {
			return fmt.Errorf("%s$ %s: %w", node.Kind, node.API, err)
		}
	}
	if len(blockers) > 0 {
		// the first is the node's refusal (as without the mode); the rest
		// are recorded beside it
		for _, e := range blockers[1:] {
			lc.refuseNode(lc.blockerNode(), "%s", e.Error())
		}
		return blockers[0]
	}
	if err := lc.applyUnlessSubs(usBase); err != nil {
		return err
	}
	if lc.depth == 1 {
		return lc.unionSlotsWhole()
	}
	return nil
}

// emit appends an instruction to the current ability.
func (lc *lowerCtx) emit(in lInstr) { lc.ab.Prog = append(lc.ab.Prog, in) }

// addTarget declares a target slot on the current ability.
func (lc *lowerCtx) addTarget(t lTarget) int {
	t.Mode = lc.mode
	t.AtRes = lc.atRes
	t.ChgSub = lc.curNode != nil && !lc.head && lc.curNode.API == "ChangeZone"
	// gorge's copy target declaration (rules copyTargetDeclarations): the
	// spell's root node, or a chosen charm mode's head
	t.Decl = lc.ab.Kind == akSpell && lc.head && !lc.atRes && (lc.depth == 1 || lc.depth == 2 && lc.mode >= 0)
	// and each announced SubAbility$ link of a plain spell (gorge
	// copyChainDeclarations); the kernel's copyLinkSlot reads a charm mode's and
	// an ability's from their slots
	t.SubDecl = lc.ab.Kind == akSpell && lc.depth == 1 && lc.mode < 0 && !lc.head && !lc.atRes &&
		lc.chainRoot != nil && lc.chainRoot.API != "Charm" && lc.chainRoot.API != "CopySpellAbility"
	if t.SubDecl && lc.curNode != nil {
		// a link excused by an unpaid optional cost announces no target for the cast (castSubTargetsOwed)
		t.SubCond = strings.ToLower(strings.TrimSpace(lc.curNode.ParamStr(cards.PKCondition)))
	}
	if t.Min.Kind == vNone {
		t.Min = cst(1)
	}
	if t.Max.Kind == vNone {
		t.Max = cst(1)
	}
	lc.livingBound(&t)
	lc.ab.Targets = append(lc.ab.Targets, t)
	i := len(lc.ab.Targets) - 1
	lc.chainT = append(lc.chainT, i)
	lc.noteOwnTarget(i)
	return i
}

func (lc *lowerCtx) lastTarget() int {
	if len(lc.chainT) == 0 {
		return -1
	}
	return lc.chainT[len(lc.chainT)-1]
}

// newLatent starts a latent ability (a program an op reaches by index) and
// returns it; the caller restores lc.ab.
func (lc *lowerCtx) newLatent(name string) *lAbility {
	ab := &lAbility{Kind: akLatent, Latent: true, Index: len(lc.out.Abilities), Name: name}
	lc.out.Abilities = append(lc.out.Abilities, ab)
	return ab
}

// grantedAbility lowers an SVar-held activated ability another permanent
// is granted (an equipment's or aura's AddAbility$) and returns its index.
// A cross-object grant's SVar names resolve off the grantor's face, the face
// lowered here, as gorge's grantedAbilitySVars reads them.
func (lc *lowerCtx) grantedAbility(name string) (int, error) {
	lc.granted++
	defer func() { lc.granted-- }()
	sa := cards.ResolveSVar(lc.face.SVars, name)
	if sa == nil || sa.Kind != "AB" {
		return 0, refuse("granted ability %q", name)
	}
	idx := len(lc.out.Abilities)
	err := lc.withAbility(lc.ab, func() error { return lc.lowerAbility(sa) })
	if err != nil {
		return 0, err
	}
	if why := grantedManaShape(lc.out.Abilities[idx]); why != "" {
		return 0, refuse("%s", why) // lower_grantedmana.go
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = "Granted"
	return idx, nil
}

// withAbility lowers fn with ab current and a fresh target chain.
func (lc *lowerCtx) withAbility(ab *lAbility, fn func() error) error {
	saved, savedT, savedMode, savedRem, savedX := lc.ab, lc.chainT, lc.mode, lc.remTgt, lc.xPaidAb
	lc.ab, lc.chainT, lc.mode, lc.remTgt = ab, nil, -1, 0
	err := fn()
	lc.ab, lc.chainT, lc.mode, lc.remTgt, lc.xPaidAb = saved, savedT, savedMode, savedRem, savedX
	return err
}

// impendingTick synthesizes Impending's end-step removal as the card's own
// trigger (gorge kw_impending.go impendingTickGrant's granted trigger: "At the
// beginning of your end step, remove a time counter from it", gated on a
// time counter remaining; a permanent not cast impending never has one, so
// the gate stands in for the grant's cast provenance).
func (lc *lowerCtx) impendingTick() error {
	// the exact line gorge's impendingTickGrant parses (rules/impending.go)
	t, ok := cards.ParseTriggerLine("Mode$ Phase | Phase$ End of Turn | ValidPlayer$ You | TriggerZones$ Battlefield | IsPresent$ Card.Self+counters_GE1_TIME | PresentCompare$ GE1 | PresentDefined$ Self | Execute$ __kwImpendingTick | TriggerDescription$ At the beginning of your end step, remove a time counter from it.")
	if !ok {
		return fmt.Errorf("trigger line")
	}
	if t.Effect = cards.ResolveSVar(lc.face.SVars, "__kwImpendingTick"); t.Effect == nil {
		return fmt.Errorf("tick body")
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return err
	}
	if len(lc.out.Abilities) != idx+1 {
		return fmt.Errorf("tick lowering")
	}
	return nil
}

// blockerNode names the top-level node an extra every-blocker refusal
// belongs to.
func (lc *lowerCtx) blockerNode() string {
	if lc.guardNode == "" {
		return "card"
	}
	return lc.guardNode
}

// noteUnreadable records, in the every-blocker census, that a refused node
// stopped before reading some of its params, so further blockers may sit
// among them.
func (lc *lowerCtx) noteUnreadable(p *params, sa *cards.SA) {
	var left []string
	for _, k := range p.unread() {
		if err := lc.probeParam(k, p.m[k]); err != nil {
			lc.refuseNode(lc.blockerNode(), "%s$ %s: %s", sa.Kind, sa.API, err.Error())
			p.used[k] = true
			continue
		}
		left = append(left, k)
	}
	if len(left) > 0 {
		lc.out.Unreadable = append(lc.out.Unreadable, refusal{Card: lc.name, Node: lc.blockerNode(), Reason: fmt.Sprintf("unread after refusal %v", left)})
	}
}

// probeAmountKeys are the amount params a refused node's stopped reading
// can hide; probeParam reads one on its own, as the node would have.
var probeAmountKeys = map[string]bool{"NumDmg": true, "NumCards": true, "NumAtt": true, "NumDef": true, "LifeAmount": true, "CounterNum": true,
	"TokenAmount": true, "Amount": true, "ChangeNum": true, "DigNum": true, "NumTurns": true}

// probeFilterKeys are the filter params probeParam reads on their own, with
// every Valid* key.
var probeFilterKeys = map[string]bool{"ChangeType": true, "ChangeValid": true, "SacValid": true, "Choices": true, "IsPresent": true, "ConditionPresent": true, "Affected": true}

func (lc *lowerCtx) probeParam(k, v string) error {
	v = strings.TrimSpace(v)
	switch {
	case v == "":
		return nil
	case probeAmountKeys[k]:
		switch v {
		case "All", "Any", "Double", "Triple", "Half":
			return nil
		}
		_, err := lc.value(v)
		return err
	case probeFilterKeys[k] || strings.HasPrefix(k, "Valid"):
		_, err := lc.filter(v)
		return err
	}
	return nil
}

// probeBody lowers a refused ability's or trigger's body on its own when the
// refusal came before the body was read, so its blockers are listed too.
func (lc *lowerCtx) probeBody(sa *cards.SA, entered int) {
	if lc.p == nil || !lc.p.allBlockers || sa == nil || lc.chains != entered {
		return
	}
	saved := lc.guardNode
	lc.guardNode = saved + " body"
	err := lc.withAbility(lc.newLatent("Probe"), func() error { return lc.chain(sa, nil) })
	if err != nil {
		lc.refuseNode(lc.guardNode, "%s", err.Error())
	}
	lc.guardNode = saved
}

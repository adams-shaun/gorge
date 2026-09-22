// Layer application is CR 613: characteristics settle in a fixed layer order
// (copy, control, text, type, color, abilities, power/toughness), and within
// layer 7 in a further sublayer order (characteristic-defining, setting,
// modifying, counters, switching). M1 only produces effects in layers 6 and
// 7, but the full ladder is defined now so a later layer 2 control-change or
// layer 4 type-change effect is an addition to this file, not a rewrite of
// it — that retrofit is the project's own top-named risk.
package rules

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// emptySVars is the shared empty SVar table a merged under-card face with no
// SVars of its own is stamped with, so ContinuousEffect.SVars stays non-nil
// for every merged face (nil means "read the source object's active face"
// downstream). It is never written to.
var emptySVars = map[string]string{}

// staticEffects reads every battlefield permanent's own S:Mode$ Continuous
// statics into ContinuousEffects, so a card whose entire rules text is a
// static (an Equipment's EquippedBy pump, an Aura's EnchantedBy pump, a
// vanilla lord's global pump) actually applies instead of silently doing
// nothing. Task 14 is what first wires static text into the layer system:
// until it, every S: line this build cared about was a play restriction
// (CantBeCast etc.), and the Mode$ Continuous statics every creature-lord
// and echo of Equipment/Enchant text carries were parsed but never turned
// into an effect -- a card with nothing but static text read as "does
// nothing".
//
// Reading these live off the battlefield permanent (the same scan activeStatics
// performs for restrictions) rather than registering them at ETB means a
// permanent placed on the battlefield by any path -- cast, a raw MoveZone in
// a test -- is covered, the effect expiry problem is solved for free (active()
// only walks battlefield permanents, so a departed source contributes nothing
// this call), and there is no registration event to keep in step with replay.
// Each static breaks into one effect per layer it touches -- AddPower/
// AddToughness is a layer-7 modify, AddKeyword a layer-6 grant, AddTypes a
// layer-4 type change -- so the layer ordering Derived applies (CR 613) still
// holds when one static carries both a pump and a keyword (exactly the Sword
// of Fire and Ice / Umezawa shape Task 14's tests build). The scan order is
// deterministic: AliveFrom(0) walks seats in fixed APNAP order, each
// battlefield zone is a slice, and each face's Statics is its parsed script
// order -- nothing here ranges a map, so the resulting option/view/settle
// order stays reproducible run to run (determinism requirement 3 of the
// dispatch).
//
// dst is the caller-owned static memo's reusable outer storage, distinct from
// activeBuf. This scan calls no callbacks and cannot re-enter; each nested
// keyword/type slice is freshly parsed and remains read-only after active()
// copies the effect values. Only the outer slots are overwritten here.
func (e *Engine) staticEffects(dst []ContinuousEffect) []ContinuousEffect {
	out := dst[:0]
	for pi, p := range e.G.AliveFrom(0) {
		// staticSourceZones (below) walks the battlefield FIRST so every
		// battlefield static keeps today's relative emission order, then the
		// non-battlefield zones an EffectZone$ can name. The shared stack is
		// walked exactly once, under the first alive seat -- the same
		// collectCostStatics discipline, which without it would collect each
		// stack card's statics once per seat.
		for _, z := range staticSourceZones {
			if z == state.ZStack && pi > 0 {
				continue
			}
			for _, id := range e.G.Zone(z, p) {
				o := e.G.Obj(id)
				if o == nil {
					continue
				}
				f := o.Face()
				if f == nil {
					continue
				}
				onBattlefield := z == state.ZBattlefield
				if !onBattlefield && len(f.Statics) == 0 {
					// Off the battlefield only the object's own face is walked
					// (an unlocked Room face and a mutated pile's under-cards
					// are battlefield-only, below), so a face printing no
					// statics emits nothing. Most of every library is this.
					continue
				}
				if onBattlefield && e.faceDownPrintedHides(o) {
					// CR 708.8: a face-down permanent's printed statics do not
					// exist while it is face down (the one gate shared with
					// activeStatics, the trigger scan and the offer loops).
					continue
				}
				// Enchantment Rooms (rules/rooms.go): once the room's second door
				// is unlocked, the ALTERNATE face's statics are live too -- a room
				// permanent's rules text is both halves' combined after the
				// unlock (CR 309.6), each face's Statics its own scan. A room's
				// unlocked face exists only on the battlefield.
				faces := []*cards.Face{f}
				if onBattlefield && o.Unlocked && isRoom(o) && len(o.Card.Faces) == 2 && int(o.FaceIdx) < len(o.Card.Faces) {
					faces = append(faces, o.Card.Faces[1-int(o.FaceIdx)])
				}
				// CR 702.140d: a mutated pile's under-card statics are live too,
				// exactly like the Rooms alternate face above. They are appended
				// in pile order AFTER the top face (and after the unlocked room
				// face, which is itself the top card's other half), so the
				// emission order -- and therefore every timestamp tie-break -- is
				// deterministic. A merged card exists only on the battlefield.
				mergedFrom := len(faces)
				if onBattlefield {
					for i := range o.MergedCards {
						if mf := o.MergedFaceAt(i); mf != nil {
							faces = append(faces, mf)
						}
					}
				}
				for fi, fc := range faces {
					// ContinuousEffect.SVars carries a table ONLY for a merged
					// under-card face -- a face that is a DIFFERENT CARD from the
					// one o.Face() resolves, so every downstream SVar read
					// (staticAmount, grantedAbilities' AddAbility$ body, the
					// staticView specCtx) would otherwise silently read the pile
					// TOP's table. The pile's own top face and an unlocked Room's
					// alternate face are faces of the SAME card and stay nil, so
					// those readers keep their o.Face() fallback: an alternate
					// face's grant resolves against the ACTIVE face's table
					// exactly as it did before merged faces joined this walk
					// (TestActionStaticMembershipPreservesOrderAndActiveFace locks
					// that -- the back face's `AddAbility$ Back` must not mint a
					// live {B} mana ability while the front face is up). A merged
					// face with no SVar table of its own gets an empty one rather
					// than nil, so "the under-card has no such SVar" resolves to
					// no grant instead of falling back to the top card's body.
					var faceSVars map[string]string
					if fi >= mergedFrom {
						faceSVars = fc.SVars
						if faceSVars == nil {
							faceSVars = emptySVars
						}
					}
					// grantQueue is the AddStaticAbility$ work queue, REUSED across
					// scans on the Engine's own buffer (staticQueueBuf): the face's
					// own statics at depth 0, then every granted static appended with
					// its depth, so a granted static's emission is the SAME body a
					// printed one runs -- one grant grammar (task
					// inbox-paramcensus-static-grant-misc). The depth-0 check below
					// bounds the recursion; the warm-rescan allocation budget
					// (static_effects_buffer_test) is why the buffer is reused,
					// never re-made per face.
					grantQueue := e.staticQueueBuf[:0]
					for _, st := range fc.Statics {
						grantQueue = append(grantQueue, staticWork{st: st})
					}
					for qi := 0; qi < len(grantQueue); qi++ {
						w := grantQueue[qi]
						st := w.st
						if st.Mode != "Continuous" {
							continue
						}
						// EffectZone$ -- the zone the static's SOURCE must sit in for
						// it to be live (Forge's StaticAbilityContinuous EffectZone$,
						// default the battlefield). This is the ONE read the other
						// static families (CantBeCast, RaiseCost/ReduceCost/SetCost,
						// the may-play walks) already make through effectZoneOK, and
						// the Continuous path now shares it. A battlefield-scoped or
						// default static keeps today's admission exactly; a
						// graveyard/command/exile-scoped static stops wrongly
						// applying while its source is on the battlefield (Anger's
						// haste grant belongs to the graveyard alone) and is instead
						// collected from the zone it names by the zone walk above.
						// An unrecognised value denies -- the fail-closed direction
						// effectZoneOK documents.
						// ExcludeZone$ -- the zone(s) the static's SOURCE must NOT sit in
						// for it to be live (Forge's mirror of EffectZone$; Grist, the
						// Hunger Tide's "As long as Grist isn't on the battlefield, it's a
						// 1/1 Insect creature in all other zones"). An exclusion with no
						// explicit EffectZone$ REPLACES the battlefield default: the static
						// is live in every other zone -- exactly the CR 604.3 every-zone
						// CDA reading minus the excluded zone(s). staticZoneAdmits (below)
						// is the ONE read both this gate and cdaPTStatic make, so the
						// emitted characteristic grant and the layer-7a P/T claim can
						// never disagree about where the static is live.
						if !e.stackSelfStaticOK(st, o) && !staticZoneAdmits(st.Params["ExcludeZone"], st.Params["EffectZone"], o.Zone) {
							continue
						}
						affects := st.Params["Affected"]
						// Forge omits Affected$ on a self-only characteristic-defining
						// static (Tarmogoyf, Krovikan Mist). Its default is the host
						// card, not "no affected object".
						if affects == "" {
							affects = "Card.Self"
						}
						// The "as long as" recheck gates (Forge's intervening-if on a
						// continuous static): IsPresent$/IsPresent2$ (an existence count
						// over every battlefield, PresentCompare$ pricing the count with
						// GE1 the default) and CheckSVar$/SVarCompare$ (the named SVar
						// compared under the threshold). staticEffects re-runs once per
						// emitted event (the staticContinuous memo's epoch key), so the
						// gate is a genuine continuous recheck: the board moves, the
						// grant follows -- Angelic Overseer's Human, Static Orb's
						// untapped state, Auriok Steelshaper's equipped state, Kiyomaro's
						// hand size. A gate this build cannot evaluate fails CLOSED
						// (the shipped statics convention rules/statics.go's
						// checkSVarHolds documents): the grant is withheld whole, never
						// silently always-applied.
						if !e.continuousGateHolds(staticView{Source: id, Controller: o.Controller, Params: st.Params, SVars: faceSVars}) {
							continue
						}
						base := ContinuousEffect{
							Source:     id,
							Timestamp:  o.Timestamp,
							Controller: o.Controller,
							Affects:    affects,
							SVars:      faceSVars,
						}
						if hasStat(st, "AddPower") || hasStat(st, "AddToughness") {
							pt := base
							pt.Layer, pt.Sub = LPT, SubModify
							pt.AddPowerExpr = st.Params["AddPower"]
							pt.AddToughnessExpr = st.Params["AddToughness"]
							out = append(out, pt)
						}
						if hasStat(st, "AddKeyword") {
							kw := base
							kw.Layer = LAbilities
							kw.AddKeywords = statKeywords(st)
							kw.AffectedZone = strings.TrimSpace(st.Params["AffectedZone"])
							out = append(out, kw)
						}
						// A printed Continuous AddAbility$ static (Ichormoon Gauntlet's
						// "Planeswalkers you control have [0]: Proliferate", a lord
						// granting an activated ability, an Equipment granting
						// "{T}: deal 1 damage") is a layer-6 ability GRANT (CR
						// 613.1f): one ContinuousEffect whose AddAbilities names the
						// SVar bodies on THIS source's face, consumed by legal.go's
						// grantedAbilities (the offer) and mana_activation.go's
						// granted-mana loop (the tap gate and payment window). The
						// grantor is base.Source and the recipient is whatever
						// Affects matches, so the two may differ -- the whole point of
						// a cross-object grant. statList splits the ` & ` and `,`
						// multi-value forms (6 corpus carriers). An AddAbility$ name
						// whose body is missing or is not an AB degrades to no grant
						// in grantedAbilities (the same totality every SVar
						// resolution takes), so no validation is needed here.
						if hasStat(st, "AddAbility") {
							ga := base
							ga.Layer = LAbilities
							ga.AddAbilities = statList(st, "AddAbility")
							ga.AffectedZone = strings.TrimSpace(st.Params["AffectedZone"])
							if len(ga.AddAbilities) > 0 {
								out = append(out, ga)
							}
						}
						// A has-all-abilities-of static (CR 613.1f): Forge's
						// GainsAbilitiesOf$ / GainsTriggerAbsOf$ name a CARD filter, and
						// every object the static's Affected$ matches gains all
						// activated and/or triggered abilities of each named card while
						// it sits in the GainsAbilitiesOfZones$ zones (Idris, Soul of
						// the TARDIS: "NICKNAME has all activated and triggered
						// abilities of the exiled card", zones Exile). Unlike
						// AddAbility$ the bodies are compiled SAs on the FOREIGN card,
						// not SVar names on this source, so the effect carries the
						// foreign faces (state.GainedFace) and the offer/trigger walks
						// read them directly. The spec is evaluated with src = this
						// static's own source, which is exactly what makes
						// `Card.ExiledWithSource` resolve (effects' ExiledWith
						// provenance). A spec that matches nothing emits no effect, the
						// fail-closed direction every grant takes; the scan re-runs per
						// event, so a card exiled later is gained on the next rescan
						// and a card that leaves the scoped zones loses its grant.
						if gainsAbilitiesOf(st) {
							// The two parameters are resolved SEPARATELY and carried on
							// separate face lists: GainsAbilitiesOf$ means ACTIVATED
							// abilities only and GainsTriggerAbsOf$ TRIGGERED only (a
							// shared untyped list made a GainsAbilitiesOf-only card fire
							// the foreign card's phase triggers and a
							// GainsTriggerAbsOf-only card offer its activated ones -- the
							// round-2 review's break). A GainsValidAbilities$ filter and a
							// GainsAbilitiesLimitPerTurn$ cap ride the ACTIVATED half
							// (both parameters are activated-ability vocabulary).
							gg := base
							gg.Layer = LAbilities
							gg.GainedZones = strings.TrimSpace(st.Params["GainsAbilitiesOfZones"])
							gg.GainsValidAbilities = strings.TrimSpace(st.Params["GainsValidAbilities"])
							gg.GainsLimitPerTurn = gainsLimitPerTurn(st)
							if spec := strings.TrimSpace(st.Params["GainsAbilitiesOf"]); spec != "" {
								gg.GainedFaces = e.gainedFacesForSpec(st, spec, id, o.Controller)
							}
							if spec := strings.TrimSpace(st.Params["GainsTriggerAbsOf"]); spec != "" {
								gg.GainedTriggerFaces = e.gainedFacesForSpec(st, spec, id, o.Controller)
							}
							if len(gg.GainedFaces) > 0 || len(gg.GainedTriggerFaces) > 0 {
								out = append(out, gg)
							}
						}
						if hasStat(st, "AddType") || hasStat(st, "AddTypes") || hasStat(st, "AddAllCreatureTypes") {
							ty := base
							ty.Layer = LType
							ty.AddTypes = statList(st, "AddTypes")
							if len(ty.AddTypes) == 0 {
								ty.AddTypes = statList(st, "AddType")
							}
							// AddAllCreatureTypes$ True (Maskwood Nexus's "creatures you
							// control are every creature type", the manland family) rides
							// the same LType emission as a flag, never a materialised
							// type list: typeCharacteristics appends the CreatureTypeWords
							// vocabulary for affected objects, so the answer stays live
							// and no non-creature word (Arcane/Alara/Ajani) can leak.
							ty.AddAllCreatureTypes = hasStat(st, "AddAllCreatureTypes")
							// AddType$ ChosenType (22 corpus files: Adaptive Automaton's
							// "CARDNAME is the chosen type in addition to its other
							// types" and its siblings): the VALUE is the static's host
							// object's own recorded "as this enters" choice, not a
							// literal type word — resolve it against the host's
							// ChosenType (staticContinuous re-runs once per event, so a
							// later Choose event re-derives the grant live). A host with
							// no recorded choice grants nothing: a chosen type this
							// build cannot read must not leak a literal "ChosenType"
							// type word onto the object.
							if resolved, ok := resolveChosenTypes(ty.AddTypes, o); ok {
								ty.AddTypes = resolved
							} else {
								ty.AddTypes = nil
							}
							// The strip flags ride the AddType emission (measured: every
							// corpus S: line carrying RemoveCardTypes$/RemoveCreatureTypes$
							// also carries AddType$): a strip-only static -- an AddType$
							// ChosenType the host has not resolved -- must still emit so
							// the strip is not silently dropped.
							ty.RemoveCardTypes = hasStat(st, "RemoveCardTypes")
							ty.RemoveCreatureTypes = hasStat(st, "RemoveCreatureTypes")
							ty.AffectedZone = strings.TrimSpace(st.Params["AffectedZone"])
							if len(ty.AddTypes) > 0 || ty.RemoveCardTypes || ty.RemoveCreatureTypes || ty.AddAllCreatureTypes {
								out = append(out, ty)
							}
						}
						// CR 613.1e colour static (Forge's SetColor$, Imprisoned in the Moon /
						// Kenrith's Transformation / Leyline of the Guildpact): the affected
						// object's colours are exactly the named set, REPLACING its printed
						// colours and every earlier layer-5 grant in timestamp order
						// (SetColor$ overwrites; it never extends). Its sibling AddColor$
						// ("...in addition to its other colors", Blade of the Oni / Angelic
						// Armaments / Deep Freeze) is the same layer-5 walk WITHOUT the
						// overwrite, so the object keeps its printed colours and gains the
						// named ones. Both share the colour-word parser: a named colour, a
						// comma list, "All" (every colour) and, for SetColor$, "Colorless"
						// (the empty set, a real overwrite to colourless). A value it cannot
						// fully parse declines through resolveChosenColors, which is the
						// layer-5 twin of resolveChosenTypes: a value naming the host's
						// recorded choice (the corpus's "ChosenColor" family -- Alloy
						// Golem, Shifting Sky, Shimmerwilds Growth's AddColor siblings)
						// resolves to the colour, while a host with NO recorded choice
						// (an unanswered ETB ask, a non-commander CDA carrier) fails
						// CLOSED: no effect is emitted and the object keeps its printed
						// colours, the same direction effAnimate's Colors$ gate takes.
						// No Note is emitted because this scan re-runs on every event;
						// a per-derivation Note would flood the log.
						if raw, isSet := st.Params["SetColor"]; isSet {
							// A resolvable characteristic-defining self SetColor$ (the
							// Transguild Courier / Sphinx of the Guildpact "CARDNAME is
							// all colors", Ghostfire "CARDNAME is colorless" class) is
							// NOT emitted from this scan: a CDA works in EVERY zone
							// (CR 604.3/208.2), so effects.ColorMaskOf's base read now
							// applies the claim there and at the layer-5 base below,
							// and emitting here too would apply it twice -- the same
							// withholding the P/T CDA below takes. The shared
							// effects.CDASetColourClaimStatic classifier is what both
							// paths read, so they cannot disagree. A CDA the helper
							// rejects (the ChosenColor family) is NOT withheld: it
							// flows to resolveChosenColors, which resolves it against
							// the host's recorded choice or fails closed. A CDA that
							// narrows itself with AffectedZone$ would be a different
							// shape -- no corpus carrier carries one (measured), and a
							// CDA's zone width is every zone by CR 604.3 anyway.
							if _, isCDA, parsed := effects.CDASetColourClaimStatic(st); isCDA && parsed {
								// withheld: the base read applies it in every zone
							} else if cols, ok := resolveChosenColors(raw, o); ok {
								sc := base
								sc.Layer = LColor
								sc.AddColors = cols
								sc.OverwriteColors = true
								sc.AffectedZone = strings.TrimSpace(st.Params["AffectedZone"])
								out = append(out, sc)
							}
						}
						if raw, isAdd := st.Params["AddColor"]; isAdd || st.Params["AddColors"] != "" {
							if !isAdd {
								raw = st.Params["AddColors"]
							}
							if cols, ok := resolveChosenColors(raw, o); ok {
								sc := base
								sc.Layer = LColor
								sc.AddColors = cols
								sc.AffectedZone = strings.TrimSpace(st.Params["AffectedZone"])
								out = append(out, sc)
							}
						}
						// CR 613.1f / 613.4b (Humility): a base-setting static runs in
						// layer 7b (SubSet), before the 7c modify a later Pump adds; and
						// a RemoveAllAbilities static is a layer-6 ability removal.
						// A P/T-setting characteristic-defining static (CharacteristicDefining$
						// True) is NOT emitted from this scan: cdaSetPT reads it directly
						// off the object's own face in derivedScalar, in EVERY zone
						// (CR 604.3/208.2 -- the layer-7a base this battlefield-only walk
						// cannot express), and emitting here too would apply the set
						// twice. A CDA whose value this build cannot resolve keeps
						// today's emission -- fail closed is the same degrade direction
						// every static gate takes.
						if hasStat(st, "SetPower") || hasStat(st, "SetToughness") {
							skip := false
							if strings.TrimSpace(st.Params["CharacteristicDefining"]) != "" {
								if _, _, hp, ht := e.cdaPTStatic(st, &effects.Ctx{Source: id, Controller: o.Controller, SVars: fc.SVars}); hp || ht {
									skip = true
								}
							}
							if !skip {
								set := base
								set.Layer, set.Sub = LPT, SubSet
								if strings.EqualFold(st.Params["CharacteristicDefining"], "true") {
									set.Sub = SubCDA
								}
								set.SetPowerExpr = st.Params["SetPower"]
								set.SetToughnessExpr = st.Params["SetToughness"]
								set.SetPowerPresent = hasStat(st, "SetPower")
								set.SetToughnessPresent = hasStat(st, "SetToughness")
								set.StaticSet = true
								set.HasSet = true
								out = append(out, set)
							}
						}
						if hasStat(st, "RemoveAllAbilities") {
							ra := base
							ra.Layer = LAbilities
							ra.RemoveAbilities = true
							out = append(out, ra)
						}
						// A may-play-from-zone grant (M2d?): the "You may play lands from
						// your graveyard" static (Conduit of Worlds, Crucible of Worlds,
						// Ramunap Excavator, ...). It changes no characteristic, so it is
						// NOT a layer effect and is carried as a rules-mod on the effect
						// itself (MayPlay + AffectedZone) rather than as a layer mark;
						// rules/legal.go's may-play walks consult it. The implemented
						// shape is the unconditional MayPlay$ True grant plus its two
						// readable riders (MayPlayIgnoreColor$ -- mana as any colour --
						// and MayPlayLimit$ 1, the once-per-turn cap); the
						// mayPlayShape guard rejects a richer grant (MayPlayIgnoreType$/
						// MayPlayWithoutManaCost$/MayPlayText$, Condition$/
						// ValidAfterStack$/Secondary$ qualifiers) so it fails closed
						// (MayPlay stays false) rather than being silently over-applied
						// against the ordinary LandsPlayed limit. Expiry is the ordinary
						// source-leaves rule (CR 611.3b) via active()'s battlefield scan.
						if mayPlayGrant(st) {
							mp := base
							mp.MayPlay = true
							mp.AffectedZone = strings.TrimSpace(st.Params["AffectedZone"])
							mp.MayPlayIgnoreColor, mp.MayPlayIgnoreType, mp.MayPlayLimit, mp.MayPlayPlayerTurn, _ = effects.MayPlayStaticParams(st.Params)
							out = append(out, mp)
						}
						// An additional-land-drops grant (Azusa, Lost but Seeking's "You
						// may play two additional lands on each of your turns", Oracle of
						// Mul Daya, Exploration, Icetill Explorer). Like the may-play
						// grant it changes no characteristic, so it is NOT a layer effect
						// and is carried as a rules-mod on the effect itself
						// (AdjustLandPlays); rules/legal.go's land-play gates consult it
						// through Engine.adjustLandPlays. The implemented shape is the
						// plain one -- a literal positive integer value and only display
						// metadata around it; the Affects spec is evaluated at the gate
						// with MatchesPlayerSpecFrom, whose own fail-closed rule (an
						// unhandled qualifier matches nobody) rejects the richer
						// Affected$ forms. A richer VALUE or rider fails closed here: an
						// AdjustLandPlays$ Unlimited/Z (Fastbond, an X-driven grant)
						// must not silently become "one more", and an IsPresent$/
						// Secondary$ qualifier changes when the grant lives. The explicit
						// whitelist, rather than a blacklist of currently-known gating
						// keys, means a newly encountered semantic parameter also fails
						// closed. Expiry is the ordinary source-leaves rule (CR 611.3b)
						// via active()'s battlefield scan; the turn scoping ("each of
						// your turns") is the offer gate itself -- a play_land option is
						// only offered to the active player in a main phase -- and the
						// per-turn reset stays events' TurnChange LandsPlayed = 0.
						if n, ok := adjustLandPlaysGrant(st.Params); ok {
							al := base
							al.AdjustLandPlays = n
							out = append(out, al)
						}
						// --- the four static-grant kinds the parameter census named
						// (task inbox-paramcensus-static-grant-misc). Each reads its
						// own key and fails closed on a shape it cannot evaluate, like
						// every grant branch above. ---

						// A static-that-grants-a-static (Exploration Broodship's
						// "STATION 3+"): AddStaticAbility$ names an SVar on the granting
						// face whose body is itself a Mode$ Continuous static, granted
						// for exactly as long as the OUTER static is live (its gate has
						// already run above, and the inner static's own gate runs when
						// its queue entry is emitted -- the same fail-closed rule). The
						// grant's HOST is the object the OUTER Affected$ spec matches
						// (the scan rebuilds per event, so the counters move, the grant
						// follows); the inner static's own Affected$ scopes what IT
						// affects, resolved against the host -- the Broodship's STATION
						// 3+ grant is an AdjustLandPlays$ 1 to You, and You is the
						// host's controller. cards.ParseStaticLines gives the body the
						// same shape a printed S: line would have, so EVERY grant branch
						// above applies to the inner static unchanged. A body this
						// parser refuses, one whose mode is not Continuous, or a host
						// the outer spec no longer matches, grants nothing.
						if name := strings.TrimSpace(st.Params["AddStaticAbility"]); name != "" && w.depth == 0 {
							if inners, ok := cards.ParseStaticLines(fc.SVars[name]); ok {
								for _, inner := range inners {
									if inner.Mode == "Continuous" &&
										e.matchesSpecFrom(affects, id, o.Controller, id) {
										grantQueue = append(grantQueue, staticWork{st: inner, depth: w.depth + 1})
									}
								}
							}
						}
						// A triggered-ability grant (Hearthhull's "STATION 8+ Whenever
						// you sacrifice a land"): AddTrigger$ names an SVar on the
						// granting face whose body is a T:-shaped trigger; the objects
						// the static's Affected$ matches gain it while the static is
						// live. cards.ParseTriggerLine gives the body the same shape a
						// printed T: line would have; rules/trigger_match.go's granted-
						// trigger walk (checkGrantedStaticTriggers, the granted-Ward/
						// granted-Dethrone precedent) matches it like any other trigger
						// and links its Execute$ from the GRANTING face's own SVar
						// table -- the table events.Apply's GrantTriggerPush resolves
						// from (the grantor rides the event's Amount), so the live
						// queue and a replayed one mint the same stack object. A
						// self-grant degenerates to the affected object; a body that
						// fails to parse grants nothing.
						if name := strings.TrimSpace(st.Params["AddTrigger"]); name != "" {
							if t, ok := cards.ParseTriggerLine(fc.SVars[name]); ok {
								gt := base
								gt.AddTrigger = &t
								out = append(out, gt)
							}
						}
						// A named-variable grant (Sword of Fire and Ice): AddSVar$ names an SVar
						// on the granting face whose body is Forge's
						// "SVar:<Name>:<Value>" grant shape -- the affected object GAINS
						// that named variable while the static is live. The corpus's
						// granted SVars are AI-evaluation hints (AE, AITap,
						// MustBeBlocked) no rules consumer reads; the engine records the
						// grant and resolves it through Engine.GrantedSVar, the lookup a
						// later CheckSVar$-style consumer of the affected object's
						// variables reads. A body in any other shape grants nothing.
						if raw := strings.TrimSpace(st.Params["AddSVar"]); raw != "" {
							if n, v, ok := parseSVarGrant(fc.SVars[raw]); ok {
								gv := base
								gv.AddSVars = map[string]string{n: v}
								out = append(out, gv)
							}
						}
						// A look-permission grant (Oracle of Mul Daya): MayLookAt$ says
						// the affected player may look at the object the Affected$ spec
						// matches -- in the corpus always the top card of the
						// controller's own library (Affected$ Card.TopLibrary+YouCtrl,
						// AffectedZone$ Library). The value names WHO may look:
						// You/Player/True are all the static's controller in every
						// corpus shape (73/34/1 raw lines at the pin); anything else
						// fails closed. Consumed by Engine.MayLookAtLibraryTop, the
						// view's reveal of that top card.
						if raw := strings.TrimSpace(st.Params["MayLookAt"]); raw != "" {
							if strings.EqualFold(raw, "You") || strings.EqualFold(raw, "Player") || strings.EqualFold(raw, "True") {
								lv := base
								lv.MayLookAt = true
								out = append(out, lv)
							}
						}
						// A control-change static (Mind Control's "You control enchanted
						// creature", Fealty to the Realm's "The monarch controls
						// enchanted creature"): GainControl$ on a Mode$ Continuous static
						// hands every object the Affected$ spec matches to the player the
						// value names, for exactly as long as the static is live. Like
						// MayPlay it changes no characteristic, so it is carried as a
						// rules-mod (GainControl) and realized by rules'
						// reconcileControlStatics (rules/control_static.go): that pass
						// registers a real tracked control grant and emits
						// events.ControlChange only where the object's controller
						// actually differs, and the tracked grant's liveness (grantEnded)
						// is this scan's own output, so an ended static -- source left,
						// gate flipped, Aura moved bearers, named player changed -- hands
						// the bearer back through expireControl's ordinary Previous
						// chain. The VALUE itself is not validated here (the scan cannot
						// resolve players): resolution happens in the reconcile, where a
						// value that names nobody -- or several players -- yields no
						// grant, the fail-closed direction. The static's "as long as"
						// gate (IsPresent$/CheckSVar$) already ran above for every
						// branch. Measured corpus population (GNU /usr/bin/grep): 42 raw
						// S:Mode$ Continuous lines carrying GainControl$, every one
						// shaped Mode/Affected/GainControl/Description with Affected$
						// *.EnchantedBy and the value You (41) or Player.isMonarch (1,
						// Fealty to the Realm); none is in any repo deck, so the golden
						// heads and the ratchet are untouched by construction.
						if raw := strings.TrimSpace(st.Params["GainControl"]); raw != "" {
							gc := base
							gc.GainControl = raw
							out = append(out, gc)
						}
					}
					e.staticQueueBuf = grantQueue
				}
			}
		}
	}
	if len(out) < len(dst) {
		clear(dst[len(out):])
	}
	return out
}

// stackSelfStaticOK admits a printed Continuous static with NO EffectZone$
// whose SOURCE sits on the stack, when the static's own text scopes itself to
// the stack (an IsPresent$/PresentZone$ or AffectedZone$ naming Stack).
// effectZoneOK's default admission is the battlefield -- right for a
// permanent's continuous statics, wrong for a spell's own on-the-stack
// static: Molten Disaster's kicked split second (IsPresent$ Card.Self+kicked
// | PresentZone$ Stack) names the stack as the zone it functions from, and
// CR 113.6 has it live exactly there, while an unqualified lord static (a
// creature spell's "creatures you control get +1/+1") still stays
// battlefield-only. PresentZone$ is a comma list in the grammar, hence the
// substring read.
// staticZoneAdmits is the source-zone admission a Continuous static's
// ExcludeZone$ and EffectZone$ parameters jointly express, the ONE read
// staticEffects' gate and cdaPTStatic's layer-7a CDA claim both make. With
// no ExcludeZone$ the ordinary EffectZone$ gate stands unchanged (empty =
// battlefield). With one, the named zones are excluded and -- absent an
// explicit EffectZone$ -- every OTHER zone admits, which is what lets a
// zone-conditional CDA (Grist) live exactly off the battlefield. An
// unrecognised word excludes NOTHING (the mirror-image direction of
// affectedZoneOK's fail-closed deny): the unparseable exclusion degrades to
// the ordinary gate, today's applies-as-gated behaviour, rather than going
// silent.
func staticZoneAdmits(exclude, effectZone string, z state.Zone) bool {
	exclude = strings.TrimSpace(exclude)
	if exclude == "" {
		return effectZoneOK(effectZone, z)
	}
	zones, all, ok := effects.ParseZones(exclude)
	if !ok {
		return effectZoneOK(effectZone, z)
	}
	if all || slices.Contains(zones, z) {
		return false
	}
	return effectZone == "" || effectZoneOK(effectZone, z)
}

func (e *Engine) stackSelfStaticOK(st cards.Static, o *state.Object) bool {
	if st.Params["EffectZone"] != "" || o == nil || o.Zone != state.ZStack {
		return false
	}
	return strings.Contains(st.Params["PresentZone"], "Stack") ||
		st.Params["AffectedZone"] == "Stack"
}

// staticSourceZones is staticEffects' per-seat source walk, in one fixed
// order: the battlefield (every default EffectZone$ static's zone), then the
// non-battlefield zones a Continuous static's EffectZone$ can name. The
// order mirrors collectCostStatics' (rules/statics.go) so the two
// zone-scoped collectors cannot drift apart; hand and library are included
// because Forge's EffectZone$ All statics (Chittering Illuminator's
// may-play-from-top-of-library grant) are live there too, and the shared
// stack is skipped for every seat after the first (see the walk).
var staticSourceZones = []state.Zone{
	state.ZBattlefield, state.ZStack, state.ZGraveyard,
	state.ZHand, state.ZLibrary, state.ZExile, state.ZCommand,
}

// staticWork is one staticEffects queue entry: the static to emit and its
// AddStaticAbility$ depth (0 = a printed static, 1 = a granted one).
type staticWork struct {
	st    cards.Static
	depth int
}

// gainedFacesForSource collects every foreign face a live has-all-abilities-of
// grant gives `source` right now: the union of the activated half (GainedFaces)
// and the triggered half (GainedTriggerFaces) of every active grant whose Affects
// spec matches source, in active()'s deterministic layer/timestamp
// order and each effect's own face order. It is the ONE recovery point
// the resolution-time owning-face reads use (findTriggerForAbilityFace for a
// gained TRIGGER, pileFaceForSA for a gained ACTIVATED ability), so the
// OptionalDecider$/intervening-if gates and the SVar-table reads all see the
// foreign card's own face exactly as the offer/queue walk did. The union is
// safe because each consumer matches its SA/trigger by pointer identity, so a
// face present only in the other half never binds. A source no
// live grant matches returns nil.
func (e *Engine) gainedFacesForSource(source state.ObjID) []state.GainedFace {
	var out []state.GainedFace
	for _, ce := range e.active() {
		if len(ce.GainedFaces) == 0 && len(ce.GainedTriggerFaces) == 0 {
			continue
		}
		if !effects.MatchesSpecFrom(e.G, ce.Affects, source, ce.Controller, ce.Source) {
			continue
		}
		out = append(out, ce.GainedFaces...)
		out = append(out, ce.GainedTriggerFaces...)
	}
	return out
}

// gainsAbilitiesOf reports whether a Mode$ Continuous static grants abilities
// off a named card (Forge's GainsAbilitiesOf$ / GainsTriggerAbsOf$). Either
// parameter alone is enough: a static may grant only activations, only
// triggers, or both (Idris, Soul of the TARDIS carries both). The two halves
// are carried on separate face lists (GainedFaces / GainedTriggerFaces)
// because the parameters mean different ability kinds.
func gainsAbilitiesOf(st cards.Static) bool {
	return strings.TrimSpace(st.Params["GainsAbilitiesOf"]) != "" ||
		strings.TrimSpace(st.Params["GainsTriggerAbsOf"]) != ""
}

// gainsLimitPerTurn parses a has-all-abilities-of static's
// GainsAbilitiesLimitPerTurn$ cap (Mairsil the Pretender's "only once each
// turn"). Only a plain integer is enforced -- the corpus's every carrier is
// a literal 1 -- and an unparseable value degrades to 0 (no cap), the
// permissive direction for an unmodelled expression.
func gainsLimitPerTurn(st cards.Static) int {
	n, err := strconv.Atoi(strings.TrimSpace(st.Params["GainsAbilitiesLimitPerTurn"]))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// gainedFacesForSpec resolves one has-all-abilities-of parameter's named
// cards: every object in the GainsAbilitiesOfZones$ zones (default
// Battlefield, Forge's StaticAbilityContinuous default) whose face matches
// the given card filter, paired with its object id. The spec is
// evaluated with src = the static's own source object, so
// `Card.ExiledWithSource` matches exactly the cards this source exiled.
//
// The walk order is fully deterministic -- the staticSourceZones order,
// then each alive seat in APNAP order, then the zone slice -- so the granted
// face list, and therefore the option and trigger order it feeds, is
// reproducible run to run. A spec matching nothing returns nil (no grant);
// an unparseable zones value returns nil, the fail-closed direction every
// grant branch takes. Faces are de-duplicated by object id.
func (e *Engine) gainedFacesForSpec(st cards.Static, spec string, src state.ObjID, controller state.PlayerID) []state.GainedFace {
	if spec == "" {
		return nil
	}
	zones, all, ok := effects.ParseZones(strings.TrimSpace(st.Params["GainsAbilitiesOfZones"]))
	if strings.TrimSpace(st.Params["GainsAbilitiesOfZones"]) == "" {
		// Forge's default zone for the has-all-abilities-of statics is the
		// battlefield (StaticAbilityContinuous's default), not every zone.
		zones, all, ok = []state.Zone{state.ZBattlefield}, false, true
	}
	if !ok {
		return nil
	}
	inZones := func(z state.Zone) bool {
		if all {
			return true
		}
		for _, want := range zones {
			if want == z {
				return true
			}
		}
		return false
	}
	var out []state.GainedFace
	seen := map[state.ObjID]bool{}
	for _, p := range e.G.AliveFrom(0) {
		for _, z := range staticSourceZones {
			if !inZones(z) {
				continue
			}
			for _, id := range e.G.Zone(z, p) {
				if seen[id] {
					continue
				}
				o := e.G.Obj(id)
				if o == nil || o.Face() == nil {
					continue
				}
				if !e.matchesSpecFrom(spec, id, controller, src) {
					continue
				}
				seen[id] = true
				out = append(out, state.GainedFace{Obj: id, Face: o.Face()})
			}
		}
	}
	return out
}

// parseSVarGrant parses Forge's AddSVar$ value shape "SVar:<Name>:<Value>":
// the named variable the affected object gains. ok is false for any other
// shape.
func parseSVarGrant(raw string) (name, value string, ok bool) {
	rest, ok := strings.CutPrefix(raw, "SVar:")
	if !ok {
		return "", "", false
	}
	name, value, ok = strings.Cut(rest, ":")
	if !ok || name == "" {
		return "", "", false
	}
	return name, value, true
}

// cdaPTStatic resolves ONE static's characteristic-defining P/T claim
// (CharacteristicDefining$ True), the layer-7a base cdaSetPT applies in
// every zone (CR 613.4a, CR 604.3/208.2). A static carrying any parameter
// beyond the implemented shape's whitelist fails closed (no claim -- the
// explicit-whitelist rule adjustLandPlaysGrant documents), as does one
// scoped to anything but the card itself, and one whose SetPower$/
// SetToughness$ value is neither a literal nor an SVar/inline Count$
// expression the evaluator resolves (EvalCountOK's verdict -- e.g.
// LifePaidOnETB's paid-life shape). Iterating st.Params only yields the
// whitelist boolean, so map order never reaches a value -- determinism is
// preserved.
func (e *Engine) cdaPTStatic(st cards.Static, ctx *effects.Ctx) (p, t int32, hasP, hasT bool) {
	for key := range st.Params {
		switch key {
		case "Mode", "CharacteristicDefining", "SetPower", "SetToughness", "Affected", "Description", "ExcludeZone":
			// The keys the implemented CDA shape (and only it) carries.
		default:
			return 0, 0, false, false
		}
	}
	if aff := strings.TrimSpace(st.Params["Affected"]); aff != "" && aff != "Card.Self" {
		return 0, 0, false, false
	}
	// ExcludeZone$ narrows the claim's zones (Grist, the Hunger Tide): the CDA
	// read is every zone by CR 604.3/208.2, minus the ones the static names --
	// and beside any explicit EffectZone$, exactly as the emission gate reads
	// the pair. The same staticZoneAdmits helper, so the layer-7a claim and
	// any emitted fallback ce cannot disagree about where the static is live.
	// A source object already gone carries no zone to admit.
	if raw := strings.TrimSpace(st.Params["ExcludeZone"]); raw != "" {
		if oz := e.G.Obj(ctx.Source); oz == nil || !staticZoneAdmits(raw, st.Params["EffectZone"], oz.Zone) {
			return 0, 0, false, false
		}
	}
	if raw, ok := st.Params["SetPower"]; ok {
		if n, ok := e.cdaValue(ctx, raw); ok {
			p, hasP = n, true
		}
	}
	if raw, ok := st.Params["SetToughness"]; ok {
		if n, ok := e.cdaValue(ctx, raw); ok {
			t, hasT = n, true
		}
	}
	return p, t, hasP, hasT
}

// cdaValue resolves one CDA P/T value: a literal integer, else the SVar
// named on the card's own face, else an inline Count$ expression -- each
// through effects.EvalCountOK's resolvability verdict, so an unmodelled
// count body fails closed (no claim) instead of degrading to a silent zero.
func (e *Engine) cdaValue(ctx *effects.Ctx, raw string) (int32, bool) {
	raw = strings.TrimSpace(raw)
	if n, err := strconv.Atoi(raw); err == nil {
		return int32(n), true
	}
	if strings.HasPrefix(raw, "Count$") {
		return effects.EvalCountOK(e, ctx, raw)
	}
	if body, ok := ctx.SVars[raw]; ok {
		return effects.EvalCountOK(e, ctx, body)
	}
	return 0, false
}

// cdaSetPT is the object's own layer-7a characteristic-defining P/T
// (CR 613.4a): the first usable CDA static on the current face (script
// order) resolves the base power and toughness cdaPTStatic's whitelist
// admits. CR 604.3/208.2 put the ability in EVERY zone, which is exactly
// why it is read directly off the face in derivedScalar rather than emitted
// from the battlefield-only static scan. A face with no usable CDA degrades
// to no claim (the printed P/T stands).
func (e *Engine) cdaSetPT(o *state.Object) (p, t int32, hasP, hasT bool) {
	f := o.Face()
	if f == nil {
		return 0, 0, false, false
	}
	ctx := &effects.Ctx{Source: o.ID, Controller: o.Controller, SVars: f.SVars}
	for _, st := range f.Statics {
		if st.Mode != "Continuous" || strings.TrimSpace(st.Params["CharacteristicDefining"]) == "" {
			continue
		}
		if pp, tt, hp, ht := e.cdaPTStatic(st, ctx); hp || ht {
			return pp, tt, hp, ht
		}
	}
	return 0, 0, false, false
}

// GrantedSVar reports the named variable a live static grant (AddSVar$ on a
// Mode$ Continuous static, e.g. Sword of Fire and Ice's MustBeBlocked on the
// equipped creature) gives id: the value Forge's "SVar:<Name>:<Value>"
// grant shape carries, or ok=false when no live grant gives id that name.
// The lookup is a read over active()'s sorted slice (never a map range), so
// the answer is deterministic; the map it reads is key-resolved, so map
// order never reaches an event, option, view or file.
func (e *Engine) GrantedSVar(id state.ObjID, name string) (string, bool) {
	for _, ce := range e.active() {
		if len(ce.AddSVars) == 0 {
			continue
		}
		if v, ok := ce.AddSVars[name]; ok && e.matchesSpecFrom(ce.Affects, id, ce.Controller, ce.Source) {
			return v, true
		}
	}
	return "", false
}

// grantedSVarsFor merges every live AddSVar$ grant that affects id into one
// map; nil when none applies (the common path allocates nothing). The
// result is a FRESH map, never the face's own table: the caller layers it
// under the printed table (a printed SVar of the same name wins, the same
// precedence the roll-publication read documents) and must not mutate
// immutable card data.
func (e *Engine) grantedSVarsFor(id state.ObjID) map[string]string {
	var merged map[string]string
	for _, ce := range e.active() {
		if len(ce.AddSVars) == 0 {
			continue
		}
		if !e.matchesSpecFrom(ce.Affects, id, ce.Controller, ce.Source) {
			continue
		}
		if merged == nil {
			merged = map[string]string{}
		}
		for n, v := range ce.AddSVars {
			merged[n] = v
		}
	}
	return merged
}

// MayLookAtLibraryTop reports whether p may look at the top card of p's own
// library right now: a live Continuous MayLookAt grant (Oracle of Mul Daya)
// whose Affected$ spec matches that top card -- the TopLibrary predicate in
// the spec itself pins the object to the top of its owner's library, and
// YouCtrl resolves against the granting static's controller, so a stolen
// Oracle reveals to its controller, never to a library owner the grant does
// not cover. A read over active()'s sorted slice; the answer is boolean, so
// no order reaches anything ordered.
func (e *Engine) MayLookAtLibraryTop(p state.PlayerID) bool {
	lib := e.G.Zone(state.ZLibrary, p)
	if len(lib) == 0 {
		return false
	}
	for _, ce := range e.active() {
		if ce.MayLookAt && e.matchesSpecFrom(ce.Affects, lib[0], ce.Controller, ce.Source) {
			return true
		}
	}
	return false
}

// continuousGateHolds evaluates the "as long as" condition gates a Mode$
// Continuous static can carry, the intervening-if that decides whether the
// grant lives at this instant: IsPresent$/IsPresent2$ (an existence count over
// every battlefield, PresentCompare$ pricing the count -- default GE1),
// CheckSVar$/SVarCompare$ (the named SVar -- or inline Count$ expression --
// compared under the threshold, no compare meaning "nonzero"), and
// Condition$ (the ability-word condition family). Every evaluator is shared
// with the restriction/cost/ability gates (rules/statics.go's presentGate,
// checkSVarHolds and costConditionHolds; rules/legal.go's
// activationConditionOK) so the ONE grammar governs every static family.
// staticEffects re-runs once per emitted event, so evaluating the gate there
// is the continuous recheck the grant needs. A gate this build cannot evaluate
// fails closed -- the shipped statics convention: an unreadable "as long as"
// must not silently always-apply.
func (e *Engine) continuousGateHolds(sv staticView) bool {
	if !e.classBandGateHolds(sv.Params, sv.Source) {
		return false
	}
	if spec, ok := sv.Params["IsPresent"]; ok && !e.presentGate(sv, spec) {
		return false
	}
	if spec, ok := sv.Params["IsPresent2"]; ok && !e.presentGate(sv, spec) {
		return false
	}
	if !e.continuousConditionHolds(sv) {
		return false
	}
	return e.checkSVarHolds(sv)
}

// continuousConditionHolds evaluates Condition$ on a Mode$ Continuous static
// -- the "Delirium --", "Threshold --", "Metalcraft --" ability-word family
// whose grant lives only while the condition is met. The evaluable values map
// onto the shared condition machinery the other static families already use:
//
//   - Delirium: the controller's graveyard holds 4+ distinct core card types
//     (rules/replacement.go's graveyardCardTypeCount, the ONE census shared
//     with rules/legal.go's activationConditionOK);
//   - PlayerTurn / NotPlayerTurn: the static's controller is or is not the
//     active player (the same reads rules/statics.go's costConditionHolds and
//     restrictionGateHolds make);
//   - Metalcraft: 3+ artifacts the controller controls (costConditionHolds'
//     Count$ arm);
//   - Threshold: 7+ cards in the controller's graveyard;
//   - Hellbent: the controller's hand is empty.
//   - Blessing: the controller holds the city's blessing (CR 702.131, the
//     Ascend latch, state.Player.Blessing -- granted by rules/ascend.go's
//     emit-side scan and spell-resolution grant).
//
// Every other value -- EnduringStory, FatefulHour, Monarch, MaxSpeed
// and anything new -- FAILS CLOSED (the gate never holds), matching every
// sibling gate's documented deny direction. MaxSpeed is safe to deny here:
// its statics carry only AddAbility$/AddStaticAbility$/AddTrigger$/
// AddReplacementEffect$/AddSVar$, never a layer-walk key, and the speed family
// is read separately by rules/speed.go's maxSpeedAbilities. An absent or empty
// Condition$ keeps holding, as before.
func (e *Engine) continuousConditionHolds(sv staticView) bool {
	raw, ok := sv.Params["Condition"]
	if !ok {
		return true
	}
	switch strings.TrimSpace(raw) {
	case "":
		return true
	case "Delirium":
		return e.graveyardCardTypeCount(sv.Controller) >= 4
	case "PlayerTurn":
		return e.G.Active == sv.Controller
	case "NotPlayerTurn":
		return e.G.Active != sv.Controller
	case "Metalcraft":
		return e.metalcraftHolds(sv.Controller)
	case "Threshold":
		return len(e.G.Zone(state.ZGraveyard, sv.Controller)) >= 7
	case "Hellbent":
		return len(e.G.Zone(state.ZHand, sv.Controller)) == 0
	case "Blessing":
		// CR 702.131: the city's blessing (Ascend). The latch is one-way
		// and only ever written by events.Apply's BlessingChange fold, so
		// the read is a plain state read.
		if int(sv.Controller) >= len(e.G.Players) {
			return false
		}
		return e.G.Players[sv.Controller].Blessing
	}
	return false
}

// adjustLandPlaysGrant reports whether a Mode$ Continuous static carries the
// additional-land-drops grant this package implements, and resolves its
// value: a literal positive integer AdjustLandPlays$ plus only display
// metadata (Description$), with the Affects player spec left to the gate's
// own fail-closed evaluation. Anything else -- a non-literal value
// (Unlimited, an SVar-driven Z), a rider that changes when the grant lives
// (IsPresent$, Condition$, CheckSVar$, SVarCompare$, Secondary$,
// EffectZone$) or any other semantic parameter -- fails closed so the grant
// is never silently under- or over-applied. Iterating the params map only
// yields a boolean, so map order never reaches an event/option/view --
// determinism is preserved.
func adjustLandPlaysGrant(params map[string]string) (int32, bool) {
	raw, ok := params["AdjustLandPlays"]
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		// A value this build cannot price as a count (Unlimited, an SVar
		// token) must not silently become a smaller grant.
		return 0, false
	}
	for key := range params {
		switch key {
		case "Mode", "AdjustLandPlays", "Affected", "Description":
			// The keys the implemented grant (and only it) carries.
		default:
			return 0, false
		}
	}
	return int32(n), true
}

// mayPlayGrant reports whether a Mode$ Continuous static carries the
// may-play grant this package implements: MayPlay$ True, an Affects
// (Affected$) spec and an AffectedZone, plus only display/placement metadata
// and the riders it reads (MayPlayIgnoreColor$, MayPlayIgnoreType$,
// MayPlayLimit$). A richer grant is out of scope and must fail closed (MayPlay
// stays false) so it is never silently over-applied -- in particular a
// MayPlayWithoutManaCost$ (free cast) static changes what the cast IS, not
// just where it may come from, and a Condition$/CheckSVar$/ValidAfterStack$/
// Secondary$ qualifier changes when the grant lives. The explicit whitelist,
// rather than a blacklist of currently-known gating keys, means a newly
// encountered semantic parameter also fails closed. Iterating st.Params only
// yields a boolean, so map order never reaches an event/option/view --
// determinism is preserved.
func mayPlayGrant(st cards.Static) bool {
	_, _, _, _, ok := effects.MayPlayStaticParams(st.Params)
	return ok
}

// hasStat reports whether a static line carries the named parameter.
func hasStat(st cards.Static, key string) bool {
	_, ok := st.Params[key]
	return ok
}

// staticAmount evaluates a static's P/T parameter at derivation time. It
// deliberately goes through effects.Num: that is the shared Forge numeric
// grammar for signed SVar names and Count$ bodies. The source and its SVar
// table are rebound on every call, so a life total, counters, or zones changing
// after the static entered changes its value without any cached snapshot.
func (e *Engine) staticAmount(ce ContinuousEffect, expr string) int32 {
	if expr == "" {
		return 0
	}
	o := e.G.Obj(ce.Source)
	if o == nil || o.Face() == nil {
		return 0
	}
	svars := ce.SVars
	if svars == nil {
		svars = o.Face().SVars
	}
	sa := &cards.SA{Params: map[string]string{"Amount": expr}}
	return effects.Num(e, &effects.Ctx{Source: ce.Source, Controller: ce.Controller, SVars: svars}, sa, "Amount", 0)
}

// addPT saturates instead of allowing a large static expression to wrap a
// characteristic through zero. Forge's calculateAmount is int-bounded too;
// keeping the clamp at this boundary makes all P/T additions deterministic.
func addPT(a, b int32) int32 {
	n := int64(a) + int64(b)
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < math.MinInt32 {
		return math.MinInt32
	}
	return int32(n)
}

// statKeywords parses AddKeyword$ through the shared Forge keyword-list
// parser. In particular its ampersands divide keywords while commas remain
// inside a keyword's parameters.
func statKeywords(st cards.Static) []string {
	return cards.SplitKeywordList(st.Params["AddKeyword"])
}

// statList parses additive TYPE parameters. Type lists retain their existing
// comma-separated grammar; they must not use SplitKeywordList, whose
// ampersand grammar is specific to keyword parameters.
func statList(st cards.Static, key string) []string {
	var out []string
	for _, v := range strings.Split(st.Params[key], ",") {
		for _, part := range strings.Split(strings.TrimSpace(v), " & ") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// resolveChosenTypes resolves the AddType$ value "ChosenType" against the
// static host's own recorded ETB choice (state.Object.ChosenType, set by the
// Choose event the cast/play-time ask emitted). Everything else passes
// through unchanged. ok is false when a ChosenType entry names a host with no
// recorded choice — the caller withholds the grant whole.
func resolveChosenTypes(list []string, o *state.Object) ([]string, bool) {
	out := make([]string, 0, len(list))
	for _, t := range list {
		if t != "ChosenType" {
			out = append(out, t)
			continue
		}
		if o == nil || o.ChosenType == "" {
			return nil, false
		}
		out = append(out, o.ChosenType)
	}
	return out, true
}

// resolveChosenColors resolves a SetColor$/AddColor$ value against the static
// host's own recorded "as this enters, choose a color" / CR 903.4b pregame
// choice (state.Object.ChosenColor, set by the Choose event the ask emitted).
// It is the layer-5 twin of resolveChosenTypes. A value of "ChosenColor"
// resolves to the host's recorded colour -- the event records a single WUBRG
// letter (rules/cast.go etbAnswer), but a full colour word is accepted too so
// the two spellings cannot drift -- and a host with NO recorded choice fails
// closed: ok=false, the caller emits nothing and the object keeps its printed
// colours (today's shipped behaviour for the whole family).
//
// Everything else passes through the ordinary colour-word parser. A bare
// WUBRG letter is accepted directly (the layer-5 walk at ~1652 reads
// strings.IndexByte("WUBRG", l[0]), so a letter element is already legal),
// which is the shape the recorded choice itself carries; a value the parser
// cannot fully recognise still fails closed, exactly as before.
func resolveChosenColors(raw string, o *state.Object) ([]string, bool) {
	if strings.EqualFold(strings.TrimSpace(raw), "ChosenColor") {
		if o == nil || o.ChosenColor == "" {
			return nil, false
		}
		if cols, ok := effects.ColorLetters(o.ChosenColor); ok && len(cols) > 0 {
			return cols, true
		}
		// A bare WUBRG letter (the recorded form) bypasses the word parser.
		if l := strings.ToUpper(strings.TrimSpace(o.ChosenColor)); len(l) == 1 && strings.IndexByte("WUBRG", l[0]) >= 0 {
			return []string{l}, true
		}
		return nil, false
	}
	return effects.ColorLetters(raw)
}

// Layer, Sublayer and ContinuousEffect moved to state/continuous.go in Task
// 19c, so effects primitives (which sit below rules and must never import
// it) can build a ContinuousEffect and hand it to this engine through
// effects.Host. These aliases and re-exported constants keep this package's
// own API -- and every existing caller and test in this package -- unchanged:
// only the canonical type definition moved, not its name or behaviour here.
type (
	Layer            = state.Layer
	Sublayer         = state.Sublayer
	ContinuousEffect = state.ContinuousEffect
)

const (
	LCopy      = state.LCopy
	LControl   = state.LControl
	LText      = state.LText
	LType      = state.LType
	LColor     = state.LColor
	LAbilities = state.LAbilities
	LPT        = state.LPT
)

const (
	SubNone     = state.SubNone
	SubCDA      = state.SubCDA
	SubSet      = state.SubSet
	SubModify   = state.SubModify
	SubCounters = state.SubCounters
	SubSwitch   = state.SubSwitch
)

// Derived is a permanent's current characteristics after every applicable
// continuous effect has been applied in CR 613 order. Nothing outside this
// file may read printed power, toughness or keywords directly — Derived (or
// the Power/Toughness/HasKeyword/Keywords accessors below) is the only path.
type Derived struct {
	Power, Toughness int32
	Keywords         []string
	Types            []string
	// Colors is the object's current colour set as WUBRG letters (CR 613.1e):
	// its face's colours (effects.ColorsOf, which already applies Devoid)
	// then every applicable layer-5 effect in timestamp order -- an
	// OverwriteColors grant replaces the set so far, a plain one extends it.
	// "" is a colourless object, not "no read": a battlefield land reads "".
	Colors string
}

// AddContinuous registers one continuous effect. A zero Timestamp is
// stamped from the game clock, so callers that do not care about relative
// ordering against other effects created in the same instant need not touch
// the clock themselves; the layer tests that DO care set Timestamp
// explicitly and bypass this.
//
// Ruling T19-a: the clock advances only through a logged ClockTick event,
// never a direct write to e.G.Clock. Object.Timestamp (see events.Move) is
// stamped from this same clock whenever a permanent enters the battlefield,
// so a direct write here would leave a game reconstructed from the log alone
// off by one on every later Timestamp — the same bug class Ruling T11-a
// already fixed for Passes/Priority.
func (e *Engine) AddContinuous(ce ContinuousEffect) {
	if ce.Timestamp == 0 {
		e.emit(events.Event{Kind: events.ClockTick})
		ce.Timestamp = e.G.Clock
	}
	// A Duration$ that spans the controller's NEXT turn (UntilYourNextTurn /
	// UntilTheEndOfYourNextTurn) gets a real turn-boundary lifetime: compute
	// the turn at whose end the effect expires from the live rotation, so it
	// outlives its source (a one-shot spell is already off the battlefield by
	// the time it registered) and is dropped by EndOfTurnCleanup when e.G.Turn
	// reaches UntilTurn -- not at the end of the current turn (UntilEOT) nor
	// never (source-leaves). Recomputing it here each re-execution is what
	// makes the boundary byte-identical on replay. If the controller cannot
	// be found in the alive rotation (eliminated) the effect gets no turn
	// boundary and falls back to the source-leaves rule.
	if effects.IsNextTurnDuration(ce.Duration) && ce.UntilTurn == 0 {
		ce.UntilTurn = e.nextTurnFor(ce.Controller)
	}
	e.continuous = append(e.continuous, ce)
	// Bump the cache version: active() (below) caches its sorted effect list
	// on (log head, continuousVersion), and this is the write that changes
	// e.continuous. The ClockTick above moved the log head too, but naming
	// the dependency explicitly here keeps active()'s invalidation correct
	// even if a future caller adds a continuous effect without an event.
	e.continuousVersion++
}

// EndEffect ends the one continuous-effect registration identified by
// (source, stamp) -- the one-shot Effect self-exile (`DB$ ChangeZone |
// Defined$ Self | Origin$ Command | Destination$ Exile`) that Forge models by
// exiling the implicit effect object it keeps in the Command zone. It drops
// exactly the entries carrying that identity (the same identity the
// replacement key and applyReplaceDamageTail's shield depletion use), so a
// source's OTHER registrations and its printed abilities are untouched.
// Engine-runtime only, rebuilt by re-execution on replay exactly like every
// other continuous-registry write; it emits no event.
func (e *Engine) EndEffect(source state.ObjID, stamp uint32) {
	if source == 0 {
		return
	}
	kept := e.continuous[:0]
	changed := false
	for _, ce := range e.continuous {
		if ce.Source == source && ce.Timestamp == stamp {
			changed = true
			continue
		}
		kept = append(kept, ce)
	}
	if !changed {
		return
	}
	e.continuous = kept
	e.continuousVersion++
}

// EndImprintedEffects ends every live DB$ Effect registration that an
// ImprintOnHost$ True Effect imprinted on the named host card (the entries
// carrying state.ContinuousEffect.ImprintOnHost with that Source) -- the
// analogue of Forge's later `DB$ ChangeZone | Defined$ Imprinted | Origin$
// Command | Destination$ Exile` exiling the imprinted effect token from the
// Command zone (Superior Foes of Spider-Man's "until you exile another card
// with this creature": the second dig's trigger exiles the FIRST effect's
// token, ending its may-play grant, before the new dig's Effect registers;
// Word of Command and Semester's End run the same idiom inside one chain).
// A registration without the marker -- the same source's OTHER effects and
// its printed abilities -- is untouched. Engine-runtime only, rebuilt by
// re-execution on replay exactly like EndEffect; it emits no event.
func (e *Engine) EndImprintedEffects(source state.ObjID) {
	if source == 0 {
		return
	}
	kept := e.continuous[:0]
	changed := false
	for _, ce := range e.continuous {
		if ce.Source == source && ce.ImprintOnHost {
			changed = true
			continue
		}
		kept = append(kept, ce)
	}
	if !changed {
		return
	}
	e.continuous = kept
	e.continuousVersion++
}

// nextTurnFor returns the turn number of the next turn (strictly after the
// current one) whose active player is p -- i.e. p's NEXT turn, the
// controller's-next-turn boundary of an UntilYourNextTurn effect.
// It walks the alive rotation from the current active player and returns 0
// (no turn boundary) if p is not alive, bounding the walk so an eliminated
// controller cannot spin the loop forever: at most AliveCount successors are
// distinct alive seats, so a full cycle without hitting p proves p is gone.
// ContinuousNamed reports whether an ACTIVE continuous effect registered by
// p carries the given name (an Effect's Name$): the ask effEffect's
// Stackable$ False dedup makes before it would register a second copy of the
// same named effect (Wrenn and Six's emblem). Scans active() so an expired
// effect never blocks a fresh registration.
func (e *Engine) ContinuousNamed(p state.PlayerID, name string) bool {
	for _, ce := range e.active() {
		if ce.Controller == p && ce.Name == name {
			return true
		}
	}
	return false
}

func (e *Engine) nextTurnFor(p state.PlayerID) int32 {
	alive := e.G.AliveCount()
	t := e.G.Turn
	q := e.G.Active
	for i := 0; i < alive; i++ {
		q = e.G.NextAlive(q)
		t++
		if q == p {
			return t
		}
	}
	return 0
}

// EndOfTurnCleanup drops every "until end of turn" effect (CR 514.2), and
// every turn-boundary effect whose expiry turn is the one now ending. Called
// from rules/combat.go's cleanupStep, which runs it on entry to the cleanup
// step.
func (e *Engine) EndOfTurnCleanup() {
	e.expireControl(controlAtCleanup)
	e.reconcileControlStatics()
	kept := e.continuous[:0]
	// expiredClones collects the clone UNITS whose LCopy marker this cleanup
	// drops, so the object's CopyFace basis can be settled after the kept
	// list is rewritten (task api-clone).
	//
	// A unit is keyed by (become object, expiry moment) rather than by the
	// become object alone: several clone units may be live on ONE permanent
	// (Mirage Mirror activated twice, a permanent copy plus a temporary one),
	// and dropping them all because one expired both wipes a still-live
	// unit's modifiers and destroys its copy. Two units that share the whole
	// key expire at the same instant by construction, so grouping by it can
	// never separate a marker from its own siblings nor merge two units whose
	// lifetimes differ.
	var expiredClones []cloneExpiry
	dropClone := func(ce ContinuousEffect) {
		k := cloneExpiryOf(ce)
		for _, seen := range expiredClones {
			if seen == k {
				return
			}
		}
		expiredClones = append(expiredClones, k)
	}
	for _, ce := range e.continuous {
		// A Permanent one-shot survives cleanup (CR 611.2a). An LCopy
		// marker, however, is never left to the generic rules alone: an
		// UntilUnattached copy has no ordinary expiry field and must be
		// tested live.
		if ce.Layer == LCopy && ce.CloneTarget != 0 &&
			strings.EqualFold(strings.TrimSpace(ce.Duration), "untilunattached") {
			if o := e.G.Obj(ce.CloneTarget); o == nil || o.AttachedTo == 0 {
				dropClone(ce)
				continue
			}
		}
		if ce.Permanent {
			kept = append(kept, ce)
			continue
		}
		if ce.UntilEOT {
			if ce.Layer == LCopy && ce.CloneTarget != 0 {
				dropClone(ce)
			}
			continue
		}
		if strings.EqualFold(strings.TrimSpace(ce.Duration), "untilendofcombat") {
			// CR 511.2: until-end-of-combat is an expired lifetime by the time
			// this turn's cleanup runs, so it is reclaimed here rather than
			// lingering in e.continuous forever.
			if ce.Layer == LCopy && ce.CloneTarget != 0 {
				dropClone(ce)
			}
			continue
		}
		if ce.UntilTurn != 0 && ce.UntilTurn == e.G.Turn {
			if ce.Layer == LCopy && ce.CloneTarget != 0 {
				dropClone(ce)
			}
			continue
		}
		kept = append(kept, ce)
	}
	if len(expiredClones) > 0 {
		// Drop the whole clone unit: the marker's sibling modifier effects
		// go with it. The match is the FULL unit key, not the become object,
		// so a second clone unit still live on the same permanent keeps its
		// own modifiers (the two-overlapping-clones defect).
		surviving := kept[:0]
		for _, ce := range kept {
			if ce.CloneTarget != 0 && cloneExpiryIn(expiredClones, cloneExpiryOf(ce)) {
				continue
			}
			surviving = append(surviving, ce)
		}
		kept = surviving
	}
	e.continuous = kept
	// Settle each affected object's CopyFace basis AFTER e.continuous is
	// rewritten so the emitted ClonePermanent events cannot re-enter this
	// cleanup's list state (the emit below applies to G.Objs only). Each
	// settle is one event, so the hash chain records the expiry exactly as it
	// records the copy.
	e.settleExpiredClones(expiredClones)
	// Bump the version for the same reason AddContinuous does: the cache is
	// keyed on continuousVersion, and this in-place rewrite (which emits no
	// event and moves no log head) drops every UntilEOT pump and every
	// expired UntilTurn effect. Without the bump, a stale active() cache
	// would keep reporting a dead pump's P/T.
	e.continuousVersion++
}

// cloneExpiry identifies ONE clone unit (task api-clone): the permanent that
// became a copy, together with the moment that copy's lifetime ends. A single
// permanent may carry several live clone units at once -- Mirage Mirror
// activated twice in a turn, or a permanent copy under a temporary one -- and
// every effect a unit registers (the layer-1 LCopy marker and its layer-4/5/6/7
// modifier siblings) is registered with the SAME lifetime fields, so this key
// separates the units without any per-unit identifier riding the effects.
//
// Two units that share the whole key expire at the same instant, so treating
// them as one is behaviourally identical; two units whose lifetimes differ
// differ in at least one field, so one can never drop the other.
type cloneExpiry struct {
	Target    state.ObjID
	Duration  string
	UntilEOT  bool
	UntilTurn int32
}

func cloneExpiryOf(ce ContinuousEffect) cloneExpiry {
	return cloneExpiry{Target: ce.CloneTarget,
		Duration:  strings.ToLower(strings.TrimSpace(ce.Duration)),
		UntilEOT:  ce.UntilEOT,
		UntilTurn: ce.UntilTurn}
}

func cloneExpiryIn(keys []cloneExpiry, k cloneExpiry) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}

// settleExpiredClones rewrites the CopyFace basis of every permanent whose
// clone units this cleanup just dropped from e.continuous.
//
// The basis is a SINGLE field on the object (state.Object.CopyFace) while a
// permanent may carry several clone units, so an expiry cannot simply clear
// it: the object must be re-based onto whichever unit is still live. CR
// 613.1a applies copy effects in timestamp order, so the survivor that wins
// is the highest-timestamp LCopy marker left for that object; with none left
// the basis is cleared, which is the single-unit case and therefore emits
// exactly the event stream this cleanup emitted before overlapping units were
// modelled (heads unmoved for every game with at most one copy per object).
//
// The re-base is one ClonePermanent naming the survivor's own source, name
// and GainThisAbility$ rider, so it goes through events.Apply like every
// other state change and a replay derives the identical face.
func (e *Engine) settleExpiredClones(expired []cloneExpiry) {
	if len(expired) == 0 {
		return
	}
	// Deterministic order: the expiry keys are collected in e.continuous scan
	// order, and each object is settled once, on its first appearance.
	var done []state.ObjID
	for _, k := range expired {
		if k.Target == 0 || objIDIn(done, k.Target) {
			continue
		}
		done = append(done, k.Target)
		var survivor *ContinuousEffect
		for i := range e.continuous {
			ce := &e.continuous[i]
			if ce.Layer != LCopy || ce.CloneTarget != k.Target {
				continue
			}
			if survivor == nil || ce.Timestamp > survivor.Timestamp {
				survivor = ce
			}
		}
		// Clear first, unconditionally. With no survivor that is the whole
		// settle (the single-unit case, byte-identical to the pre-overlap
		// build). With one, it puts the object back on its PRINTED face
		// before the re-base, which is what the survivor's copy is taken
		// against -- notably GainThisAbility$, whose fold appends the become
		// object's own current face abilities and would otherwise append the
		// EXPIRING copy's.
		e.emit(events.Event{Kind: events.ClonePermanent, Obj: k.Target})
		if survivor == nil {
			continue
		}
		ev := events.Event{Kind: events.ClonePermanent, Obj: k.Target,
			IDs: []state.ObjID{survivor.CloneSource}, Player: survivor.Controller,
			Text: survivor.CloneName}
		if survivor.CloneGainThisAbility {
			ev.Counter = "gain-this-ability"
		}
		e.emit(ev)
	}
}

// effectMoveSweep is the move-driven lifetime of Effect-created continuous
// effects, run from Engine.emit after every MoveZone has been applied:
//
//   - ForgetOnMoved$ <zone> (Atsushi's, Rakdos's, Opposition Agent's may-play
//     effects, Incinerate's CantRegenerate): a remembered card that moved
//     FROM that zone leaves the effect's Remembered set — "you may play
//     those cards for as long as they remain exiled" ends the moment the
//     played card leaves exile — so the grant's Affected$ Card.IsRemembered
//     list follows what the effect actually holds.
//   - ExileOnMoved$ <zone> (Vines of Vastwood's blinked target, Party
//     Thrasher's chosen card): a remembered card that moved FROM that zone
//     ENDS the whole effect — Forge's "the effect is exiled".
//
// Zone names parse through the shared effects.ParseZone; an unparseable name
// can never match, so the effect simply never sweeps — the honest no-op for
// a value this build cannot read. Like EndOfTurnCleanup this is an in-place
// rewrite of e.continuous that emits no event and moves no log head; a
// replay rebuilds it by re-executing the same registrations against the same
// moves, so it reproduces byte-identically.
func (e *Engine) effectMoveSweep(ev events.Event) {
	if len(e.continuous) == 0 {
		return
	}
	kept := e.continuous[:0]
	changed := false
	for _, ce := range e.continuous {
		// A clone unit -- the layer-1 LCopy marker and every sibling modifier
		// effect, all carrying CloneTarget -- ends the instant the become
		// object leaves the battlefield (CR 400.7: it is a new object and its
		// CopyFace basis has already been cleared by Move). Dropping the whole
		// unit here is the structural owner of a clone's source-leaves
		// lifetime: without it a permanent copy's modifiers would keep applying
		// to a re-entered object and e.continuous would grow unbounded. The
		// expiring durations (UntilEOT / UntilTurn / until-combat /
		// until-unattached) are still handled by EndOfTurnCleanup; this sweep
		// adds the leave-the-battlefield case those branches cannot see.
		if ce.CloneTarget != 0 {
			if o := e.G.Obj(ce.CloneTarget); o == nil || o.Zone != state.ZBattlefield {
				changed = true
				continue
			}
		}
		forget, exile := ce.ForgetOnMoved, ce.ExileOnMoved
		if forget != "" && effects.ParseZone(forget) == ev.From && objIDIn(ce.Remembered, ev.Obj) {
			ce.Remembered = objIDWithout(ce.Remembered, ev.Obj)
			changed = true
		}
		if exile != "" && effects.ParseZone(exile) == ev.From && objIDIn(ce.Remembered, ev.Obj) {
			changed = true
			continue // the effect ends: not kept
		}
		kept = append(kept, ce)
	}
	if !changed {
		return
	}
	e.continuous = kept
	e.continuousVersion++
}

// effectCastSweep is the cast-driven lifetime of Effect-created continuous
// effects carrying ForgetOnCast$ (task param:api:Effect.ForgetOnCast): the
// first qualifying spell cast ENDS the whole effect -- Marshland
// Bloodcaster's "Rather than pay the mana cost of the NEXT spell you cast
// this turn", the one-cast cascade grants (Dark Apostle, Bigger on the
// Inside, Sloppity Bilepiper, World War Hulk), Kaza/Maelstrom Muse/
// Elminster's one-shot reduction. The spec is a card spec over the cast
// spell, You-relative to the effect's controller (the oracle's "spell YOU
// cast"), evaluated with the same machinery the other Effect specs use
// (MatchesSpecCtx against the effect's own source/controller context and
// remembered set). A cast that only targets nothing (a CastInfo-less land
// play never reaches this path: lands are never put on the stack) and a
// spell the spec does not name (an opponent's cast, a creature spell under
// a noncreature-only rider) leave the grant standing.
//
// Run from payCast's fireDeferredCastTrigger -- the deferred re-walk of the
// cast's held PutOnStack, AFTER payment -- so the sweep's timing is the
// completed cast: an ABORTED proposal (one reversed before payment, CR
// 733.1) never consumes the grant, while a completed cast -- even one later
// countered, which CR 601.2i still counts as cast -- does. Like
// effectMoveSweep this is an in-place rewrite of e.continuous that emits no
// event and moves no log head; a replay rebuilds it by re-executing the
// same registrations against the same casts, so it reproduces
// byte-identically.
func (e *Engine) effectCastSweep(ev events.Event) {
	if len(e.continuous) == 0 {
		return
	}
	kept := e.continuous[:0]
	changed := false
	for _, ce := range e.continuous {
		spec := strings.TrimSpace(ce.ForgetOnCast)
		if spec == "" {
			kept = append(kept, ce)
			continue
		}
		sc := e.specCtx(ce.Source, ce.Controller)
		for _, r := range ce.Remembered {
			sc.Remembered = append(sc.Remembered, state.Target{Obj: r})
		}
		if effects.MatchesSpecCtx(e.G, spec, ev.Obj, sc) {
			changed = true
			continue // the effect ends: not kept
		}
		kept = append(kept, ce)
	}
	if !changed {
		return
	}
	e.continuous = kept
	e.continuousVersion++
}

// effectCounterSweep is the counter-driven lifetime of Effect-created
// continuous effects (task vow1; ForgetCounter$), run from Engine.emit after
// every CounterChange has been applied: a remembered card whose count of the
// named kind reached zero after that removal leaves the effect's Remembered
// set -- Promise of Loyalty's "for as long as it has a vow counter on it",
// Quicksilver Fountain's FLOOD, Obsidian Fireheart's BLAZE (18 corpus
// carriers). The measured semantics this build pins: a count that DROPS
// without reaching zero keeps the card (a multi-countered card loses the
// restriction only when its LAST such counter goes), and an ADDITION never
// forgets anything. Like effectMoveSweep this is an in-place rewrite of
// e.continuous that emits no event and moves no log head; a replay rebuilds
// it by re-executing the same registrations against the same counter events,
// so it reproduces byte-identically.
func (e *Engine) effectCounterSweep(ev events.Event) {
	if ev.Amount >= 0 || len(e.continuous) == 0 {
		return
	}
	kept := e.continuous[:0]
	changed := false
	for _, ce := range e.continuous {
		if ce.ForgetCounter != "" && ce.ForgetCounter == ev.Counter && objIDIn(ce.Remembered, ev.Obj) {
			if o := e.G.Obj(ev.Obj); o == nil || o.Counter(ev.Counter) == 0 {
				ce.Remembered = objIDWithout(ce.Remembered, ev.Obj)
				changed = true
			}
		}
		kept = append(kept, ce)
	}
	if !changed {
		return
	}
	e.continuous = kept
	e.continuousVersion++
}

// objIDIn reports whether ids holds id.
func objIDIn(ids []state.ObjID, id state.ObjID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// objIDWithout returns ids without the first occurrence of id.
func objIDWithout(ids []state.ObjID, id state.ObjID) []state.ObjID {
	for i, x := range ids {
		if x == id {
			out := make([]state.ObjID, 0, len(ids)-1)
			out = append(out, ids[:i]...)
			out = append(out, ids[i+1:]...)
			return out
		}
	}
	return ids
}

// isCombatStep reports whether s is one of the combat phase's five steps
// (begin-combat through end-combat). UntilEndOfCombat effects (CR 511.2) are
// active for exactly this span and expire the moment play leaves end combat.
func isCombatStep(s state.Step) bool {
	return s >= state.StepBeginCombat && s <= state.StepEndCombat
}

// active returns the effects that still exist, sorted into CR 613 order:
// layer, then sublayer, then timestamp. Ties within a (layer, sublayer,
// timestamp) triple — two effects created in the same AddContinuous batch
// without distinct timestamps — keep the order they were registered in,
// because sort.SliceStable never reorders equal elements; that registration
// order is itself deterministic (single goroutine, no map iteration), so the
// whole sort is reproducible run to run and safe for replay.
//
// Effects whose source has left the battlefield are dropped, which is what
// makes a lord's static bonus vanish the instant the lord dies. An effect
// marked UntilEOT is different: it is a one-shot pump that already resolved
// (Giant Growth), so it outlives its source and is only removed by
// EndOfTurnCleanup.
func (e *Engine) active() []ContinuousEffect {
	e.activeDepth++
	defer func() { e.activeDepth-- }()
	// Cached hit: derived only reads the returned slice, never mutates it, so
	// every Derived call of a board build shares this one sorted list. The
	// key is the log head plus the continuous-mutation version; a mismatch
	// means something the list depends on changed and the cache is stale.
	if e.activeEpoch == len(e.L.Events) && e.activeVersion == e.continuousVersion {
		return e.activeBuf
	}
	e.activeEpoch = len(e.L.Events)
	e.activeVersion = e.continuousVersion
	buf := e.activeBuf[:0]
	if e.activeDepth > 1 {
		// Re-entrant (a nested Derived mid-rebuild): own a private list rather
		// than overwrite the outer call's result mid-range. Same guard Task A2
		// uses for forEachObject. (This path is effectively unreachable — a
		// Derived call never emits an event, so the epoch cannot move mid-
		// range — but it keeps the buffer discipline airtight.
		buf = nil
	}
	// Duration-honouring expiry. A Permanent one-shot lasts until the end of
	// the game (CR 611.2a) regardless of where its source went; an
	// UntilEndOfCombat one-shot lasts only through the combat phase (CR
	// 511.2), so it is kept while the step is a combat step and dropped the
	// moment play moves past end combat. These take precedence over the
	// UntilEOT/source-leaves rules below, which model the other two
	// lifetimes.
	for _, ce := range e.continuous {
		if ce.Permanent {
			buf = append(buf, ce)
			continue
		}
		if strings.EqualFold(strings.TrimSpace(ce.Duration), "untilendofcombat") {
			if isCombatStep(e.G.Step) {
				buf = append(buf, ce)
			}
			continue
		}
		if ce.UntilEOT {
			buf = append(buf, ce)
			continue
		}
		if ce.UntilTurn != 0 {
			// A turn-boundary effect outlives its source (a one-shot spell is
			// already gone) and is active through its own expiry turn, dropped
			// only by EndOfTurnCleanup when e.G.Turn reaches UntilTurn. Keep it
			// while the current turn is at or before that boundary; the
			// `<=` is the guard that keeps an effect from lingering if a
			// cleanup were ever skipped.
			if e.G.Turn <= ce.UntilTurn {
				buf = append(buf, ce)
			}
			continue
		}
		if o := e.G.Obj(ce.Source); o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		buf = append(buf, ce)
	}
	// The static-derived effects come from the memoized scan (see
	// Engine.staticContinuous): refreshed once per emitted event, not per
	// Derived call, so a board-wide scan does not dominate the hottest path.
	// A version-only rebuild (EndOfTurnCleanup dropping an UntilEOT pump) is a
	// subset of the rebuild condition that leaves the battlefield permanent
	// set, and therefore the static memo, untouched — so the two are checked
	// independently exactly as before.
	if e.staticEpoch != len(e.L.Events) {
		e.staticEpoch = len(e.L.Events)
		e.staticContinuous = e.staticEffects(e.staticContinuous)
	}
	buf = append(buf, e.staticContinuous...)
	sort.SliceStable(buf, func(i, j int) bool {
		if buf[i].Layer != buf[j].Layer {
			return buf[i].Layer < buf[j].Layer
		}
		if buf[i].Sub != buf[j].Sub {
			return buf[i].Sub < buf[j].Sub
		}
		if buf[i].Timestamp != buf[j].Timestamp {
			return buf[i].Timestamp < buf[j].Timestamp
		}
		// A full tie inside layer 6 between an ability-REMOVING effect and an
		// ability-granting one (a static line carrying both RemoveAllAbilities$
		// True and AddKeyword$ -- Darksteel Mutation, Deep Freeze, Stasis Field,
		// Spider-Man No More; measured 4 corpus files) applies removal first:
		// the oracle's "loses all OTHER abilities" grants after stripping (CR
		// 613.1f's removal-then-grant reading of a simultaneous pair). Without
		// this tie-break the stable sort keeps the scanner's emission order and
		// the removal wipes the very grant on its own line. Timestamps still
		// dominate: a LATER removal (Humility entering after) still wipes an
		// earlier grant.
		if buf[i].Layer == LAbilities && buf[i].RemoveAbilities != buf[j].RemoveAbilities {
			return buf[i].RemoveAbilities
		}
		return false
	})
	if e.activeDepth <= 1 {
		// Keep the grown, sorted buffer on the Engine for the next build or
		// cache hit; a re-entrant build's private buffer is discarded on return.
		e.activeBuf = buf
	}
	return buf
}

// faceDownBasis is CR 708.5's synthetic printed face for a face-down
// battlefield permanent: a vanilla 2/2 creature. Only exported fields are
// read off it (derivedScalarFrom pins the 2/2 base itself, since Face's
// parsed P/T is unexported); nothing writes to it.
var faceDownBasis = &cards.Face{Types: []string{"Creature"}}

// faceDownPrintedHides is the one CR 708.8 gate every printed-face scan
// shares: while a battlefield object is face down, its printed abilities,
// triggers and statics do not exist. The ability-offer loop, the
// mana-ability collector, the trigger scan and both static scans all
// consult it, so no printed face of a manifested card can leak into any
// offer or queue while it is face down.
func (e *Engine) faceDownPrintedHides(o *state.Object) bool {
	return o != nil && o.FaceDown && o.Zone == state.ZBattlefield
}

// typeCharacteristics applies layer 4 before anything that tests a type. The
// accumulated types-so-far list passed into the shared effects filter is what
// lets a later effect select a creature made a Goblin by an earlier layer-4
// effect rather than looking back at its printed face. atStack is
// derivedWith's zone override for AffectedZone$ Stack grants; the zero value
// reads the object's live zone.
func (e *Engine) typeCharacteristics(id state.ObjID, atStack state.Zone) []string {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return nil
	}
	// CR 708.5: a face-down battlefield permanent's type set is exactly
	// {Creature} -- its printed types do not exist while it is face down
	// (even a manifested land) -- unless a ChangeZone FaceDownSetType$
	// replaced the set (Yedora's face-down Forest). That set is the BASE the
	// layer-4 walk below then modifies like any other type set (CR 613.1c):
	// a Maskwood Nexus granting every creature type reaches a manifested
	// 2/2 exactly as it reaches a face-up creature, while the printed face
	// stays hidden (no printed word reappears merely from a type grant).
	base := o.Face().Types
	if o.FaceDown && o.Zone == state.ZBattlefield {
		base = o.FaceDownTypeWords()
	}
	zone := o.Zone
	if atStack != 0 {
		zone = atStack
	}
	// Fast path: with no layer-4 effect active anywhere the derived list IS
	// the printed list. Returning the face slice directly (every caller only
	// reads it) keeps the common game -- no Animate/type-granting static in
	// play -- allocation-free; the legal-actions pass reaches here through
	// HasKeyword's Derived read, and the cost/action-statics hotspot pins
	// measure that pass.
	anyLType := false
	for _, ce := range e.active() {
		if ce.Layer == LType {
			anyLType = true
			break
		}
	}
	if !anyLType {
		return reconfigureTypeSwitch(o, bestowedTypeSwitch(o, base))
	}
	ty := append([]string(nil), base...)
	for _, ce := range e.active() {
		if ce.Layer != LType || !e.matchesWithTypes(ce, id, ty, atStack) {
			continue
		}
		if ce.AffectedZone != "" && !ce.MayPlay {
			if zones, all, ok := effects.ParseZones(ce.AffectedZone); !ok || (!all && !slices.Contains(zones, zone)) {
				continue
			}
		}
		if ce.RemoveCardTypes {
			// RemoveCardTypes$ keeps only the SUPERTYPES: a subtype is tied to
			// its card type (CR 205.2-family), so losing the card type loses
			// its subtypes, and the flat type list cannot attribute a subtype
			// word to a surviving type. Both flags together are therefore
			// "everything but supertypes" -- Darksteel Mutation's oracle.
			kept := ty[:0]
			for _, t := range ty {
				if isSupertype(t) {
					kept = append(kept, t)
				}
			}
			ty = kept
		}
		if ce.RemoveCreatureTypes {
			kept := ty[:0]
			for _, t := range ty {
				if !isCreatureSubtype(t) {
					kept = append(kept, t)
				}
			}
			ty = kept
		}
		if ce.RemoveLegendary {
			// NonLegendary$ True (CR 205.4's supertype): drop only the
			// Legendary word, leaving every other supertype (Basic, Snow,
			// World, Ongoing) in place -- distinct from RemoveCardTypes,
			// which keeps supertypes and drops everything else.
			kept := ty[:0]
			for _, t := range ty {
				if !strings.EqualFold(t, "Legendary") {
					kept = append(kept, t)
				}
			}
			ty = kept
		}
		ty = append(ty, ce.AddTypes...)
		if ce.AddAllCreatureTypes {
			ty = appendAllCreatureTypes(ty)
		}
	}
	return reconfigureTypeSwitch(o, bestowedTypeSwitch(o, ty))
}

// appendAllCreatureTypes materialises the layer-4 "all creature types"
// grant (CR 613.1c alongside AddTypes) into the walk's type list: every
// creature-subtype word the shared CreatureTypeWords vocabulary knows, in
// sorted (deterministic) order. Duplicates of a word the printed face or an
// earlier effect already carry are harmless -- every consumer reads the
// list with EqualFold scans or Contains -- so the helper does not pay for
// a dedupe pass. The effects filter's type predicates (hasTypeCtx) answer
// every creature-subtype predicate and base from this list through
// ExtraTypes, exactly as Changeling's intrinsic CDA is answered through
// hasType.
func appendAllCreatureTypes(types []string) []string {
	for _, w := range effects.CreatureTypeWordList() {
		types = append(types, w)
	}
	return types
}

// bestowedTypeSwitch applies CR 702.114e's type switch to a DERIVED type
// list: a bestowed card attached to a creature is an Aura, not a creature --
// the printed "Enchantment Creature" pair loses its Creature half and gains
// Aura -- and an unattached bestowed card (or anything not bestowed) keeps
// the list unchanged, returning the SAME slice so the common game stays
// byte-identical and allocation-free. Derived live state
// (state.Object.BestowedAttached), never a stored marker, so every replay
// derives the switch identically. Creature SUBTYPES deliberately stay: the
// subtype words are inert on an Aura in every filter this engine evaluates
// (no Aura filter reads "Archon"), and stripping them would widen the diff
// into every subtype-affected static.
func bestowedTypeSwitch(o *state.Object, types []string) []string {
	if !o.BestowedAttached() {
		return types
	}
	out := make([]string, 0, len(types)+1)
	for _, t := range types {
		if t == "Creature" {
			continue
		}
		out = append(out, t)
	}
	return append(out, "Aura")
}

// reconfigureTypeSwitch applies CR 702.150c's switch to a DERIVED type
// list: a Reconfigure card attached to a creature is not a creature -- the
// printed "Artifact Creature Equipment <subtype>" list loses only its
// Creature half and keeps Equipment/Artifact and the subtypes (the same
// deliberate keep-subtypes narrowing bestowedTypeSwitch practises: the
// subtype words are inert on a non-creature in every filter this engine
// evaluates, and stripping them would widen the diff into every
// subtype-affected static). An unattached reconfigure card (or anything
// not printed with the keyword) keeps the list unchanged, returning the
// SAME slice so the common game stays byte-identical and allocation-free.
// A face-down battlefield permanent keeps its CR 708.5 set: its printed
// face (and with it the Reconfigure keyword the switch keys on) does not
// exist while face down, so the switch must not strip Creature from a
// manifested reconfigure card's vanilla 2/2.
func reconfigureTypeSwitch(o *state.Object, types []string) []string {
	if !o.ReconfiguredAttached() || (o.FaceDown && o.Zone == state.ZBattlefield) {
		return types
	}
	out := make([]string, 0, len(types))
	for _, t := range types {
		if t == "Creature" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (e *Engine) matchesWithTypes(ce ContinuousEffect, id state.ObjID, types []string, atStack state.Zone) bool {
	// The cast-provenance qualifiers (castprov1/2/3 — the_twelfth_doctor's
	// `Affected$ Card.YouCtrl+!wasCastFromYourHand`, quandrix_the_proof's
	// `Instant.wasCastByYou+wasCastFromYourHand`) are split out before the
	// filter match, through the combined entry point; its Contains guard is
	// the early-out, so every Affected$ spec without the tokens costs three
	// Contains calls on this shared hot path.
	affects, ok := e.castProvenanceAdmitsWindow(ce.Affects, id, ce.Controller, atStack != 0)
	if !ok {
		return false
	}
	// ExtraTypes is the walk's types-so-far list for THIS object: a later
	// layer-4 effect selects a creature an earlier one made a Goblin, and a
	// layer-7 lord's Affected$ sees the derived type. A value slice, not a
	// callable, keeps the context stack-allocated on this hot path.
	sc := e.specCtx(ce.Source, ce.Controller)
	sc.AsStack = atStack != 0
	sc.ExtraTypes = types
	// The compiled predicate sidecar answers type and colour predicates
	// against the PRINTED face (effects/compiled_predicate.go's
	// matchesCompiledBase/matchesCompiledTerm call hasType/ColorsOf), so it
	// cannot see ExtraTypes. Leaving it in place here would let it return
	// PredicateNo before the textual oracle -- the one oracle whose hasTypeCtx
	// reads ExtraTypes -- ever runs, so a layer-4 grant would silently miss
	// every object whose printed face does not already carry the queried type
	// (a manifested Forest under Maskwood Nexus is the measured case). The
	// sidecar is a pure optimisation that falls back to the text path
	// whenever it is unsure; clearing it for a derived-type match makes that
	// fallback unconditional, so every compiled spec is judged by the
	// ExtraTypes-aware oracle and no future compiled spec can miss the
	// synthetic face-down base (CR 708.5) either.
	sc.PredicatePrograms = nil
	return effects.MatchesSpecCtx(e.G, affects, id, sc)
}

// derivedScalar returns only an object's derived power and toughness — the
// subset of Derived that combat, legal, cast, trigger and bot predicates read
// constantly (and, through the Chars interface, every projected view). It
// never builds the keyword/type slices Derived carries, and its effect list
// comes from active()'s cached, buffer-reused build. Its P/T is identical to
// what the full Derived computes because it first derives layer-4 types and
// hands them to every filter used by a layer-7 effect. Layer-6 keywords
// cannot affect P/T applicability in the supported grammar; layer-4 changes
// can, and typeCharacteristics below is deliberately shared rather than
// skipped. This remains a saving because the keyword slice itself is not
// needed for scalar reads.

func (e *Engine) derivedScalar(id state.ObjID) (power, toughness int32) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return 0, 0
	}
	f := o.Face()
	return e.derivedScalarFrom(id, o, f, e.active())
}

func (e *Engine) derivedScalarFrom(id state.ObjID, o *state.Object, f *cards.Face, active []ContinuousEffect) (power, toughness int32) {
	if o != nil && o.FaceDown && o.Zone == state.ZBattlefield {
		// CR 708.5's base: a face-down battlefield permanent is a 2/2
		// creature; its printed P/T and any printed characteristic-defining
		// ability do not exist while it is face down. A FaceDownSetType$ that
		// does not include Creature derives 0/0 (Yedora's Forest land), and a
		// FaceDownPower$/FaceDownToughness$ pair overrides the 2/2 default
		// (Magar's 3/3). Layer-7 effects on top still apply in the walk below.
		power, toughness = 2, 2
		if !o.EffectiveIsCreature() {
			power, toughness = 0, 0
		}
		if o.FaceDownHasPT {
			power, toughness = o.FaceDownPower, o.FaceDownToughness
		}
	} else {
		power, toughness = int32(f.Power()), int32(f.Toughness())
		// Layer 7a (CR 613.4a): the object's own characteristic-defining ability
		// (CharacteristicDefining$ True) sets the base P/T that every later
		// layer applies on top of, in EVERY zone (CR 604.3/208.2 -- Master of
		// Etherium is its artifact count in hand and graveyard too, which the
		// battlefield-only static scan cannot express). Applied before the
		// effect walk below, so a layer-7b set still overrides it and a 7c
		// modify still stacks on it. staticEffects withholds the resolvable CDAs
		// from its emission exactly so this read is not applied twice.
		if p, tp, hp, ht := e.cdaSetPT(o); hp || ht {
			if hp {
				power = p
			}
			if ht {
				toughness = tp
			}
		}
	}
	// typeCharacteristics is 837910f4's layer-4-aware type derivation; the
	// active list comes in as a parameter (230574a2's plumbing) because
	// active() is a cached, idempotent read — same slice, no recomputation.
	types := e.typeCharacteristics(id, 0)
	for _, ce := range active {
		if ce.Layer != LPT {
			continue
		}
		if !e.matchesWithTypes(ce, id, types, 0) {
			continue
		}
		switch ce.Sub {
		case SubCDA, SubSet:
			if ce.HasSet {
				if ce.StaticSet {
					// A static can set just power or just toughness. Its omitted
					// parameter must leave the printed/earlier-layer value alone,
					// rather than treating the empty expression as numeric zero.
					if ce.SetPowerPresent {
						power = ce.SetPower
						if ce.SetPowerExpr != "" {
							power = e.staticAmount(ce, ce.SetPowerExpr)
						}
					}
					if ce.SetToughnessPresent {
						toughness = ce.SetToughness
						if ce.SetToughnessExpr != "" {
							toughness = e.staticAmount(ce, ce.SetToughnessExpr)
						}
					}
				} else {
					// Effects created through the original numeric API (Animate
					// and direct ContinuousEffect callers) predate per-component
					// presence flags and deliberately retain their paired setter
					// semantics.
					power, toughness = ce.SetPower, ce.SetToughness
				}
			}
		case SubModify:
			addPower, addToughness := ce.AddPower, ce.AddToughness
			if ce.AddPowerExpr != "" {
				addPower = e.staticAmount(ce, ce.AddPowerExpr)
			}
			if ce.AddToughnessExpr != "" {
				addToughness = e.staticAmount(ce, ce.AddToughnessExpr)
			}
			power = addPT(power, addPower)
			toughness = addPT(toughness, addToughness)
		}
	}
	// 7d: counters apply after every other layer-7 effect (CR 613.4).
	if n := o.Counter("P1P1"); n != 0 {
		power += n
		toughness += n
	}
	if n := o.Counter("M1M1"); n != 0 {
		power -= n
		toughness -= n
	}
	return power, toughness
}

// Derived computes an object's current characteristics: printed values from
// its face, then every applicable continuous effect in layer order, then
// layer 7d counters last. A malformed or missing object degrades to the
// zero Derived rather than panicking — layer inputs ultimately come from
// parsed card text, and a nonexistent ObjID or an ability/token object with
// no Face() must never crash the match goroutine.
//
// The Keywords and Types slices alias the Engine's scratch buffers
// (derivedKW / derivedTypes, engine.go) and are reused across calls: after
// the first call's buffers grow to size they are rewritten, never
// reallocated, so repeated Derived builds are allocation-free. That is only
// sound because every caller treats the returned slices as read-only and
// does not retain them past building its own view — view.Project and
// botpolicy both copy (append([]string(nil), ...)) synchronously, and the
// loops in HasKeyword and protectedFrom only range. Sharing would be wrong
// if a caller held one Derived's slices while calling Derived again (the
// next call would rewrite the shared buffers), so the discipline is
// load-bearing; derivedDepth guards re-entry the way active()'s activeDepth
// guards its cache (a nested Derived mid-build gets private owned buffers
// instead of clobbering the outer build's).
func (e *Engine) Derived(id state.ObjID) Derived {
	return e.derivedWith(id, 0)
}

// Characteristics returns the three derived facts botpolicy projects in one
// pass. Keywords aliases Engine scratch storage exactly as Derived does; a
// caller that keeps it across another characteristics query must copy it.
func (e *Engine) Characteristics(id state.ObjID) (power, toughness int32, keywords []string) {
	d := e.Derived(id)
	return d.Power, d.Toughness, d.Keywords
}

// derivedWith is Derived with an optional ZONE OVERRIDE for the AffectedZone$
// gate: atStack != 0 evaluates the grants against that zone instead of the
// object's live one. The convoke announcement (CR 601.2b) happens while the
// announced spell is still in hand -- the engine pushes it to the stack only
// later in its own cast flow -- so convokeCost/hasCastConvoke evaluate an
// AffectedZone$ Stack grant against ZStack via this override; everything
// else reads the live zone.
func (e *Engine) derivedWith(id state.ObjID, atStack state.Zone) Derived {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return Derived{}
	}
	f := o.Face()
	// CR 708.5: while a battlefield object is face down its printed face
	// does not exist -- the derived basis is a vanilla 2/2 Creature face
	// (derivedScalarFrom pins the 2/2 base; this synthetic face carries no
	// keywords or printed types, and the colour basis below is overridden
	// to none). Layer effects from OTHER permanents still apply on top (an
	// Anthem pumps a manifested 2/2 to 3/3); the printed-face scans never
	// reach here because faceDownPrintedHides gates them all off (CR 708.8).
	faceDown := o.FaceDown && o.Zone == state.ZBattlefield
	if faceDown {
		f = faceDownBasis
	}
	active := e.active()
	power, toughness := e.derivedScalarFrom(id, o, f, active)
	zone := o.Zone
	if atStack != 0 {
		zone = atStack
	}
	e.derivedDepth++
	kw := e.derivedKW
	ty := e.derivedTypes
	if e.derivedDepth > 1 {
		// Re-entrant (a nested Derived mid-build): own private buffers rather
		// than overwrite the outer call's backing arrays mid-range. Same guard
		// Task A2 uses for forEachObject and this file uses for active(). (This
		// path is effectively unreachable — MatchesSpecFrom reads faces, never
		// calls Derived — but it keeps the buffer discipline airtight.)
		kw = nil
		ty = nil
	}
	kw = append(kw[:0], f.Keywords...)
	kw = append(kw, o.IntrinsicKeywords...)
	// CR 122.1b: a marker counter whose kind names a keyword grants that
	// keyword to the permanent it sits on (Forge's CounterKeywordType emits a
	// Mode$ Continuous | AddKeyword$ static, EffectZone$ All). Appended here,
	// ahead of the layer walk, so the grant is a base keyword the layer-6
	// walk then removes or replaces exactly as it would Forge's static -- a
	// RemoveAbilities/RemoveKeywords effect clears it and a later layer-6
	// grant re-adds on top. Iterating o.Counters (a fixed-order slice) keeps
	// this deterministic; cards.CounterKeyword is the single classifier, so
	// every counter-to-keyword read agrees. Order is buttoned by the counter
	// slice, which is append-order stable.
	for _, c := range o.Counters {
		if kwName, ok := cards.CounterKeyword(c.Kind); ok && c.N > 0 {
			kw = append(kw, kwName)
		}
	}
	// CR 708.5's cloak variant: a CLOAKED face-down card is a 2/2 creature
	// with ward {2} -- the ward is part of the cloak status itself, not a
	// printed or granted ability (the printed face does not exist while face
	// down, CR 708.8, and faceDownBasis carries no keywords). Appending it
	// here -- ahead of the layer walk, exactly where a layer-6 grant would
	// land -- is what feeds checkGrantedWardTriggers's derived-keyword scan
	// (rules/trigger_match.go), so targeting a cloaked 2/2 meets the real
	// pay-or-counter ask. Leaving the battlefield clears both flags together
	// (events.Apply's Move reset), so the ward drops with the face-down
	// status.
	if faceDown && o.Cloaked {
		kw = append(kw, "Ward:2")
	}
	// CR 702.157b: a suspected creature has menace. The designation is a
	// status, not an ability, so appending it here -- ahead of the layer
	// walk, exactly where the cloak's status ward lands -- is the same grant
	// shape; leaving the battlefield or another player gaining control
	// clears it (events.Apply's Move and ControlChange folds), so the menace
	// drops with the designation.
	if o.Suspected {
		kw = append(kw, "Menace")
	}
	// Layer 4 runs first through typeCharacteristics (see above), so every
	// later effect's Affected$ filter — and every layer-4 effect's own —
	// sees the derived type list, not the printed face.
	ty = append(ty[:0], e.typeCharacteristics(id, atStack)...)
	// Layer 5's base is the face's colour set (the mana cost, an explicit
	// Colors: line, Devoid-applied). The letters compose in a fixed [5]bool so
	// the layer walk below never touches a map.
	// ColorMaskOf is ColorsOf's compact bitmask (230574a2); the match keeps
	// 837910f4's type-aware wrapper — a bare SpecContext carries no
	// ExtraTypes, so MatchesSpecCtx here would regress to printed types only.
	col := effects.ColorMaskOf(o)
	if faceDown {
		col = 0 // CR 708.5: a face-down permanent has no colours
	}
	for _, ce := range active {
		if !e.matchesWithTypes(ce, id, ty, atStack) {
			continue
		}
		// An AffectedZone$ qualifier on a characteristic grant narrows where
		// the granted characteristics function (Chief Engineer's "Artifact
		// spells you cast have convoke" carries AffectedZone$ Stack, so the
		// grant reaches the spell while it is on the stack and never a copy
		// of the same card sitting in hand). Parse failure stays closed.
		if ce.AffectedZone != "" && !ce.MayPlay {
			if zones, all, ok := effects.ParseZones(ce.AffectedZone); !ok || (!all && !slices.Contains(zones, zone)) {
				continue
			}
		}
		switch ce.Layer {
		case LAbilities:
			// CR 613.1f / 613.4b: an ability-removing effect (Humility)
			// clears the object's printed and earlier-granted keywords before
			// later layer-6 grants re-add anything.
			if ce.RemoveAbilities {
				kw = kw[:0]
			}
			if len(ce.RemoveKeywords) > 0 {
				// CR 613.1f: this effect's own named keywords leave the
				// accumulated list BEFORE its AddKeywords append, so a
				// single effect that both removes and grants (mirage
				// phalanx's RemoveKeywords$ Soulbond | AddKeywords$ Haste)
				// yields the card text's result regardless of how the
				// timestamps order neighbour effects. A keyword is matched
				// by its HEAD (cards.KeywordHead), so a parameterised print
				// is removable by name.
				keptKW := kw[:0]
				for _, k := range kw {
					if !containsKeywordHead(ce.RemoveKeywords, k) {
						keptKW = append(keptKW, k)
					}
				}
				kw = keptKW
			}
			kw = append(kw, ce.AddKeywords...)
		case LType:
			// Already applied in typeCharacteristics above — layer 4 must
			// settle before any filter that tests a type runs.
		case LColor:
			// CR 613.1e: colour-set and colour-add effects apply in timestamp
			// order; an OverwriteColors grant replaces everything so far (an
			// empty set means an overwrite to colourless, the Animate
			// Colors$ Colorless shape), a plain one extends it.
			if ce.OverwriteColors {
				col = 0
			}
			// Letter elements are bounds-checked: state.ContinuousEffect is
			// exported, so a malformed element (empty, or not a WUBRG letter)
			// must be skipped, never an index panic -- a parse path in this
			// walk never crashes the match goroutine.
			for _, l := range ce.AddColors {
				if len(l) == 0 {
					continue
				}
				if i := strings.IndexByte("WUBRG", l[0]); i >= 0 {
					col |= effects.ColorMask(1 << i)
				}
			}
		}
	}
	colors := col.String()
	if e.derivedDepth <= 1 {
		// Keep the grown buffers on the Engine for the next build; a re-entrant
		// build's private buffers are discarded on return.
		e.derivedKW = kw
		e.derivedTypes = ty
	}
	e.derivedDepth--
	return Derived{Power: power, Toughness: toughness, Keywords: kw, Types: ty, Colors: colors}
}

func (e *Engine) Power(id state.ObjID) int32 {
	p, _ := e.derivedScalar(id)
	return p
}
func (e *Engine) Toughness(id state.ObjID) int32 {
	_, t := e.derivedScalar(id)
	return t
}

// HasKeyword matches case-insensitively, like its sibling cards.Face.HasKeyword
// (Ruling T19-b) — every existing call site already goes through that
// case-insensitive comparison, so an exact-match Engine.HasKeyword would have
// been a silent trap for the first caller with non-canonical-cased input.
func (e *Engine) HasKeyword(id state.ObjID, kw string) bool {
	for _, k := range e.Derived(id).Keywords {
		if strings.EqualFold(cardsKeywordHead(k), kw) {
			return true
		}
	}
	return false
}

// derivedKeywordParam is Face.KeywordParam over the object's CURRENT derived
// keyword list (printed plus layer-6 granted), so a keyword a continuous
// effect delivered (Underworld Breach's AddKeyword$ Escape grant, Snapcaster
// Mage's Flashback) is readable exactly where the printed one would be.
func (e *Engine) derivedKeywordParam(id state.ObjID, head string) (string, bool) {
	for _, k := range e.Derived(id).Keywords {
		if strings.EqualFold(cardsKeywordHead(k), head) {
			if i := strings.IndexByte(k, ':'); i >= 0 {
				return strings.TrimSpace(k[i+1:]), true
			}
			return "", true
		}
	}
	return "", false
}

// ToxicValue is the object's total toxic N (CR 702.164), SUMMED over every
// `Toxic:<N>` entry on its CURRENT derived keyword list -- CR 702.164c makes
// multiple toxic instances cumulative (a printed Toxic 2 Ixhel equipped by
// Prosthetic Injector's AddKeyword$ Toxic:1 is toxic 3), so the read cannot
// stop at the first entry the way the singleton derivedKeywordParam helper
// does. A layer-6 grant (the Rat lord, an Aura, an Equipment) is readable
// exactly where the printed K:Toxic line is, the same derived read
// HasKeyword/derivedKeywordParam give every other keyword. Each entry whose
// parameter is absent or not a positive integer contributes 0 (a non-numeric
// N can only be a malformed script, so failing closed to no poison from that
// entry is the conservative direction); the whole read reports 0 when the
// object has no toxic at all.
func (e *Engine) ToxicValue(id state.ObjID) int {
	total := 0
	for _, k := range e.Derived(id).Keywords {
		if !strings.EqualFold(cardsKeywordHead(k), "Toxic") {
			continue
		}
		raw := ""
		if i := strings.IndexByte(k, ':'); i >= 0 {
			raw = strings.TrimSpace(k[i+1:])
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			continue
		}
		total += n
	}
	return total
}

// IsCreature reads the current layer-derived type list. In particular, a
// planeswalker animated by a layer-4 effect is a creature for damage marking,
// even though its printed face is not.
func (e *Engine) IsCreature(id state.ObjID) bool {
	for _, typ := range e.Derived(id).Types {
		if typ == "Creature" {
			return true
		}
	}
	return false
}

// Colors is the object's current layer-5 colour set as WUBRG letters (see
// Derived.Colors); "" is a colourless object. Every rules-side colour read
// about a live object goes through this (objColors below for callers that
// already hold the *state.Object) rather than effects.ColorsOf's face read,
// so an animated manland's granted colours are real everywhere the engine
// consults them -- protection qualities, Fear's black-blocker test, convoke's
// colour contributions, the Count$...$Colors heads.
func (e *Engine) Colors(id state.ObjID) string {
	return e.Derived(id).Colors
}

// objColors is Colors for a caller holding the object rather than the id:
// a battlefield permanent reads its derived (layer-5) colours; anything off
// the battlefield has no continuous characteristics (CR 613.6 -- a spell on
// the stack shows its face's colours) and falls back to the face read,
// which also covers LKI snapshots keyed by an id that may no longer resolve.
func (e *Engine) objColors(o *state.Object) string {
	if o != nil && o.Zone == state.ZBattlefield {
		return e.Colors(o.ID)
	}
	return effects.ColorsOf(o)
}

// cardTypeWords are the card types; supertypeWords the supertypes. Every
// other type word on a face is a subtype, so RemoveCreatureTypes' strip is
// "drop what is neither" -- the same split Forge's own type vocabulary makes.
var (
	cardTypeWords  = []string{"Artifact", "Battle", "Creature", "Enchantment", "Instant", "Land", "Planeswalker", "Sorcery", "Tribal"}
	supertypeWords = []string{"Basic", "Legendary", "Ongoing", "Snow", "World"}
)

// isSupertype reports whether t is a supertype word (the only thing a
// RemoveCardTypes strip keeps: card types and their subtypes go).
func isSupertype(t string) bool {
	for _, w := range supertypeWords {
		if strings.EqualFold(w, t) {
			return true
		}
	}
	return false
}

// isCreatureSubtype reports whether t is a subtype word (a creature type
// under RemoveCreatureTypes' reading): not a card type and not a supertype.
func isCreatureSubtype(t string) bool {
	for _, w := range cardTypeWords {
		if strings.EqualFold(w, t) {
			return false
		}
	}
	for _, w := range supertypeWords {
		if strings.EqualFold(w, t) {
			return false
		}
	}
	return true
}

// containsKeywordHead reports whether the keyword k matches any name in
// names by keyword HEAD (cards.KeywordHead strips a parameter tail), so
// RemoveKeywords$ Protection removes a printed "Protection:..." grant and
// RemoveKeywords$ Soulbond removes the bare keyword.
func containsKeywordHead(names []string, k string) bool {
	head := cards.KeywordHead(k)
	for _, n := range names {
		if strings.EqualFold(cards.KeywordHead(n), head) {
			return true
		}
	}
	return false
}

// RegenerationDisallowed implements effects.Host for the CantRegenerate
// restriction (Task ce1): reports whether an Effect-registered restriction
// forbids id from regenerating. Consulted by effects.ReplaceDestruction, so
// Incinerate's "creature can't be regenerated this turn" actually blocks the
// shield-consumption path instead of being a Note. Scanning e.active() keeps
// the expiry discipline identical to every other continuous effect: a
// this-turn restriction is UntilEOT and is dropped at cleanup, a permanent-
// sourced one disappears when its source leaves the battlefield.
func (e *Engine) RegenerationDisallowed(id state.ObjID) bool {
	for _, ce := range e.active() {
		if ce.Restriction != "CantRegenerate" {
			continue
		}
		if e.restrictionApplies(ce, id) {
			return true
		}
	}
	return false
}

// restrictionBlocksTarget reports whether an Effect-registered CantTarget
// restriction (Vines of Vastwood) prevents the player actor from targeting id
// with a spell or ability. Called from rules/stack.go's askTarget alongside
// the protectedFrom check, so a creature granted "can't be the target of
// spells or abilities your opponents control this turn" is actually withheld
// from the opponent's targeting options.
func (e *Engine) restrictionBlocksTarget(id state.ObjID, actor state.PlayerID) bool {
	for _, ce := range e.active() {
		if ce.Restriction != "CantTarget" {
			continue
		}
		if !e.restrictionApplies(ce, id) {
			continue
		}
		if !e.restrictionActorMatches(ce, actor) {
			continue
		}
		return true
	}
	return false
}

// restrictionApplies reports whether a registered restriction's ValidCard$/
// ValidTarget$ spec selects the object id. A spec containing IsRemembered is
// resolved through the ordinary object matcher with the effect's remembered
// set bound to the SpecContext -- the general filter implements IsRemembered
// (both bare and compound: Card.IsRemembered+Creature keeps both halves),
// which is the dominant shape for Vines/Incinerate; any other spec falls back
// to the same matcher so a restriction that names a quality (CantTarget with
// ValidCard$ Creature, say) still works.
func (e *Engine) restrictionApplies(ce ContinuousEffect, id state.ObjID) bool {
	spec := ce.RestrictParams["ValidCard"]
	if spec == "" {
		spec = ce.RestrictParams["ValidTarget"]
	}
	if spec == "" {
		// The ValidCards$ plural spelling: Forge allows both on a restriction
		// body, and one CanAttackDefender grant (Wakestone Gargoyle's
		// `ValidCards$ Creature.YouCtrl+withDefender`) spells it. Corpus
		// census: no Cant* body carries ValidCards$ without ValidCard$, so
		// the fallback is unreachable for every pre-existing restriction.
		spec = ce.RestrictParams["ValidCards"]
	}
	if spec == "" {
		return len(ce.Remembered) > 0
	}
	sc := e.specCtx(ce.Source, ce.Controller)
	for _, r := range ce.Remembered {
		sc.Remembered = append(sc.Remembered, state.Target{Obj: r})
	}
	return effects.MatchesSpecCtx(e.G, spec, id, sc)
}

// restrictionActorMatches scopes a CantTarget restriction by Activator$:
// Vines of Vastwood's Activator$ Player.Opponent means the restriction only
// bites when the player targeting the creature is an opponent of the effect's
// controller (the caster of Vines). A restriction with no Activator$ applies
// to any actor.
func (e *Engine) restrictionActorMatches(ce ContinuousEffect, actor state.PlayerID) bool {
	spec, ok := ce.RestrictParams["Activator"]
	if !ok {
		return true
	}
	return effects.MatchesPlayerSpec(e.G, spec, actor, ce.Controller)
}

// SacrificeBlocked implements effects.Host for the CantSacrifice restriction
// (task combatrestriction1): reports whether id is forbidden from being
// sacrificed at all — an Effect-registered CantSacrifice restriction (Call for
// Aid's "You can't sacrifice those creatures this turn") or a face
// CantSacrifice static (the simple Card.Self carriers). Consulted at every
// sacrifice candidate choke point (effSacrifice's eligible pool and its
// object-target paths, effSacrificeAll, and the cast/activation/mana/ward/
// unless Sac-cost candidate walks), so a blocked permanent is never offered,
// never asked, and never taken.
//
// forCost (vc-static1) names the call site's provenance: the cost-driven
// callers (the cast/activation/mana/ward/unless Sac-cost candidate walks)
// pass true, the effect-driven ones (effSacrifice, effSacrificeAll) false. A
// face static's ForCost$/ValidCause$ scoping reads the split: ForCost$ False
// lines never restrict a cost sacrifice, and ValidCause$ lines are evaluated
// only on the effect path, where actionCause() names the resolving wrapper.
func (e *Engine) SacrificeBlocked(id state.ObjID, forCost bool) bool {
	for _, ce := range e.active() {
		if ce.Restriction != "CantSacrifice" {
			continue
		}
		if e.restrictionApplies(ce, id) {
			return true
		}
	}
	for _, sv := range e.activeStatics("CantSacrifice") {
		if !effects.CantSacrificeRestrictionParamsReadable(sv.Params) {
			continue
		}
		// vc-static1: the cause-scoping parameters, evaluated before the
		// ValidCard match so an unevaluable shape stays skipped (the
		// permissive direction) instead of blanket-blocking. ForCost$ True
		// restricts only COST sacrifices, and the cost call sites' provenance
		// (the pending cast/activation identity, not actionCause) is not
		// modelled, so those lines stay skipped whole -- recorded in the
		// combatrestriction1 row of AGENTS.md with its two carriers
		// (angel_of_jubilation, yasharn_implacable_earth).
		switch sv.Params["ForCost"] {
		case "True":
			continue
		case "False":
			if forCost {
				continue
			}
		}
		// ValidCause$ names the kind of spell/ability that must be causing
		// the sacrifice. Only the effect-driven call sites (forCost false)
		// have a meaningful actionCause; a cost sacrifice has none, and a
		// ValidCause spec (a Spell/Activated/Triggered kind) can never name a
		// cost anyway, so a cost site skips these lines the same way -- the
		// permissive, oracle-correct direction either way.
		if spec := sv.Params["ValidCause"]; spec != "" {
			if forCost || !e.causeSpecAdmits(spec, sv.Source) {
				continue
			}
		}
		if spec := sv.Params["ValidCard"]; spec != "" &&
			effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(sv.Source, sv.Controller)) {
			return true
		}
	}
	return false
}

// PutCounterBlocked reports whether a counter of kind would be placed on obj
// (object form) or player (player form) is forbidden -- a real CantPutCounter
// restriction static (task cantputcounter1): an Effect-registered one (Melira,
// the Living Cure's "you can't get additional poison counters this turn",
// registered by effEffect from the Effect's StaticAbilities$ NoMorePoison) or
// a face S:Mode$ CantPutCounter static (Solemnity, Melira's Keepers,
// Blightbeetle, Darksteel Angel, Tatterkite, Melira Sylvok Outcast, Phila
// Unsealed). Consulted at the counter-placement choke point in
// rules/replacement.go, BEFORE any AddCounter replacement, so a prohibition
// with no accompanying R:Event$ AddCounter line is still enforced and the
// event is swallowed rather than folded.
//
// Reading (both forms fail closed on the other's event kind): CounterType$
// names the kind (absent = all kinds), ValidPlayer$ scopes the player form,
// ValidCard$/ValidObject$ scopes the object form. An unscoped line blocks
// both forms. Both routes are consulted, mirroring SacrificeBlocked /
// attackBlocked.
func (e *Engine) PutCounterBlocked(kind string, obj state.ObjID, player state.PlayerID, playerForm bool) bool {
	for _, ce := range e.active() {
		if ce.Restriction != "CantPutCounter" {
			continue
		}
		if !counterKindMatches(ce.RestrictParams["CounterType"], kind) {
			continue
		}
		if playerForm {
			if spec := strings.TrimSpace(ce.RestrictParams["ValidPlayer"]); spec != "" {
				if restrictionPlayerSpecMatches(e.G, spec, player, ce.Controller, ce.RememberedPlayers) {
					return true
				}
				continue
			}
			if strings.TrimSpace(ce.RestrictParams["ValidCard"]) != "" || strings.TrimSpace(ce.RestrictParams["ValidObject"]) != "" {
				continue
			}
			return true
		}
		objSpec := ce.RestrictParams["ValidCard"]
		if objSpec == "" {
			objSpec = ce.RestrictParams["ValidObject"]
		}
		if strings.TrimSpace(objSpec) != "" {
			if e.restrictionApplies(ce, obj) {
				return true
			}
			continue
		}
		if strings.TrimSpace(ce.RestrictParams["ValidPlayer"]) != "" {
			continue
		}
		return true
	}
	for _, sv := range e.activeStatics("CantPutCounter") {
		if !effects.CantPutCounterParamsReadable(sv.Params) {
			continue
		}
		if !counterKindMatches(sv.Params["CounterType"], kind) {
			continue
		}
		if playerForm {
			if spec := strings.TrimSpace(sv.Params["ValidPlayer"]); spec != "" {
				if restrictionPlayerSpecMatches(e.G, spec, player, sv.Controller, nil) {
					return true
				}
				continue
			}
			if strings.TrimSpace(sv.Params["ValidCard"]) != "" || strings.TrimSpace(sv.Params["ValidObject"]) != "" {
				continue
			}
			return true
		}
		spec := strings.TrimSpace(sv.Params["ValidCard"])
		if spec == "" {
			spec = strings.TrimSpace(sv.Params["ValidObject"])
		}
		if spec != "" {
			if effects.MatchesSpecCtx(e.G, spec, obj, e.specCtx(sv.Source, sv.Controller)) {
				return true
			}
			continue
		}
		if strings.TrimSpace(sv.Params["ValidPlayer"]) != "" {
			continue
		}
		return true
	}
	return false
}

// counterKindMatches implements a CantPutCounter line's CounterType$ gate: an
// absent kind admits every counter kind, a stated kind matches only the event's
// own, and anything else fails closed.
func counterKindMatches(restriction, kind string) bool {
	restriction = strings.TrimSpace(restriction)
	return restriction == "" || restriction == kind
}

// attackBlocked reports whether creature id is forbidden from being declared
// attacking defender this combat — an Effect-registered CantAttack restriction
// (Call for Aid's "You can't attack that player this turn") or a face
// CantAttack static (the Vow cycle's "can't attack you"). A restriction with
// no Target$ (the "Creatures can't attack." shapes) blocks every defender.
// Consulted at the two (attacker, defender) enforcement points — askAttackers'
// option filter and validateAttackers — and by mustAttackRequired's
// attackPairAvailable gate (CR 508.1d's "if able").
func (e *Engine) attackBlocked(id state.ObjID, defender state.PlayerID) bool {
	for _, ce := range e.active() {
		if ce.Restriction != "CantAttack" {
			continue
		}
		if !e.restrictionApplies(ce, id) {
			continue
		}
		if !restrictionPlayerTargetMatches(e.G, ce.RestrictParams["Target"], defender, ce.Controller, ce.RememberedPlayers) {
			continue
		}
		return true
	}
	for _, sv := range e.activeStatics("CantAttack") {
		if !effects.CantRestrictionParamsReadable(sv.Params) {
			continue
		}
		spec := sv.Params["ValidCard"]
		if spec == "" || !effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(sv.Source, sv.Controller)) {
			continue
		}
		if !restrictionPlayerTargetMatches(e.G, sv.Params["Target"], defender, sv.Controller, nil) {
			continue
		}
		return true
	}
	return false
}

// restrictionPlayerTargetMatches resolves a CantAttack restriction's Target$
// player spec (the defender-side scoping: "can't attack THAT player") against
// the defender under attack. The corpus spells it as a comma-separated list of
// player specs ("You,Planeswalker.YouCtrl" — this build has no
// planeswalker-attack path, so a walker clause is a player spec that matches
// nobody and the You half carries the read); any part matching blocks the
// pair. An absent Target$ applies to every defender.
func restrictionPlayerTargetMatches(g *state.Game, spec string, defender, controller state.PlayerID, rememberedPlayers []state.PlayerID) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return true
	}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if restrictionPlayerSpecMatches(g, part, defender, controller, rememberedPlayers) {
			return true
		}
	}
	return false
}

// restrictionPlayerSpecMatches resolves ONE player spec of a restriction's
// Target$ against the defender, with the one extension the ordinary
// MatchesPlayerSpec grammar cannot answer: an IsRemembered clause (Player.
// IsRemembered, and its ! negation and + compounds) resolves against the
// registered effect's captured player set (state.ContinuousEffect.
// RememberedPlayers — Call for Aid's RememberObjects$ TargetedPlayer), not
// against a source object's event-backed list, which a one-shot sorcery
// source does not carry. A face static passes an empty remembered set, so its
// IsRemembered clauses match nobody (fail closed).
func restrictionPlayerSpecMatches(g *state.Game, spec string, defender, controller state.PlayerID, rememberedPlayers []state.PlayerID) bool {
	if !strings.Contains(spec, "IsRemembered") {
		return effects.MatchesPlayerSpec(g, spec, defender, controller)
	}
	for _, clause := range strings.Split(spec, "+") {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		if neg, has := clauseIsRemembered(clause); has {
			found := false
			for _, p := range rememberedPlayers {
				if p == defender {
					found = true
					break
				}
			}
			if found == neg {
				return false
			}
			continue
		}
		if !effects.MatchesPlayerSpec(g, clause, defender, controller) {
			return false
		}
	}
	return true
}

// clauseIsRemembered reports whether one "+"-clause of a player spec carries
// the IsRemembered qualifier (in either polarity, under the spec's own
// dot-separated token grammar) and which polarity it is.
func clauseIsRemembered(clause string) (neg, has bool) {
	for _, tok := range strings.Split(clause, ".") {
		tok = strings.TrimSpace(tok)
		if strings.EqualFold(tok, "!IsRemembered") {
			return true, true
		}
		if strings.EqualFold(tok, "IsRemembered") {
			return false, true
		}
	}
	return false, false
}

// Keywords exists for Ruling F2: Task 23's view.Chars interface needs a
// Keywords(state.ObjID) []string method, and Engine.Derived already returns
// a Derived struct — a method of the same name on Engine could not satisfy
// an interface expecting a slice. This is that method; Derived(id).Keywords
// remains the field other engine-internal code should read when it also
// wants Power/Toughness/Types in the same call.
func (e *Engine) Keywords(id state.ObjID) []string { return e.Derived(id).Keywords }

// fogActive reports whether an api:Fog continuous effect (Restriction
// "PreventCombatDamage", effects/fog.go) is currently active. Consulted by
// the combat-damage step's damage passes (rules/combat.go): while it holds,
// no combat damage is dealt that turn. The active() list already applies the
// UntilEOT expiry, so a Fog cast on turn N contributes nothing from turn N+1
// on.
func (e *Engine) fogActive() bool {
	for _, ce := range e.active() {
		if ce.Restriction == "PreventCombatDamage" {
			return true
		}
	}
	return false
}

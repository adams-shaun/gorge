package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// zoneValidPrefixes is the fixed-order prefix dispatch for Forge's
// zone-suffixed "Valid" filter family in Defined$ values (definedSpec's
// ValidGraveyard/ValidHand/... branch, the twin of count.go's countZone). A
// slice, never a map: the dispatch order is deterministic and the first
// matching prefix wins (the prefixes are mutually exclusive anyway --
// ValidGraveyard's is not a prefix of ValidHand's -- so the order only has
// to be stable, and never map-ordered).
var zoneValidPrefixes = []struct {
	prefix string
	zone   state.Zone
}{
	{"ValidGraveyard ", state.ZGraveyard},
	{"ValidHand ", state.ZHand},
	{"ValidLibrary ", state.ZLibrary},
	{"ValidExile ", state.ZExile},
	{"ValidBattlefield ", state.ZBattlefield},
}

// Defined resolves a Defined$ parameter to concrete targets. With no Defined$
// at all, Forge's own rule applies: an ability that declares ValidTgts$ (it
// has real targets to name) acts on the chosen ones; an ability with no
// ValidTgts$ acts on its own source (R-10's default).
//
// Every return here is a defensive copy, never a slice sharing a backing
// array with Ctx.Targets or Ctx.Remembered: Ctx is threaded by pointer through
// Resolve, so a caller that filters the returned slice in place (the ordinary
// out := s[:0]; for range append(out, ...) idiom) must not be able to corrupt
// state a later effect in the same Sub chain still relies on.
// GainedFacesOfDefined resolves Forge's GainsAbilitiesOfDefined$ dynamic set
// into the foreign faces consumed by the activated-ability grant path. It is
// shared by printed statics and Effect-delivered statics so both routes use
// Defined's object-reference semantics and preserve its deterministic order.
func GainedFacesOfDefined(h Host, c *Ctx, spec string) []state.GainedFace {
	if c == nil || strings.TrimSpace(spec) == "" {
		return nil
	}
	sa := &cards.SA{Params: map[string]string{"Defined": strings.TrimSpace(spec)}}
	var out []state.GainedFace
	seen := make(map[state.ObjID]bool)
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer || t.Obj == 0 || seen[t.Obj] {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		seen[t.Obj] = true
		out = append(out, state.GainedFace{Obj: t.Obj, Face: o.Face()})
	}
	return out
}

func Defined(h Host, c *Ctx, sa *cards.SA) []state.Target {
	if ts, ok := knownDefinedTargets(h, c, sa.Params["Defined"]); ok {
		return ts
	}
	// Keep Defined's historical per-member fallback for a mixed known/unknown
	// expression. knownDefinedTargets is deliberately stricter for callers
	// that need a fail-closed fetch-list classification, not a new public
	// contract for ordinary effects.
	if sa.Params["Defined"] == "Imprinted" || sa.Params["Defined"] == "ImprintedLKI" {
		// Ordinary (non-fetch-list) Imprinted resolution: the source's
		// Imprinted association. knownDefinedTargets deliberately does NOT
		// recognise this selector (TestImprintedDefinedLibraryFetchFailsClosed
		// pins a hidden-library Imprinted fetch as a fail-closed no-op, since
		// Imprinted has no persisted library-position context), so this stays
		// scoped to Defined's own broader fallback contract.
		g := h.Game()
		if o := g.Obj(c.Source); o != nil {
			out := make([]state.Target, 0, len(o.Imprinted))
			for _, id := range o.Imprinted {
				if g.Obj(id) != nil {
					out = append(out, state.Target{Obj: id})
				}
			}
			return out
		}
		return nil
	}
	if strings.Contains(sa.Params["Defined"], " & ") {
		var out []state.Target
		for _, part := range strings.Split(sa.Params["Defined"], " & ") {
			copy := *sa
			copy.Params = make(map[string]string, len(sa.Params))
			for k, v := range sa.Params {
				copy.Params[k] = v
			}
			copy.Params["Defined"] = strings.TrimSpace(part)
			out = append(out, Defined(h, c, &copy)...)
		}
		return out
	}
	// Forge's "Defined$ Valid <filter>" form (88 raw corpus lines: Angelic
	// Skirmisher's "Defined$ Valid Creature.YouCtrl" combat grant, the
	// double-power pump family): every battlefield object the filter admits,
	// in the deterministic APNAP seat/zone walk. Evaluated with the resolving
	// controller as You; an unmodelled predicate fails closed to an empty set
	// like every filter. (ValidStack is knownDefinedTargets' own prefix above
	// and never reaches here.)
	if spec := sa.Params["Defined"]; spec == "Valid" || strings.HasPrefix(spec, "Valid ") {
		return battlefieldValidTargets(h, c, strings.TrimSpace(strings.TrimPrefix(spec, "Valid")))
	}
	// Forge's rule: an ability that names targets acts on them; one that
	// names none acts on its source. A sub-ability that wants its
	// parent's targets says so explicitly (Defined$ Targeted /
	// ParentTarget), which every script in the corpus does.
	if _, targeted := sa.Params["ValidTgts"]; targeted {
		// The generic pre-ask's answered set (task mvts1) outranks the
		// resolution's own Ctx.Targets: this dispatch asked for and received
		// ITS OWN targets, and the resolution-level list is either the outer
		// SA's (the CLOBBER inherit) or empty. Non-nil (possibly empty) only
		// while the pre-asked body dispatches; the wrapper clears it after.
		if c.PickedTargets != nil {
			return copyTargets(c.PickedTargets)
		}
		return copyTargets(c.Targets)
	}
	return []state.Target{{Obj: c.Source}}
}

// battlefieldValidTargets is Forge's "Defined$ Valid <filter>" battlefield
// sweep: every battlefield object the filter admits, in the deterministic
// APNAP seat/zone walk, evaluated with the resolving controller as You; an
// unmodelled predicate fails closed to an empty set like every filter. It is
// the ONE implementation shared by Defined's bare-Valid branch and
// definedSpec's fail-closed recognition of the same selector (the
// zone-suffixed family is its per-zone twin), so the two can never disagree
// about what the battlefield form means.
func battlefieldValidTargets(h Host, c *Ctx, filt string) []state.Target {
	g := h.Game()
	var out []state.Target
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecCtx(g, filt, id, c.SpecContext(c.Controller)) {
				out = append(out, state.Target{Obj: id})
			}
		}
	}
	return out
}

// knownDefinedTargets resolves a Defined$ form only when every selector in it
// is modelled. Unlike Defined, it never falls back to the source or chosen
// targets: callers such as a hidden-library ChangeZone need to distinguish an
// actual object fetch list from an unrecognised selector. Forge's " & " joins
// independent selectors, not their intersection, so known members are joined
// in script order. One unknown member makes the whole expression unknown --
// the fail-closed direction.
func knownDefinedTargets(h Host, c *Ctx, spec string) ([]state.Target, bool) {
	g := h.Game()
	// Defined$ ValidStack <spec>: every stack object matching the spec. It is
	// a prefix, not a whole-value case, because the spec rides in the same
	// parameter after one space ("ValidStack Spell.OppCtrl,...").
	if stackSpec, ok := strings.CutPrefix(spec, "ValidStack"); ok {
		return validStackTargets(g, strings.TrimSpace(stackSpec), c), true
	}
	// Defined$ Remembered.<spec>: the subset of the resolution's Remembered
	// objects matching <spec>, evaluated as a Card filter (Regrowth-shaped
	// "each card exiled this way" follow-ups that narrow Remembered by type
	// or predicate rather than acting on the whole set).
	if filterSpec, ok := strings.CutPrefix(spec, "Remembered."); ok {
		var out []state.Target
		for _, t := range c.Remembered {
			if !t.IsPlayer {
				if o := g.Obj(t.Obj); o != nil && MatchesObjectCtx(g, "Card."+filterSpec, o, c.SpecContext(c.Controller)) {
					out = append(out, t)
				}
			}
		}
		return out, true
	}
	if ts, ok := definedSpec(h, c, spec); ok {
		return ts, true
	}
	if !strings.Contains(spec, " & ") {
		return nil, false
	}
	var out []state.Target
	for _, part := range strings.Split(spec, " & ") {
		ts, ok := knownDefinedTargets(h, c, strings.TrimSpace(part))
		if !ok {
			return nil, false
		}
		out = append(out, ts...)
	}
	return out, true
}

// ChosenTargets resolves the "chosen" answer a resolution or source object
// holds: the in-flight choice while the resolution carries one, else the
// source object's event-backed chosen list (the Choose "chosen" fold the
// ChooseCard/ChooseSource/ChoosePlayer family emits). It is THE one read every
// chosen-referent consumer goes through -- definedSpec's ChosenCard/
// ChosenPlayer/ChosenCardController and rules' rememberedSpecContext (which
// seeds SpecContext.Chosen so a ChosenCard/ChosenCardStrict filter spec can
// gate a replacement's ValidSource$) -- so the resolution-time and
// replacement-time bindings cannot drift apart. An unbound choice yields nil.
func ChosenTargets(g *state.Game, c *Ctx) []state.Target {
	return resolutionChosenCards(g, c)
}

// ChosenTargetsFrom is ChosenTargets for a source object named directly
// (rules' replacement path has the registration's source id, not a Ctx).
func ChosenTargetsFrom(g *state.Game, source state.ObjID) []state.Target {
	if o := g.Obj(source); o != nil {
		return copyTargets(o.Chosen)
	}
	return nil
}

// definedSpec resolves one RECOGNISED Defined$ value. The bool distinguishes
// "this spec names an object reference this build models" from "unknown
// spec": Defined's public contract keeps the chosen-targets fallback for
// anything unmodelled, but a caller that must not guess (damage.go's
// DamageSource$ resolution, damage.go's ValidPlayers$ resolution) reads the
// bool and fails closed to its own conservative default instead of silently
// redirecting at the chosen targets.
func definedSpec(h Host, c *Ctx, spec string) ([]state.Target, bool) {
	g := h.Game()
	switch spec {
	case "":
		return nil, false
	case "Self", "Parent", "EffectSource", "OriginalHost", "CorrectedSelf":
		// EffectSource/OriginalHost name the ability's own source object --
		// the permanent that pushed the resolving ability, or the card that
		// originally generated it before any copies. newDamageRider unwraps
		// an ability stack object to that source afterwards, so handing
		// back the raw c.Source here is the same object every other
		// source-defaulting path yields. CorrectedSelf is Forge's
		// identity-corrected source (Shorecrasher Elemental's `DBReturn`
		// re-fetches the card just exiled by its own cost): the object id is
		// stable across that self-exile -- no zone change mints a new id --
		// so the raw c.Source is already the corrected identity.
		return []state.Target{{Obj: c.Source}}, true
	case "You":
		return []state.Target{{Player: c.Controller, IsPlayer: true}}, true
	case "TopOfLibrary", "BottomOfLibrary":
		// Library order is top-first. These selectors name one known card, not
		// a player whose whole library should be searched; hidden-origin
		// ChangeZone therefore consumes the returned identity as its fetch list.
		lib := g.Zone(state.ZLibrary, c.Controller)
		if len(lib) == 0 {
			return nil, true
		}
		i := 0
		if spec == "BottomOfLibrary" {
			i = len(lib) - 1
		}
		return []state.Target{{Obj: lib[i]}}, true
	case "TriggeredOpponentVotedSame", "TriggeredOpponentVotedDiff":
		// The canonical vote-finished carrier's two List$ referent sets
		// (trig:Vote): the players other than the TRIGGER SOURCE'S CONTROLLER
		// who voted for a choice that controller voted for / for a different
		// one. rules/trigger_referents' Vote case re-splits the carrier's raw
		// ballots against e.controllerOf(source) -- the vote caster's own
		// controller is never the anchor -- and the per-stack capture is
		// rebuilt by replay from the same event bytes. Absent (a non-vote
		// context) they fail closed to the empty set, ok=true -- the same
		// convention FlippedHeads/FlippedTails takes, so a reader acts on
		// nobody rather than guessing at a fallback target.
		ps := c.TriggeredOpponentsVotedSame
		if spec == "TriggeredOpponentVotedDiff" {
			ps = c.TriggeredOpponentsVotedDiff
		}
		out := make([]state.Target, 0, len(ps))
		for _, p := range ps {
			out = append(out, state.Target{Player: p, IsPlayer: true})
		}
		return out, true
	case "FlippedHeads", "FlippedTails":
		// Forge's RememberResult$ flip-result memory: DB$ FlipCoin |
		// RememberResult$ True, then a chained sub reading Defined$
		// FlippedHeads/FlippedTails (Goblin Assassin's tails sacrifice and
		// Mana Clash's ValidPlayers$ FlippedTails damage are the live
		// carriers). When RememberResult$ is True, effFlipCoin appends every
		// flip to Ctx.FlipMemory.Results, so the reader returns the real
		// flippers of that side rather than the empty set. A resolution with no
		// remembered result resolves to the empty set,
		// ok=true (fail closed to nobody, the pre-existing convention).
		wantHeads := spec == "FlippedHeads"
		var out []state.Target
		seen := make(map[state.PlayerID]bool)
		var results []FlipResult
		if c.FlipMemory != nil {
			results = c.FlipMemory.Results
		}
		for _, fr := range results {
			if fr.Heads != wantHeads || seen[fr.Player] {
				continue
			}
			seen[fr.Player] = true
			out = append(out, state.Target{Player: fr.Player, IsPlayer: true})
		}
		return out, true
	case "Remembered":
		return copyTargets(c.Remembered), true
	case "ImprintedLKI":
		// Forge's LKI spelling of the imprint pile, distinct from the bare
		// "Imprinted" case below: the SOURCE's persistent imprint association,
		// deliberately NOT the RepeatEach subject binding "Imprinted" takes --
		// a delayed trigger registering after a RepeatEach loop must see every
		// token/card imprinted across the whole loop (Kharasha Foothills,
		// Shredder, Shadow Master's RememberObjects$ ImprintedLKI DelTrig),
		// not the last iteration's subject. All five corpus DelTrig carriers
		// read it exactly this way.
		return imprintPileTargets(g, c), true
	case "Imprinted", "ImprintedController":
		// Two populations share the spelling. Inside a RepeatEach iteration
		// (this build's own binding) Forge's UseImprinted$ names the loop's
		// CURRENT SUBJECT: Heroism pumps it, Stench of Evil deals its damage
		// to its controller — and ImprintedController is only ever that
		// iteration's subject's controller. Outside a loop iteration the
		// spelling is the source's persistent imprint pile (Mirrorworks'
		// exiled-with-imprint list, CR 607.2a links only while the card stays
		// in exile); no corpus RepeatEach body resolves against a source that
		// also carries a persistent imprint, so the two never collide.
		if c.RepeatSubject.IsPlayer {
			return []state.Target{{Player: c.RepeatSubject.Player, IsPlayer: true}}, true
		}
		// Prefer the last-known controller the ChangeZone captured: events.Apply's
		// Move resets a battlefield departure's controller to its owner (CR
		// 400.7), so the live object answers the WRONG seat for a stolen
		// creature (Forge stores a Card LKI copy at the same point).
		if spec == "ImprintedController" {
			if p, ok := lkiControllerFor(c, c.RepeatSubject.Obj); ok {
				return []state.Target{{Player: p, IsPlayer: true}}, true
			}
		}
		if o := g.Obj(c.RepeatSubject.Obj); o != nil {
			if spec == "ImprintedController" {
				return []state.Target{{Player: o.Controller, IsPlayer: true}}, true
			}
			return []state.Target{{Obj: c.RepeatSubject.Obj}}, true
		}
		if spec == "ImprintedController" {
			return nil, true
		}
		return imprintPileTargets(g, c), true
	case "RememberedCard":
		// Forge's RememberedCard names the resolution's remembered CARD entries
		// in remember order: the ChooseCard answers RememberChosen$ captured
		// (Dreams of Steel and Oil's "Exile the chosen cards" -- a mixed-Hand
		// Origin$ whose two picks must BOTH move), the manifest family's
		// RememberManifested$ capture (Valgavoth's Onslaught's counters) and
		// the imprint seeds. The reading matches this package's own
		// RememberObjects$/RememberSacrificed$ consumers (the misc.go remember
		// walk, cardflow.go's Atsushi seed): every non-player Ctx.Remembered
		// entry. Being known here also keeps the fetch-list classifier honest
		// -- a mixed-Hand Origin$ carrying it stays on the already-answered
		// object path instead of the source default.
		var remembered []state.Target
		for _, t := range c.Remembered {
			if !t.IsPlayer {
				remembered = append(remembered, t)
			}
		}
		return remembered, true
	case "ChosenCard", "ChosenPlayer":
		// ChooseCard/ChoosePlayer bind the current resolution's most recent
		// choice here. This is deliberately distinct from Remembered: Forge
		// only copies the answer there when RememberChosen$ is set. A later,
		// independently resolving ability reads the same event-backed choice
		// from its source permanent.
		return ChosenTargets(g, c), true
	case "Player.IsRemembered":
		// Forge's Player.IsRemembered names the source permanent's persistent
		// player-Remembered list -- the same set the filter spelling of the
		// same name reads (MatchesPlayerSpecFrom). The resolution-time
		// remember stays the fallback for a context with no source or an
		// empty persistent list (Only Blood Ends Your Nightmares' RepeatEach
		// remembers its current opponent only in the resolution; a mid-chain
		// ChoosePlayer's RememberChosen$ answer is in both). Sower of
		// Discord is why the persistent list wins: its DamageDoneOnce half
		// reflects damage onto the card's remembered player, and the event
		// role a Damage trigger captures (Ctx.Remembered = the damaged
		// player) is not a remember at all.
		if o := g.Obj(c.Source); o != nil {
			if ps := playersOf(o.Remembered); len(ps) > 0 {
				return ps, true
			}
		}
		return playersOf(c.Remembered), true
	case "Player.Chosen":
		// Forge's Player.Chosen names the most recent ChoosePlayer answer:
		// the in-flight choice while this resolution holds one (the mid-chain
		// family -- Booby Trap, Infernal Denizen, Cruel Entertainment -- and
		// the current-resolution convention ChosenPlayer above keeps), else
		// the source permanent's event-backed choice. Sower of Discord is why
		// the persistent fallback matters: its DamageDoneOnce half reflects
		// damage onto the card's chosen player from a trigger resolution
		// that itself chose nothing.
		if c.ChosenValid {
			return playersOf(c.Chosen), true
		}
		if ps := playersOf(c.Chosen); len(ps) > 0 {
			return ps, true
		}
		if o := g.Obj(c.Source); o != nil {
			return playersOf(o.Chosen), true
		}
		return nil, true
	case "RememberedController":
		return controllersOf(g, c.Remembered), true
	case "RememberedOwner":
		return ownersOf(g, c.Remembered), true
	case "TargetedController", "TargetedPlayer":
		return controllersOf(g, c.Targets), true
	case "ChosenController":
		return controllersOf(g, c.Chosen), true
	case "ChosenCardController":
		// Forge's ChosenCardController (Deflecting Palm's retaliation, New Way
		// Forward's redirect): the controller of the chosen CARD. The chosen
		// binding is the same one ChosenCard reads -- the in-flight choice
		// while this resolution holds one, else the source object's
		// event-backed chosen list -- reduced to its controllers. An unbound
		// choice yields nothing, so the body acts on nobody rather than
		// inventing a seat.
		return controllersOf(g, ChosenTargets(g, c)), true
	case "Targeted", "ParentTarget", "ParentTargeted", "ThisTargetedCard":
		return copyTargets(c.Targets), true
	case "TriggeredAttackers":
		// Forge's plural attack-batch referent (Love on the Battlefield's
		// "those creatures gain first strike"): the attackers the
		// AttackersDeclared trigger fired for. The queue entry's Remembered
		// carries the declared batch (triggerRemembered's DeclareAttackers
		// case), so this resolves the whole per-defender attacker group.
		return objectsOf(c.Remembered), true
	case "TriggeredTargetLKICopy":
		// The BEARER the Attached referent walk captured (triggerReferents'
		// Attached case): the permanent an Aura/Equipment became attached to
		// -- Enormous Energy Blade's "tap that creature". Only the Attached
		// walk sets TriggerBearer, so no other mode's provenance moves: a
		// BecomesTarget trigger's TriggerTarget role is its OWN source
		// permanent (the enchanted creature, Horobi himself), and this
		// spelling keeps resolving that mode's Remembered entry (the
		// targeting spell) exactly as it always has.
		if c.TriggerBearer != 0 {
			return []state.Target{{Obj: c.TriggerBearer}}, true
		}
		return objectsOf(c.Remembered), true
	case "TriggeredObject", "TriggeredObjectLKICopy":
		if c.DelayedObject != 0 {
			return []state.Target{{Obj: c.DelayedObject}}, true
		}
		return objectsOf(c.Remembered), true
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCard",
		"TriggeredNewCardLKICopy",
		"TriggeredSourceSA", "TriggeredAttacker",
		"TriggeredAttackerLKICopy",
		"RememberedLKI":
		// M1 does not model LKI copies, new-object identity or the
		// ability-vs-card distinction separately: every one of these forms
		// names the same Remembered object entry a trigger captured.
		// TriggeredObject/TriggeredObjectLKICopy are the event-object
		// spellings the CounterPlayerAddedAll batch triggers read (Rikku's
		// RememberObjects$ TriggeredObjectLKICopy on its DB$ Effect body) --
		// the triggering event's object, exactly what triggerRemembered seeds
		// Remembered with for every non-zero ev.Obj. It is also the object a
		// Mode$ Unattached trigger became unattached FROM (the former bearer
		// rules/trigger_match.go's triggerRemembered carries on the event's
		// IDs): the Grafted Exoskeleton cycle reads it as its SacrificeAll
		// referent, so it must resolve like the rest of the family rather
		// than fall through to the source-default fallback.
		// TriggeredSourceSA is the targeting spell/ability a BecomesTarget
		// trigger captured (Reality Smasher's counter, Kira's and the
		// glasskite family's counters -- 18 corpus files); its Controller
		// variant resolves in unlessPayerTargets, its object here.
		// The TriggeredBlocker spellings moved OUT of this case (trig:Blocks):
		// a Blocks trigger's Remembered carries the pair's ATTACKER, so the
		// blocker role is the only exact referent -- see the case below.
		return objectsOf(c.Remembered), true
	case "TriggeredBlocker", "TriggeredBlockerLKICopy":
		// The pair's BLOCKER (trig:Blocks): prefer the fire-time TriggerBlocker
		// role when the Blocks capture set it (Remembered names the attacker
		// there); the role-absent fallback -- the AttackerBlockedByCreature
		// queue entries, whose Remembered IS the blocker, and hand-built
		// contexts -- keeps the old Remembered read, so the Flanking shapes
		// resolve exactly as before this field existed.
		if c.TriggerBlocker != 0 {
			return []state.Target{{Obj: c.TriggerBlocker}}, true
		}
		return objectsOf(c.Remembered), true
	case "DelayTriggerRememberedLKI":
		// The registration's own capture (objects only, the LKI spelling's
		// read), with the pre-field fallback to Remembered for a context
		// built without one.
		if len(c.DelayedRemembered) > 0 {
			return objectsOf(c.DelayedRemembered), true
		}
		return objectsOf(c.Remembered), true
	case "DelayTriggerRemembered":
		// The delayed trigger's remembered set AS-IS, players included (task
		// mordorparams1): a DelayedTrigger registration that remembered a
		// PLAYER (Arcane Denial's RememberObjects$ RememberedController —
		// "Its controller may draw up to two cards" names the countered
		// spell's CONTROLLER, a player, never an object) must resolve to
		// that player for the Draw the Execute$ runs; the objectsOf read the
		// M1 comment describes dropped the entry and the whole draw silently
		// no-oped. Object-remembered registrations are unchanged (the set is
		// passed through verbatim); the LKI forms above keep the objects-only
		// read their LKI semantics name.
		if len(c.DelayedRemembered) > 0 {
			return copyTargets(c.DelayedRemembered), true
		}
		return copyTargets(c.Remembered), true
	case "TriggeredSpellAbility":
		// The activation arm (abcopy1): an ability-cast trigger's Remembered
		// names the SOURCE PERMANENT (an AbilityPush's Obj -- the minted
		// ability wrapper never travels on the event), so the fire-time
		// TriggerAbility role is the only exact referent: the wrapper is on
		// the stack and a copy/counter/rewrite of it is stack-legal. The
		// role-absent fallback (the spell-cast arm, where Remembered IS the
		// cast spell, and hand-built contexts) keeps the Remembered entry.
		if c.TriggerAbility != 0 {
			return []state.Target{{Obj: c.TriggerAbility}}, true
		}
		return objectsOf(c.Remembered), true
	case "TriggeredTarget":
		// The object or player that received the triggering event. Spiteful
		// Shadows uses this as a DamageSource$: the enchanted creature, not the
		// Aura whose trigger is resolving, deals the reflected damage. Preserve
		// the target's kind here; callers that require an object (the damage
		// rider) already reject player entries rather than guessing. When the
		// causing event's mode did not capture a TriggerTarget (a hand-built
		// context), fall back to the chosen targets -- Defined's pre-branch
		// convention for a trigger selector whose provenance was not recorded.
		if c.TriggerTarget.Obj != 0 || c.TriggerTarget.IsPlayer {
			return []state.Target{c.TriggerTarget}, true
		}
		return copyTargets(c.Targets), true
	case "TriggeredSource":
		// The damage source the causing event recorded (pg2's
		// TriggerContext.TriggerSource): a DamageDone execute's "that source
		// deals ..." reading. Prefer the event role when the firing trigger
		// captured one -- for a DamageDone trigger Remembered holds the
		// DAMAGED object, so the old objectsOf fallback names the recipient,
		// not the dealer. No corpus card uses Defined$ TriggeredSource (the
		// 6 DamageSource$ TriggeredSource lines are the only users), and the
		// fallback keeps a non-trigger context behaving exactly as before.
		if c.TriggerSource != 0 {
			return []state.Target{{Obj: c.TriggerSource}}, true
		}
		return objectsOf(c.Remembered), true
	case "TriggeredSourceController", "TriggeredTargetController":
		// The controller of the source/target the causing event recorded:
		// Flameblade Angel's and Harsh Justice's "deals 1 damage to that
		// source's controller", Greatbow Doyen's "to that creature's
		// controller". The role is preferred when the trigger captured one
		// (a DamageDone trigger's Remembered is the DAMAGED object, whose
		// controller is exactly wrong for the source form); the fallback --
		// Remembered[0]'s controller -- is deciderFromSpec's convention for
		// the same two spellings on OptionalDecider$ lines, so both reads of
		// one spelling agree wherever the role is absent.
		ref := c.TriggerSource
		if spec == "TriggeredTargetController" {
			if c.TriggerTarget.Obj != 0 || c.TriggerTarget.IsPlayer {
				if c.TriggerTarget.IsPlayer {
					return []state.Target{{Player: c.TriggerTarget.Player, IsPlayer: true}}, true
				}
				ref = c.TriggerTarget.Obj
			} else if len(c.Remembered) > 0 {
				ref = c.Remembered[0].Obj
			}
		} else if ref == 0 && len(c.Remembered) > 0 {
			ref = c.Remembered[0].Obj
		}
		if o := g.Obj(ref); o != nil {
			return []state.Target{{Player: o.Controller, IsPlayer: true}}, true
		}
		return nil, true
	case "TriggeredTargets":
		// The batch's matching TARGET set (trig:DamageAll): Breeches, Brazen
		// Plunderer's "exile the top card of each of those opponents'
		// libraries" reads Defined$ TriggeredTargets -- every target the
		// batch's matching Damage events named, players and objects both, in
		// first-seen order. An absent set falls back to the singleton
		// TriggeredTarget semantics (the same role-absent convention).
		if len(c.TriggerDamageTargets) > 0 {
			return copyTargets(c.TriggerDamageTargets), true
		}
		return definedSpec(h, c, "TriggeredTarget")
	case "TriggeredSourcesController":
		// The controllers of the batch's matching SOURCE set (trig:DamageAll):
		// Nelly Borca's "you and the controller of those creatures each draw a
		// card" reads Defined$ TriggeredSourcesController & You. Controllers
		// are read live at resolution (the singular spelling's read) and
		// deduplicated in first-seen source order; a controller whose source
		// object is gone contributes nothing. An absent set falls back to the
		// singular TriggeredSourceController semantics.
		if len(c.TriggerDamageSources) > 0 {
			var out []state.Target
			seen := map[state.PlayerID]bool{}
			for _, id := range c.TriggerDamageSources {
				if o := g.Obj(id); o != nil && !seen[o.Controller] {
					seen[o.Controller] = true
					out = append(out, state.Target{Player: o.Controller, IsPlayer: true})
				}
			}
			return out, true
		}
		return definedSpec(h, c, "TriggeredSourceController")
	case "Convoked":
		// CR 702.66's "each creature that convoked it" (task connive1): the
		// creatures the caster tapped to help pay for the resolving spell's
		// cast, carried by the pay-time CastInfo's FlagConvoked IDs into
		// Object.Convoked (Lethal Scheme's connive sub while the spell is on
		// the stack; Venerated Loxodon's and Zephyr Singer's ETB triggers
		// after it resolves into a permanent). Absent (a cast with no
		// convoke, a copy, a creature that left play) resolves to the EMPTY
		// set, ok=true -- the same convention FlippedHeads/FlippedTails
		// takes, so a reader acts on nobody rather than guessing at a
		// fallback target.
		if o := g.Obj(c.Source); o != nil {
			out := make([]state.Target, 0, len(o.Convoked))
			for _, id := range o.Convoked {
				if g.Obj(id) != nil {
					out = append(out, state.Target{Obj: id})
				}
			}
			return out, true
		}
		return nil, true
	case "ReplacedCard":
		// The card a zone-change replacement is acting on. Outside such a
		// replacement (or after the object ceased to exist), resolve nothing.
		if c.Replaced != 0 && g.Obj(c.Replaced) != nil {
			return []state.Target{{Obj: c.Replaced}}, true
		}
		return nil, true
	case "ReplacedTarget":
		// Damage replacements may affect either an object or a player. Preserve
		// that distinction rather than deriving a player through object zero.
		if c.ReplacementTarget.IsPlayer {
			if int(c.ReplacementTarget.Player) < len(g.Players) {
				return []state.Target{c.ReplacementTarget}, true
			}
			return nil, true
		}
		if c.ReplacementTarget.Obj != 0 && g.Obj(c.ReplacementTarget.Obj) != nil {
			return []state.Target{c.ReplacementTarget}, true
		}
		return nil, true
	case "ReplacedSource":
		if c.ReplacementSource != 0 && g.Obj(c.ReplacementSource) != nil {
			return []state.Target{{Obj: c.ReplacementSource}}, true
		}
		return nil, true
	case "ReplacedSourceController":
		if o := g.Obj(c.ReplacementSource); o != nil && int(o.Controller) < len(g.Players) {
			return []state.Target{{Player: o.Controller, IsPlayer: true}}, true
		}
		return nil, true
	case "ReplacedTargetController":
		if c.ReplacementTarget.IsPlayer {
			return []state.Target{c.ReplacementTarget}, true
		}
		if o := g.Obj(c.ReplacementTarget.Obj); o != nil && int(o.Controller) < len(g.Players) {
			return []state.Target{{Player: o.Controller, IsPlayer: true}}, true
		}
		return nil, true
	case "TriggeredDefendingPlayer":
		if out := oneTriggerPlayer(c.DefendingPlayer); out != nil {
			return out, true
		}
		return playersOf(c.Remembered), true
	case "TriggeredPlayer":
		if out := oneTriggerPlayer(c.TriggerPlayer); out != nil {
			return out, true
		}
		return playersOf(c.Remembered), true
	case "TriggeredAttackingPlayer":
		if out := oneTriggerPlayer(c.AttackingPlayer); out != nil {
			return out, true
		}
		return nil, true
	case "TriggeredAttackedTarget":
		if out := oneTriggerPlayer(c.AttackedTarget); out != nil {
			return out, true
		}
		return nil, true
	case "TriggeredActivator":
		if out := oneTriggerPlayer(c.TriggerActivator); out != nil {
			return out, true
		}
		return nil, true
	case "TriggeredCardController":
		if p, ok := TriggeredCardController(g, c.TriggerContext, c.Remembered); ok {
			return []state.Target{{Player: p, IsPlayer: true}}, true
		}
		return nil, true
	case "TriggeredCardOwner", "NonTriggeredCardOwner":
		// These selectors use the triggering card's immutable owner (CR
		// 108.3), never a remembered-object fallback. A stolen creature that
		// dies is still its owner's (Oft-Nabbed Goat's "its owner draws").
		// If TriggerCard is absent or no longer resolves, both forms fail
		// closed to the empty set rather than guessing from the source.
		triggered := g.Obj(c.TriggerCard)
		if triggered == nil {
			return nil, true
		}
		if spec == "TriggeredCardOwner" {
			return []state.Target{{Player: triggered.Owner, IsPlayer: true}}, true
		}
		var out []state.Target
		for _, p := range g.AliveFrom(0) {
			if p != triggered.Owner {
				out = append(out, state.Target{Player: p, IsPlayer: true})
			}
		}
		return out, true
	case "TriggeredAttackerController", "TriggeredBlockerController":
		// The controller of the triggering event's attacker or blocker. The
		// Blocks mode captures both roles per pair (rules/trigger_match.go's
		// checkBlocksTriggers); TriggeredBlockerController prefers the
		// captured TriggerBlocker and TriggeredAttackerController the captured
		// AttackingPlayer role. The role-absent fallback is the remembered
		// object's controller through the one shared TriggeredCardController
		// resolver -- the AttackerBlockedByCreature queue entries (Remembered
		// = the blocker) and hand-built contexts resolve exactly as before
		// the Blocks capture existed.
		if spec == "TriggeredBlockerController" && c.TriggerBlocker != 0 {
			if o := g.Obj(c.TriggerBlocker); o != nil {
				return []state.Target{{Player: o.Controller, IsPlayer: true}}, true
			}
			return nil, true
		}
		if spec == "TriggeredAttackerController" && c.AttackingPlayer.IsPlayer {
			return []state.Target{{Player: c.AttackingPlayer.Player, IsPlayer: true}}, true
		}
		if p, ok := TriggeredCardController(g, c.TriggerContext, c.Remembered); ok {
			return []state.Target{{Player: p, IsPlayer: true}}, true
		}
		return nil, true
	case "ExiledWith":
		var out []state.Target
		for _, id := range g.Zone(state.ZExile, c.Controller) {
			if o := g.Obj(id); o != nil && o.ExiledWith == c.Source {
				out = append(out, state.Target{Obj: id})
			}
		}
		return out, true
	case "Equipped", "Enchanted", "AttachedTo":
		// The corpus spells this three ways depending on whether the source
		// is Equipment, an Aura, or a generic script; all three name the
		// same field (Task 14 wires its producer).
		if o := g.Obj(c.Source); o != nil && o.AttachedTo != 0 && g.Obj(o.AttachedTo) != nil {
			return []state.Target{{Obj: o.AttachedTo}}, true
		}
		return nil, true
	case "Opponent", "Player.Opponent", "Player.Other":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				out = append(out, state.Target{Player: p, IsPlayer: true})
			}
		}
		return out, true
	case "ReplacedPlayer":
		// The draw-er of the replaced Draw event (Breathstealer's Crypt draws
		// and reveals for "that player"). Set only on a Draw replacement's
		// own context; nil outside one.
		if c.ReplacedPlayer.IsPlayer {
			return []state.Target{{Player: c.ReplacedPlayer.Player, IsPlayer: true}}, true
		}
		return nil, true
	case "NonReplacedPlayer":
		// Every OTHER player (Zur's Weirding: "any other player may pay 2
		// life"), in AliveFrom order, excluding the draw-er.
		if !c.ReplacedPlayer.IsPlayer {
			return nil, true
		}
		var out []state.Target
		for _, p := range g.AliveFrom(0) {
			if p != c.ReplacedPlayer.Player {
				out = append(out, state.Target{Player: p, IsPlayer: true})
			}
		}
		return out, true
	case "Player":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			out = append(out, state.Target{Player: p, IsPlayer: true})
		}
		return out, true
	}
	// TriggeredDefender(.qualifier): the defending player the firing Attacks/
	// AttackersDeclared trigger's event names (c.DefendingPlayer -- Myr
	// Battlesphere's "deals X damage to the player or planeswalker it's
	// attacking", whose script spells the referent TriggeredDefender while
	// the engine's own binding is TriggeredDefendingPlayer). A qualifier is
	// evaluated the same way the Player fallback below evaluates one; an
	// unmet qualifier fails closed to the empty set, never a guessed
	// fallback. Outside a combat trigger the role is absent and the set is
	// empty.
	if base, qual, _ := strings.Cut(spec, "."); base == "TriggeredDefender" && !strings.Contains(spec, " & ") {
		if !c.DefendingPlayer.IsPlayer {
			return nil, true
		}
		if qual != "" && !MatchesPlayerSpecFrom(g, qual, c.DefendingPlayer.Player, c.Controller, c.Source) {
			return nil, true
		}
		return []state.Target{c.DefendingPlayer}, true
	}
	// Player.<state-qualifier>: a compound spelling this build's fixed cases
	// do not name (Player.lifeEQ13, Player.controlsCreature.powerGE4_GE1,
	// Player.withMostTypeCreature, ...) resolves through the trigger-side
	// player-filter grammar (MatchesPlayerSpecFrom) over every living seat,
	// the same bridge DamageAll's ValidPlayers$ resolution already uses
	// (effects/damage.go's validPlayers). Qualifiers the grammar cannot
	// evaluate fail closed INSIDE the filter -- every seat matches nothing --
	// so this returns the empty set with ok=true: the effect acts on nobody
	// rather than falling back to the spell's chosen (object) targets, which
	// is the wrong set for a player-valued effect.
	//
	// The same bridge covers the QUALIFIED Opponent./Other./You. spellings
	// whose base the grammar already evaluates (Opponent.IsCorrupted -- Feed
	// the Infection's Corrupted arm, Ixhel's TrigExile): the candidate walk
	// is the same AliveFrom sweep, with the controller skipped for base
	// Opponent so the Opponent.IsX candidate pool is opponents only (the
	// grammar's own Opponent base read also excludes `you`, so the two agree
	// when a compound carries a second qualifier). Without this, definedSpec
	// fell through to `nil, false` and the source fallback made the CASTER
	// the acting player -- the opposite seat lost the life.
	if base, _, _ := strings.Cut(spec, "."); base == "Player" || base == "Opponent" || base == "Other" || base == "You" {
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			if base == "Opponent" && p == c.Controller {
				continue
			}
			if MatchesPlayerSpecFrom(g, spec, p, c.Controller, c.Source) {
				out = append(out, state.Target{Player: p, IsPlayer: true})
			}
		}
		return out, true
	}
	// Forge's zone-suffixed Valid filter family ("Defined$ ValidGraveyard
	// Aura.YouOwn" -- Retether's mass return, and 16 more raw ChangeZone
	// lines; the same spelling Count$ValidGraveyard already reads through
	// count.go's countZone): the named zone's cards the filter admits,
	// evaluated with the resolving controller as You -- the same walk the
	// "Valid <filter>" battlefield branch runs, over the zone the prefix
	// names instead of the battlefield. Unmodelled predicates fail closed
	// INSIDE the filter (an empty set, ok=true), never a guessed fallback.
	for _, zf := range zoneValidPrefixes {
		if filt, ok := strings.CutPrefix(spec, zf.prefix); ok {
			var out []state.Target
			for _, id := range g.Zone(zf.zone, c.Controller) {
				if MatchesSpecCtx(g, strings.TrimSpace(filt), id, c.SpecContext(c.Controller)) {
					out = append(out, state.Target{Obj: id})
				}
			}
			return out, true
		}
	}
	// Forge's bare "Defined$ Valid <filter>" form (88 raw corpus lines:
	// Redoubled Stormsinger's "Defined$ Valid Creature.token+YouCtrl+
	// ThisTurnEntered", Angelic Skirmisher's "Defined$ Valid
	// Creature.YouCtrl" combat grant). It is the battlefield twin of the
	// zone-suffixed family above and MUST be recognised here: a fail-closed
	// caller (knownDefinedTargets) treats an unrecognised selector as an
	// unresolvable fetch list, so omitting it makes every such copy/search
	// mint nothing. The sweep is the shared battlefieldValidTargets helper
	// Defined's own bare-Valid branch calls, so the two cannot drift.
	//
	// The `!strings.Contains(spec, " & ")` guard is load-bearing: a compound
	// "Valid Creature & Player" must NOT be captured whole here. Forge's " & "
	// joins independent selectors (a union, not an intersection), and
	// knownDefinedTargets splits on " & " only AFTER definedSpec returns
	// ok=false -- so matching the compound here hands the whole string to
	// battlefieldValidTargets as the mangled filter "Creature & Player",
	// which matches nothing, replacing a correct union with an empty set.
	// With the guard, the compound falls through to that split and each part
	// ("Valid Creature", "Player") is resolved on its own.
	if !strings.Contains(spec, " & ") && (spec == "Valid" || strings.HasPrefix(spec, "Valid ")) {
		return battlefieldValidTargets(h, c, strings.TrimSpace(strings.TrimPrefix(spec, "Valid"))), true
	}
	// Any Defined$ form this build does not model falls back to the chosen
	// targets rather than silently acting on nothing (the caller decides via
	// the bool whether that fallback is acceptable).
	return nil, false
}

// rememberedWithSource returns the remembered group a Forge SVAR/condition
// named plain "Remembered" reads: the SOURCE object's persistent event-backed
// Remembered list first (Forge's executing ability shares the HOST CARD's
// remembered list, so a later trigger of the same card -- Skyclave
// Apparition's leave trigger, whose X is the card the earlier ETB trigger
// remembered -- reads what an earlier resolution of the same source
// recorded), then every ctx entry that is neither already present nor the
// source itself, deduplicated by object id. The ctx-except-self rule keeps
// the walk's own remembers (some legs record only at ctx level) while
// leaving out the trigger REFERENT capture: a trigger that fires on its own
// source's movement carries that source in ctx.Remembered, Forge keeps the
// referent in the separate Triggered* property family, and counting it as
// card-level remembered would inflate every count (X would read the leaving
// Skyclave's mana value next to the exiled bear's). Players in ctx pass
// through after the objects. Deterministic (slices in order, no map range
// reaches a caller's output) and allocation-only: it writes no state and
// emits no event.
//
// imprintPileTargets resolves the SOURCE's persistent imprint association
// (state.Object.Imprinted + ImprintTokens): the exiled cards -- Imprint links
// an exiled card only while the linked card remains in exile (CR 607.2a); its
// persistent ID cannot follow it later -- plus the minted tokens an
// ImprintTokens$ True effect recorded. The tokens are battlefield permanents,
// so they resolve while they exist -- the exiled-card zone filter must not
// apply to them. One home shared by the "Imprinted" (non-RepeatSubject arm)
// and "ImprintedLKI" definedSpec cases, so the two spellings read one pile.
func imprintPileTargets(g *state.Game, c *Ctx) []state.Target {
	o := g.Obj(c.Source)
	if o == nil {
		return nil
	}
	out := make([]state.Target, 0, len(o.Imprinted)+len(o.ImprintTokens))
	for _, id := range o.Imprinted {
		if imprintAssociationContains(g, o, id) {
			out = append(out, state.Target{Obj: id})
		}
	}
	for _, id := range o.ImprintTokens {
		if imprintAssociationContains(g, o, id) {
			out = append(out, state.Target{Obj: id})
		}
	}
	// SeekFound (ImprintFound$ True) names cards the seek moved to a HAND:
	// Forge's continuation reads imprintedCards without a zone filter, so these
	// resolve wherever they currently sit. Kept in its own list so the CR
	// 607.2a exile-only rule above still holds for the ordinary Imprinted
	// association.
	for _, id := range o.SeekFound {
		if imprintAssociationContains(g, o, id) {
			out = append(out, state.Target{Obj: id})
		}
	}
	return out
}

// imprintAssociationContains is the shared liveness rule for Defined$ Imprinted
// and the IsImprinted object predicate. Ordinary imprint links expire when the
// linked card leaves exile; token and SeekFound associations have their own
// distinct zone semantics and only require the linked object to exist.
func imprintAssociationContains(g *state.Game, source *state.Object, id state.ObjID) bool {
	if source == nil {
		return false
	}
	linked := g.Obj(id)
	if linked == nil {
		return false
	}
	for _, linkedID := range source.Imprinted {
		if linkedID == id && linked.Zone == state.ZExile {
			return true
		}
	}
	for _, linkedID := range source.ImprintTokens {
		if linkedID == id {
			return true
		}
	}
	for _, linkedID := range source.SeekFound {
		if linkedID == id {
			return true
		}
	}
	return false
}

func rememberedWithSource(h Host, c *Ctx) []state.Target {
	out := make([]state.Target, 0, len(c.Remembered))
	seen := make(map[state.ObjID]bool, len(c.Remembered))
	if o := h.Game().Obj(c.Source); o != nil {
		for _, t := range o.Remembered {
			if !t.IsPlayer && !seen[t.Obj] {
				seen[t.Obj] = true
				out = append(out, t)
			}
		}
	}
	for _, t := range c.Remembered {
		if t.IsPlayer {
			out = append(out, t)
			continue
		}
		if t.Obj != c.Source && !seen[t.Obj] {
			seen[t.Obj] = true
			out = append(out, t)
		}
	}
	return out
}

// lkiControllerFor returns the last-known controller ChangeZone's
// RememberLKI$ move captured for id, if this resolution captured one. The
// live object cannot answer it: events.Apply's Move resets a battlefield
// departure's controller to its owner (CR 400.7). This is the read Forge's
// Card LKI copy gives readers such as RepeatEach's TokenOwner$
// ImprintedController (Curse of the Swine).
func lkiControllerFor(c *Ctx, id state.ObjID) (state.PlayerID, bool) {
	if id == 0 {
		return 0, false
	}
	for _, e := range c.ChangeZoneLKI {
		if e.Obj == id {
			return e.Controller, true
		}
	}
	return 0, false
}

// objectsOf returns Remembered's object entries (IsPlayer false) as a fresh
// slice -- never aliasing Ctx.Remembered, for the reason copyTargets' own
// doc comment gives.
func objectsOf(ts []state.Target) []state.Target {
	var out []state.Target
	for _, t := range ts {
		if !t.IsPlayer {
			out = append(out, t)
		}
	}
	return out
}

// playersOf returns Remembered's player entries (IsPlayer true) as a fresh
// slice.
func oneTriggerPlayer(t state.Target) []state.Target {
	if !t.IsPlayer {
		return nil
	}
	return []state.Target{t}
}

// plainRememberedSelector reports whether a Defined$/ValidPlayers selector is
// the plain Remembered family: it starts with "Remembered" and does NOT end
// with Controller or Owner. Forge's AbilityUtils.addPlayer maps a remembered
// CARD to its controller/owner only for those two suffixes; for every other
// Remembered spelling a remembered card contributes no player at all. The
// shared PlayerOf mapping would instead read a remembered card's controller
// for EVERY spelling, which is the leak this guards: a RepeatEach iteration's
// Remembered is the loop subject PLUS whatever the previous iteration
// RememberChose$, so a chooser defined as `Remembered` would otherwise add
// the previously chosen card's controller as a second chooser and re-ask that
// player with the collective pool (Summon: Valefor re-asking the first
// opponent on the second iteration).
func plainRememberedSelector(sel string) bool {
	if !strings.HasPrefix(sel, "Remembered") {
		return false
	}
	return !strings.HasSuffix(sel, "Controller") && !strings.HasSuffix(sel, "Owner")
}

func playersOf(ts []state.Target) []state.Target {
	var out []state.Target
	for _, t := range ts {
		if t.IsPlayer {
			out = append(out, t)
		}
	}
	return out
}

func controllersOf(g *state.Game, ts []state.Target) []state.Target {
	return relatedPlayers(g, ts, false)
}

func ownersOf(g *state.Game, ts []state.Target) []state.Target {
	return relatedPlayers(g, ts, true)
}

func relatedPlayers(g *state.Game, ts []state.Target, owner bool) []state.Target {
	seen := map[state.PlayerID]bool{}
	var out []state.Target
	for _, t := range ts {
		p := t.Player
		if !t.IsPlayer {
			o := g.Obj(t.Obj)
			if o == nil {
				continue
			}
			p = o.Controller
			if owner {
				p = o.Owner
			}
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, state.Target{Player: p, IsPlayer: true})
		}
	}
	return out
}

// copyTargets returns a defensive copy of s: same elements, independent
// backing array. A nil s yields nil, not an empty-but-non-nil slice, so
// Defined's observable results are unchanged for every input — only the
// aliasing is fixed.
// eventRemember records one remembered card on the resolution's source with
// the event-backed Choose entry Forge's host.addRemembered writes. The
// ctx-level list a chained SubAbility reads is the caller's job; this is the
// persistent half -- the source object's event-backed Remembered list, which
// survives the resolution and is what Card.IsRemembered and
// Count$RememberedSize read later (Forge's host card remembered list).
func eventRemember(h Host, c *Ctx, id state.ObjID) {
	if c.Source == 0 {
		return
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "remembered", IDs: []state.ObjID{id}})
}

// eventForgetChanged implements ForgetChanged$ True (Forge ChangeZoneEffect's
// host.removeRemembered on each moved card): the moved object leaves BOTH
// halves of the remembered state -- the resolution's Ctx.Remembered set and
// the source object's persistent event-backed Remembered list, which
// Card.IsRemembered and Count$RememberedSize read later. It self-gates on the
// parameter (Forge reads the key unconditionally per moved card), so callers
// pair it beside their RememberChanged$ handling without a second guard.
func eventForgetChanged(h Host, c *Ctx, sa *cards.SA, id state.ObjID) {
	if !strings.EqualFold(strings.TrimSpace(sa.Params["ForgetChanged"]), "True") {
		return
	}
	forgetRememberedOne(h, c, id)
}

// forgetRememberedOne drops ONE object from both halves of the remembered
// state -- the resolution's Ctx.Remembered set and the source object's
// persistent event-backed Remembered list (the "forget-remembered" Choose
// event events/apply.go folds) -- and is the one shared body for every
// forget rider: ForgetChanged$ (a zone change forgets what it moved) and
// Play's ForgetPlayed$ (a card the Play actually began to play is no longer
// a "you didn't play it" candidate). Callers own their own parameter gate.
func forgetRememberedOne(h Host, c *Ctx, id state.ObjID) {
	next := make([]state.Target, 0, len(c.Remembered))
	for _, t := range c.Remembered {
		if !t.IsPlayer && t.Obj == id {
			continue
		}
		next = append(next, t)
	}
	c.Remembered = next
	if c.Source != 0 {
		h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "forget-remembered", IDs: []state.ObjID{id}})
	}
}

// clearEventRemembered mirrors Forge host.clearRemembered.  Rider primitives
// call it before replacing their ctx set, so Count$RememberedSize and a later
// resolution observe exactly the same persistent set as the current chain.
// imprint records Forge's host.addImprintedCards. TargetedSource names the
// source card of a targeted stack ability when one exists; ordinary targets
// are themselves cards. The one shared resolver is used by every API so a
// future ImprintCards$ rider cannot be accidentally skipped by its primitive.
func imprint(h Host, c *Ctx, sa *cards.SA) {
	if c.Source == 0 || strings.TrimSpace(sa.Params["ImprintCards"]) == "" {
		return
	}
	var ids []state.ObjID
	for _, t := range Defined(h, c, &cards.SA{Params: map[string]string{"Defined": sa.Params["ImprintCards"]}}) {
		if t.IsPlayer {
			continue
		}
		id := t.Obj
		if sa.Params["ImprintCards"] == "TargetedSource" {
			if o := h.Game().Obj(id); o != nil && o.Source != 0 {
				id = o.Source
			}
		}
		if h.Game().Obj(id) != nil {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		h.Emit(events.Event{Kind: events.Imprint, Obj: c.Source, IDs: ids})
	}
}

func clearEventRemembered(h Host, c *Ctx) {
	if c.Source != 0 {
		if o := h.Game().Obj(c.Source); o != nil && len(o.Remembered) > 0 {
			h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "clear-remembered"})
		}
	}
}

// resolutionChosenCards is the shared read of the current resolution's
// chosen-card set: the ChooseCard binding (Ctx.Chosen, live when the choice
// resolved in this walk) or, across an ask's suspension, the source object's
// event-backed Chosen list (the Choose "chosen" fold). Count$ChosenSize,
// Defined$ ChosenCard and CopySpellAbility's DefinedTarget$ ChosenCard all
// resolve through it, so a count, a defined fetch and a per-target copy can
// never disagree about what "the chosen cards" names.
func resolutionChosenCards(g *state.Game, c *Ctx) []state.Target {
	if c.ChosenValid || len(c.Chosen) > 0 {
		return copyTargets(c.Chosen)
	}
	if o := g.Obj(c.Source); o != nil {
		return copyTargets(o.Chosen)
	}
	return nil
}

func copyTargets(s []state.Target) []state.Target {
	return append([]state.Target(nil), s...)
}

// moveZoneEvent preserves an exile's source provenance in MoveZone's existing
// IDs carrier. All effect primitives that move a card into exile use this one
// constructor, so ExiledWithSource is derived from the logged move rather than
// a live-only side table.
func moveZoneEvent(c *Ctx, id state.ObjID, from, to state.Zone) events.Event {
	ev := events.Event{Kind: events.MoveZone, Obj: id, From: from, To: to}
	if to == state.ZExile && exileProvenanceNeeded(c) {
		ev.IDs = []state.ObjID{c.Source}
	}
	return ev
}

// exileProvenanceNeeded avoids changing every ordinary exile event merely
// because it shares the movement primitive. A source needs the association
// only when its own compiled script later names ExiledWithSource; testing the
// immutable SVar table makes that decision stable through replay.
func exileProvenanceNeeded(c *Ctx) bool {
	if c == nil || c.Source == 0 {
		return false
	}
	for _, body := range c.SVars {
		if strings.Contains(body, "ExiledWithSource") {
			return true
		}
	}
	return false
}

// faceStaticsNameExiledWithSource reports whether the source CARD's own
// Static lines name ExiledWithSource -- the S: static spelling of the same
// provenance need the SVar scan in exileProvenanceNeeded covers (Intellect
// Devourer's MayPlay grant, the shared-fate family). Iterating the param map
// only feeds a boolean OR, so map order never reaches an event.
func faceStaticsNameExiledWithSource(h Host, src state.ObjID) bool {
	o := h.Game().Obj(src)
	if o == nil || o.Face() == nil {
		return false
	}
	for _, st := range o.Face().Statics {
		for k, v := range st.Params {
			switch k {
			case "Affected", "AffectedZone", "Description":
				// The keys a static names its card filters and text by; the
				// ExiledWithSource provenance claim lives in one of these.
				if strings.Contains(v, "ExiledWithSource") {
					return true
				}
			}
		}
	}
	return false
}

// PlayerOf resolves a target to a player: an explicit player target, or the
// controller of a targeted object.
func PlayerOf(h Host, c *Ctx, t state.Target) state.PlayerID {
	if t.IsPlayer {
		return t.Player
	}
	if o := h.Game().Obj(t.Obj); o != nil {
		return o.Controller
	}
	return c.Controller
}

// validStackToken is one comma-separated token of a Defined$ ValidStack spec.
// The kind half is state.StackKindToken -- the SAME kind/controller/card-type
// grammar target legality's TargetType$ census parses (rules delegates to
// state; effects cannot import rules, which is exactly why the grammar lives
// in state) -- plus the two qualifiers only a ValidStack spec carries:
//
//   - "Other" (Reverse the Polarity's, Swift Silence's `Spell.Other`): the
//     object is not the resolving ability's own source object. Same
//     relative-to-source reading the card-spec grammar's Other predicate
//     gives ValidTgts$.
//   - "sharesNameWith <card-spec>" (Grimoire Thief's
//     `Spell.sharesNameWith ExiledWithSource`): the object's face name is
//     the name of some card matching <card-spec>, evaluated with the
//     ordinary object matcher. An inner spec this build cannot resolve
//     (ExiledWithSource needs exile provenance it does not track) matches no
//     card, so the name set is empty and the token admits nothing -- the
//     fail-closed direction, never widened.
//   - "otherAbility" (Ulalek, Fused Atrocity's
//     `Ability.YouCtrl+otherAbility`): the object is not the ability
//     currently RESOLVING, nor any other instance or copy of that same
//     printed ability. The anchor is Ctx.ResolvingObj -- the stack-object
//     wrapper rules knows as e.resolvingObj / rp.obj -- NOT Ctx.Source,
//     which for an ability resolution is the source PERMANENT (Ruling
//     T20-b) and is not on the stack, so a Source-anchored exclusion would
//     exclude nothing and Ulalek's trigger would copy itself (each copy
//     asking its pay question again -- an unbounded regress). The family
//     half (same Source permanent AND same Ability pointer -- StackCopy
//     preserves both, and every mint of one printed trigger shares the
//     parsed slice's pointer) is the loop guard's second half, needed the
//     moment MORE THAN ONE instance of the trigger can be on the stack at
//     once: a paid Ulalek trigger's copy is itself an ability wrapper, the
//     copy asks the same {C}{C} question on resolution, and a
//     deterministic host answers it the same way every time -- the walk
//     never terminates. Oracle text would copy other instances and let
//     each copy's controller decline; this build's hosts cannot express a
//     decline (the recorded stand-in, see the abcopy3 row). A context with
//     ResolvingObj zero (a hand-built one, or a resolution path that never
//     set it) falls back to the Ctx.Source id alone -- never widened.
type validStackToken struct {
	kt             state.StackKindToken
	other          bool
	otherAbility   bool
	sharesNameWith string
}

// validStackTokens parses a Defined$ ValidStack spec into its tokens. A
// token whose base names no stack kind is skipped (the same non-stack-token
// rule the TargetType$ census applies); a spec with no usable token at all
// degrades to Spell-only -- the narrow default, never a widened one.
func validStackTokens(spec string) []validStackToken {
	spellOnly := validStackToken{kt: state.StackKindToken{Kinds: [3]bool{state.StackKindSpell: true}}}
	var toks []validStackToken
	for _, t := range strings.Split(spec, ",") {
		kt, ok := state.StackKindTokenOf(t)
		if !ok {
			continue
		}
		tok := validStackToken{kt: kt}
		_, rest, _ := strings.Cut(strings.TrimSpace(t), ".")
		for _, q := range strings.Split(rest, ".") {
			q = strings.TrimSpace(q)
			if inner, is := strings.CutPrefix(q, "sharesNameWith"); is {
				tok.sharesNameWith = strings.TrimSpace(inner)
				continue
			}
			// A `+`-compound qualifier (Ulalek's `YouCtrl+otherAbility`) is
			// one dot-split token; its halves are conjunctive. State's own
			// qualifier switch ignores the compound entirely, so the
			// controller half is recovered here onto tok.kt -- the token
			// validStackAdmits re-reads through state.StackKindAdmits. The
			// plain forms keep their exact state-side reading either way.
			for _, sub := range strings.Split(q, "+") {
				switch strings.TrimSpace(sub) {
				case "YouCtrl":
					tok.kt.YouCtrl = true
				case "OppCtrl":
					tok.kt.OppCtrl = true
				case "Other":
					tok.other = true
				case "otherAbility":
					tok.otherAbility = true
				}
			}
		}
		toks = append(toks, tok)
	}
	if len(toks) == 0 {
		return []validStackToken{spellOnly}
	}
	return toks
}

// validStackTargets resolves a Defined$ ValidStack spec to the stack objects
// it names, at resolution time, in stack order (the stack zone's arena
// order -- the same enumeration the target census uses). Kind membership and
// controller qualifiers go through state.StackKindAdmits, so this arm cannot
// drift from what target legality offers; Other, otherAbility and
// sharesNameWith are the ValidStack-only qualifiers validStackToken carries.
// The otherAbility exclusion anchors on the RESOLVING WRAPPER
// (Ctx.ResolvingObj, falling back to Ctx.Source when zero) and its whole
// printed-ability family -- see validStackToken's doc.
func validStackTargets(g *state.Game, spec string, c *Ctx) []state.Target {
	toks := validStackTokens(spec)
	// One name set per distinct sharesNameWith inner spec, built before any
	// admission test so a card's position on the stack cannot order anything.
	// Distinct specs are collected in token order and matched cards are
	// walked in arena order -- no map range reaches a target list.
	var nameSets map[string]map[string]bool
	for _, tok := range toks {
		if tok.sharesNameWith == "" {
			continue
		}
		if nameSets == nil {
			nameSets = map[string]map[string]bool{}
		}
		if _, done := nameSets[tok.sharesNameWith]; done {
			continue
		}
		names := map[string]bool{}
		sc := c.SpecContext(c.Controller)
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Face() == nil {
				continue
			}
			if MatchesObjectCtx(g, tok.sharesNameWith, o, sc) {
				names[o.Face().Name] = true
			}
		}
		nameSets[tok.sharesNameWith] = names
	}
	anchorID := c.ResolvingObj
	if anchorID == 0 {
		anchorID = c.Source // a hand-built context degrades to the same shape, never widened
	}
	// The anchor's family identity: same source permanent AND same Ability
	// pointer (the resolving wrapper's own mint). Nil when the anchor is a
	// spell resolution (a card object has no Ability -- the exclusion then
	// degrades to the plain id test below) or the anchor object is gone.
	var anchor *state.Object
	if anchorID != 0 {
		anchor = g.Obj(anchorID)
	}
	var out []state.Target
	for _, oid := range g.Zone(state.ZStack, 0) {
		o := g.Obj(oid)
		if o == nil {
			continue
		}
		if !validStackAdmits(toks, state.StackKindOf(g, o), o, o.Controller, c.Controller, c.Source, anchor, anchorID, nameSets) {
			continue
		}
		out = append(out, state.Target{Obj: oid})
	}
	return out
}

// validStackAdmits reports whether any token admits the stack object o.
// Token semantics are OR, the same as the TargetType$ census; each token's
// kind/controller half is state.StackKindAdmits on that one token, and the
// ValidStack-only qualifiers narrow it further.
func validStackAdmits(toks []validStackToken, k state.StackObjKind, o *state.Object,
	controller, you state.PlayerID, source state.ObjID, anchor *state.Object, anchorID state.ObjID,
	nameSets map[string]map[string]bool) bool {
	for _, tok := range toks {
		if !state.StackKindAdmits([]state.StackKindToken{tok.kt}, k, o, controller, you) {
			continue
		}
		if tok.other && o.ID == source {
			continue
		}
		if tok.otherAbility {
			// Same printed ability as the resolving one: the resolving wrapper
			// itself, every other instance of it, and every copy of either.
			if anchor != nil && anchor.Ability != nil &&
				o.Ability == anchor.Ability && o.Source == anchor.Source {
				continue
			}
			if o.ID == anchorID {
				continue
			}
		}
		if tok.sharesNameWith != "" {
			f := o.Face()
			if f == nil || !nameSets[tok.sharesNameWith][f.Name] {
				continue
			}
		}
		return true
	}
	return false
}

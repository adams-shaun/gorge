package oracle

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// fdnkernel.go: the adapter hooks for package kernel on a generated card
// set (dirac/cardset), with the FDN* functions wrapping the "fdn" set.

// fdnSet is the generated FDN set as reached through cardset.ByName. A missing
// table panics with ByName's error, which names the command that builds it.
func fdnSet() cardset.Set {
	s, err := cardset.ByName("fdn")
	if err != nil {
		panic(err)
	}
	return s
}

// SetDeckSupported reports whether every card of a gorge deck is in set and
// supported.
func SetDeckSupported(set cardset.Set, d []*cards.Card) bool {
	supported := set.Supported()
	for _, c := range d {
		i, ok := set.Lookup(cardName(c))
		if !ok || !supported[i] {
			return false
		}
	}
	return true
}

// FDNDeckSupported is SetDeckSupported over the "fdn" set.
func FDNDeckSupported(d []*cards.Card) bool { return SetDeckSupported(fdnSet(), d) }

// SetSide is RealKernel over a table built from set (SetMatchup's) with
// the generated table's adapter hooks: gorge ability indices through
// set.AbilityOrigin (a latent ability, granted / delayed / reflexive, has
// none), tokens named by their script's face name, and the ChGen* choice
// kinds canonicalised as gorge poses them.
func SetSide(set cardset.Set, tbl *kop.Table, cfg Config) SideFactory {
	pool := make([]int, len(tbl.Cards))
	var names []string
	for i := range tbl.Cards {
		pool[i] = -1
		if j, ok := set.Lookup(tbl.Cards[i].Name); ok {
			pool[i] = j
		} else {
			// a multi-face card's other face or a token is a pool entry
			// beside the deck cards: its abilities are that
			// face's.
			if names == nil {
				names = set.Names()
			}
			for j, n := range names {
				if n == tbl.Cards[i].Name {
					pool[i] = j
				}
			}
		}
	}
	rk := RealKernel(tbl, nil, cfg)
	return func(m *Matchup, seed uint64) (Side, error) {
		side, err := rk(m, seed)
		if err != nil {
			return nil, err
		}
		r := side.(*realSide)
		// a token's gorge name is its script's face name ("Goblin"), the
		// table's is the script stem ("r_1_1_goblin")
		for i := range tbl.Cards {
			if tbl.Cards[i].Types&kop.TToken != 0 && m.reg != nil {
				if tc, ok := m.reg.Token(tbl.Cards[i].Name); ok {
					r.gname[i] = cardName(tc)
				}
			}
		}
		// a token's other face shares its card's gorge name (the front
		// face's): gorge keeps one token card and flips FaceIdx, while the
		// kernel turns the object onto the face's CardID.
		for i := range tbl.Cards {
			if f := tbl.Cards[i].Faces; len(f) > 0 && int(f[0]) < len(r.gname) {
				r.gname[i] = r.gname[f[0]]
			}
		}
		// a decayed token's Attacks trigger is gorge's keyword trigger
		// (lower_decayed.go): a stack object with no face index
		decayed := make([]bool, len(tbl.Cards))
		for i := range tbl.Cards {
			if tbl.Cards[i].Types&kop.TToken == 0 || m.reg == nil {
				continue
			}
			if tc, ok := m.reg.Token(tbl.Cards[i].Name); ok && len(tc.Faces) > 0 && tc.Faces[0] != nil {
				for _, k := range tc.Faces[0].Keywords {
					decayed[i] = decayed[i] || k == "Decayed"
				}
			}
		}
		r.choose = fdnChoose
		r.gdk = fdnDKind
		// a kernel-native ward trigger is gorge's Ward keyword trigger
		ward := make([]uint8, len(tbl.Cards))
		for i := range tbl.Cards {
			ward[i] = 255
			if m.reg == nil {
				continue
			}
			if gc, ok := m.reg.Lookup(tbl.Cards[i].Name); ok && len(gc.Faces) > 0 && gc.Faces[0] != nil {
				for j, tr := range gc.Faces[0].Triggers {
					if tr.Params["Keyword"] == "Ward" {
						ward[i] = 0x80 | uint8(j)
					}
				}
			}
		}
		r.origin = func(c dirac.CardID, ab uint8) (uint8, bool) {
			if ab == 253 { // kernel abWard
				return ward[c], ward[c] != 255
			}
			if ab >= 253 || int(c) >= len(pool) || pool[c] < 0 {
				return 0, false
			}
			if abs := tbl.Cards[c].Abilities; int(ab) < len(abs) && abs[ab].AFlags&(kop.AfLatent|kop.AfStation) != 0 {
				// a Station ability: gorge's minted body has no face index
				return 255, true
			}
			if decayed[c] && int(ab) < len(tbl.Cards[c].Abilities) && tbl.Cards[c].Abilities[ab].Kind == kop.AbTriggered {
				return 255, true
			}
			o := set.AbilityOrigin(pool[c], int(ab))
			return o, o != 0xFF
		}
		return r, nil
	}
}

// FDNSide is SetSide over the "fdn" set.
func FDNSide(tbl *kop.Table, cfg Config) SideFactory { return SetSide(fdnSet(), tbl, cfg) }

// fdnChoose canonicalises the generated table's resolution asks as gorge
// poses them (doc.go's grammar; option kinds from gorge's KChoose).
func fdnChoose(r *realSide, a dirac.Action) (Canon, bool) {
	kind := func(k string) CanonKind {
		c, _ := chooseCode(k)
		return ActChoose + CanonKind(c)
	}
	src, mode := a.Source(), a.Mode()
	top := RefNone
	if n := r.kp.NStack; n > 0 {
		top = Ref(r.kp.Stack[n-1].Source)
	}
	switch r.d.Sub {
	case dirac.ChWardPay:
		// gorge poses a ward cost as modes [pay, decline] (decline alone
		// when it can't be paid)
		if r.d.N == 1 || mode == 1 {
			return Pack(ActMode, RefNone, RefNone, 0), true
		}
		return Pack(ActMode, RefNone, RefNone, 1), true
	case dirac.ChGenSearch:
		return Pack(kind("search"), src, RefNone, 0), true
	case dirac.ChGenKczZone:
		// gorge's originAlternativeZones pick: the option index (0 the
		// library, 1 the alternative zone)
		return Pack(kind("zone"), RefNone, RefNone, mode), true
	case dirac.ChGenHiddenPick:
		return Pack(kind("hidden_pick"), src, RefNone, 0), true
	case dirac.ChGenHandMove:
		return Pack(kind("hand_move"), src, RefNone, 0), true
	case dirac.ChGenReveal:
		return Pack(kind("reveal"), src, RefNone, 0), true
	case dirac.ChGenDig:
		return Pack(kind("dig"), src, RefNone, 0), true
	case dirac.ChGenBolster:
		return Pack(kind("counter_pick"), src, RefNone, 0), true
	case dirac.ChGenProlif:
		// gorge's proliferate pick: a permanent by Obj, a player by Player
		if src >= dirac.PlayerTarget0 {
			return Pack(kind("proliferate"), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
		}
		return Pack(kind("proliferate"), src, RefNone, 0), true
	case dirac.ChCastDivide:
		// gorge's CR 601.2d division ask: a repeatable "divide" pick over
		// the targets, a player target named by Player
		if src >= dirac.PlayerTarget0 {
			return Pack(kind("divide"), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
		}
		return Pack(kind("divide"), src, RefNone, 0), true
	case dirac.ChGenDivide:
		// gorge's resolution division: a repeatable pick over the targets
		if src >= dirac.PlayerTarget0 {
			return Pack(kind("player"), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
		}
		return Pack(kind("card"), src, RefNone, 0), true
	case dirac.ChKsrTarget:
		// gorge's effChangeTargets "Choose new target" KChoose: a "player"
		// option for a player, a "card" option for an object or a stack
		// ability
		if src >= dirac.PlayerTarget0 {
			return Pack(kind("player"), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
		}
		return Pack(kind("card"), src, RefNone, 0), true
	case dirac.ChGenChooseCard, dirac.ChGenResTarget, dirac.ChGenChooseEach:
		// (ChGenChooseEach: gorge's ChooseCard per permanent type, Liliana's -9)
		return Pack(kind("card"), src, RefNone, 0), true
	case dirac.ChGenEnchant:
		return Pack(kind("enchant"), src, RefNone, 0), true
	case dirac.ChGenClone:
		// gorge's ETB "clone" ask: the templates by object, then "Enter as
		// itself" naming no object
		if mode == 1 {
			return Pack(kind("clone"), RefNone, RefNone, 0), true
		}
		return Pack(kind("clone"), src, RefNone, 0), true
	case dirac.ChGenPayPick:
		if blight, modes := r.s.FeatPayBlight(); blight {
			if modes {
				return Pack(kind("blightcost"), src, RefNone, 0), true
			}
			return Pack(kind("trigger_cost_blight"), src, RefNone, 0), true
		}
		return Pack(kind("sacrifice"), src, RefNone, 0), true
	case dirac.ChGenBlightPick:
		return Pack(kind("blight"), src, RefNone, 0), true
	case dirac.ChCostBlight:
		return Pack(kind("blightcost"), src, RefNone, 0), true
	case dirac.ChGenPayReturn:
		return Pack(kind("returncost"), src, RefNone, 0), true
	case dirac.ChGenPayExile:
		return Pack(kind("exile_cost"), src, RefNone, 0), true
	case dirac.ChGenPileSplit:
		return Pack(kind("twopiles"), src, RefNone, 0), true
	case dirac.ChGenPileChoose:
		if mode == 0 {
			return Pack(kind("pile-a"), RefNone, RefNone, 0), true
		}
		return Pack(kind("pile-b"), RefNone, RefNone, 1), true
	case dirac.ChGenTopBottom:
		if mode == 1 {
			// gorge's "primary" option names no player: its zero PlayerID
			return Pack(kind("primary"), RefNone, RefPlayer, 0), true
		}
		return Pack(kind("bottom"), RefNone, RefNone, 0), true
	case dirac.ChGenPayMode:
		// gorge lists [pay, decline]; an unpayable cost leaves decline alone
		if r.d.N == 1 || mode == 1 {
			return Pack(ActMode, RefNone, RefNone, 0), true
		}
		return Pack(ActMode, RefNone, RefNone, 1), true
	case dirac.ChGenPay:
		if mode == 1 {
			return Pack(kind("trigger_cost_pay"), top, RefNone, 0), true
		}
		return Pack(kind("trigger_cost_decline"), top, RefNone, 0), true
	case dirac.ChGenBranch:
		// gorge lists only the payable choices, indexed densely
		n := uint8(0)
		for _, o := range r.d.Options[:r.d.N] {
			if o.Mode() < mode {
				n++
			}
		}
		return Pack(ActMode, RefNone, RefNone, n), true
	case dirac.ChGenCastFree:
		// gorge's modes over the castable cards, in object order, an index
		// kept after a cast
		return Pack(ActMode, RefNone, RefNone, r.s.CastFreeIndex(dirac.Obj(src))), true
	case dirac.ChGenLookAck:
		return Pack(kind("yes"), RefNone, RefNone, 0), true
	case dirac.ChGenExplore:
		// gorge's explore: a KChoose of "graveyard" / "top" over the revealed card
		if mode == 1 {
			return Pack(kind("top"), src, RefNone, 0), true
		}
		return Pack(kind("graveyard"), src, RefNone, 0), true
	case dirac.ChGenEndure:
		// gorge's endure: a KChoose of "endure_counters" / "endure_spirit"
		// over the permanent
		if mode == 1 {
			return Pack(kind("endure_spirit"), src, RefNone, 0), true
		}
		return Pack(kind("endure_counters"), src, RefNone, 0), true
	case dirac.ChGenTapOrUntap:
		// gorge's taporuntap: a KChoose of "tap" / "untap" over the permanent
		if mode == 1 {
			return Pack(kind("tap"), src, RefNone, 0), true
		}
		return Pack(kind("untap"), src, RefNone, 0), true
	case dirac.ChGenPeekReveal:
		// gorge's reveal_optional over a PeekAndReveal: yes carries the
		// top card as its Obj, no names nothing
		if mode == 1 {
			return Pack(kind("yes"), src, RefNone, 0), true
		}
		return Pack(kind("no"), RefNone, RefNone, 0), true
	case dirac.ChGenConfirm:
		// gorge's hand_move_confirm: a KChoose of yes / no
		if mode == 1 {
			return Pack(kind("yes"), RefNone, RefNone, 0), true
		}
		return Pack(kind("no"), RefNone, RefNone, 0), true
	case dirac.ChRoom:
		// gorge's venture "room" KChoose: the option index in the room's
		// printed NextRoom$ order
		return Pack(kind("room"), RefNone, RefNone, mode), true
	case dirac.ChCloneCopyPick:
		// gorge's clone_choice KChoose over battlefield permanents
		return Pack(kind("permanent"), src, RefNone, 0), true
	case dirac.ChHideaway:
		// gorge's hideaway_pick KChoose over the top cards
		return Pack(kind("hideaway"), src, RefNone, 0), true
	case dirac.ChTrigX:
		// gorge's triggeredCostXAsk: a KChoose of "trigger_cost_x", the
		// option's Amount the X; canonical like the NumTimes count (canon.go),
		// the kernel's X pick (Shanna)
		return Pack(kind("x"), RefNone, RefNone, mode), true
	case dirac.ChWardDiscard:
		// gorge's ward_discard KChoose over the payer's hand
		return Pack(kind("ward_discard"), src, RefNone, 0), true
	case dirac.ChWardSac:
		// gorge's ward_sac KChoose over the payer's matching permanents
		return Pack(kind("ward_sac"), src, RefNone, 0), true
	case dirac.ChCostBehold:
		// gorge's beholdCostAsk KChoose over the battlefield then hand
		// candidates
		return Pack(kind("beholdcost"), src, RefNone, 0), true
	case dirac.ChCostEvidence:
		// gorge's evidence KChoose over the payer's graveyard
		return Pack(kind("evidence"), src, RefNone, 0), true
	case dirac.ChWardEvidence:
		// gorge's ward_evidence KChoose over the payer's graveyard
		return Pack(kind("ward_evidence"), src, RefNone, 0), true
	case dirac.ChGenUntapPick:
		// gorge's "untap" KChoose over the UntapType$ candidates
		return Pack(kind("untap"), src, RefNone, 0), true
	case dirac.ChGenDiscUnless:
		// gorge's discard_unless: a KChoose of unless / ordinary, both
		// canonical kinds naming no object
		if mode == 1 {
			return Pack(kind("unless"), RefNone, RefNone, 0), true
		}
		return Pack(kind("ordinary"), RefNone, RefNone, 0), true
	case dirac.ChGenMayShuffle:
		if mode == 1 {
			return Pack(kind("yes"), RefNone, RefNone, 0), true
		}
		return Pack(kind("no"), RefNone, RefNone, 0), true
	case dirac.ChRemCtrKind:
		// gorge's counter_kind pick (removeCounterAny), compared by the
		// counter kind (mode), not gorge's carrier-order position
		return Pack(kind("counter"), RefNone, RefNone, mode), true
	case dirac.ChGenColor:
		// gorge indexes its colour options densely: with an Exclude$
		// colour dropped (Citadel Gate) the index is the colour's rank
		// among the offered ones (identity over all five)
		n := uint8(0)
		for _, o := range r.d.Options[:r.d.N] {
			if o.Mode() < mode {
				n++
			}
		}
		return Pack(kind("color"), RefNone, RefNone, n), true
	case dirac.ChGenManaChoice:
		// a resolving Mana effect's colour pick (Chandra, Hope's
		// Beacon): gorge's "mana" option, valued by its colour
		return Pack(kind("mana"), RefNone, RefNone, mode), true
	case dirac.ChGenType:
		// the vocabulary index in (target, mode), as gorge's labelValue
		return Pack(kind("type"), RefNone, Ref(a.Target()), mode), true
	case dirac.ChGenReplOrder:
		// gorge's KReplacement order ask: one option per replacement, its source
		// and occurrence among that source's options (the kernel mode is the
		// global scan index; two replacements can share a source, e.g. a
		// recipient's prevention and its shield counter)
		occ := uint8(0)
		for _, o := range r.d.Options[:r.d.N] {
			if o.Source() == src && o.Mode() < mode {
				occ++
			}
		}
		return Pack(ActReplacement, src, RefNone, occ), true
	case dirac.ChGenPip:
		// gorge's pip announcement (rules manaAsk): pay_W..pay_G
		return Pack(kind("pay"), RefNone, RefNone, mode), true
	case dirac.ChGenLeyline:
		if mode == 1 {
			return Pack(kind("opening_yes"), src, RefNone, 0), true
		}
		return Pack(kind("opening_no"), src, RefNone, 0), true
	case dirac.ChRoomDoorHalf:
		// gorge effUnlockDoor's "Lock or unlock a door?" KChoose
		if mode == 1 {
			return Pack(kind("lock"), RefNone, RefNone, 0), true
		}
		return Pack(kind("unlock"), RefNone, RefNone, 0), true
	case dirac.ChRoomDoor:
		// gorge effUnlockDoor's "Choose a Room door" KChoose: both doors name
		// the same Room and gorge's disambiguate gives the k-th repeat mode k
		// in option (face) order; the kernel carries that ordinal in Target.
		return Pack(kind("card"), src, RefNone, a.Target()), true
	}
	return 0, false
}

// fdnDKind is the decision kind gorge poses a generated-table ask as.
func fdnDKind(r *realSide) (DKind, bool) { return fdnSubKind(r.d.Sub) }

func fdnSubKind(sub dirac.ChoiceKind) (DKind, bool) {
	switch sub {
	case dirac.ChGenBranch, dirac.ChGenCastFree, dirac.ChGenPayMode, dirac.ChWardPay:
		return DModes, true
	case dirac.ChGenReplOrder:
		return DReplacement, true
	}
	return 0, false
}

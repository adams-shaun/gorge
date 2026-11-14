package oracle

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// realkernel.go adapts package kernel
// to the oracle's Side: it canonicalises the kernel's Actions (Ak*) into the
// oracle's grammar (doc.go) and converts dirac.Projection into the oracle's
// Projection. Where the kernel's action does not carry what the canonical
// form needs (a trigger-order pick names a pending-trigger index, not the
// trigger's source; a DkChoose pick does not say what is chosen), the
// translation is the best the exported API allows and is listed in
// RealKernelGaps.

// RealKernelGaps lists what the adapter cannot translate 1:1 with the
// kernel's exported API (the API the oracle needs beyond the contract).
var RealKernelGaps = []string{
	"ChOptional names no source: the adapter takes the resolving entry's object (State.FeatSource).",
	"ChWardPay has no gorge counterpart in the canonical grammar yet (mapped to choose:number).",
}

// KernelMatchup builds the gorge matchup for two kernel decks of tbl: each
// card resolved in reg by its kernel name, through names (kernel name ->
// gorge name) when it maps one.
func KernelMatchup(reg *cards.Registry, tbl *kop.Table, labels [2]string, decks [2][]kop.CardID, names map[string]string, mulligans int) (*Matchup, error) {
	return KernelMatchupN(reg, tbl, labels[:], decks[:], names, mulligans)
}

// KernelMatchupN is KernelMatchup at len(decks) seats (2..MaxSeats), seat s
// playing decks[s] under labels[s]; at two seats it is KernelMatchup.
func KernelMatchupN(reg *cards.Registry, tbl *kop.Table, labels []string, decks [][]kop.CardID, names map[string]string, mulligans int) (*Matchup, error) {
	gd := make([][]*cards.Card, len(decks))
	for s := range decks {
		for _, id := range decks[s] {
			n := tbl.Cards[id].Name
			if g, ok := names[n]; ok {
				n = g
			}
			c, ok := reg.Lookup(n)
			if !ok {
				return nil, fmt.Errorf("kernel card %q: %q not in the gorge corpus", tbl.Cards[id].Name, n)
			}
			gd[s] = append(gd[s], c)
		}
	}
	return NewMatchupN(reg, labels, gd, mulligans)
}

// RealKernel is a SideFactory over package kernel playing tbl. The matchup's
// gorge decks are mapped back to tbl's cards by name (names: kernel name ->
// gorge name, as given to KernelMatchup); a name the map does not fix up
// resolves through the corpus (canon.GorgeName). Elision follows cfg.PassElision
// (dirac.Config.NoElide when off).
func RealKernel(tbl *kop.Table, names map[string]string, cfg Config) SideFactory {
	return func(m *Matchup, seed uint64) (Side, error) {
		gorgeName := func(n string) string {
			if g, ok := names[n]; ok {
				n = g
			}
			return canon.GorgeName(m.reg, n)
		}
		rev := make(map[string]kop.CardID, len(tbl.Cards))
		for i, c := range tbl.Cards {
			n := gorgeName(c.Name)
			if _, dup := rev[n]; !dup {
				rev[n] = kop.CardID(i)
			}
		}
		n := m.NSeat()
		if n > dirac.MaxSeats {
			return nil, fmt.Errorf("realkernel: a matchup of %d seats; this kernel build holds %d (the dirac_mp build tag holds %d)", n, dirac.MaxSeats, MaxSeats)
		}
		var decks [dirac.MaxSeats][]kop.CardID
		for s := 0; s < n; s++ {
			for _, c := range m.SeatDeck(s) {
				id, ok := rev[cardName(c)]
				if !ok {
					return nil, fmt.Errorf("realkernel: %q is not in table %s", cardName(c), tbl.Name)
				}
				decks[s] = append(decks[s], id)
			}
		}
		var sides [2][]kop.CardID
		for s := 0; s < 2; s++ {
			for _, c := range m.Sideboards[s] {
				id, ok := rev[cardName(c)]
				if !ok {
					return nil, fmt.Errorf("realkernel: sideboard %q is not in table %s", cardName(c), tbl.Name)
				}
				sides[s] = append(sides[s], id)
			}
		}
		r := &realSide{tbl: tbl, m: m, n: uint8(n)}
		m.chars = newCharVocab(tbl)
		r.gname = make([]string, len(tbl.Cards))
		r.tidx = make([]CardIndex, len(tbl.Cards))
		for i, c := range tbl.Cards {
			n := c.Name
			if len(c.Faces) > 0 {
				n = tbl.Cards[c.Faces[0]].Name // a face is its card (gorge names the front)
			}
			n = gorgeName(n)
			r.gname[i] = n
			r.tidx[i] = NoCardIndex
			for j, t := range m.Table {
				if t == n {
					r.tidx[i] = CardIndex(j)
				}
			}
		}
		r.kcfg = dirac.LockstepConfig(cfg.surface())
		r.kcfg.NoElide = !cfg.PassElision
		r.kcfg.Mulligans = uint8(m.Mulligans)
		r.kcfg.Commander, r.kcfg.Commanders = m.kernelCommanders()
		r.kcfg.RecordChance = cfg.RecordChance
		r.kcfg.ArrangeRestOrder = cfg.ArrangeRestOrder
		for s := 0; s < 2; s++ {
			if len(sides[s]) > dirac.MaxSide {
				return nil, fmt.Errorf("realkernel: sideboard of %d cards, the kernel holds %d", len(sides[s]), dirac.MaxSide)
			}
			r.kcfg.SideN[s] = uint8(copy(r.kcfg.Side[s][:], sides[s]))
		}
		r.s.InitN(tbl, decks, uint8(n), seed, r.kcfg)
		return r, nil
	}
}

// surface is the kernel surface cfg's GorgeAutopaySurface selects.
func (cfg Config) surface() dirac.Surface {
	if cfg.GorgeAutopaySurface {
		return dirac.SurfaceAutopay
	}
	return dirac.SurfaceFull
}

type realSide struct {
	s     dirac.State
	kcfg  dirac.Config // what s was built with
	tbl   *kop.Table
	m     *Matchup
	n     uint8       // seats
	gname []string    // per kernel CardID: the gorge card name
	tidx  []CardIndex // per kernel CardID: the matchup table index
	d     dirac.Decision
	opts  []Canon
	kp    dirac.Projection
	// nameless: tokens seen as dirac.ProjNameless. Gorge keeps a copied token's
	// blank card after it leaves, and the kernel's object holds the printed one.
	nameless [(MaxInst + 63) / 64]uint64
	// origin, when set, maps a card's kop ability index to gorge's
	// (SetSide: set.AbilityOrigin).
	origin func(dirac.CardID, uint8) (uint8, bool)
	// generated: the table is kgen's, whose ChGen* asks have their own canonical forms.
	generated bool
	tr        canon.Translator
}

// OccupancyHook, when set, sees the kernel's state at every decision the
// lockstep poses (kfuzz -stats). The kernel itself carries no tracking.
var OccupancyHook func(*dirac.State)

// KernelStateOf is the kernel state behind a RealKernel side, the decision its
// last Step posed and that decision's options as Canons, in the decision's order.
// All nil for any other side.
func KernelStateOf(s Side) (*dirac.State, *dirac.Decision, []Canon) {
	r, ok := s.(*realSide)
	if !ok {
		return nil, nil, nil
	}
	return &r.s, &r.d, r.opts
}

// translator reads r's position and posed decision.
func (r *realSide) translator() *canon.Translator {
	r.tr = canon.Translator{S: &r.s, D: &r.d, KP: &r.kp, Tbl: r.tbl, GName: r.gname, Origin: r.origin, Generated: r.generated}
	if r.m != nil {
		r.tr.Reg = r.m.reg
	}
	return &r.tr
}

func (r *realSide) canon(a dirac.Action) Canon { return r.translator().Action(a) }

func (r *realSide) dkind() DKind { return r.translator().DKind() }

func (r *realSide) cardAbility(card dirac.CardID, idx uint8) uint8 {
	return r.translator().CardAbility(card, idx)
}

func (r *realSide) stackAbility(srcCard, card dirac.CardID, ab uint8) uint8 {
	return r.translator().StackAbility(srcCard, card, ab)
}

func (r *realSide) Step() CDecision {
	r.s.StepInto(&r.d)
	if OccupancyHook != nil {
		OccupancyHook(&r.s)
	}
	r.s.Project(&r.kp)
	r.opts = r.opts[:0]
	for _, a := range r.d.Options[:r.d.N] {
		r.opts = append(r.opts, r.canon(a))
	}
	out := CDecision{Kind: r.dkind(), Player: r.d.Player, Options: append([]Canon(nil), r.opts...)}
	sortCanon(out.Options)
	return out
}

func (r *realSide) Apply(c Canon) {
	for i, x := range r.opts {
		if x == c {
			r.s.Apply(r.d.Options[i])
			return
		}
	}
	panic(fmt.Sprintf("realkernel: %v is not one of the kernel's options", c))
}

func (r *realSide) Terminal() (bool, int8) { return r.s.Terminal() }

// Decided implements Side: the kernel ends the game when it is decided, so
// its decided result is its terminal result.
func (r *realSide) Decided() (bool, int8) { return r.s.Terminal() }

// Dirty implements Side. The kernel has no commit boundary to expose, so it
// reports dirty at every posed decision; mixed runs use the gorge side's
// Dirty as the comparison cadence either way.
func (r *realSide) Dirty() bool { return true }

// Turn implements Side.
func (r *realSide) Turn() int { return r.s.Turn() }

// Err implements Side: the kernel has no deferred internal failure; a
// rejected action panics.
func (r *realSide) Err() error { return nil }

// CapacityStall names the capacity the kernel's game was stopped at
// (State.StallReason), "" when it was not.
func (r *realSide) CapacityStall() string {
	return dirac.StallName(r.s.StallReason())
}

// StallIsUnmodelled reports whether the game stopped at a rule gap rather than a
// declared capacity (dirac.StallClassOf).
func (r *realSide) StallIsUnmodelled() bool {
	return dirac.StallIsUnmodelled(r.s.StallReason())
}

// Project converts the kernel's projection into the oracle's.
func (r *realSide) Project(p *Projection) {
	r.s.Project(&r.kp)
	r.s.ProjectChars(&r.kp)
	kp := &r.kp
	*p = Projection{}
	p.NInst = kp.NObj
	for i := range p.Cards {
		p.Cards[i] = CardProj{Card: NoCardIndex, Zone: ZoneNone, Pos: 255, AttachedTo: RefNone, Blocks: RefNone}
	}
	for i := 0; i < int(kp.NObj) && i < MaxInst; i++ {
		o := &kp.Objs[i]
		c := &p.Cards[i]
		pc := o.Card
		if o.Flags&dirac.ProjBase != 0 {
			pc = o.Base // gorge projects a copy effect's permanent as its printed card
		}
		if int(pc) < len(r.gname) {
			c.NameHash = nameHash(r.gname[pc])
			if o.Flags2&dirac.ProjNameless != 0 {
				r.nameless[i/64] |= 1 << (i % 64)
			}
			if r.nameless[i/64]>>(i%64)&1 != 0 {
				c.NameHash = nameHash("")
			}
			c.Face = r.tbl.Cards[pc].Face
			if cd := &r.tbl.Cards[pc]; cd.FaceMode == kop.FacePrototype {
				c.Face = 0 // the prototype def is the card's one printed face (gorge keeps FaceIdx 0 and a cast flag)
			}
			if !o.Token && o.Flags&dirac.ProjCopy == 0 { // a spell copy is no deck card (gorge project.go)
				c.Card = r.tidx[pc]
			}
		}
		switch {
		case o.Zone == kop.ZoneNone:
			c.Zone = ZoneCeased
		case o.Zone.Phased():
			c.Zone = uint8(kop.ZoneBattlefield) - 1 // a phased-out permanent is still on the battlefield (CR 702.26d)
		case o.Zone == kop.ZoneSideboard:
			c.Zone = uint8(state.ZSideboard)
		default:
			c.Zone = uint8(o.Zone) - 1
		}
		c.Owner, c.Controller = o.Owner, o.Ctrl
		switch o.Zone {
		case kop.ZoneLibrary, kop.ZoneBattlefield, kop.ZoneGraveyard, kop.ZoneExile, kop.ZonePhased, kop.ZonePhasedInd:
			c.Pos = o.ZoneIdx
		}
		var f uint16
		if o.Flags&dirac.ProjTapped != 0 {
			f |= FTapped
		}
		if o.Flags&dirac.ProjCopy != 0 {
			f |= FCopy
		}
		if o.Flags&dirac.ProjSick != 0 {
			f |= FSummonSick
		}
		if o.Flags&dirac.ProjAttacking != 0 {
			f |= FAttacking
		}
		if o.Flags&dirac.ProjBlocking != 0 {
			f |= FBlocking
		}
		if o.Flags&dirac.ProjMonstrous != 0 {
			f |= FMonstrous
		}
		if o.Flags2&dirac.ProjSuspected != 0 {
			f |= FSuspected
		}
		if o.Flags2&dirac.ProjRenowned != 0 {
			f |= FRenowned
		}
		if o.Zone.Phased() {
			f |= FPhasedOut
		}
		if o.Flags&dirac.ProjFaceDown != 0 {
			f |= FFaceDown
		}
		if o.Token {
			f |= FToken
		}
		c.Flags = f
		c.Damage = int16(o.Damage)
		c.Power, c.Toughness = int16(o.Power), int16(o.Tough)
		for k, n := range o.Counters {
			slot := CtrOther
			switch kop.CounterKind(k) {
			case kop.CtrP1P1:
				slot = CtrP1P1
			case kop.CtrM1M1:
				slot = CtrM1M1
			case kop.CtrLoyalty:
				slot = CtrLoyalty
			case kop.CtrShield:
				slot = CtrShieldC
			}
			c.Counters[slot] += int16(n)
		}
		for x, n := range o.CountersX {
			c.Counters[CtrX+x] += int16(n)
		}
		// gorge's deathtouch mark is a counter of its own (InternalCounterMarker)
		c.Counters[CtrOther] += int16(r.s.FeatDeathtouchMark(dirac.Obj(i)))
		// and so is its regeneration shield ("Shield", the same marker family)
		c.Counters[CtrOther] += int16(r.s.FeatRegenShields(dirac.Obj(i)))
		c.AttachedTo, c.Blocks = Ref(o.Attached), Ref(o.Blocks)
		if o.Zone == kop.ZoneBattlefield || o.Zone.Phased() {
			ch := &kp.Chars[i]
			c.Chars = CharsProj{Types: uint16(ch.Types), Subtypes: ch.Subs, AllTypes: ch.AllSub, Colors: uint8(ch.Colors), Keywords: uint32(ch.Kw &^ kwStaticBits)}
		}
	}
	p.Initiative = r.s.Initiative() // CR 726 (act_initiative.go)
	p.DayNight = r.s.DayNight()     // CR 731 (kw_daynight.go)

	p.Monarch = uint8(r.s.Monarch() + 1) // CR 725 (act_monarch.go)
	for s := 0; s < int(r.n) && s < MaxSeats; s++ {
		sp := p.seat(s)
		*sp.room = r.s.DungeonRoom(uint8(s))
		*sp.life = int32(kp.Life[s])
		*sp.poison = int16(r.s.Poison(uint8(s))) // infect / toxic (kw_infect.go)
		if s < 2 {
			// the kernel packs speed for two seats; a further seat's stays zero
			*sp.speed = uint8(r.s.Speed(uint8(s))) // CR 702.179 (kw_speed.go)
		}
		if uint8(s) == kp.Active {
			*sp.lands = kp.Lands[s] // the active player's only (project.go)
		}
		for k := 0; k < 6 && k < len(kp.Pool[s]); k++ {
			sp.pool[k] = int16(kp.Pool[s][k])
		}
		*sp.lib, *sp.hand = kp.LibN[s], kp.HandN[s]
		*sp.casts = kp.CmdCast[s]
		for q := 0; q < int(r.n) && q < MaxSeats; q++ {
			*p.cmdDamage(s, q) = kp.CmdDmg[s][q]
		}
	}
	if r.n > 2 {
		p.Seats.NSeat = r.n
		for s := 0; s < int(r.n) && s < MaxSeats; s++ {
			p.Seats.Lost[s] = r.s.FeatLost(uint8(s))
		}
	}
	p.NStack = kp.NStack
	_, _, building := r.s.BuildingCast()
	for i := 0; i < int(kp.NStack) && i < MaxStack; i++ {
		e := &kp.Stack[i]
		st := &p.Stack[i]
		st.Kind, st.Source, st.Controller = e.Kind, Ref(e.Source), e.Ctrl
		st.Ability = 255
		st.Modes = r.stackModes(e)
		if !building || i < int(kp.NStack)-1 {
			st.X = int16(e.X) // gorge writes Object.X as the cast commits, not as X is announced
		}
		if e.Kind != 0 {
			srcCard := e.Card
			if int(e.Source) < int(kp.NObj) {
				srcCard = kp.Objs[e.Source].Card
			}
			st.Ability = r.stackAbility(srcCard, e.Card, e.Ability)
		}
		for k := range st.Targets {
			st.Targets[k] = RefNone
		}
		for _, slot := range r.stackTargetOrder(e) {
			si, t := int(slot), e.Targets[slot]
			if dirac.TCode(t) != dirac.NoTarget && int(st.NTargets) < MaxTgts {
				st.Targets[st.NTargets] = Ref(t)
				if t == dirac.StackTargetGone {
					st.Targets[st.NTargets] = RefNone // a stack ability that left (gorge's vanished object)
				}
				st.NTargets++
			}
			for j := 0; j < int(e.NExtra); j++ {
				if int(e.ExtraSlot[j]) == si && int(st.NTargets) < MaxTgts {
					st.Targets[st.NTargets] = Ref(e.Extra[j])
					st.NTargets++
				}
			}
		}
	}
	p.Turn = int32(kp.Turn)
	switch st := kp.Step; {
	case st <= kop.StepBlockers:
		p.Step = uint8(st)
	case st == kop.StepFirstStrikeDamage || st == kop.StepDamage:
		p.Step = 7 // gorge's single combat-damage step
	default:
		p.Step = uint8(st) - 1
	}
	p.Active, p.Priority = kp.Active, kp.Prio
	if kp.Turn == 0 {
		p.Active = 0 // gorge's Game.Active before the first turn (kernel Projection names the starter)
	}
	p.RNGDraws = kp.Draws
	p.Over, p.Winner = kp.Over, kp.Winner
}

// stackModes is e's chosen modes: its mode byte also carries cast provenance
// outside the ability's NModes bits.
func (r *realSide) stackModes(e *dirac.ProjStack) uint8 {
	if int(e.Card) >= len(r.tbl.Cards) || int(e.Ability) >= len(r.tbl.Cards[e.Card].Abilities) {
		return 0
	}
	a := &r.tbl.Cards[e.Card].Abilities[e.Ability]
	if a.Repeats() {
		return a.ModeSet(e.Mode) // a count per mode: the modes chosen at least once
	}
	return e.Mode & uint8(1<<a.NModes-1)
}

// stackTargetOrder is the order e's target slots project in: gorge's, for an
// entry that has a card ability behind it.
func (r *realSide) stackTargetOrder(e *dirac.ProjStack) [kop.MaxTargets]uint8 {
	if int(e.Card) >= len(r.tbl.Cards) || int(e.Ability) >= len(r.tbl.Cards[e.Card].Abilities) {
		var order [kop.MaxTargets]uint8
		for i := range order {
			order[i] = uint8(i)
		}
		return order
	}
	return dirac.StackTargetOrder(&r.tbl.Cards[e.Card].Abilities[e.Ability], e.Mode)
}

// Close releases the kernel's registration of the side's table (a run
// that builds a table per matchup releases each when its game is done).
func (r *realSide) Close() { dirac.ReleaseTable(r.tbl) }

// ChanceTranscript implements ChanceSource for a kernel built with
// Config.RecordChance: every IntN the kernel drew, in order. It is nil when
// recording was off or the kernel's transcript overflowed
// dirac.MaxChanceRuns (a game always draws the toss, so nil is never a real
// empty transcript).
func (r *realSide) ChanceTranscript() []ChanceDraw {
	tr, ok := r.s.ChanceTranscript()
	if !ok {
		return nil
	}
	out := make([]ChanceDraw, len(tr))
	for i, c := range tr {
		out[i] = ChanceDraw{Bound: c.Bound, Value: c.Value}
	}
	return out
}

// CloseSide closes a side that holds kernel resources (a RealKernel side).
func CloseSide(s Side) {
	if c, ok := s.(interface{ Close() }); ok {
		c.Close()
	}
}

var _ Side = (*realSide)(nil)

// SetMatchup builds pool decks a and b for both sides: the gorge matchup
// (Corpus.Matchup) and the kernel table (set.Matchup) the SetSide adapter
// plays.
func SetMatchup(set cardset.Set, c *Corpus, a, b string, mulligans int) (*Matchup, *kop.Table, error) {
	m, err := c.Matchup(a, b, mulligans)
	if err != nil {
		return nil, nil, err
	}
	var es [2][]cardset.Entry
	for s := 0; s < 2; s++ {
		f, err := c.deckFile(m.Names[s])
		if err != nil {
			return nil, nil, err
		}
		for _, e := range f.Cards {
			es[s] = append(es[s], cardset.Entry{Name: e.Name, Count: e.Count})
		}
	}
	tbl, _, _, err := set.Matchup(es[0], es[1])
	if err != nil {
		return nil, nil, err
	}
	return m, tbl, nil
}

// SetMatchupN is SetMatchup for a pod of len(names) decks (2..MaxSeats): one
// table over every deck (set.MatchupN) and the gorge matchup built from that
// table's decks (KernelMatchupN), seat s playing names[s], with its deck
// files' commanders (Corpus.MatchupN). At two seats it is SetMatchup.
func SetMatchupN(set cardset.Set, c *Corpus, names []string, mulligans int) (*Matchup, *kop.Table, error) {
	if len(names) == 2 {
		return SetMatchup(set, c, names[0], names[1], mulligans)
	}
	es := make([][]cardset.Entry, len(names))
	for s, n := range names {
		f, err := c.deckFile(n)
		if err != nil {
			return nil, nil, fmt.Errorf("deck %s: %w", n, err)
		}
		for _, e := range f.Cards {
			es[s] = append(es[s], cardset.Entry{Name: e.Name, Count: e.Count})
		}
	}
	tbl, ids, err := set.MatchupN(es)
	if err != nil {
		return nil, nil, err
	}
	m, err := KernelMatchupN(c.Reg, tbl, names, ids, nil, mulligans)
	if err != nil {
		return nil, nil, err
	}
	if err := c.readCommanders(m); err != nil {
		return nil, nil, err
	}
	return m, tbl, nil
}

// FDNMatchup is SetMatchup over the "fdn" set.
func FDNMatchup(c *Corpus, a, b string, mulligans int) (*Matchup, *kop.Table, error) {
	return SetMatchup(fdnSet(), c, a, b, mulligans)
}

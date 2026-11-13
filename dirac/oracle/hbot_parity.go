package oracle

import (
	"context"
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// hbot_parity.go measures a kernel search bot (dirac/search.Bot: the
// search's answer at every unsearched decision and its candidate 0) against
// the bot the kernel search ports: gorge's heuristic bot with auto-pay mana
// (heuristic.NewBot(seed).EnableAutoPayMana(), the bot bots/azmcts's
// autopay walk asks for both seats). One lockstep game (gorge and the
// kernel, the same decks and seed) in which gorge's bot chooses every
// action over its own Board (botpolicy.BoardFromGameInto, payment actions
// built), and at the first canonical pick of every gorge decision the
// kernel bot is asked the same decision in shadow on the kernel's state. A
// declaration (attackers, blockers) and a trigger order are compared as
// whole answers (the kernel bot's picks run on a copy of the state until
// the declaration closes); any other decision by its first canonical
// action, as SBTParityGame does.
//
// gorge's bot sometimes answers a priority with a manual mana tap (its
// wantsManual path: a castable card it has no payment plan for). The
// canonical surface has no mana taps; the tap is aimed at
// Board.TapIntent's card, so that card's plain cast stands for the answer
// (ParityRec.Class "manual" marks it in the report).

// KernelBot is the kernel search's bot contract (dirac/search.Bot).
type KernelBot interface {
	Pick(s *dirac.State, d *dirac.Decision) int
}

// HBotParityResult is one game.
type HBotParityResult struct {
	Res       Result
	Recs      []ParityRec
	Unoffered int // gorge answers with no offered canonical action (followed by the first option)
	Manual    int // gorge priority answers that were a manual mana tap (compared as the tapped-for cast)
}

// HBotParityOptions selects the bot rules the two sides share. The zero
// value is the default (every rule off).
type HBotParityOptions struct {
	HoldInstant bool        // gorge TimingRules.HoldInstant; mk must arm the kernel bot too
	Set         cardset.Set // the set the pairing's decks come from (nil: the FDN pool)
	// Observe, when set, sees each gorge decision as it is asked, with gorge's
	// bot's answer as canonical actions (a test's reach condition reads it).
	Observe func(c *PickCtx, gorge []Canon)
}

// HBotParityGame plays pairing (a, b) of the FDN pool under cfg with gorge's
// heuristic autopay bot choosing, comparing bot (built for the matchup's
// table) at every decision.
func HBotParityGame(c *Corpus, a, b string, seed uint64, cfg Config, maxDecisions int, mk func(tbl *kop.Table) KernelBot) (HBotParityResult, error) {
	return HBotParityGameOpts(c, a, b, seed, cfg, maxDecisions, mk, HBotParityOptions{})
}

// HBotParityGameOpts is HBotParityGame with opt's rules armed on gorge's bot.
func HBotParityGameOpts(c *Corpus, a, b string, seed uint64, cfg Config, maxDecisions int, mk func(tbl *kop.Table) KernelBot, opt HBotParityOptions) (HBotParityResult, error) {
	var (
		m    *Matchup
		tbl  *kop.Table
		side Side
		err  error
	)
	if opt.Set != nil {
		if m, tbl, err = SetMatchup(opt.Set, c, a, b, 0); err == nil {
			side, err = SetSide(opt.Set, tbl, cfg)(m, seed)
		}
	} else if m, tbl, err = FDNMatchup(c, a, b, 0); err == nil {
		side, err = FDNSide(tbl, cfg)(m, seed)
	}
	if err != nil {
		return HBotParityResult{}, err
	}
	return hbotParity(c, m, tbl, side, 2, seed, cfg, maxDecisions, mk, opt)
}

// HBotParityPod is HBotParityGameOpts for a pod of len(names) decks
// (2..dirac.MaxSeats, SetMatchupN; opt.Set nil is the FDN pool): gorge's
// heuristic autopay bot chooses at every seat, and the kernel bot is asked
// every decision in shadow.
func HBotParityPod(c *Corpus, names []string, seed uint64, cfg Config, maxDecisions int, mk func(tbl *kop.Table) KernelBot, opt HBotParityOptions) (HBotParityResult, error) {
	if len(names) < 2 || len(names) > dirac.MaxSeats {
		return HBotParityResult{}, fmt.Errorf("a pod of %d decks (2..%d)", len(names), dirac.MaxSeats)
	}
	set := opt.Set
	if set == nil {
		set = fdnSet()
	}
	m, tbl, err := SetMatchupN(set, c, names, 0)
	if err != nil {
		return HBotParityResult{}, err
	}
	side, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		return HBotParityResult{}, err
	}
	return hbotParity(c, m, tbl, side, len(names), seed, cfg, maxDecisions, mk, opt)
}

// hbotParity plays the lockstep game on side, gorge's bot choosing at each
// of the n seats.
func hbotParity(c *Corpus, m *Matchup, tbl *kop.Table, side Side, n int, seed uint64, cfg Config, maxDecisions int, mk func(tbl *kop.Table) KernelBot, opt HBotParityOptions) (HBotParityResult, error) {
	defer CloseSide(side)
	k := side.(*realSide)
	pol := &hbotParityPol{k: k, bot: mk(tbl), names: NewSBTCards(c.Reg, tbl, nil), observe: opt.Observe}
	for p := 0; p < n; p++ {
		pol.gbots[p] = heuristic.NewBot(SeatSeed(seed, p)).EnableAutoPayMana()
		if opt.HoldInstant {
			pol.gbots[p].EnableTimingRules(botpolicy.TimingRules{HoldInstant: true})
		}
	}
	pol.gseq = ^uint64(0)
	rc := RunConfig{Config: cfg, MaxDecisions: maxDecisions}
	var out HBotParityResult
	out.Res = Lockstep(m, seed, k, pol, rc)
	out.Recs, out.Unoffered, out.Manual = pol.recs, pol.unoffered, pol.manual
	return out, nil
}

type hbotParityPol struct {
	k         *realSide
	bot       KernelBot
	names     *SBTCards
	gbots     [dirac.MaxSeats]*heuristic.Bot
	brd       botpolicy.Board
	gseq      uint64
	queue     []Canon
	recs      []ParityRec
	unoffered int
	manual    int
	dbg       string
	observe   func(c *PickCtx, gorge []Canon)
	// scratch for the kernel bot's whole answer
	ts dirac.State
	td dirac.Decision
}

func (p *hbotParityPol) Pick(c *PickCtx) Canon {
	gd := c.G.Pending()
	if gd != nil && gd.Seq != p.gseq {
		p.gseq = gd.Seq
		var manual bool
		p.queue, manual = p.gorgeAnswer(c, gd)
		if p.observe != nil {
			p.observe(c, p.queue)
		}
		var dbg []string
		// hbot.Debugf is a package global: touch it only when debugging, so
		// parallel parity runs (the oracle tests) never write it.
		debug := os.Getenv("KHBOT_DEBUG") != ""
		if debug {
			hbot.Debugf = func(f string, a ...any) { dbg = append(dbg, fmt.Sprintf(f, a...)) }
		}
		kq := p.kernelAnswer()
		if debug {
			hbot.Debugf = nil
		}
		p.record(c, p.queue, kq, manual)
		if r := &p.recs[len(p.recs)-1]; !r.Agree && os.Getenv("KHBOT_DEBUG") != "" {
			r.Where += "\n          gorge asks " + describeDecision(c.G.E.Pending()) + "\n          " + p.dbg
			for _, x := range dbg {
				r.Where += "\n          port: " + x
			}
		}
	}
	for len(p.queue) > 0 {
		x := p.queue[0]
		p.queue = p.queue[1:]
		for _, o := range c.D.Options {
			if o == x {
				return x
			}
		}
	}
	p.unoffered++
	for _, o := range c.D.Options {
		if o.Kind() == ActDone {
			return o
		}
	}
	return c.D.Options[0]
}

// gorgeAnswer is gorge's heuristic autopay bot's answer to gd as canonical
// actions; manual reports a manual mana tap read as its card's cast.
func (p *hbotParityPol) gorgeAnswer(c *PickCtx, gd *decision.Decision) ([]Canon, bool) {
	e := c.G.E
	if gd.Kind == decision.KPriority {
		e.EnsurePaymentActions()
		if ed := e.Pending(); ed != nil && ed.Seq == gd.Seq {
			gd = ed
		}
	}
	bot := p.gbots[gd.Player]
	brd := botpolicy.BoardFromGameInto(e.G, e, gd.Player, &p.brd)
	if os.Getenv("KHBOT_DEBUG") != "" && gd.Kind == decision.KPriority {
		id, ok := brd.CastableNow(gd.Player, gd)
		ti, tok := brd.TapIntent(gd)
		p.dbg = fmt.Sprintf("castableNow=%d,%v tapIntent=%d,%v anyCastableNow=%v potential=%v/%v unpayable=%v", id, ok, ti, tok, brd.AnyCastableNow(gd.Player, gd), gd.PotentialCasts, gd.PotentialCastsKnown, gd.PotentialUnpayableCasts)
		for _, oid := range e.G.Zone(state.ZBattlefield, gd.Player) {
			if o := e.G.Obj(oid); o != nil && o.Face() != nil {
				mp := o.Face().ManaProduction()
				p.dbg += fmt.Sprintf(" bf[%d %s t=%v prod=%v]", oid, o.Face().Name, o.Tapped, mp.Colour)
			}
		}
		for _, oid := range e.G.Zone(state.ZHand, gd.Player) {
			if o := e.G.Obj(oid); o != nil && o.Face() != nil {
				p.dbg += fmt.Sprintf(" hand[%d %s]", oid, o.Face().Name)
			}
		}
	}
	in, err := bot.DecideBoard(context.Background(), brd, *gd)
	if err != nil {
		return nil, false
	}
	ref := c.G.InstRef
	if in.Payment != nil {
		for _, pa := range gd.PaymentActions {
			if pa.ID == in.Payment.ActionID {
				mc, _ := castModeCode(pa.Cast.CastMode())
				return []Canon{Pack(ActCast, ref(pa.Cast.Object), RefNone, mc)}, false
			}
		}
		return nil, false
	}
	if gd.Kind == decision.KPriority {
		if len(in.Choices) != 1 {
			return nil, false
		}
		o := optByIndex(gd, in.Choices[0])
		if o == nil {
			return nil, false
		}
		switch o.Kind {
		case "pass":
			return []Canon{Pack(ActPass, RefNone, RefNone, 0)}, false
		case "play_land":
			return []Canon{Pack(ActPlayLand, ref(o.Obj), RefNone, landFaceCode(o.Mode))}, false
		case "cast":
			return []Canon{castOptCanon(ref(o.Obj), o.Mode, beamAlt(o.Mode, o.AltCostIndex, o.Cost))}, false
		case "ability":
			return []Canon{Pack(ActActivate, ref(o.Obj), RefNone, uint8(o.Ability))}, false
		case "activate":
			// the manual path: follow the bot's taps on a clone of the
			// engine to the play they float mana for
			return p.followManual(c, in), true
		}
		return nil, false
	}
	fr := c.G.fr
	if fr == nil || fr.seq != gd.Seq {
		return nil, false
	}
	var out []Canon
	for _, ch := range in.Choices {
		for i := range fr.d.Options {
			if fr.d.Options[i].Index == ch && fr.valid[i] {
				out = append(out, fr.acts[i])
			}
		}
	}
	if fr.multi && len(out) < fr.d.Max {
		out = append(out, Pack(ActDone, RefNone, RefNone, 0))
	}
	if len(out) == 0 && gd.Kind == decision.KTarget {
		out = append(out, Pack(ActDone, RefNone, RefNone, 0))
	}
	return out, false
}

// followManual plays gorge's bot on a clone of the engine from its manual
// mana tap (intent in) until it answers a priority decision with a
// canonical play (a cast, an ability, a land, a pass): the play the taps
// were for. Decisions the taps pose (a mana colour) are the bot's too.
func (p *hbotParityPol) followManual(c *PickCtx, in decision.Intent) []Canon {
	e := c.G.E.Clone()
	me := in.Player
	ref := c.G.InstRef
	var brd botpolicy.Board
	for n := 0; n < 40; n++ {
		if err := e.Submit(in); err != nil {
			return []Canon{Pack(ActPass, RefNone, RefNone, 0)}
		}
		d := e.Pending()
		if d == nil || d.Player != me {
			return []Canon{Pack(ActPass, RefNone, RefNone, 0)}
		}
		if d.Kind == decision.KPriority {
			e.EnsurePaymentActions()
			d = e.Pending()
		}
		b := botpolicy.BoardFromGameInto(e.G, e, d.Player, &brd)
		var err error
		in, err = p.gbots[me].DecideBoard(context.Background(), b, *d)
		if err != nil {
			return []Canon{Pack(ActPass, RefNone, RefNone, 0)}
		}
		if d.Kind != decision.KPriority {
			continue
		}
		if in.Payment != nil {
			for _, pa := range d.PaymentActions {
				if pa.ID == in.Payment.ActionID {
					mc, _ := castModeCode(pa.Cast.CastMode())
					return []Canon{Pack(ActCast, ref(pa.Cast.Object), RefNone, mc)}
				}
			}
			return nil
		}
		if len(in.Choices) != 1 {
			return nil
		}
		o := optByIndex(d, in.Choices[0])
		if o == nil {
			return nil
		}
		switch o.Kind {
		case "activate":
			continue
		case "pass":
			return []Canon{Pack(ActPass, RefNone, RefNone, 0)}
		case "play_land":
			return []Canon{Pack(ActPlayLand, ref(o.Obj), RefNone, landFaceCode(o.Mode))}
		case "cast":
			return []Canon{castOptCanon(ref(o.Obj), o.Mode, beamAlt(o.Mode, o.AltCostIndex, o.Cost))}
		case "ability":
			return []Canon{Pack(ActActivate, ref(o.Obj), RefNone, uint8(o.Ability))}
		}
		return nil
	}
	return []Canon{Pack(ActPass, RefNone, RefNone, 0)}
}

// kernelAnswer is the kernel bot's answer to the kernel's pending decision
// in canonical actions: one pick, or a declaration's / trigger order's
// whole run of picks (played on a copy of the state).
func (p *hbotParityPol) kernelAnswer() []Canon {
	r := p.k
	d := &r.d
	if d.N == 0 {
		return nil
	}
	i := p.bot.Pick(&r.s, d)
	if i < 0 || i >= int(d.N) {
		return nil
	}
	a := d.Options[i]
	out := []Canon{r.canon(a)}
	if d.Kind != dirac.DkAttackers && d.Kind != dirac.DkBlockers && d.Kind != dirac.DkTriggerOrder {
		return out
	}
	if dc, ok := p.bot.(interface {
		Declaration(*dirac.State, *dirac.Decision, []dirac.Action) []dirac.Action
	}); ok && d.Kind != dirac.DkTriggerOrder {
		// the bot's whole declaration (a last pick the kernel elides is
		// never asked)
		out = out[:0]
		for _, x := range dc.Declaration(&r.s, d, nil) {
			out = append(out, r.canon(x))
		}
		return out
	}
	p.ts.CopyFrom(&r.s)
	kind, pl, left := d.Kind, d.Player, d.N
	for n := 0; n < 64 && a.Kind() != dirac.AkDone; n++ {
		p.ts.Apply(a)
		p.ts.StepInto(&p.td)
		if p.td.Kind != kind || p.td.Player != pl || p.td.N == 0 {
			break
		}
		// A trigger order's picks each leave one trigger fewer; any other
		// count is the next batch's order (its last pick elided between).
		if kind == dirac.DkTriggerOrder && p.td.N != left-1 {
			break
		}
		left = p.td.N
		j := p.bot.Pick(&p.ts, &p.td)
		if j < 0 || j >= int(p.td.N) {
			break
		}
		a = p.td.Options[j]
		out = append(out, p.canonAt(&p.ts, &p.td, a))
	}
	return out
}

// canonAt canonicalises a of decision d at state s (a scratch state: the
// side's canonicaliser reads its own state and decision, so they are
// swapped in for the call).
func (p *hbotParityPol) canonAt(s *dirac.State, d *dirac.Decision, a dirac.Action) Canon {
	r := p.k
	var saveS dirac.State
	saveS.CopyFrom(&r.s)
	saveD := r.d
	r.s.CopyFrom(s)
	r.d = *d
	r.s.Project(&r.kp)
	c := r.canon(a)
	r.s.CopyFrom(&saveS)
	r.d = saveD
	r.s.Project(&r.kp)
	return c
}

func (p *hbotParityPol) record(c *PickCtx, gq, kq []Canon, manual bool) {
	k := c.D.Kind
	g, s := gq, kq
	if !whole(k) {
		if len(g) > 1 {
			g = g[:1]
		}
		if len(s) > 1 {
			s = s[:1]
		}
	} else {
		g, s = dropDone(g), dropDone(s)
		if k == DTriggerOrder && len(s) == len(g)-1 {
			g = g[:len(s)] // the last trigger is the kernel's elided pick
		}
		if k != DTriggerOrder {
			sortCanon(g)
			sortCanon(s)
		}
	}
	rec := ParityRec{Kind: k.String(), Agree: equalActions(g, s), Gorge: canonList(g), SBT: canonList(s)}
	e := c.G.E
	if len(gq) > 0 {
		offered := false
		for _, o := range c.D.Options {
			if o == gq[0] {
				offered = true
			}
		}
		if !offered && !(whole(k) && len(g) == 0) {
			rec.Unoffered = true
			rec.Class = "unoffered " + k.String() + ": " + shape(gq)
			if manual {
				rec.Class = "unoffered manual " + shape(gq)
			}
			rec.Where = fmt.Sprintf("turn %d %s p%d: gorge %s (offered %s)", e.G.Turn, e.G.Step, c.D.Player, p.namesOf(gq), p.namesOf(c.D.Options))
		}
	}
	if manual {
		p.manual++
	}
	rec.Equiv = rec.Agree || equalActions(p.byCard(g), p.byCard(s))
	if !rec.Agree && !rec.Unoffered {
		rec.Class = k.String() + ": " + shape(g) + " / " + shape(s)
		if manual {
			rec.Class = "manual " + rec.Class
		}
		rec.Where = fmt.Sprintf("turn %d %s p%d life %d/%d: gorge %s, port %s", e.G.Turn, e.G.Step, c.D.Player,
			e.G.Players[0].Life, e.G.Players[1].Life, p.namesOf(g), p.namesOf(s))
	}
	p.recs = append(p.recs, rec)
}

// byCard and namesOf reuse sbt_parity.go's renderers over this game's side.
func (p *hbotParityPol) byCard(cs []Canon) []Canon {
	sp := &sbtParityPol{k: p.k, cs: p.names}
	return sp.byCard(cs)
}

func (p *hbotParityPol) namesOf(cs []Canon) string {
	sp := &sbtParityPol{k: p.k, cs: p.names}
	return sp.names(cs)
}

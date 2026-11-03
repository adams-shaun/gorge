package oracle

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/bots/spellbench/builtins"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// sbt_parity.go measures the sbt agent against the sb-tactical it ports:
// one lockstep game (gorge and the kernel, the same decks and seed) in which
// gorge's own sb-tactical seats (builtins.NewTactical, the live engine as
// planner, as gorge self-plays them) choose every action, and at the first
// canonical pick of every gorge decision the kernel-side agent is asked the
// same decision in shadow, on the kernel's state. A whole declaration
// (attackers, blockers) and a trigger order are compared as answers; any
// other decision by its first canonical action.
//
// At priority both sides are read through builtins.Seat.TacticalPriority:
// the comparison is of the choice, not of gorge's mana lowering (the
// canonical surface pays a chosen play itself, GorgeSide.lower).

// ParityRec is one compared decision.
type ParityRec struct {
	Kind  string // canonical decision kind
	Agree bool   // the same canonical answer
	Equiv bool   // the same answer up to which copy of a card (by card table index)
	Gorge string // gorge's answer
	SBT   string // the agent's answer
	Class string // a disagreement's class
	Where string // turn, step, player and the deciding seat's context
	// Unoffered: gorge's answer is not among the canonical options (the
	// lockstep surface cannot play it); not counted as a comparison
	Unoffered bool
}

// SBTParityResult is one game.
type SBTParityResult struct {
	Res        Result
	Recs       []ParityRec
	Unoffered  int // gorge answers with no offered canonical action (followed by the first option)
	Restricted int // gorge priority choices the surface does not offer, compared as gorge's best offered play
}

// SBTParityGame plays pairing p of the FDN pool (FDNMatchup, FDNSide) under
// cfg with sb-tactical choosing, comparing the agent at every decision.
func SBTParityGame(c *Corpus, a, b string, seed uint64, cfg Config, maxDecisions int) (SBTParityResult, error) {
	m, tbl, err := FDNMatchup(c, a, b, 0)
	if err != nil {
		return SBTParityResult{}, err
	}
	side, err := FDNSide(tbl, cfg)(m, seed)
	if err != nil {
		return SBTParityResult{}, err
	}
	defer CloseSide(side)
	k := side.(*realSide)
	cs := NewSBTCards(c.Reg, tbl, nil)
	ag := NewSBT(cs)
	var seeds [2]uint64
	for p := 0; p < 2; p++ {
		seeds[p] = SeatSeed(seed, p)
	}
	ag.NewGame(seeds)
	pol := &sbtParityPol{k: k, ag: ag, cs: cs}
	lookup := builtins.NewRegistryLookup(c.Reg)
	for p := 0; p < 2; p++ {
		pol.gseats[p] = builtins.NewTactical(builtins.AutoPay, seeds[p], lookup, builtins.DefaultTacticalWeights())
	}
	pol.gseq = ^uint64(0)
	rc := RunConfig{Config: cfg, MaxDecisions: maxDecisions}
	var out SBTParityResult
	out.Res = Lockstep(m, seed, k, pol, rc)
	out.Recs, out.Unoffered, out.Restricted = pol.recs, pol.unoffered, pol.restricted
	return out, nil
}

// SeatSeed is the per-seat seed self-play gives seat p of a game seeded seed.
func SeatSeed(seed uint64, p int) uint64 {
	x := seed ^ uint64(p+1)*0x9e3779b97f4a7c15
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

type sbtParityPol struct {
	k          *realSide
	ag         *SBT
	cs         *SBTCards
	gseats     [2]*builtins.Seat
	gseq       uint64
	queue      []Canon
	recs       []ParityRec
	unoffered  int
	restricted int // gorge priority choices not offered, replaced by its best offered candidate
}

func (p *sbtParityPol) Pick(c *PickCtx) Canon {
	gd := c.G.Pending()
	if gd != nil && gd.Seq != p.gseq {
		p.gseq = gd.Seq
		p.queue = p.gorgeAnswer(c, gd)
		gdesc := describeDecision(c.G.E.Pending())
		sq := p.sbtAnswer()
		p.record(c, p.queue, sq)
		if r := &p.recs[len(p.recs)-1]; !r.Agree {
			r.Where += "\n          gorge asks " + gdesc + "\n          sbt asks   " + describeDecision(p.ag.last)
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
	// the declaration ends, else the first option
	for _, o := range c.D.Options {
		if o.Kind() == ActDone {
			return o
		}
	}
	return c.D.Options[0]
}

// gorgeAnswer is gorge's sb-tactical answer to gd as canonical actions.
func (p *sbtParityPol) gorgeAnswer(c *PickCtx, gd *decision.Decision) []Canon {
	e := c.G.E
	seat := p.gseats[gd.Player&1]
	if gd.Kind == decision.KPriority {
		e.EnsurePaymentActions()
		if ed := e.Pending(); ed != nil && ed.Seq == gd.Seq {
			gd = ed // the engine's own decision, its PaymentActions built (a gorge host builds them for an autopay seat)
		}
		seat.SetPlanner(e)
	}
	v := view.Project(e.G, e, gd.Player, gd)
	v.Round = view.RoundOf(e.G, e.L.Events)
	if gd.Kind == decision.KPriority {
		cands, best, ok := seat.TacticalPriority(v, *gd)
		if !ok {
			return []Canon{Pack(ActPass, RefNone, RefNone, 0)}
		}
		// The canonical surface does not offer every play gorge's seat
		// ranks (a play gorge's autopay surface cannot make: Eaten Alive's
		// either-or cost): the seat's choice among the offered plays is its
		// best-scoring offered candidate (ties to the earlier, as
		// pickPriority breaks them).
		first := p.prioCanon(c, gd, cands[best].Key)
		if p.offered(c, first) {
			return first
		}
		bi, bs := -1, -1e18
		for i, x := range cands {
			if x.Score > bs && p.offered(c, p.prioCanon(c, gd, x.Key)) {
				bi, bs = i, x.Score
			}
		}
		p.restricted++
		if bi >= 0 {
			return p.prioCanon(c, gd, cands[bi].Key)
		}
		return first
	}
	in, err := seat.Decide(context.Background(), v, *gd)
	if err != nil {
		return nil
	}
	fr := c.G.fr
	if fr == nil || fr.seq != gd.Seq {
		return nil
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
	return out
}

func (p *sbtParityPol) offered(c *PickCtx, cs []Canon) bool {
	if len(cs) == 0 {
		return false
	}
	for _, o := range c.D.Options {
		if o == cs[0] {
			return true
		}
	}
	return false
}

// prioCanon is a gorge priority candidate key's canonical action.
func (p *sbtParityPol) prioCanon(c *PickCtx, gd *decision.Decision, k builtins.PriorityKey) []Canon {
	ref := func(id state.ObjID) Ref { return c.G.InstRef(id) }
	{
		switch k.Src {
		case builtins.KeyOption:
			o := optByIndex(gd, k.Opt)
			switch o.Kind {
			case "pass":
				return []Canon{Pack(ActPass, RefNone, RefNone, 0)}
			case "play_land":
				return []Canon{Pack(ActPlayLand, ref(o.Obj), RefNone, 0)}
			case "cast":
				return []Canon{castOptCanon(ref(o.Obj), o.Mode, beamAlt(o.Mode, o.AltCostIndex, o.Cost))}
			case "ability":
				return []Canon{Pack(ActActivate, ref(o.Obj), RefNone, uint8(o.Ability))}
			}
		case builtins.KeyPlan:
			for _, pa := range gd.PaymentActions {
				if pa.Cast.Object == k.Obj {
					mc, _ := castModeCode(pa.Cast.CastMode())
					return []Canon{Pack(ActCast, ref(k.Obj), RefNone, mc)}
				}
			}
		case builtins.KeyPotential:
			switch k.Kind {
			case "cast":
				mc, _ := castModeCode(k.Mode)
				return []Canon{Pack(ActCast, ref(k.Obj), RefNone, mc|uint8(k.Alt)<<4)}
			case "ability":
				return []Canon{Pack(ActActivate, ref(k.Obj), RefNone, uint8(k.Ability))}
			}
		}
		return nil
	}
}

// sbtAnswer is the agent's answer to the kernel's pending decision.
func (p *sbtParityPol) sbtAnswer() []Canon {
	r := p.k
	acts := p.ag.Answer(&r.s, &r.d)
	out := make([]Canon, 0, len(acts))
	for _, a := range acts {
		out = append(out, r.canon(a))
	}
	return out
}

// whole reports the kinds compared as whole answers.
func whole(k DKind) bool { return k == DAttackers || k == DBlockers || k == DTriggerOrder }

func (p *sbtParityPol) record(c *PickCtx, gq, sq []Canon) {
	k := c.D.Kind
	g, s := gq, sq
	if !whole(k) {
		if len(g) > 1 {
			g = g[:1]
		}
		if len(s) > 1 {
			s = s[:1]
		}
	} else {
		g, s = dropDone(g), dropDone(s)
		if k != DTriggerOrder {
			sortCanon(g)
			sortCanon(s)
		}
	}
	rec := ParityRec{Kind: k.String(), Agree: equalActions(g, s), Gorge: canonList(g), SBT: canonList(s)}
	// a gorge answer the canonical surface does not offer (a play gorge's
	// autopay surface cannot make: Eaten Alive's either-or cost, ...) is no
	// comparison: the game follows another option
	if len(gq) > 0 {
		offered := false
		for _, o := range c.D.Options {
			if o == gq[0] {
				offered = true
			}
		}
		if !offered {
			rec.Unoffered = true
			e := c.G.E
			rec.Class = "unoffered " + k.String() + ": " + shape(gq)
			rec.Where = fmt.Sprintf("turn %d %s p%d: gorge %s (offered %s)", e.G.Turn, e.G.Step, c.D.Player, p.names(gq), p.names(c.D.Options))
		}
	}
	rec.Equiv = rec.Agree || equalActions(p.byCard(g), p.byCard(s))
	if !rec.Agree && !rec.Unoffered {
		rec.Class = k.String() + ": " + shape(g) + " / " + shape(s)
		e := c.G.E
		rec.Where = fmt.Sprintf("turn %d %s p%d life %d/%d: gorge %s, sbt %s", e.G.Turn, e.G.Step, c.D.Player,
			e.G.Players[0].Life, e.G.Players[1].Life, p.names(g), p.names(s))
	}
	p.recs = append(p.recs, rec)
}

// describeDecision renders a gorge decision for a report.
func describeDecision(d *decision.Decision) string {
	if d == nil {
		return "nil"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %q min %d max %d src %d", d.Kind, d.Prompt, d.Min, d.Max, d.Source)
	if d.TargetEffect != nil {
		fmt.Fprintf(&b, " effect %s", d.TargetEffect.API)
	}
	b.WriteString(" [")
	for i, o := range d.Options {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%d:%s", o.Index, o.Kind)
		if o.Label != "" {
			fmt.Fprintf(&b, "%q", o.Label)
		}
		if o.Obj != 0 {
			fmt.Fprintf(&b, " obj%d", o.Obj)
		}
		if o.Mode != "" {
			fmt.Fprintf(&b, " mode %s", o.Mode)
		}
	}
	b.WriteString("]")
	for _, pa := range d.PaymentActions {
		fmt.Fprintf(&b, " pay(%d %s plans=%d)", pa.Cast.Object, pa.Cast.CastMode(), len(pa.Plans))
	}
	return b.String()
}

func dropDone(cs []Canon) []Canon {
	out := make([]Canon, 0, len(cs))
	for _, x := range cs {
		if x.Kind() != ActDone {
			out = append(out, x)
		}
	}
	return out
}

func canonList(cs []Canon) string {
	var b strings.Builder
	for i, x := range cs {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(x.String())
	}
	return b.String()
}

// shape is an answer's class: its action kinds.
func shape(cs []Canon) string {
	if len(cs) == 0 {
		return "none"
	}
	var ks []string
	for _, x := range cs {
		s := x.String()
		if i := strings.IndexByte(s, '('); i >= 0 {
			s = s[:i]
		}
		if len(ks) == 0 || ks[len(ks)-1] != s {
			ks = append(ks, s)
		}
	}
	return strings.Join(ks, "+")
}

// byCard replaces every instance reference with its card (sorted).
func (p *sbtParityPol) byCard(cs []Canon) []Canon {
	kp := &p.k.kp
	p.k.s.Project(kp)
	card := func(x Ref) Ref {
		if x < RefStack && int(x) < int(kp.NObj) {
			return Ref(kp.Objs[x].Card & 0xFF)
		}
		return x
	}
	out := make([]Canon, len(cs))
	for i, x := range cs {
		out[i] = Pack(x.Kind(), card(x.Src()), card(x.Tgt()), x.Mode())
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// names renders an answer with card names.
func (p *sbtParityPol) names(cs []Canon) string {
	kp := &p.k.kp
	nm := func(x Ref) string {
		switch {
		case x == RefNone:
			return "-"
		case x >= RefPlayer:
			return fmt.Sprintf("P%d", x-RefPlayer)
		case x >= RefStack:
			return fmt.Sprintf("S%d", x-RefStack)
		case int(x) < int(kp.NObj) && p.cs.card(kp.Objs[x].Card) != nil:
			return p.cs.card(kp.Objs[x].Card).name
		}
		return fmt.Sprintf("#%d", x)
	}
	var parts []string
	for _, x := range cs {
		s := x.String()
		if i := strings.IndexByte(s, '('); i >= 0 {
			s = s[:i]
		}
		parts = append(parts, fmt.Sprintf("%s(%s,%s,%d)", s, nm(x.Src()), nm(x.Tgt()), x.Mode()))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
}

var _ = dirac.NoObj
var _ = kop.ZoneHand

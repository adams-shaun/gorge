package gorgebe

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerAbility(ir.AbTriggered, printTriggered) }

// isSelfFilter reports whether f is exactly "this object".
func isSelfFilter(f *ir.Filter) bool {
	return f != nil && f.Op == ir.FPred && f.Pred == ir.PIs && len(f.Args) == 0 && f.Ref != nil && f.Ref.Kind == ir.RefSelf && f.Word == ""
}

// triggerSelf reports whether t's event is about the source itself: the
// subject is the source, or for a cast the spell is.
func triggerSelf(t *ir.Trigger) bool {
	if t.Event == ir.EvCast {
		return isSelfFilter(t.Object)
	}
	return isSelfFilter(t.Subject)
}

// params is an ordered parameter list with replace-or-append set.
type params []kv

func (ps *params) set(k, v string) {
	for i := range *ps {
		if (*ps)[i].K == k {
			(*ps)[i].V = v
			return
		}
	}
	*ps = append(*ps, kv{k, v})
}

func (ps *params) has(k string) bool {
	for _, x := range *ps {
		if x.K == k {
			return true
		}
	}
	return false
}

func printTriggered(p *pr, a *ir.Ability) error {
	tr := a.Trigger
	if tr == nil {
		return unprintable("event:none", a.Src)
	}
	alts := []*ir.Trigger{tr}
	if tr.Event == ir.EvOr {
		alts = tr.Args
		if len(alts) == 0 {
			return unprintable("event:or", tr.Src)
		}
		for _, x := range alts {
			if x == nil || x.Event == ir.EvOr {
				return unprintable("event:or", tr.Src)
			}
		}
		for _, x := range alts[1:] {
			if triggerSelf(x) != triggerSelf(alts[0]) {
				return p.printMixedOr(a, tr, alts)
			}
		}
	}
	defer func() { p.ev, p.evSelf = ir.EvNone, false }()
	p.ev, p.evSelf = alts[0].Event, triggerSelf(alts[0])
	head, err := p.program(a.Body)
	if err != nil {
		return err
	}
	// A "may" step at the head of the body is the trigger's own optionality:
	// Forge asks it as the trigger goes on the stack, with OptionalDecider$ on
	// the T: line (the engine reads it there, not on the body's node).
	optional := ""
	for i, kvp := range head.Params {
		if kvp.K == "OptionalDecider" && kvp.V == "You" {
			optional = kvp.V
			head.Params = append(head.Params[:i], head.Params[i+1:]...)
			break
		}
	}
	payCostAbility(head)
	name := p.sub("Trig"+head.API, head)
	for i, x := range alts {
		p.ev, p.evSelf = x.Event, triggerSelf(x)
		ps, err := p.triggerParams(a, tr, x)
		if err != nil {
			return err
		}
		if i > 0 {
			ps.set("Secondary", "True")
		}
		if optional != "" {
			ps.set("OptionalDecider", optional)
		}
		ps.set("Execute", name)
		p.lines = append(p.lines, triggerLine(ps))
	}
	return nil
}

// triggerLine renders ps as a T: line.
func triggerLine(ps params) string {
	var b strings.Builder
	b.WriteString("T:")
	for j, kvp := range ps {
		if j > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(kvp.K + "$ " + kvp.V)
	}
	return b.String()
}

// printMixedOr prints "this or another X enters/dies" as the one trigger
// Forge writes: the alternatives' Valid lists joined in ValidCard$. Enter
// triggers keep the source alternative's parameters (no TriggerZones$), death
// triggers the other's (TriggerZones$ Battlefield).
func (p *pr) printMixedOr(a *ir.Ability, outer *ir.Trigger, alts []*ir.Trigger) error {
	ev := alts[0].Event
	for _, x := range alts {
		if x.Event != ev || (ev != ir.EvEnters && ev != ir.EvDies) || x.Cmp != nil || x.Word != "" || x.OncePerTurn || x.Cond != nil {
			return unprintable("event:or-mixed", outer.Src)
		}
	}
	defer func() { p.ev, p.evSelf = ir.EvNone, false }()
	p.ev, p.evSelf = ev, false
	head, err := p.program(a.Body)
	if err != nil {
		return err
	}
	name := p.sub("Trig"+head.API, head)
	var valid []string
	var base params
	for _, x := range alts {
		p.ev, p.evSelf = x.Event, triggerSelf(x)
		ps, err := p.triggerParams(a, outer, x)
		if err != nil {
			return err
		}
		v := ""
		for _, kvp := range ps {
			if kvp.K == "ValidCard" {
				v = kvp.V
			}
		}
		valid = append(valid, v)
		if triggerSelf(x) == (ev == ir.EvEnters) {
			base = ps
		}
	}
	base.set("ValidCard", strings.Join(valid, ","))
	base.set("Execute", name)
	p.lines = append(p.lines, triggerLine(base))
	return nil
}

// triggerParams is one trigger line's parameters for alternative x of outer.
func (p *pr) triggerParams(a *ir.Ability, outer, x *ir.Trigger) (params, error) {
	bad := func(what string) error { return unprintable("event:"+what, x.Src) }
	self := p.evSelf
	var ps params
	zones := func() { ps.set("TriggerZones", "Battlefield") }
	subj := func() (string, error) { return p.valid(x.Subject) }
	cmpUsed := false
	switch x.Event {
	case ir.EvEnters:
		ps = params{{"Mode", "ChangesZone"}}
		if self {
			ps.set("Origin", "Any")
			ps.set("Destination", "Battlefield")
			ps.set("ValidCard", "Card.Self")
		} else {
			v, err := subj()
			if err != nil {
				return nil, err
			}
			ps.set("Origin", "Any")
			ps.set("Destination", "Battlefield")
			ps.set("ValidCard", v)
			zones()
		}
	case ir.EvDies, ir.EvPutIntoGraveyard, ir.EvLeaves:
		dest := "Graveyard"
		if x.Event == ir.EvLeaves {
			dest = "Any"
		}
		ps = params{{"Mode", "ChangesZone"}, {"Origin", "Battlefield"}, {"Destination", dest}}
		switch {
		case self:
			ps.set("ValidCard", "Card.Self")
		case x.Event == ir.EvDies:
			v, err := subj()
			if err != nil {
				return nil, err
			}
			ps.set("ValidCard", v)
			zones()
		default:
			return nil, bad(x.Event.String())
		}
	case ir.EvAttacks:
		ps = params{{"Mode", "Attacks"}}
		if self {
			ps.set("ValidCard", "Card.Self")
		} else {
			v, err := subj()
			if err != nil {
				return nil, err
			}
			ps.set("ValidCard", v)
			zones()
		}
	case ir.EvUpkeep, ir.EvEndStep, ir.EvBeginCombat:
		phase := map[ir.EventKind]string{ir.EvUpkeep: "Upkeep", ir.EvEndStep: "End of Turn", ir.EvBeginCombat: "BeginCombat"}[x.Event]
		ps = params{{"Mode", "Phase"}, {"Phase", phase}}
		if x.Subject != nil {
			v, err := subj()
			if err != nil {
				return nil, err
			}
			if v != "Player" {
				ps.set("ValidPlayer", v)
			}
		}
		zones()
	case ir.EvDraws:
		v, err := subj()
		if err != nil {
			return nil, err
		}
		ps = params{{"Mode", "Drawn"}}
		switch v {
		case "You":
			ps.set("ValidCard", "Card.YouCtrl")
		case "Opponent":
			ps.set("ValidCard", "Card.OppCtrl")
		default:
			return nil, bad("draws")
		}
		zones()
		if x.Cmp != nil {
			cmpUsed = true
			if x.Cmp.Op != ir.CmpEQ || !isConst(x.Cmp.V) {
				return nil, bad("cmp")
			}
			ps.set("Number", strconv.Itoa(int(x.Cmp.V.N)))
		}
	case ir.EvGainsLife, ir.EvLosesLife:
		v, err := subj()
		if err != nil {
			return nil, err
		}
		mode := "LifeGained"
		if x.Event == ir.EvLosesLife {
			mode = "LifeLost"
		}
		ps = params{{"Mode", mode}, {"ValidPlayer", v}}
		zones()
		if x.Cmp != nil {
			cmpUsed = true
			if x.Cmp.Op != ir.CmpEQ || !isConst(x.Cmp.V) || x.Cmp.V.N != 1 {
				return nil, bad("cmp")
			}
			ps.set("FirstTime", "True")
		}
	case ir.EvCast:
		ps = params{{"Mode", "SpellCast"}}
		if self {
			ps.set("ValidCard", "Card.Self")
		} else {
			o, err := p.valid(x.Object)
			if err != nil {
				return nil, err
			}
			s, err := subj()
			if err != nil {
				return nil, err
			}
			ps.set("ValidCard", o)
			ps.set("ValidActivatingPlayer", s)
			zones()
		}
	case ir.EvDealsCombatDamageToPlayer, ir.EvDealsNoncombatDamageToPlayer:
		src := "Card.Self"
		if !self {
			v, err := subj()
			if err != nil {
				return nil, err
			}
			src = damageSource(v)
		}
		tgt := "Player"
		if x.Object != nil {
			v, err := p.valid(x.Object)
			if err != nil {
				return nil, err
			}
			tgt = v
		}
		combat := "True"
		if x.Event == ir.EvDealsNoncombatDamageToPlayer {
			combat = "False"
		}
		ps = params{{"Mode", "DamageDone"}, {"ValidSource", src}, {"ValidTarget", tgt}, {"CombatDamage", combat}}
		if !self {
			zones()
		}
	case ir.EvDealsCombatDamage:
		src := "Card.Self"
		if !self {
			v, err := subj()
			if err != nil {
				return nil, err
			}
			src = damageSource(v)
		}
		if x.Object != nil {
			return nil, bad("deals-combat-damage")
		}
		ps = params{{"Mode", "DamageDealtOnce"}, {"CombatDamage", "True"}, {"ValidSource", src}}
		if !self {
			zones()
		}
	case ir.EvPlayerAttacks:
		who, err := subj()
		if err != nil {
			return nil, err
		}
		if who != "You" && who != "Opponent" {
			return nil, bad("player-attacks")
		}
		ps = params{{"Mode", "AttackersDeclared"}}
		if x.Object != nil {
			v, err := p.valid(x.Object)
			if err != nil {
				return nil, err
			}
			ps.set("ValidAttackers", v)
		}
		if x.Cmp != nil {
			cmpUsed = true
			if x.Object == nil || x.Cmp.Op == ir.CmpNone || !isConst(x.Cmp.V) {
				return nil, bad("cmp")
			}
			ps.set("ValidAttackersAmount", cmpWord(x.Cmp.Op)+strconv.Itoa(int(x.Cmp.V.N)))
		}
		ps.set("AttackingPlayer", who)
		zones()
	case ir.EvPlaysLand:
		if x.Subject == nil || x.Object == nil || !isYouFilter(x.Subject) {
			return nil, bad("plays-land")
		}
		own := &ir.Filter{Subject: ir.SubjPermanent, Op: ir.FPred, Pred: ir.PControlledBy, Ref: &ir.Ref{Kind: ir.RefYou}}
		v, err := p.valid(&ir.Filter{Op: ir.FAll, Args: []*ir.Filter{x.Object, own}, Src: x.Src})
		if err != nil {
			return nil, err
		}
		ps = params{{"Mode", "LandPlayed"}, {"ValidCard", v}}
		zones()
	case ir.EvBecomesTarget:
		tgt := "Card.Self"
		if !self {
			v, err := subj()
			if err != nil {
				return nil, err
			}
			tgt = v
		}
		o, err := p.valid(x.Object)
		if err != nil {
			return nil, err
		}
		terms := strings.Split(o, ",")
		for i, t := range terms {
			switch {
			case t == "Card":
				terms[i] = "Spell"
			case strings.HasPrefix(t, "Card."):
				terms[i] = "Spell" + t[len("Card"):]
			default:
				return nil, bad("becomes-target")
			}
		}
		ps = params{{"Mode", "BecomesTarget"}, {"ValidTarget", tgt}, {"ValidSource", strings.Join(terms, ",")}}
		zones()
	case ir.EvCountersPut:
		if x.Word == "" {
			return nil, bad("counters-put")
		}
		o, err := p.valid(x.Object)
		if err != nil {
			return nil, err
		}
		if o == "Permanent.Self" {
			o = "Card.Self" // Forge's spelling of the source
		}
		ps = params{{"Mode", "CounterAddedOnce"}, {"CounterType", x.Word}, {"ValidCard", o}}
		if x.Subject != nil {
			s, err := subj()
			if err != nil {
				return nil, err
			}
			ps.set("ValidSource", s)
		}
		zones()
	default:
		return nil, bad(x.Event.String())
	}
	if x.Cmp != nil && !cmpUsed {
		return nil, bad("cmp")
	}
	if x.Word != "" && x.Event != ir.EvCountersPut {
		return nil, bad("word")
	}
	if a.Zones != 0 && a.Zones != ir.MakeZoneSet(ir.ZoneBattlefield) {
		ps.set("TriggerZones", zoneList(a.Zones))
	}
	if x.OncePerTurn || outer.OncePerTurn {
		ps.set("ActivationLimit", "1")
	}
	conds := []*ir.Cond{a.Cond, x.Cond, outer.Cond}
	if outer == x {
		conds = conds[:2]
	}
	var cond *ir.Cond
	for _, c := range conds {
		if c == nil {
			continue
		}
		if cond != nil {
			return nil, bad("two-conds")
		}
		cond = c
	}
	if cond != nil && self {
		// "if it was kicked" on a self trigger is a qualifier of the source in ValidCard$.
		if w := castFactWord(cond); w != "" {
			for i := range ps {
				if ps[i].K == "ValidCard" && ps[i].V == "Card.Self" {
					ps[i].V += "+" + w
					cond = nil
				}
			}
		}
	}
	if cond != nil {
		f, err := p.cond(cond)
		if err != nil {
			return nil, err
		}
		f = p.settle(f)
		cs, err := f.onTrigger()
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			ps.set(c.K, c.V)
		}
	}
	return ps, nil
}

// isYouFilter reports whether f is exactly "the player is you".
func isYouFilter(f *ir.Filter) bool {
	return f != nil && f.Op == ir.FPred && f.Pred == ir.PPlayerIs && len(f.Args) == 0 && isYou(f.Ref) && f.Word == "" && f.Colors == 0
}

// castFactWord is Forge's Valid qualifier ("kicked", "surged") for a condition
// that tests only that the entering source was cast that way, or "".
func castFactWord(c *ir.Cond) string {
	if c.Kind == ir.CRefMatches && c.Ref != nil && (c.Ref.Kind == ir.RefSelf || c.Ref.Kind == ir.RefEventObject) && c.Filter != nil &&
		c.Filter.Op == ir.FNot && len(c.Filter.Args) == 1 && c.Filter.Args[0] != nil &&
		c.Filter.Args[0].Op == ir.FPred && (c.Filter.Args[0].Pred == ir.PSubtype || c.Filter.Args[0].Pred == ir.PCardType) {
		// "if it wasn't a Demon" is known as the trigger fires, from the source's
		// last known information: a qualifier of the source in ValidCard$.
		return "non" + Respell(c.Filter.Args[0].Word)
	}
	if c.Kind != ir.CRefMatches || c.Ref == nil || c.Ref.Kind != ir.RefEventObject || c.Filter == nil {
		return ""
	}
	switch {
	case isPred(c.Filter, ir.PKicked):
		return "kicked"
	case isPred(c.Filter, ir.PSurged):
		return "surged"
	}
	return ""
}

// damageSource adds the emblems to "a source you control": an emblem is a
// source of damage too, and Forge lists it.
func damageSource(v string) string {
	if v == "Card.YouCtrl" {
		return v + ",Emblem.YouCtrl"
	}
	return v
}

// payCostAbility turns "you may pay C; if you do" on the head of a trigger
// body into Forge's form: an ability of its own, AB$ with Cost$ C.
func payCostAbility(head *node) {
	cost, ok := head.get("UnlessCost")
	if sw, _ := head.get("UnlessSwitched"); !ok || sw != "True" || head.Ability {
		return
	}
	if payer, _ := head.get("UnlessPayer"); payer != "You" {
		return
	}
	for _, k := range []string{"UnlessCost", "UnlessPayer", "UnlessSwitched"} {
		head.Params = removeParam(head.Params, k)
	}
	head.Params = append([]kv{{"Cost", cost}}, head.Params...)
	head.Ability = true
}

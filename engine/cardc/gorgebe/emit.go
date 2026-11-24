package gorgebe

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
)

// This file is the printer core that every wave-3 family prints into
// (docs/design/2026-11-23-typed-script-values/briefs/cc-21a-gorgebe-refs-filters.md).
// It knows no family's spellings: ref.go, filter.go and target.go (CC-21a),
// value.go and cond.go (CC-21b), the step printers (CC-22a/b) and the ability
// printers (CC-23) install themselves from their own files' init.

// kv is one script parameter, printed "K$ V".
type kv struct{ K, V string }

// node is one printed ability (SP$, AB$ or DB$): its API, its parameters in
// print order, and its sub-ability.
type node struct {
	API    string
	Params []kv
	Sub    *node
	Kind   string // "" for DB; "AB" for a sub-ability that carries its own Cost$
}

// set replaces k's value, or appends k.
func (n *node) set(k, v string) {
	for i := range n.Params {
		if n.Params[i].K == k {
			n.Params[i].V = v
			return
		}
	}
	n.Params = append(n.Params, kv{k, v})
}

// get returns k's value.
func (n *node) get(k string) (string, bool) {
	for _, x := range n.Params {
		if x.K == k {
			return x.V, true
		}
	}
	return "", false
}

// last returns the final node of n's sub-ability chain.
func (n *node) last() *node {
	for n.Sub != nil {
		n = n.Sub
	}
	return n
}

// condForm is a condition, renderable on an ability (onAbility) or on a
// trigger or static (onTrigger). Exactly one of SVar, Present and Turn is set.
type condForm struct {
	SVar    string // the SVar holding a count; Cmp compares it
	Present string // a Valid list; Cmp compares how many objects match it
	Defined string // with Present: whose match is counted ("" is Self)
	Field   bool   // with Present: the count is of the whole battlefield, not the source
	Cmp     string // GE1, EQ0, LE10, ...; "" is GE1
	Turn    string // "You": it is your turn; "Opponent": an opponent's turn
	Named   string // "Threshold" or "Hellbent": Forge's named form of the count Body compared by Cmp
	Body    string // the count body not yet bound to an SVar (with Named or Choose); see plain
	Choose  string // a count head ("Count$Morbid") that Forge also spells as an if/else's amount choice
}

// plain returns c in its count form: a Named condition becomes the SVar of
// its Body, for the places (a negation, an if-step) with no named spelling.
func (p *pr) plain(c condForm) condForm {
	if c.Body == "" {
		return c
	}
	c.SVar, c.Named, c.Choose = p.countVar(c.Body), "", ""
	c.Body = ""
	return c
}

// settle is plain, except that a Named condition keeps its name.
func (p *pr) settle(c condForm) condForm {
	if c.Named != "" {
		return c
	}
	return p.plain(c)
}

// svCond is the condition "body <cmp>"; the two counts Forge names
// (Threshold, Hellbent) carry that name too.
func (p *pr) svCond(body, cmp string) condForm {
	switch {
	case body == "Count$ValidGraveyard Card.YouOwn" && cmp == "GE7":
		return condForm{Named: "Threshold", Body: body, Choose: "Count$Threshold", Cmp: cmp}
	case body == "Count$CardsInYourHand" && cmp == "EQ0":
		return condForm{Named: "Hellbent", Body: body, Choose: "Count$Hellbent", Cmp: cmp}
	}
	for _, h := range []string{"Count$Morbid", "Count$Revolt", "Count$Kicked"} {
		if body == h+".1.0" && cmp == "GE1" {
			return condForm{Choose: h, Body: body, Cmp: cmp}
		}
	}
	return condForm{SVar: p.countVar(body), Cmp: cmp}
}

func (c condForm) cmp() string {
	if c.Cmp == "" {
		return "GE1"
	}
	return c.Cmp
}

// onAbility renders c as an ability's Condition*$ parameters.
func (c condForm) onAbility() ([]kv, error) {
	switch {
	case c.SVar != "":
		return []kv{{"ConditionCheckSVar", c.SVar}, {"ConditionSVarCompare", c.cmp()}}, nil
	case c.Present != "":
		if c.Field {
			return []kv{{"ConditionPresent", c.Present}, {"ConditionCompare", c.cmp()}}, nil
		}
		d := c.Defined
		if d == "" {
			d = "Self"
		}
		return []kv{{"ConditionDefined", d}, {"ConditionPresent", c.Present}, {"ConditionCompare", c.cmp()}}, nil
	case c.Turn == "You":
		return []kv{{"ConditionPlayerTurn", "True"}}, nil
	}
	return nil, unprintable("cond:on-ability", "")
}

// onTrigger renders c as a trigger's or static's gate parameters.
func (c condForm) onTrigger() ([]kv, error) {
	switch {
	case c.SVar != "":
		return withCmp([]kv{{"CheckSVar", c.SVar}}, "SVarCompare", c.cmp()), nil
	case c.Present != "":
		if c.Field {
			return withCmp([]kv{{"IsPresent", c.Present}}, "PresentCompare", c.cmp()), nil
		}
		if c.Defined != "" && c.Defined != "Self" {
			return withCmp([]kv{{"IsPresent", c.Present}}, "PresentCompare", c.cmp(), kv{"PresentDefined", c.Defined}), nil
		}
		// IsPresent$ counts every match on the battlefield, so a test of the
		// source itself restricts each term to Self.
		terms := strings.Split(c.Present, ",")
		for i, t := range terms {
			if strings.Contains(t, ".") {
				terms[i] = t + "+Self"
			} else {
				terms[i] = t + ".Self"
			}
		}
		return withCmp([]kv{{"IsPresent", strings.Join(terms, ",")}}, "PresentCompare", c.cmp()), nil
	case c.Named != "":
		return []kv{{c.Named, "True"}}, nil
	case c.Turn == "You":
		return []kv{{"PlayerTurn", "True"}}, nil
	case c.Turn == "Opponent":
		return []kv{{"OpponentTurn", "True"}}, nil
	}
	return nil, unprintable("cond:on-trigger", "")
}

// withCmp appends the comparison parameter unless it is Forge's default, GE1.
func withCmp(ps []kv, key, cmp string, rest ...kv) []kv {
	if cmp != "GE1" {
		ps = append(ps, kv{key, cmp})
	}
	return append(ps, rest...)
}

// onStatic renders c as a static's gate: Forge's Condition$ for the named
// forms, the trigger gate otherwise.
func (c condForm) onStatic() ([]kv, error) {
	switch {
	case c.Named != "":
		return []kv{{"Condition", c.Named}}, nil
	case c.Turn == "You":
		return []kv{{"Condition", "PlayerTurn"}}, nil
	case c.Turn == "Opponent":
		return []kv{{"Condition", "NotPlayerTurn"}}, nil
	}
	return c.onTrigger()
}

// onActivation renders c as an activated ability's gate: Activation$ for the
// named forms, the trigger gate otherwise.
func (c condForm) onActivation() ([]kv, error) {
	if c.Named != "" {
		return []kv{{"Activation", c.Named}}, nil
	}
	return c.onTrigger()
}

// negCmp is the comparison that holds exactly when op does not.
var negCmp = map[string]string{"GE": "LT", "LT": "GE", "LE": "GT", "GT": "LE", "EQ": "NE", "NE": "EQ"}

// not returns the negation of c.
func (c condForm) not() (condForm, error) {
	if c.Named != "" {
		return condForm{}, unprintable("cond:not-named", "")
	}
	if c.Turn != "" {
		if c.Turn == "You" {
			c.Turn = "Opponent"
		} else {
			c.Turn = "You"
		}
		return c, nil
	}
	cmp := c.cmp()
	if len(cmp) < 3 || negCmp[cmp[:2]] == "" {
		return condForm{}, unprintable("cond:compare:"+cmp, "")
	}
	c.Cmp = negCmp[cmp[:2]] + cmp[2:]
	if n, err := strconv.Atoi(cmp[2:]); err == nil && cmp[:2] == "LE" {
		c.Cmp = "GE" + strconv.Itoa(n+1) // Forge's "more than n" is GE(n+1)
	}
	return c, nil
}

// frame is one Program's target declarations.
type frame struct {
	targets  []*ir.Target
	declared []bool
	own      int // with several targets: the slot the step being printed is the operand of, or -1
}

// parent reports whether a reference to slot, in a program of several targets,
// names a slot another node declared: Forge spells that ParentTarget.
func (f *frame) parent(slot uint8) bool {
	return f != nil && len(f.targets) > 1 && f.own >= 0 && int(slot) != f.own
}

// pr prints one face's abilities.
type pr struct {
	lines  []string     // A:, T:, S: and R: lines, in order
	svars  []kv         // SVar:<K>:<V> lines, in allocation order
	ev     ir.EventKind // the enclosing trigger's or replacement's event; EvNone elsewhere
	evSelf bool         // that event's object is the source itself ("when this enters")
	frames []*frame
	// onPrev is set by a step that must adjust the node printed before it
	// (a delayed trigger remembers what the previous step moved); program
	// calls it with the chain tail and clears it.
	onPrev  func(prev *node) error
	sacSelf bool                   // the activated ability being printed sacrifices its source as a cost
	zones   ir.ZoneSet             // the zones the ability being printed works from; 0 is the battlefield
	face    *ir.Face               // the face being printed; nil in unit tests
	excess  bool                   // the damage step before this one binds ExcessSVar$ Excess
	prev    *ir.Step               // the step printed just before the one being printed, in its program
	cdaAt   int                    // 1 + the index in lines of the last power-only characteristic-defining static, 0 for none
	addCost []string               // additional costs of the spell being printed, in Cost$ spelling
	tokens  map[string]*cards.Card // Forge token scripts by name (cards.Registry.Tokens); nil when unknown
}

// abilityZone is the single zone the ability being printed works from, or
// ZoneNone when it works from several.
func (p *pr) abilityZone() ir.Zone {
	if p.face == nil { // a unit test's printer, with no ability
		return ir.ZoneNone
	}
	zs := p.zones
	if zs == 0 {
		return ir.ZoneBattlefield
	}
	if z := zs.Zones(); len(z) == 1 {
		return z[0]
	}
	return ir.ZoneNone
}

// The family printers. Each is nil until its file installs it from init.
var (
	refFn    func(p *pr, r *ir.Ref) (string, error)    // ref.go (CC-21a): a Defined$ spelling
	validFn  func(p *pr, f *ir.Filter) (string, error) // filter.go (CC-21a): a Valid list
	targetFn func(p *pr, t *ir.Target) ([]kv, error)   // target.go (CC-21a): ValidTgts$ and friends
	valueFn  func(p *pr, v *ir.Value) (string, error)  // value.go (CC-21b): a count expression or a literal
	condFn   func(p *pr, c *ir.Cond) (condForm, error) // cond.go (CC-21b)
)

func unprintable(what, path string) error { return &Unprintable{What: what, Path: path} }

// defined spells r as a Defined$ value.
func (p *pr) defined(r *ir.Ref) (string, error) {
	if r == nil {
		return "", unprintable("ref:nil", "")
	}
	if refFn == nil {
		return "", unprintable("ref:"+r.Kind.String(), r.Src)
	}
	return refFn(p, r)
}

// valid spells f as a Valid list (ValidTgts$, ValidCards$, ...).
func (p *pr) valid(f *ir.Filter) (string, error) {
	if f == nil {
		return "", unprintable("filter:nil", "")
	}
	if validFn == nil {
		return "", unprintable("filter:"+f.Op.String(), f.Src)
	}
	return validFn(p, f)
}

// target spells one target declaration as parameters.
func (p *pr) target(t *ir.Target) ([]kv, error) {
	if t == nil {
		return nil, unprintable("target:nil", "")
	}
	if targetFn == nil {
		return nil, unprintable("target", t.Src)
	}
	return targetFn(p, t)
}

// amount spells v as an amount parameter: a decimal literal, or the name of
// an SVar holding v's count expression.
func (p *pr) amount(v *ir.Value) (string, error) { return p.amountAs("", v) }

// amountAs is amount with a chosen name stem ("X") for a new count SVar.
func (p *pr) amountAs(stem string, v *ir.Value) (string, error) {
	if v == nil {
		return "", unprintable("value:nil", "")
	}
	if v.Kind == ir.VConst {
		return strconv.Itoa(int(v.N)), nil
	}
	if v.Kind == ir.VExcessDamage {
		// the name the damage step binds with ExcessSVar$
		if !p.excess {
			return "", unprintable("value:excess-damage", v.Src)
		}
		return "Excess", nil
	}
	if v.Kind == ir.VReceiptAmount && v.Word == "life-lost" {
		// the life lost by the step before: Forge's LoseLife fills this SVar
		if !p.hasSVar("AFLifeLost") {
			p.svars = append(p.svars, kv{"AFLifeLost", "Number$0"})
		}
		return "AFLifeLost", nil
	}
	if valueFn == nil {
		return "", unprintable("value:"+v.Kind.String(), v.Src)
	}
	body, err := valueFn(p, v)
	if err != nil {
		return "", err
	}
	if _, err := strconv.Atoi(body); err == nil {
		return body, nil
	}
	if stem != "" {
		return p.countVarAs(stem, body), nil
	}
	return p.countVar(body), nil
}

// cond spells c.
func (p *pr) cond(c *ir.Cond) (condForm, error) {
	if c == nil {
		return condForm{}, unprintable("cond:nil", "")
	}
	if condFn == nil {
		return condForm{}, unprintable("cond:"+c.Kind.String(), c.Src)
	}
	return condFn(p, c)
}

// countVar returns the SVar holding body, adding it when new. Count$xPaid is
// X; any other body is N, N2, N3, ...
func (p *pr) countVar(body string) string {
	if body == "Count$xPaid" {
		return p.countVarAs("X", body)
	}
	for _, s := range p.svars {
		if s.V == body {
			return s.K
		}
	}
	// Forge names its count SVars X, then Y, then Z.
	// A face that uses xPaid keeps X for it. (A printer with no face is a unit
	// test's, which names every count N.)
	for _, name := range []string{"X", "Y", "Z"} {
		if p.face == nil {
			break
		}
		if name == "X" && usesX(reflect.ValueOf(p.face)) {
			continue
		}
		if !p.hasSVar(name) {
			p.svars = append(p.svars, kv{name, body})
			return name
		}
	}
	return p.countVarAs("N", body)
}

// countVarAs is countVar with a chosen name stem for a new SVar.
func (p *pr) countVarAs(base, body string) string {
	for _, s := range p.svars {
		if s.V == body {
			return s.K
		}
	}
	name := p.fresh(base)
	p.svars = append(p.svars, kv{name, body})
	return name
}

// fresh returns base, or base2, base3, ..., whichever no SVar has yet.
func (p *pr) fresh(base string) string {
	name := base
	for i := 2; p.hasSVar(name); i++ {
		name = base + strconv.Itoa(i)
	}
	return name
}

func (p *pr) hasSVar(name string) bool {
	for _, s := range p.svars {
		if s.K == name {
			return true
		}
	}
	return false
}

// render returns n as "<kind>$ <API> | K$ V | ... | SubAbility$ <name>",
// adding each sub-ability as an SVar DB<API>[k] holding "DB$ ...".
func (p *pr) render(kind string, n *node) string {
	impliedTgtZone(n)
	var b strings.Builder
	b.WriteString(kind + "$ " + n.API)
	for _, x := range n.Params {
		b.WriteString(" | " + x.K + "$ " + x.V)
	}
	if n.Sub != nil {
		b.WriteString(" | SubAbility$ " + p.sub("DB"+n.Sub.API, n.Sub))
	}
	return b.String()
}

// impliedTgtZone is the node-level spelling fixes: a discard's opponents are
// "Opponent", and it drops a ChangeZone's TgtZone$ that only repeats its Origin$:
// the cast census reads the target zone from Origin$ (engine/rules
// cast_targets.go originImpliedTargetZone), and Forge writes it that way.
func impliedTgtZone(n *node) {
	if n.API == "Discard" {
		// Forge spells the opponents of a discard "Opponent".
		if d, ok := n.get("Defined"); ok && d == "Player.Opponent" {
			n.set("Defined", "Opponent")
		}
		return
	}
	if n.API != "ChangeZone" {
		return
	}
	tz, ok := n.get("TgtZone")
	if !ok {
		return
	}
	if o, _ := n.get("Origin"); o == tz && (o == "Graveyard" || o == "Exile") {
		for i, x := range n.Params {
			if x.K == "TgtZone" {
				n.Params = append(n.Params[:i], n.Params[i+1:]...)
				break
			}
		}
	}
}

// sub adds n as an SVar named base[k] holding "DB$ ..." and returns the name,
// for SubAbility$, Execute$ and ReplaceWith$.
func (p *pr) sub(base string, n *node) string {
	name := p.fresh(base)
	i := len(p.svars)
	p.svars = append(p.svars, kv{name, ""}) // reserve the name before the chain renders
	kind := "DB"
	if n.Kind != "" {
		kind = n.Kind
	}
	p.svars[i].V = p.render(kind, n)
	return name
}

// stepFunc prints the steps at the head of steps: the nodes it printed,
// chained in order, and how many steps it consumed (at least 1). It must
// handle steps[0].Optional or return an Unprintable.
type stepFunc func(p *pr, steps []*ir.Step) (*node, int, error)

var stepFns = map[ir.Verb]stepFunc{} // read by key only

// registerStep installs f for v. A second registration of v panics.
func registerStep(v ir.Verb, f stepFunc) {
	if stepFns[v] != nil {
		panic(fmt.Sprintf("gorgebe: verb %s registered twice", v))
	}
	stepFns[v] = f
}

// abilityFunc prints a, appending its lines (and SVars) to p.
type abilityFunc func(p *pr, a *ir.Ability) error

var abilityFns = map[ir.AbilityKind]abilityFunc{} // read by key only

// registerAbility installs f for k. A second registration of k panics.
func registerAbility(k ir.AbilityKind, f abilityFunc) {
	if abilityFns[k] != nil {
		panic(fmt.Sprintf("gorgebe: ability kind %s registered twice", k))
	}
	abilityFns[k] = f
}

// program prints prog as a chain of nodes in step order. A program with
// targets opens a frame its steps declare them in.
func (p *pr) program(prog *ir.Program) (*node, error) {
	if prog == nil || len(prog.Steps) == 0 {
		return nil, unprintable("program:empty", "")
	}
	var fr *frame
	if len(prog.Targets) > 0 {
		fr = &frame{targets: prog.Targets, declared: make([]bool, len(prog.Targets)), own: -1}
		p.frames = append(p.frames, fr)
		defer func() { p.frames = p.frames[:len(p.frames)-1] }()
	}
	savedPrev := p.prev
	p.prev = nil
	defer func() { p.prev = savedPrev }()
	var head, tail *node
	for i := 0; i < len(prog.Steps); {
		st := prog.Steps[i]
		f := stepFns[st.Verb]
		if f == nil {
			return nil, unprintable("verb:"+st.Verb.String(), st.Src)
		}
		var pre []*node // TargetOnly nodes declaring the slots before this step's own
		if fr != nil && len(fr.targets) > 1 {
			fr.own = -1
			if o := slotRef(st.Obj); o != nil && o.Kind == ir.RefTarget && int(o.Slot) < len(fr.targets) {
				fr.own = int(o.Slot)
				for j := 0; j < fr.own; j++ {
					if fr.declared[j] {
						continue
					}
					ps, err := p.target(fr.targets[j])
					if err != nil {
						return nil, err
					}
					fr.declared[j] = true
					pre = append(pre, &node{API: "TargetOnly", Params: ps})
				}
			}
		}
		n, used, err := f(p, prog.Steps[i:])
		if err != nil {
			return nil, err
		}
		if n == nil || used < 1 {
			return nil, unprintable("verb:"+st.Verb.String(), st.Src)
		}
		if p.onPrev != nil {
			hook := p.onPrev
			p.onPrev = nil
			if tail == nil {
				return nil, unprintable("verb:"+st.Verb.String()+"-first", st.Src)
			}
			if err := hook(tail); err != nil {
				return nil, err
			}
		}
		for k := len(pre) - 1; k >= 0; k-- {
			pre[k].Sub, n = n, pre[k]
		}
		p.prev = prog.Steps[i+used-1]
		if head == nil {
			head = n
		} else {
			tail.Sub = n
		}
		tail = n.last()
		i += used
	}
	if len(prog.Targets) > 0 {
		for i, d := range p.frame().declared {
			if !d {
				return nil, unprintable("target:unused", prog.Targets[i].Src)
			}
		}
	}
	return head, nil
}

// frame returns the innermost frame, or nil outside any.
func (p *pr) frame() *frame {
	if len(p.frames) == 0 {
		return nil
	}
	return p.frames[len(p.frames)-1]
}

// declare returns slot's target parameters the first time a node names the
// slot, and nil after. Wave 3 prints one target slot per program.
func (p *pr) declare(slot uint8, src string) ([]kv, error) {
	f := p.frame()
	if f == nil || int(slot) >= len(f.targets) {
		return nil, unprintable("target:unbound", src)
	}
	for j := 0; j < int(slot); j++ {
		if !f.declared[j] {
			return nil, unprintable("target:multi", src)
		}
	}
	if f.declared[slot] {
		return nil, nil
	}
	f.declared[slot] = true
	return p.target(f.targets[slot])
}

// operand puts r on n: a target slot's declaration the first time the slot
// is named, else key$ <Defined>.
func (p *pr) operand(n *node, key string, r *ir.Ref) error {
	if r == nil {
		return unprintable("ref:nil", "")
	}
	r = slotRef(r)
	if r.Kind == ir.RefTarget {
		ps, err := p.declare(r.Slot, r.Src)
		if err != nil {
			return err
		}
		if ps != nil {
			for _, x := range ps {
				n.set(x.K, x.V)
			}
			return nil
		}
	}
	d, err := p.defined(r)
	if err != nil {
		return err
	}
	n.set(key, d)
	return nil
}

// attachWord is Forge's word for what the source is attached to: "Equipped"
// for equipment, "Enchanted" for an aura, "" when the face is neither.
func (p *pr) attachWord() string {
	if p.face == nil {
		return ""
	}
	for _, t := range p.face.Subtypes {
		switch t {
		case "Equipment":
			return "Equipped"
		case "Aura":
			return "Enchanted"
		}
	}
	return ""
}

// slotRef is the target slot r names when r is "each object target slot k
// chose" (RefAll of "is target k"), which Forge spells as that slot's own
// target declaration; any other r is returned as is.
func slotRef(r *ir.Ref) *ir.Ref {
	if r == nil || r.Kind != ir.RefAll {
		return r
	}
	f := r.Filter
	if f != nil && f.Op == ir.FPred && f.Pred == ir.PIs && len(f.Args) == 0 && f.Word == "" && f.Ref != nil && f.Ref.Kind == ir.RefTarget {
		return f.Ref
	}
	return r
}

var valueType = reflect.TypeOf(ir.Value{})

// usesX reports whether an ir.Value of kind VX is reachable from v.
func usesX(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return false
		}
		if v.Type().Elem() == valueType && v.Interface().(*ir.Value).Kind == ir.VX {
			return true
		}
		return usesX(v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() && usesX(v.Field(i)) {
				return true
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if usesX(v.Index(i)) {
				return true
			}
		}
	}
	return false
}

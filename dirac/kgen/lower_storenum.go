package kgen

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// lower_storenum.go: StoreSVar's per-object number (the persistent own-SVar
// form; gorge effStoreSVar writes Object.RuntimeSVars, and every reader checks
// that store before the printed SVar body). One write op, kop.ExtStoreNum, and
// one read expression, kop.ExSVarNum, over the object-set store (dirac/
// act_storenum.go).
//
// A name is "stored" for a face when some StoreSVar of an ability or trigger
// writes it; the face then reads it only through lc.value, which lowers the
// read to ExSVarNum over the printed body. Reads through any other path
// (a nested SVar$ head, a modifier operand) and the condition forms that name
// the SVar (CheckSVar$, ConditionCheckSVar$) refuse by name: the kernel would
// answer the printed value where gorge answers the stored one.
//
// What lowers: an activated ability's or a trigger's write of Type$ Number,
// Calculate or CountSVar whose Expression$ is a literal, an SVar or an SVar
// with a /Op suffix, onto a card whose SVar has a printed body. The kernel
// mirrors gorge: the number belongs to the object and only a departure from
// the battlefield clears it. A spell's write lowers only when its chain resets
// the name on every path (spellStoreResets: gorge keeps the number on the card
// in every zone it visits until it next leaves the battlefield); Tempt with
// Bunnies and Tempt with Vengeance count the accepted offers this way.
// Refused by name: Type$ Triggered, Targeted and the rest, a spell's write
// without that reset, a mana or latent ability's, and a table's 16th distinct
// SVar name.

// refuseStoredNested is the refusal of a stored name read through a nested
// SVar head or a modifier operand.
func refuseStoredNested(name string) error {
	return refuse("api StoreSVar: SVar %s is written by a StoreSVar and read through a nested SVar head or modifier operand", name)
}

// storesNum reports whether a StoreSVar of this face writes name as a number.
func (lc *lowerCtx) storesNum(name string) bool {
	if lc.numNames == nil {
		lc.numNames = storedNumNames(lc.face)
	}
	return lc.numNames[name]
}

// storedNumNames is the SVar names the face's StoreSVar nodes write through
// the number store: every Number, Calculate or CountSVar write except the
// repeat gate's and the join-forces accumulator's own (lower_storesvar.go),
// and the first-cast stamp's name (Count$YouCastThisGame; lower_firstcast.go).
func storedNumNames(f *cards.Face) map[string]bool {
	out := map[string]bool{}
	gates := map[string]bool{} // repeat gates and join-forces accumulators: their own lowerings
	var cand []string
	walkFaceNodes(f, func(sa *cards.SA) {
		if g := strings.TrimSpace(sa.Params["RepeatCheckSVar"]); g != "" {
			gates[g] = true
		}
		if sa.API != "StoreSVar" {
			return
		}
		if sa.Params["UnlessCost"] != "" {
			gates[strings.TrimSpace(sa.Params["SVar"])] = true // the accumulator's add; its reset is no number store either
			return
		}
		if strings.TrimSpace(sa.Params["Expression"]) == "Count$YouCastThisGame" {
			gates[strings.TrimSpace(sa.Params["SVar"])] = true
			return
		}
		switch strings.TrimSpace(sa.Params["Type"]) {
		case "Number", "Calculate", "CountSVar":
			cand = append(cand, strings.TrimSpace(sa.Params["SVar"]))
		}
	})
	for _, n := range cand {
		if n != "" && !gates[n] {
			out[n] = true
		}
	}
	return out
}

// operandBody is the printed body of the SVar a value modifier's operand
// names. A name a StoreSVar writes has none here (ok false, so the caller
// refuses): the operand would read its printed value where gorge reads the
// stored number.
func (lc *lowerCtx) operandBody(name string) (string, bool) {
	if lc.storesNum(name) {
		return "", false
	}
	body, ok := lc.face.SVars[name]
	return body, ok
}

// storedNumValue is the read of a stored name: ExSVarNum over the printed body.
func (lc *lowerCtx) storedNumValue(name string) (lValue, error) {
	def, err := lc.printedDefault(name)
	if err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vSVarNum, S: name, Sub: []lValue{def}}, nil
}

// printedDefault is the value the name's printed SVar body reads as.
func (lc *lowerCtx) printedDefault(name string) (lValue, error) {
	body, ok := lc.face.SVars[name]
	if !ok {
		return lValue{}, refuse("api StoreSVar: SVar %s has no printed body to read before its first write", name)
	}
	v, err := lc.svarValue(body, 0)
	if err != nil {
		return lValue{}, refuse("api StoreSVar: SVar %s=%q: %v", name, body, err)
	}
	return v, nil
}

// storeNum lowers one StoreSVar node to ExtStoreNum.
func (lc *lowerCtx) storeNum(ps *params, name, typ, expr string) error {
	switch typ {
	case "Number", "Calculate", "CountSVar":
	default:
		return refuse("api StoreSVar Type$ %s", typ)
	}
	if name == "" || expr == "" {
		return refuse("api StoreSVar without SVar$ or Expression$")
	}
	if lc.ab == nil {
		return refuse("api StoreSVar outside an ability")
	}
	switch lc.ab.Kind {
	case akActivated, akTriggered:
	case akSpell:
		// gorge keeps the number on the card past the resolution: the spell
		// must reset it on every path (spellStoreResets, after its chain)
		if lc.mode >= 0 {
			return refuse("api StoreSVar in a spell's charm mode")
		}
	default:
		return refuse("api StoreSVar in a %s ability", abilityKindNames[lc.ab.Kind])
	}
	if !lc.storesNum(name) {
		return refuse("api StoreSVar: SVar %s", name)
	}
	if lc.numCondRead(name) {
		return refuse("api StoreSVar: SVar %s is read by a CheckSVar$ condition", name)
	}
	def, err := lc.printedDefault(name)
	if err != nil {
		return err
	}
	base, op, hasOp := strings.Cut(expr, "/")
	v, err := lc.value(base)
	if err != nil {
		return refuse("api StoreSVar Expression$ %s: %v", expr, err)
	}
	if hasOp {
		if v, err = lc.applyModifier(v, op, 0); err != nil {
			return refuse("api StoreSVar Expression$ %s: %v", expr, err)
		}
	}
	// a write of the printed default deletes the record when that default is a
	// literal the C field holds (kop.StoreNumC)
	c := 0
	if def.isConst() && kop.StoreNumDefFits(def.N) {
		c = def.N<<1 | 1
	}
	ps.consume("SVar", "Type", "Expression")
	lc.emit(lInstr{Op: "ExtStoreNum", B: v, C: c, CS: name})
	return nil
}

// spellStoreResets holds a spell's number writes to gorge's reading of them:
// the number stays on the card after the resolution (Object.RuntimeSVars is
// cleared only when the object leaves the battlefield, CR 400.7), so a spell
// that writes a name must hand it back to its printed value on every path, or
// the next cast of that card (or a copy of it in another zone) would read the
// leftover. The tempting-offer sorceries do: the chain's last write of the
// name is an unconditional Number write of the printed default (a literal),
// outside any loop or condition, so the record is gone when the resolution
// ends. Any other spell write refuses by name. Called once the spell's chain
// is lowered.
func (lc *lowerCtx) spellStoreResets() error {
	ab := lc.ab
	if ab == nil || ab.Kind != akSpell {
		return nil
	}
	last := map[string]int{}
	var names []string
	for i := range ab.Prog {
		if in := &ab.Prog[i]; in.Op == "ExtStoreNum" {
			if _, ok := last[in.CS]; !ok {
				names = append(names, in.CS)
			}
			last[in.CS] = i
		}
	}
	sort.Strings(names)
	for _, n := range names {
		i := last[n]
		in := &ab.Prog[i]
		if !in.B.isConst() || in.C&1 == 0 || in.B.N != in.C>>1 {
			return refuse("api StoreSVar in a spell: %s is not reset to its printed default by the chain's last write (gorge keeps the number on the card past the resolution)", n)
		}
		for j := 0; j < i; j++ {
			if p := &ab.Prog[j]; p.Skip > 0 && j+p.Skip >= i {
				return refuse("api StoreSVar in a spell: the reset of %s is inside a loop or condition, not on every path", n)
			}
		}
	}
	return nil
}

// numCondRead reports whether a node of the face reads name through a
// CheckSVar$-style condition param.
func (lc *lowerCtx) numCondRead(name string) bool {
	hit := false
	walkFaceNodes(lc.face, func(sa *cards.SA) {
		for k, v := range sa.Params {
			if strings.HasSuffix(k, "CheckSVar") && strings.TrimSpace(v) == name {
				hit = true
			}
		}
	})
	for i := range lc.face.Triggers {
		for k, v := range lc.face.Triggers[i].Params {
			if strings.HasSuffix(k, "CheckSVar") && strings.TrimSpace(v) == name {
				hit = true
			}
		}
	}
	return hit
}

// walkFaceNodes calls fn on every ability node of the face: each ability's and
// trigger's SubAbility chain, and the chain of any SVar a parameter names (a
// Repeat's RepeatSubAbility$, a Charm's Choices$), once per name. The visit
// order does not reach any output: callers collect sets.
func walkFaceNodes(f *cards.Face, fn func(*cards.SA)) {
	seen := map[string]bool{}
	var walk func(sa *cards.SA)
	walk = func(sa *cards.SA) {
		for ; sa != nil; sa = sa.Sub {
			fn(sa)
			for k, v := range sa.Params {
				names := []string{v}
				if k == "Choices" {
					names = strings.Split(v, ",") // a GenericChoice's branch SVars (lower_storechoice.go)
				}
				for _, n := range names {
					n = strings.TrimSpace(n)
					if _, ok := f.SVars[n]; ok && !seen[n] {
						seen[n] = true
						walk(cards.ResolveSVar(f.SVars, n))
					}
				}
			}
		}
	}
	for _, sa := range f.Abilities {
		walk(sa)
	}
	for i := range f.Triggers {
		walk(f.Triggers[i].Effect)
	}
	for i := range f.Repls {
		walk(f.Repls[i].With) // an as-enters body that stores (lower_paylife.go)
	}
}

// svarIndex is name's index in the table's SVarNames, assigned at first use.
func (cb *cardBuilder) svarIndex(name string) (int16, error) {
	pt := cb.pt
	for i, n := range pt.svarNames {
		if n == name {
			return int16(i), nil
		}
	}
	if kop.SVarNamesFull(len(pt.svarNames)) {
		return 0, refuse("api StoreSVar: SVar %s is the table's %dth stored name (the store names at most 15)", name, len(pt.svarNames)+1)
	}
	pt.svarNames = append(pt.svarNames, name)
	return int16(len(pt.svarNames) - 1), nil
}

func init() {
	exprKinds[vSVarNum] = kop.ExSVarNum
	instrOp("ExtStoreNum", (*instrCtx).opExtStoreNum)
}

func (ic *instrCtx) opExtStoreNum() error {
	ki, in := &ic.ki, &ic.in
	idx, err := ic.cb.svarIndex(in.CS)
	if err != nil {
		return err
	}
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtStoreNum))
	ki.C = kop.StoreNumC(int(idx), in.C&1 != 0, in.C>>1)
	return nil
}

// chainTail lowers an ability's chain and then its end-of-chain checks: the
// spell's StoreSVar resets, the remembered-opponent clear (lower_remembered.go)
// and the any-activator window, read once the chain has declared its targets
// (lower_anyactivator.go).
func (lc *lowerCtx) chainTail(sa *cards.SA, ps *params, ab *lAbility) error {
	if err := lc.chain(sa, ps); err != nil {
		return err
	}
	if err := lc.spellStoreResets(); err != nil {
		return err
	}
	lc.clearRemOpp()
	return anyActivatorWindow(ab)
}

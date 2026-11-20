package kgen

import "strings"

// cloneNewName reads a Clone's NewName$, "except its name is <Name>" (CR
// 707.9b: a name is a copiable value, so the exception rides the copy and a
// copy of the copy inherits it). The kernel's exception override names a
// table card as the layer-3 name (kop.Override.NameCard, the card
// FeatNameCard and nameCardAt read), so the name resolves here, at kgen
// time, to a card of the pool: a name no pool card prints refuses by name
// (the name table is the pool's). Every corpus NewName$ is the copying
// card's own name (Sakashima the Impostor, Absorbing Man), which the pool
// always holds. It sets exc.NameOf and reports the exception as present.
func (lc *lowerCtx) cloneNewName(ps *params, exc *lOverride, has *bool, what string) error {
	v, ok := ps.get("NewName")
	if !ok {
		return nil
	}
	name := strings.TrimSpace(v)
	if name == "" {
		return refuse("%s NewName$ is empty", what)
	}
	if !lc.p.cardOfTable(name) {
		return refuse("%s NewName$ %q names no card of the pool", what, name)
	}
	exc.NameOf, *has = name, true
	return nil
}

// cardOfTable reports whether name is a card of the pool TABLE, the same name
// set the builder keys cardIdx by (lowerPool names every lw.cards entry from
// p.names). byName can resolve a name Lookup accepts that the table does not
// print (a face name), so validating against it alone would pass lowering and
// then fail the build's cardIdx lookup.
func (p *pool) cardOfTable(name string) bool {
	if _, ok := p.byName[name]; !ok {
		return false
	}
	if len(p.names) == 0 {
		return true // a synthetic pool names its cards by byName alone
	}
	for _, n := range p.names {
		if n == name {
			return true
		}
	}
	return false
}

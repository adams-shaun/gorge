package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The table-prep registries. A feature registers its prep facts from its own
// file, so activating a manifest code adds no term to prepareGen's gate and no
// bool to genPrep (gen.go), the two lines every such change used to edit:
//
//   - genNeed(f): prepareGen builds a genPrep for every table f reports, as
//     well as for the tables its own gate opens. A table with a genPrep takes
//     the generated forms of the static and cost paths, so a need says the
//     feature reads them.
//   - newGenUse(f): a per-table flag. g.uses.has(u) reports whether f held
//     for the table, and a table f holds for gets a genPrep. It stands in for
//     a new genPrep bool; the existing bools stay, so no hot read changes.
//
// Both run at table prep only, never in a transition. A use's bit is its
// registration order, which only this prep reads: it reaches no table, State
// byte or hash.
//
// Usage, at package level in the feature's file:
//
//	var useFoo = newGenUse(func(t *kop.Table) bool { return tableHasFoo(t) })
//	func init() { genNeed(func(t *kop.Table) bool { return tableReadsBar(t) }) }

// genUse is a flag bit in genPrep.uses (newGenUse).
type genUse uint16

// useWords bounds the registered uses at 64 each; newGenUse panics past it.
const useWords = 4

// useSet is genPrep.uses: one bit per registered genUse.
type useSet [useWords]uint64

// has reports u's bit.
func (s *useSet) has(u genUse) bool { return s[u>>6]&(1<<(u&63)) != 0 }

func (s *useSet) set(u genUse) { s[u>>6] |= 1 << (u & 63) }

var (
	genNeeds []func(*kop.Table) bool
	genUses  []func(*kop.Table) bool
)

// genNeed registers need: prepareGen builds a genPrep for a table need
// reports.
func genNeed(need func(*kop.Table) bool) { genNeeds = append(genNeeds, need) }

// newGenUse registers detect and returns the bit it sets in genPrep.uses for
// a table it reports.
func newGenUse(detect func(*kop.Table) bool) genUse {
	if len(genUses) == useWords*64 {
		panic("dirac: genPrep.uses is full: raise useWords (genneed.go)")
	}
	genUses = append(genUses, detect)
	return genUse(len(genUses) - 1)
}

// genNeeded reports whether a registered need or use holds for t: prepareGen's
// gate term.
func genNeeded(t *kop.Table) bool {
	for _, f := range genNeeds {
		if f(t) {
			return true
		}
	}
	for _, f := range genUses {
		if f(t) {
			return true
		}
	}
	return false
}

// tableUses is t's genPrep.uses.
func tableUses(t *kop.Table) (s useSet) {
	for i, f := range genUses {
		if f(t) {
			s.set(genUse(i))
		}
	}
	return s
}

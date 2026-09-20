package searchprobe

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"slices"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/state"
)

type proposalCard struct {
	ID   state.ObjID
	Name string
}

type positionConstraint struct {
	Index int
	Name  string
	Obj   state.ObjID
}

type deadlineConstraint struct {
	Through int
	Name    string
	Count   int
}

// exclusionConstraint is the deadline's dual: at most Cap copies of Name may
// occupy shuffle positions below Through. Cap 0 is the sampler's
// PolicyCompetition lever -- the named card must not be drawn early enough to
// fill the hand a bot prefers over the observed cast -- and Cap > 0 leaves
// room for the copies that are already known to be in hand at plan time.
// It is a hard feasibility constraint counted exactly like a deadline, so the
// permutation sampler re-draws rather than merely penalising such an order.
type exclusionConstraint struct {
	Through int
	Name    string
	Cap     int
}

type proposalRandom interface {
	Uint64() uint64
}

type constraintCounter struct {
	n           int
	stop        int
	names       []string
	nameIndex   map[string]int
	fixedObj    map[int]proposalCard
	fixedName   map[int]string
	exactPrefix [][]int
	initialFree []int
	deadlines   []deadlineConstraint
	exclusions  []exclusionConstraint
	memo        map[string]*big.Int
	factorials  []*big.Int
}

type constrainedPermutation struct {
	cards      []proposalCard
	positions  []positionConstraint
	deadlines  []deadlineConstraint
	exclusions []exclusionConstraint
	counter    *constraintCounter
	available  []proposalCard
	total      *big.Int
	weight     float64
}

type constraintPlanCache struct {
	plans []*constrainedPermutation
}

func newConstraintPlanCache() *constraintPlanCache { return &constraintPlanCache{} }

func (c *constraintPlanCache) get(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint) (*constrainedPermutation, error) {
	return c.getWithExclusions(cards, positions, deadlines, nil)
}

func (c *constraintPlanCache) getWithExclusions(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint, exclusions []exclusionConstraint) (*constrainedPermutation, error) {
	for _, plan := range c.plans {
		if slices.Equal(plan.cards, cards) && slices.Equal(plan.positions, positions) && slices.Equal(plan.deadlines, deadlines) && slices.Equal(plan.exclusions, exclusions) {
			return plan, nil
		}
	}
	plan, err := newConstrainedPermutation(cards, positions, deadlines, exclusions)
	if err != nil {
		return nil, err
	}
	c.plans = append(c.plans, plan)
	return plan, nil
}

func newConstrainedPermutation(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint, exclusions []exclusionConstraint) (*constrainedPermutation, error) {
	counter, available, err := newConstraintCounterWithExclusions(cards, positions, deadlines, exclusions)
	if err != nil {
		return nil, err
	}
	total := counter.total(available)
	logFactorial, _ := math.Lgamma(float64(len(cards) + 1))
	return &constrainedPermutation{
		cards: append([]proposalCard(nil), cards...), positions: append([]positionConstraint(nil), positions...),
		deadlines: append([]deadlineConstraint(nil), deadlines...), exclusions: append([]exclusionConstraint(nil), exclusions...),
		counter:   counter,
		available: available, total: total, weight: logBigInt(total) - logFactorial,
	}, nil
}

func sampleConstrainedPermutation(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint, r proposalRandom) ([]state.ObjID, float64, bool, error) {
	plan, err := newConstrainedPermutation(cards, positions, deadlines, nil)
	if err != nil {
		return nil, 0, false, err
	}
	return plan.sample(r)
}

func (p *constrainedPermutation) sample(r proposalRandom) ([]state.ObjID, float64, bool, error) {
	total := p.total
	if total.Sign() == 0 {
		return nil, 0, false, nil
	}
	if r == nil {
		return nil, 0, false, fmt.Errorf("missing permutation randomness")
	}
	rank := randomBigBelow(r, total)
	counter := p.counter
	order := make([]state.ObjID, counter.n)
	remaining := append([]proposalCard(nil), p.available...)
	counts := append([]int(nil), counter.initialFree...)
	other := len(remaining) - sumInts(counts)
	for pos := 0; pos < counter.n; pos++ {
		if card, ok := counter.fixedObj[pos]; ok {
			order[pos] = card.ID
			continue
		}
		chosen := -1
		for i, card := range remaining {
			if name := counter.fixedName[pos]; name != "" && card.Name != name {
				continue
			}
			nextOther := other
			countIndex, relevant := counter.nameIndex[card.Name]
			if relevant {
				counts[countIndex]--
			} else {
				nextOther--
			}
			completions := counter.count(pos+1, counts, nextOther)
			if rank.Cmp(completions) < 0 {
				chosen = i
				other = nextOther
				break
			}
			if relevant {
				counts[countIndex]++
			}
			rank.Sub(rank, completions)
		}
		if chosen < 0 {
			return nil, 0, false, fmt.Errorf("constraint unranking exhausted at position %d", pos)
		}
		order[pos] = remaining[chosen].ID
		remaining = append(remaining[:chosen], remaining[chosen+1:]...)
	}
	return order, p.weight, true, nil
}

func newConstraintCounter(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint) (*constraintCounter, []proposalCard, error) {
	return newConstraintCounterWithExclusions(cards, positions, deadlines, nil)
}

func newConstraintCounterWithExclusions(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint, exclusions []exclusionConstraint) (*constraintCounter, []proposalCard, error) {
	c := &constraintCounter{n: len(cards), fixedObj: make(map[int]proposalCard), fixedName: make(map[int]string), memo: make(map[string]*big.Int)}
	c.factorials = make([]*big.Int, len(cards)+1)
	c.factorials[0] = big.NewInt(1)
	for i := 1; i <= len(cards); i++ {
		c.factorials[i] = new(big.Int).Mul(c.factorials[i-1], big.NewInt(int64(i)))
	}
	byID := make(map[state.ObjID]proposalCard, len(cards))
	relevant := make(map[string]bool)
	for _, card := range cards {
		if card.ID == 0 || byID[card.ID].ID != 0 {
			return nil, nil, fmt.Errorf("invalid or duplicate physical card %d", card.ID)
		}
		byID[card.ID] = card
	}
	usedObjects := make(map[state.ObjID]bool)
	for _, p := range positions {
		if p.Index < 0 || p.Index >= len(cards) || p.Name == "" && p.Obj == 0 {
			return nil, nil, fmt.Errorf("invalid position constraint %+v", p)
		}
		card := proposalCard{}
		if p.Obj != 0 {
			var ok bool
			card, ok = byID[p.Obj]
			if !ok || usedObjects[p.Obj] || p.Name != "" && p.Name != card.Name {
				return nil, nil, fmt.Errorf("invalid position constraint %+v", p)
			}
			usedObjects[p.Obj] = true
		}
		if old, ok := c.fixedObj[p.Index]; ok && old.ID != card.ID || c.fixedName[p.Index] != "" && p.Name != "" && c.fixedName[p.Index] != p.Name {
			return nil, nil, fmt.Errorf("conflicting position constraint %+v", p)
		}
		if p.Obj != 0 {
			if name := c.fixedName[p.Index]; name != "" && name != card.Name {
				return nil, nil, fmt.Errorf("conflicting position constraint %+v", p)
			}
			c.fixedObj[p.Index] = card
			c.fixedName[p.Index] = card.Name
		} else {
			if old, ok := c.fixedObj[p.Index]; ok && old.Name != p.Name {
				return nil, nil, fmt.Errorf("conflicting position constraint %+v", p)
			}
			c.fixedName[p.Index] = p.Name
		}
		relevant[c.fixedName[p.Index]] = true
		if p.Index+1 > c.stop {
			c.stop = p.Index + 1
		}
	}
	for _, d := range deadlines {
		if d.Through < 0 || d.Through > len(cards) || d.Name == "" || d.Count < 0 {
			return nil, nil, fmt.Errorf("invalid deadline constraint %+v", d)
		}
		relevant[d.Name] = true
		if d.Through > c.stop {
			c.stop = d.Through
		}
	}
	for _, e := range exclusions {
		if e.Through < 0 || e.Through > len(cards) || e.Name == "" || e.Cap < 0 {
			return nil, nil, fmt.Errorf("invalid exclusion constraint %+v", e)
		}
		relevant[e.Name] = true
		if e.Through > c.stop {
			c.stop = e.Through
		}
	}
	c.names = make([]string, 0, len(relevant))
	for name := range relevant {
		c.names = append(c.names, name)
	}
	sort.Strings(c.names)
	c.nameIndex = make(map[string]int, len(c.names))
	for i, name := range c.names {
		c.nameIndex[name] = i
	}
	c.exactPrefix = make([][]int, len(cards)+1)
	for i := range c.exactPrefix {
		c.exactPrefix[i] = make([]int, len(c.names))
	}
	for pos := 0; pos < len(cards); pos++ {
		copy(c.exactPrefix[pos+1], c.exactPrefix[pos])
		if card, ok := c.fixedObj[pos]; ok {
			c.exactPrefix[pos+1][c.nameIndex[card.Name]]++
		}
	}
	available := make([]proposalCard, 0, len(cards)-len(c.fixedObj))
	c.initialFree = make([]int, len(c.names))
	for _, card := range cards {
		if usedObjects[card.ID] {
			continue
		}
		available = append(available, card)
		if i, ok := c.nameIndex[card.Name]; ok {
			c.initialFree[i]++
		}
	}
	c.deadlines = append([]deadlineConstraint(nil), deadlines...)
	c.exclusions = append([]exclusionConstraint(nil), exclusions...)
	return c, available, nil
}

func (c *constraintCounter) total(available []proposalCard) *big.Int {
	remaining := append([]int(nil), c.initialFree...)
	return c.count(0, remaining, len(available)-sumInts(remaining))
}

func (c *constraintCounter) count(pos int, remaining []int, other int) *big.Int {
	// Past every constraint's Through the remaining suffix is unconstrained:
	// a deadline is already satisfied or not, and an exclusion's window has
	// closed. Returning here (before the checks below) keeps an unranking
	// call at a position beyond stop from re-evaluating a closed window.
	if pos > c.stop {
		return c.factorials[c.n-pos]
	}
	if !c.constraintsHold(pos, remaining) {
		return new(big.Int)
	}
	if pos >= c.stop {
		return c.factorials[c.n-pos]
	}
	key := countKey(pos, remaining, other)
	if cached := c.memo[key]; cached != nil {
		return cached
	}
	total := new(big.Int)
	if _, ok := c.fixedObj[pos]; ok {
		total.Set(c.count(pos+1, remaining, other))
	} else if name := c.fixedName[pos]; name != "" {
		i := c.nameIndex[name]
		if remaining[i] > 0 {
			multiplicity := remaining[i]
			remaining[i]--
			total.Mul(c.count(pos+1, remaining, other), big.NewInt(int64(multiplicity)))
			remaining[i]++
		}
	} else {
		for i, multiplicity := range remaining {
			if multiplicity == 0 {
				continue
			}
			remaining[i]--
			branch := new(big.Int).Mul(c.count(pos+1, remaining, other), big.NewInt(int64(multiplicity)))
			remaining[i]++
			total.Add(total, branch)
		}
		if other > 0 {
			branch := new(big.Int).Mul(c.count(pos+1, remaining, other-1), big.NewInt(int64(other)))
			total.Add(total, branch)
		}
	}
	c.memo[key] = total
	return total
}

func (c *constraintCounter) constraintsHold(pos int, remaining []int) bool {
	for _, d := range c.deadlines {
		if pos < d.Through {
			continue
		}
		i := c.nameIndex[d.Name]
		placed := c.exactPrefix[pos][i] + c.initialFree[i] - remaining[i]
		if placed < d.Count {
			return false
		}
	}
	for _, e := range c.exclusions {
		// An exclusion is anti-monotone: "at most Cap copies before Through"
		// can only be judged at exactly Through, because at a later position
		// the placed count also includes cards placed after the window. The
		// DFS visits every position up to stop, so each exclusion is judged
		// once, at its own boundary.
		if pos != e.Through {
			continue
		}
		i := c.nameIndex[e.Name]
		placed := c.exactPrefix[pos][i] + c.initialFree[i] - remaining[i]
		if placed > e.Cap {
			return false
		}
	}
	return true
}

func countKey(pos int, remaining []int, other int) string {
	var b strings.Builder
	b.Grow(len(remaining) + 3)
	var encoded [binary.MaxVarintLen64]byte
	write := func(value int) {
		n := binary.PutUvarint(encoded[:], uint64(value))
		_, _ = b.Write(encoded[:n])
	}
	write(pos)
	write(len(remaining))
	for _, n := range remaining {
		write(n)
	}
	write(other)
	return b.String()
}

func randomBigBelow(r proposalRandom, limit *big.Int) *big.Int {
	if limit.Cmp(big.NewInt(1)) == 0 {
		return new(big.Int)
	}
	bits := limit.BitLen()
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
	for {
		out := new(big.Int)
		for generated := 0; generated < bits; generated += 64 {
			out.Lsh(out, 64)
			out.Or(out, new(big.Int).SetUint64(r.Uint64()))
		}
		out.And(out, mask)
		if out.Cmp(limit) < 0 {
			return out
		}
	}
}

func logBigInt(n *big.Int) float64 {
	f := new(big.Float).SetInt(n)
	mantissa := new(big.Float)
	exponent := f.MantExp(mantissa)
	m, _ := mantissa.Float64()
	return math.Log(m) + float64(exponent)*math.Ln2
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

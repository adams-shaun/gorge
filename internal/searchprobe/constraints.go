package searchprobe

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
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
	memo        map[string]*big.Int
}

func sampleConstrainedPermutation(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint, r proposalRandom) ([]state.ObjID, float64, bool, error) {
	counter, available, err := newConstraintCounter(cards, positions, deadlines)
	if err != nil {
		return nil, 0, false, err
	}
	total := counter.count(0, append([]int(nil), counter.initialFree...), len(available)-sumInts(counter.initialFree))
	if total.Sign() == 0 {
		return nil, 0, false, nil
	}
	if r == nil {
		return nil, 0, false, fmt.Errorf("missing permutation randomness")
	}
	rank := randomBigBelow(r, total)
	order := make([]state.ObjID, counter.n)
	remaining := append([]proposalCard(nil), available...)
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
			nextCounts := append([]int(nil), counts...)
			nextOther := other
			if j, ok := counter.nameIndex[card.Name]; ok {
				nextCounts[j]--
			} else {
				nextOther--
			}
			completions := counter.count(pos+1, nextCounts, nextOther)
			if rank.Cmp(completions) < 0 {
				chosen = i
				counts, other = nextCounts, nextOther
				break
			}
			rank.Sub(rank, completions)
		}
		if chosen < 0 {
			return nil, 0, false, fmt.Errorf("constraint unranking exhausted at position %d", pos)
		}
		order[pos] = remaining[chosen].ID
		remaining = append(remaining[:chosen], remaining[chosen+1:]...)
	}
	logFactorial, _ := math.Lgamma(float64(len(cards) + 1))
	return order, logBigInt(total) - logFactorial, true, nil
}

func newConstraintCounter(cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint) (*constraintCounter, []proposalCard, error) {
	c := &constraintCounter{n: len(cards), fixedObj: make(map[int]proposalCard), fixedName: make(map[int]string), memo: make(map[string]*big.Int)}
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
	return c, available, nil
}

func (c *constraintCounter) count(pos int, remaining []int, other int) *big.Int {
	if !c.deadlinesHold(pos, remaining) {
		return new(big.Int)
	}
	if pos >= c.stop {
		return factorial(c.n - pos)
	}
	key := countKey(pos, remaining, other)
	if cached := c.memo[key]; cached != nil {
		return new(big.Int).Set(cached)
	}
	total := new(big.Int)
	if _, ok := c.fixedObj[pos]; ok {
		total.Set(c.count(pos+1, remaining, other))
	} else if name := c.fixedName[pos]; name != "" {
		i := c.nameIndex[name]
		if remaining[i] > 0 {
			next := append([]int(nil), remaining...)
			multiplicity := next[i]
			next[i]--
			total.Mul(c.count(pos+1, next, other), big.NewInt(int64(multiplicity)))
		}
	} else {
		for i, multiplicity := range remaining {
			if multiplicity == 0 {
				continue
			}
			next := append([]int(nil), remaining...)
			next[i]--
			branch := new(big.Int).Mul(c.count(pos+1, next, other), big.NewInt(int64(multiplicity)))
			total.Add(total, branch)
		}
		if other > 0 {
			branch := new(big.Int).Mul(c.count(pos+1, remaining, other-1), big.NewInt(int64(other)))
			total.Add(total, branch)
		}
	}
	c.memo[key] = new(big.Int).Set(total)
	return total
}

func (c *constraintCounter) deadlinesHold(pos int, remaining []int) bool {
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
	return true
}

func countKey(pos int, remaining []int, other int) string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(pos))
	b.WriteByte('/')
	for _, n := range remaining {
		b.WriteString(strconv.Itoa(n))
		b.WriteByte(',')
	}
	b.WriteString(strconv.Itoa(other))
	return b.String()
}

func factorial(n int) *big.Int {
	out := big.NewInt(1)
	for i := 2; i <= n; i++ {
		out.Mul(out, big.NewInt(int64(i)))
	}
	return out
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

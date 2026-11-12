package kgen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// setlistDir is the registered set-list directory, relative to the module
// root (where `go run ./dirac/cmd/kgen` runs). The acceptance ratchet
// (ratchet_test.go) pins the same lists; -census-std ranks refusal classes
// by how many of their cards they block.
const setlistDir = "dirac/cardset/setlists"

// standardListPath names the Standard-20 set codes, one per line.
const standardListPath = "dirac/cardset/standard.txt"

// stdClassRow is one class in the Standard-carry ranking.
type stdClassRow struct {
	class       string
	tier        string
	cards       int // distinct Standard-list cards carrying the class
	sole        int // of those, cards whose only class this is
	sets        []stdSetCount
	corpusCards int // distinct ALL.txt cards carrying the class
}

// stdSetCount is one set's distinct card count for a class.
type stdSetCount struct {
	set   string
	cards int
}

// stdSetRow is one set's acceptance and greedy 90 % cover.
type stdSetRow struct {
	set      string
	std      bool
	listed   int
	accepted int
	need     int
	order    []greedyStep
	unlocks  int
}

// greedyStep is one class in a per-set completion order.
type greedyStep struct {
	class string
	cards int
}

// jointStep is one class in the joint 90 % order.
type jointStep struct {
	class     string
	close     int
	remaining int
}

// stdJoint is a joint 90 % order plus its gap and refined-label count.
type stdJoint struct {
	steps       []jointStep
	short       int
	totalLabels int
}

// stdTokenRow is one token-unpacked class and its Standard-card count.
type stdTokenRow struct {
	class string
	cards int
}

// stdUmbrellaRow is one umbrella value and its Standard-card count.
type stdUmbrellaRow struct {
	class string
	value string
	cards int
}

// stdBundleRow is one greedy disjoint-ish bundle (2-3 refined labels).
type stdBundleRow struct {
	cards   int
	covered int
	bundle  string
	sets    []stdSetCount
	first   int
}

// stdTSVRow is one line of the -tsv refusal dump.
type stdTSVRow struct {
	set    string
	card   string
	via    string
	node   string
	class  string
	reason string
	label  string
}

// stdCensusResult is the -census-std report.
type stdCensusResult struct {
	sets          []stdSetRow
	classes       []stdClassRow
	joint         stdJoint
	refined       stdJoint
	refinedTier   map[string]string
	tokenClasses  []stdTokenRow
	umbrellas     []stdUmbrellaRow
	bundles       []stdBundleRow
	tsv           []stdTSVRow
	stdTotalShort int
	allAccepted   int
	allListed     int
}

// censusAllBlockers (-census-std -all-blockers) lowers each list in the
// every-blocker mode: past a refusal to the next node of the chain, the next
// ability, trigger, static or replacement, recording each refusal. Rows of
// class partialClass name a refused node's params left unread.
var censusAllBlockers bool

const partialClass = "partial:unread-after-refusal"

// stdList is one registered set list (the ALL list is kept separately).
type stdList struct {
	set   string
	names []string
}

// listResult is one set list lowered alone, with its refusals unpacked.
type listResult struct {
	list         stdList
	accepted     int
	rows         []stdTSVRow
	byCard       map[string][]stdTSVRow
	expanded     map[string][]string
	refined      map[string][]string
	tokenClasses map[string][]string
	cardOrder    []string
}

// computeStdCensus lowers every set list alone, exactly as the acceptance
// ratchet (ratchet_test.go) lowers it. The Standard ranking uses only the
// sets named by `std`; an ALL list is used only for `corpus cards`. Token
// refusals are unpacked before a class or value is counted.
func computeStdCensus(dir, cardsDir string, std []string) (*stdCensusResult, error) {
	return computeStdCensusAll(dir, "", cardsDir, std)
}

// computeSetlistCensus is the census of a registered set-list directory: its
// ALL list is the corpus's own (setlistall.go), which is not a tracked file.
func computeSetlistCensus(dir, cardsDir string, std []string) (*stdCensusResult, error) {
	all, err := corpusAllList(cardsDir)
	if err != nil {
		return nil, err
	}
	return computeStdCensusAll(dir, all, cardsDir, std)
}

// computeStdCensusAll is computeStdCensus with the ALL list read from allPath;
// allPath "" reads only what dir holds.
func computeStdCensusAll(dir, allPath, cardsDir string, std []string) (*stdCensusResult, error) {
	stdSet := map[string]bool{}
	var stdOrder []string
	for _, s := range std {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, s+".txt")); err != nil {
			return nil, fmt.Errorf("unknown set %q: no %s.txt", s, s)
		}
		stdSet[s] = true
		stdOrder = append(stdOrder, s)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var lists []stdList
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		set := strings.TrimSuffix(e.Name(), ".txt")
		names, err := readCardList(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		lists = append(lists, stdList{set: set, names: names})
	}
	if allPath != "" {
		names, err := readCardList(allPath)
		if err != nil {
			return nil, err
		}
		lists = append(lists, stdList{set: "ALL", names: names})
	}
	if len(lists) == 0 {
		return nil, fmt.Errorf("no set lists in %s", dir)
	}
	sort.Slice(lists, func(i, j int) bool { return lists[i].set < lists[j].set })

	res := &stdCensusResult{refinedTier: map[string]string{}}
	stdCards := map[string]map[string]bool{}
	soleCards := map[string]map[string]bool{}
	perSet := map[string]map[string]map[string]bool{}
	corpus := map[string]map[string]bool{}
	umbrella := map[string]map[string]map[string]bool{}
	tokenClasses := map[string]map[string]bool{}
	refinedCardLabels := map[string]map[string]bool{}
	refinedCardSets := map[string]map[string]bool{}
	refinedLabels := map[string]bool{}
	refinedTierClass := map[string]string{}
	stdResults := map[string]*listResult{}
	stdNeed := map[string]int{}

	for _, l := range lists {
		lr, err := lowerStdList(l, cardsDir)
		if err != nil {
			return nil, err
		}
		res.tsv = append(res.tsv, lr.rows...)
		if l.set == "ALL" {
			res.allAccepted = lr.accepted
			res.allListed = len(l.names)
			for card, cs := range lr.expanded {
				for _, cl := range cs {
					if corpus[cl] == nil {
						corpus[cl] = map[string]bool{}
					}
					corpus[cl][card] = true
				}
			}
			continue
		}
		isStd := stdSet[l.set]
		row := stdSetRow{
			set:      l.set,
			std:      isStd,
			listed:   len(l.names),
			accepted: lr.accepted,
			need:     shortOf90(len(l.names), lr.accepted),
		}
		row.order, row.unlocks = completionGreedy(lr.expanded, row.need)
		res.sets = append(res.sets, row)
		if !isStd {
			continue
		}
		stdResults[l.set] = lr
		stdNeed[l.set] = row.need
		res.stdTotalShort += row.need
		for card, cs := range lr.expanded {
			if len(cs) == 0 {
				continue
			}
			if refinedCardSets[card] == nil {
				refinedCardSets[card] = map[string]bool{}
			}
			refinedCardSets[card][l.set] = true
			for _, cl := range cs {
				if stdCards[cl] == nil {
					stdCards[cl] = map[string]bool{}
					perSet[cl] = map[string]map[string]bool{}
				}
				stdCards[cl][card] = true
				if perSet[cl][l.set] == nil {
					perSet[cl][l.set] = map[string]bool{}
				}
				perSet[cl][l.set][card] = true
			}
			if len(cs) == 1 {
				if soleCards[cs[0]] == nil {
					soleCards[cs[0]] = map[string]bool{}
				}
				soleCards[cs[0]][card] = true
			}
			for _, lab := range lr.refined[card] {
				refinedLabels[lab] = true
				if refinedCardLabels[card] == nil {
					refinedCardLabels[card] = map[string]bool{}
				}
				refinedCardLabels[card][lab] = true
			}
			for _, r := range lr.byCard[card] {
				if old, ok := refinedTierClass[r.label]; !ok || r.class < old {
					refinedTierClass[r.label] = r.class
				}
				if strings.HasPrefix(r.class, "other:") && strings.Contains(r.class, `"_"`) {
					if umbrella[r.class] == nil {
						umbrella[r.class] = map[string]map[string]bool{}
					}
					if umbrella[r.class][r.label] == nil {
						umbrella[r.class][r.label] = map[string]bool{}
					}
					umbrella[r.class][r.label][card] = true
				}
			}
			for _, cl := range lr.tokenClasses[card] {
				if tokenClasses[cl] == nil {
					tokenClasses[cl] = map[string]bool{}
				}
				tokenClasses[cl][card] = true
			}
		}
	}

	sort.Slice(res.sets, func(i, j int) bool { return res.sets[i].set < res.sets[j].set })

	for cl, cards := range stdCards {
		row := stdClassRow{
			class:       cl,
			tier:        stdTier(cl),
			cards:       len(cards),
			sole:        len(soleCards[cl]),
			corpusCards: len(corpus[cl]),
		}
		for set, cs := range perSet[cl] {
			row.sets = append(row.sets, stdSetCount{set: set, cards: len(cs)})
		}
		sort.Slice(row.sets, func(i, j int) bool {
			if row.sets[i].cards != row.sets[j].cards {
				return row.sets[i].cards > row.sets[j].cards
			}
			return row.sets[i].set < row.sets[j].set
		})
		res.classes = append(res.classes, row)
	}
	sort.Slice(res.classes, func(i, j int) bool {
		a, b := res.classes[i], res.classes[j]
		if as, bs := a.cards+a.sole, b.cards+b.sole; as != bs {
			return as > bs
		}
		if a.corpusCards != b.corpusCards {
			return a.corpusCards > b.corpusCards
		}
		return a.class < b.class
	})

	for cl, cards := range tokenClasses {
		res.tokenClasses = append(res.tokenClasses, stdTokenRow{class: cl, cards: len(cards)})
	}
	sort.Slice(res.tokenClasses, func(i, j int) bool {
		if res.tokenClasses[i].cards != res.tokenClasses[j].cards {
			return res.tokenClasses[i].cards > res.tokenClasses[j].cards
		}
		return res.tokenClasses[i].class < res.tokenClasses[j].class
	})

	for cl, vals := range umbrella {
		for val, cards := range vals {
			res.umbrellas = append(res.umbrellas, stdUmbrellaRow{class: cl, value: val, cards: len(cards)})
		}
	}
	sort.Slice(res.umbrellas, func(i, j int) bool {
		a, b := res.umbrellas[i], res.umbrellas[j]
		if a.class != b.class {
			return a.class < b.class
		}
		if a.cards != b.cards {
			return a.cards > b.cards
		}
		return a.value < b.value
	})

	for lab, cl := range refinedTierClass {
		res.refinedTier[lab] = stdTier(cl)
	}
	stdCodes := make([]string, 0, len(stdNeed))
	for code := range stdNeed {
		stdCodes = append(stdCodes, code)
	}
	sort.Strings(stdCodes)
	ref := map[string]map[string][]string{}
	refRefined := map[string]map[string][]string{}
	for set, lr := range stdResults {
		ref[set] = lr.expanded
		refRefined[set] = lr.refined
	}
	res.joint = makeStdJoint(stdCodes, ref, stdNeed)
	res.refined = makeStdJoint(stdCodes, refRefined, stdNeed)
	res.refined.totalLabels = len(refinedLabels)

	orders := make([][]string, 0, len(stdOrder))
	for _, set := range stdOrder {
		if lr := stdResults[set]; lr != nil {
			orders = append(orders, lr.cardOrder)
		}
	}
	cards := dedupeBundleCards(orders, refinedCardLabels, refinedCardSets)
	res.bundles = computeBundles(cards)
	return res, nil
}

// lowerStdList lowers one set list alone and unpacks its token refusals.
func lowerStdList(l stdList, cardsDir string) (*listResult, error) {
	p, err := loadCardNames(append([]string(nil), l.names...), cardsDir)
	if err != nil {
		return nil, err
	}
	p.allBlockers = censusAllBlockers
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		return nil, err
	}
	tokens := make(map[string]*lCard, len(lw.tokens))
	for _, t := range lw.tokens {
		tokens[t.Name] = t
	}
	lr := &listResult{
		list:         l,
		byCard:       map[string][]stdTSVRow{},
		expanded:     map[string][]string{},
		refined:      map[string][]string{},
		tokenClasses: map[string][]string{},
	}
	for _, c := range lw.cards {
		lr.cardOrder = append(lr.cardOrder, c.Name)
		if c.loweredOK() {
			lr.accepted++
			lr.rows = append(lr.rows, stdTSVRow{set: l.set, card: c.Name})
			continue
		}
		rows := cardTSVRows(l.set, c, tokens)
		lr.rows = append(lr.rows, rows...)
		for _, u := range c.Unreadable {
			// beside the card's rows, not among them: not a blocker class
			lr.rows = append(lr.rows, stdTSVRow{set: l.set, card: c.Name, node: u.Node, class: partialClass, reason: u.Reason})
		}
		lr.byCard[c.Name] = rows
		lr.expanded[c.Name] = classesFromRows(rows)
		lr.refined[c.Name] = labelsFromRows(rows)
		own := map[string]bool{}
		tok := map[string]bool{}
		for _, r := range rows {
			if r.via == "" {
				if r.class != "other:token refused" {
					own[r.class] = true
				}
				continue
			}
			if r.class != "other:token refused" {
				tok[r.class] = true
			}
		}
		var introduced []string
		for cl := range tok {
			if !own[cl] {
				introduced = append(introduced, cl)
			}
		}
		if len(introduced) > 0 {
			sort.Strings(introduced)
			lr.tokenClasses[c.Name] = introduced
		}
	}
	return lr, nil
}

// cardTSVRows emits one row per refusal class, plus the refused token's own
// refusals as extra rows with via set to the token's name.
func cardTSVRows(set string, c *lCard, tokens map[string]*lCard) []stdTSVRow {
	var rows []stdTSVRow
	for _, r := range c.Refusals {
		for _, cl := range refusalClasses(r.Reason) {
			rows = append(rows, stdTSVRow{
				set:    set,
				card:   c.Name,
				node:   r.Node,
				class:  cl,
				reason: r.Reason,
				label:  refineLabel(cl, r.Reason),
			})
		}
		if r.Reason != "token refused" {
			continue
		}
		name := strings.TrimPrefix(r.Node, "token ")
		tok := tokens[name]
		if tok == nil {
			continue
		}
		for _, tr := range tok.Refusals {
			for _, cl := range refusalClasses(tr.Reason) {
				rows = append(rows, stdTSVRow{
					set:    set,
					card:   c.Name,
					via:    name,
					node:   tr.Node,
					class:  cl,
					reason: tr.Reason,
					label:  refineLabel(cl, tr.Reason),
				})
			}
		}
	}
	return rows
}

// classesFromRows expands a card's token rows and drops the wrapper when
// the token's own classes are known.
func classesFromRows(rows []stdTSVRow) []string {
	own := map[string]bool{}
	tok := map[string]bool{}
	for _, r := range rows {
		if r.via != "" {
			if r.class != "other:token refused" {
				tok[r.class] = true
			}
			continue
		}
		own[r.class] = true
	}
	out := map[string]bool{}
	for c := range own {
		if c != "other:token refused" {
			out[c] = true
		}
	}
	for c := range tok {
		out[c] = true
	}
	if own["other:token refused"] && len(tok) == 0 {
		out["other:token refused"] = true
	}
	return sortedNames(out)
}

// labelsFromRows is classesFromRows over refined labels.
func labelsFromRows(rows []stdTSVRow) []string {
	own := map[string]bool{}
	tok := map[string]bool{}
	hastok := false
	for _, r := range rows {
		if r.via != "" {
			tok[r.label] = true
			continue
		}
		if r.class == "other:token refused" {
			hastok = true
			continue
		}
		own[r.label] = true
	}
	out := map[string]bool{}
	for l := range own {
		out[l] = true
	}
	for l := range tok {
		out[l] = true
	}
	if hastok && len(tok) == 0 {
		out["other:token refused"] = true
	}
	return sortedNames(out)
}

// sortedNames returns a map's keys sorted, so every lowering pool is built
// in the same order regardless of map iteration.
func sortedNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// shortOf90 is how many more accepted cards a set needs to reach 90 %.
func shortOf90(listed, accepted int) int {
	if listed == 0 {
		return 0
	}
	short := (9*listed+9)/10 - accepted
	if short < 0 {
		return 0
	}
	return short
}

// completionGreedy covers a set's refused cards with the fewest classes:
// always take the class that is the sole class of the most cards, then the
// largest exact fractional coverage, then the shortest name, then the
// lexicographically largest name.
func completionGreedy(ref map[string][]string, need int) ([]greedyStep, int) {
	if need <= 0 {
		return nil, 0
	}
	cards := make(map[string]map[string]bool, len(ref))
	maxN := 0
	for card, cs := range ref {
		m := map[string]bool{}
		for _, c := range cs {
			m[c] = true
		}
		if len(m) == 0 {
			continue
		}
		cards[card] = m
		if len(m) > maxN {
			maxN = len(m)
		}
	}
	L := lcmUpTo(maxN)
	got := 0
	var order []greedyStep
	for got < need && len(cards) > 0 {
		imm := map[string]int{}
		score := map[string]int64{}
		for _, cs := range cards {
			if len(cs) == 1 {
				for c := range cs {
					imm[c]++
				}
			}
			inc := L / int64(len(cs))
			for c := range cs {
				score[c] += inc
			}
		}
		best := ""
		for c := range score {
			if best == "" || betterCompletion(c, best, imm, score) {
				best = c
			}
		}
		if best == "" {
			break
		}
		order = append(order, greedyStep{class: best, cards: imm[best]})
		for card, cs := range cards {
			delete(cs, best)
			if len(cs) == 0 {
				delete(cards, card)
				got++
			}
		}
	}
	return order, got
}

func betterCompletion(a, b string, imm map[string]int, score map[string]int64) bool {
	if imm[a] != imm[b] {
		return imm[a] > imm[b]
	}
	if score[a] != score[b] {
		return score[a] > score[b]
	}
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a > b
}

// jointGreedy closes every Standard set's remaining gap with the fewest
// classes: always take the class that is the sole class of the most
// still-uncovered cards across all Standard sets, then the largest exact
// fractional coverage, then the lexicographically largest name.
func jointGreedy(sets []string, ref map[string]map[string][]string, need map[string]int) []jointStep {
	rem := make(map[string]int, len(sets))
	total := 0
	for _, s := range sets {
		rem[s] = need[s]
		total += need[s]
	}
	cards := make(map[string]map[string]map[string]bool, len(sets))
	maxN := 0
	for _, s := range sets {
		cards[s] = map[string]map[string]bool{}
		for card, cs := range ref[s] {
			m := map[string]bool{}
			for _, c := range cs {
				m[c] = true
			}
			if len(m) == 0 {
				continue
			}
			cards[s][card] = m
			if len(m) > maxN {
				maxN = len(m)
			}
		}
	}
	L := lcmUpTo(maxN)
	var steps []jointStep
	for total > 0 {
		imm := map[string]int{}
		score := map[string]int64{}
		for _, s := range sets {
			if rem[s] <= 0 {
				continue
			}
			for _, cs := range cards[s] {
				if len(cs) == 1 {
					for c := range cs {
						imm[c]++
					}
				}
				inc := L / int64(len(cs))
				for c := range cs {
					score[c] += inc
				}
			}
		}
		best := ""
		for c := range score {
			if best == "" || betterJoint(c, best, imm, score) {
				best = c
			}
		}
		if best == "" {
			break
		}
		gain := 0
		for _, s := range sets {
			for card, cs := range cards[s] {
				delete(cs, best)
				if len(cs) == 0 {
					delete(cards[s], card)
					if rem[s] > 0 {
						rem[s]--
						gain++
					}
				}
			}
		}
		total = 0
		for _, s := range sets {
			total += rem[s]
		}
		steps = append(steps, jointStep{class: best, close: gain, remaining: total})
	}
	return steps
}

func betterJoint(a, b string, imm map[string]int, score map[string]int64) bool {
	if imm[a] != imm[b] {
		return imm[a] > imm[b]
	}
	if score[a] != score[b] {
		return score[a] > score[b]
	}
	return a > b
}

func makeStdJoint(sets []string, ref map[string]map[string][]string, need map[string]int) stdJoint {
	j := stdJoint{}
	for _, s := range sets {
		j.short += need[s]
	}
	j.steps = jointGreedy(sets, ref, need)
	return j
}

func lcmUpTo(n int) int64 {
	if n < 1 {
		n = 1
	}
	l := int64(1)
	for i := int64(2); i <= int64(n); i++ {
		l = l / gcd64(l, i) * i
	}
	return l
}

func gcd64(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// stdCardLabels is one Standard card's refined labels and sets, for bundles.
type stdCardLabels struct {
	labels []string
	sets   []string
}

// dedupeBundleCards flattens per-set card orders into the Standard union: a
// card refused in several sets is one card, listed at its first sighting,
// and a card with no refined labels is skipped.
func dedupeBundleCards(orders [][]string, labelsOf, setsOf map[string]map[string]bool) []stdCardLabels {
	var cards []stdCardLabels
	seen := map[string]bool{}
	for _, order := range orders {
		for _, card := range order {
			labels := labelsOf[card]
			if len(labels) == 0 || seen[card] {
				continue
			}
			seen[card] = true
			cards = append(cards, stdCardLabels{labels: sortedNames(labels), sets: sortedNames(setsOf[card])})
		}
	}
	return cards
}

// computeBundles finds 2-3 label bundles carried by at least three cards,
// ranked by exact bundle size then by how many cards they cover.
func computeBundles(cards []stdCardLabels) []stdBundleRow {
	type bundle struct {
		labels []string
		cards  int
		first  int
	}
	byKey := map[string]*bundle{}
	for _, c := range cards {
		labels := uniqueStrings(c.labels)
		if len(labels) < 2 || len(labels) > 3 {
			continue
		}
		key := strings.Join(labels, "\x00")
		if byKey[key] == nil {
			byKey[key] = &bundle{labels: labels, first: len(byKey)}
		}
		byKey[key].cards++
	}
	var rows []stdBundleRow
	for _, b := range byKey {
		if b.cards < 3 {
			continue
		}
		covered := 0
		setCards := map[string]int{}
		for _, c := range cards {
			if !subsetOf(c.labels, b.labels) {
				continue
			}
			covered++
			for _, s := range c.sets {
				setCards[s]++
			}
		}
		var sets []stdSetCount
		for s, n := range setCards {
			sets = append(sets, stdSetCount{set: s, cards: n})
		}
		sort.Slice(sets, func(i, j int) bool {
			if sets[i].cards != sets[j].cards {
				return sets[i].cards > sets[j].cards
			}
			return sets[i].set < sets[j].set
		})
		rows = append(rows, stdBundleRow{
			cards:   b.cards,
			covered: covered,
			bundle:  strings.Join(b.labels, " + "),
			sets:    sets,
			first:   b.first,
		})
	}
	// Ties keep first-seen order, which is the reference's Counter order and
	// the order the Goal's bundle rows depend on; bundle-name order would move
	// equal 11/11 and 10/10 rows.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].cards != rows[j].cards {
			return rows[i].cards > rows[j].cards
		}
		if rows[i].covered != rows[j].covered {
			return rows[i].covered > rows[j].covered
		}
		return rows[i].first < rows[j].first
	})
	return rows
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	for _, s := range in {
		seen[s] = true
	}
	return sortedNames(seen)
}

func subsetOf(a, b []string) bool {
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[s] = true
	}
	for _, s := range a {
		if !set[s] {
			return false
		}
	}
	return true
}

var (
	reRefineStrip   = regexp.MustCompile(`^((DB|SP|AB)\$ \w+: )+`)
	reRefineDefined = regexp.MustCompile(`^Defined\$ "(.*)"$`)
	reRefineFilter  = regexp.MustCompile(`^filter "(.*)": unknown filter head "(.*)"$`)
	reRefineZone    = regexp.MustCompile(`^AffectedZone "(\w+)" without MayPlay`)
)

// refineLabel turns an umbrella refusal into its concrete value.
func refineLabel(class, reason string) string {
	if !strings.HasPrefix(class, "other:") || !strings.Contains(class, `"_"`) {
		return class
	}
	r := reRefineStrip.ReplaceAllString(reason, "")
	if m := reRefineDefined.FindStringSubmatch(r); m != nil {
		v := m[1]
		if strings.HasPrefix(v, "Valid ") {
			return `Defined$ "Valid <filter>"`
		}
		if strings.HasPrefix(v, "TriggeredCardLKICopy") {
			return `Defined$ "TriggeredCardLKICopy<...>"`
		}
		return `Defined$ "` + v + `"`
	}
	if m := reRefineFilter.FindStringSubmatch(r); m != nil {
		h := m[2]
		if strings.Contains(h, ";") {
			h = "<A;B type list>"
		}
		if strings.HasPrefix(h, "EACH") {
			h = "EACH <...>"
		}
		return `filter head "` + h + `"`
	}
	if m := reRefineZone.FindStringSubmatch(r); m != nil {
		return `AffectedZone "` + m[1] + `" without MayPlay`
	}
	r = reDigits.ReplaceAllString(r, "N")
	if rs := []rune(r); len(rs) > 110 {
		r = string(rs[:110])
	}
	return r
}

// stdSetCodes reads the Standard-20 list, or splits a -sets flag value.
func stdSetCodes(flagValue string) ([]string, error) {
	if strings.TrimSpace(flagValue) == "" {
		return readCardList(standardListPath)
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range strings.Split(flagValue, ",") {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-sets names no set codes")
	}
	return out, nil
}

// runStdCensus prints the eight -census-std tables and, if asked, writes the
// per-refusal TSV dump.
func runStdCensus(dir, cardsDir string, std []string, tsvPath string, w io.Writer) error {
	res, err := computeSetlistCensus(dir, cardsDir, std)
	if err != nil {
		return err
	}
	writeStdCensus(w, res)
	if tsvPath != "" {
		f, err := os.Create(tsvPath)
		if err != nil {
			return err
		}
		if err := writeStdTSV(f, res); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	return nil
}

func writeStdCensus(w io.Writer, res *stdCensusResult) {
	fmt.Fprintln(w, "## Per set (lower+build, the ratchet basis)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| set | std | accepted | listed | % | short of 90 % |")
	fmt.Fprintln(w, "|---|---|---:|---:|---:|---:|")
	for _, s := range res.sets {
		pct := 0.0
		if s.listed > 0 {
			pct = 100 * float64(s.accepted) / float64(s.listed)
		}
		fmt.Fprintf(w, "| %s | %s | %d | %d | %.1f | %d |\n", s.set, stdLabel(s.std), s.accepted, s.listed, pct, s.need)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Std total short of 90 %%: %d\n", res.stdTotalShort)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Class ranking (Standard sets only, tokens unpacked)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| rank | class | tier | std cards | std sole | sets (cards) | corpus cards |")
	fmt.Fprintln(w, "|---:|---|---|---:|---:|---|---:|")
	for i, r := range res.classes {
		fmt.Fprintf(w, "| %d | %s | %s | %d | %d | %s | %d |\n",
			i+1, mdEscape(r.class), r.tier, r.cards, r.sole, stdSetCounts(r.sets), r.corpusCards)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Per set: classes to 90 % accepted (card completion)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| set | need | classes | unlocks | order | std |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---|---|")
	for _, s := range res.sets {
		order := "-"
		if len(s.order) > 0 {
			parts := make([]string, len(s.order))
			for i, st := range s.order {
				parts[i] = fmt.Sprintf("%s (%d)", st.class, st.cards)
			}
			order = mdEscape(strings.Join(parts, ", "))
		}
		fmt.Fprintf(w, "| %s | %d | %d | %d | %s | %s |\n", s.set, s.need, len(s.order), s.unlocks, order, stdLabel(s.std))
	}
	fmt.Fprintln(w)

	writeJointSection(w, "## Joint order (every Standard set to 90 %)", "classes", res.joint, stdTier)

	fmt.Fprintln(w, "## Token refusals unpacked")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| token class | std cards |")
	fmt.Fprintln(w, "|---|---:|")
	for _, r := range res.tokenClasses {
		fmt.Fprintf(w, "| %s | %d |\n", mdEscape(r.class), r.cards)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Umbrella values")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| class | value | std cards |")
	fmt.Fprintln(w, "|---|---|---:|")
	for _, r := range res.umbrellas {
		fmt.Fprintf(w, "| %s | %s | %d |\n", mdEscape(r.class), mdEscape(r.value), r.cards)
	}
	fmt.Fprintln(w)

	writeJointSection(w, "## Refined joint order (class + value)", "refined classes", res.refined, func(s string) string {
		if t := res.refinedTier[s]; t != "" {
			return t
		}
		return stdTier(s)
	})

	fmt.Fprintln(w, "## Bundles")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| cards | covered | bundle | sets |")
	fmt.Fprintln(w, "|---:|---:|---|---|")
	for _, b := range res.bundles {
		fmt.Fprintf(w, "| %d | %d | %s | %s |\n", b.cards, b.covered, mdEscape(b.bundle), stdSetCounts(b.sets))
	}
}

func writeJointSection(w io.Writer, title, noun string, j stdJoint, tierOf func(string) string) {
	fmt.Fprintln(w, title)
	fmt.Fprintln(w)
	if j.totalLabels > 0 {
		fmt.Fprintf(w, "%d %s close the Standard sets' 90 %% gap (short %d; %d refined labels)\n", len(j.steps), noun, j.short, j.totalLabels)
	} else {
		fmt.Fprintf(w, "%d %s close the Standard sets' 90 %% gap (short %d)\n", len(j.steps), noun, j.short)
	}
	for _, k := range []int{25, 50, 75, 100, 150, 200, 250, 300} {
		if k <= len(j.steps) {
			fmt.Fprintf(w, "after %d: remaining %d\n", k, j.steps[k-1].remaining)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| rank | class | tier | +close | remaining |")
	fmt.Fprintln(w, "|---:|---|---|---:|---:|")
	for i, st := range j.steps {
		fmt.Fprintf(w, "| %d | %s | %s | +%d | %d |\n", i+1, mdEscape(st.class), tierOf(st.class), st.close, st.remaining)
	}
	fmt.Fprintln(w)
}

func writeStdTSV(w io.Writer, res *stdCensusResult) error {
	for _, r := range res.tsv {
		if r.class == "" {
			if _, err := fmt.Fprintf(w, "%s\t%s\t\t\t\t\t\n", tsvField(r.set), tsvField(r.card)); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			tsvField(r.set), tsvField(r.card), tsvField(r.via), tsvField(r.node),
			tsvField(r.class), tsvField(r.reason), tsvField(r.label)); err != nil {
			return err
		}
	}
	return nil
}

func tsvField(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}

func stdLabel(std bool) string {
	if std {
		return "std"
	}
	return "non-std"
}

func stdSetCounts(cs []stdSetCount) string {
	if len(cs) == 0 {
		return "-"
	}
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = fmt.Sprintf("%s %d", c.set, c.cards)
	}
	return strings.Join(parts, ", ")
}

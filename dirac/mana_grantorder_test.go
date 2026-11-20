package dirac

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The grant walk's order equivalence: grantIdxs and contGrants read their
// source order off grantOrder (a (seat, timestamp) key over the grant-capable
// statics) instead of listenerOrder's sort of every static slot on each query
// (sweep 2, mana_grantedoffers.go). These are the pre-change bodies verbatim
// (2be5569b72's, listenerOrder over statM), run beside the real ones on real
// pool boards: TestGrantWalkOrderEquivalence plays the pools with the random
// policy and, at every decision, compares both index lists and both callback
// sequences at every occupied slot. The references stall nowhere (the real
// path's capacity stall is replaced by the same skip), and the comparison
// restores over and stalled, so the probes leave the played game as it was.

// contGrantsRef is contGrants before grantOrder: listenerOrder's walk over
// statM, the same filters and the same callback order.
func (s *State) contGrantsRef(k *prep, sl Slot, lost bool, lts uint16, f func(c CardID, ab uint8, n int)) {
	var order [MaxPerm]Slot
	n := s.listenerOrder(s.statM, &order)
	for oi := 0; oi < n; oi++ {
		src := order[oi]
		cid := s.cardAt(src)
		if !k.gen.has(cid, gfContGrant) || s.perms[src].attached == sl || s.genLost(k, src) {
			continue
		}
		if lost && s.layerTs(src) < lts {
			continue
		}
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant == 0 || cd.Abilities[a.Static.Grant-1].Kind != kop.AbMana {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: s.perms[src].ctrl, obj: s.perms[src].obj}
			if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, src, layerKw) {
				continue
			}
			prev := s.g.grantSrc
			s.g.grantSrc = st.obj + 1
			f(cid, a.Static.Grant-1, 1)
			s.g.grantSrc = prev
		}
	}
}

// grantIdxsRef is grantIdxs before grantOrder: the place scan reads its pos
// off listenerOrder's full statM walk. Its one stall (the maxGrants capacity)
// is the same skip without the side effect, so a board at the capacity
// compares the same list the real path returns.
func (s *State) grantIdxsRef(k *prep, sl Slot, out *[maxGrants]int16) (n int) {
	g := k.gen
	var order [MaxPerm]Slot
	no := 0
	if g.anyContGrant {
		no = s.listenerOrder(s.statM, &order)
	}
	var key [maxGrants]uint8
	s.grantedAbilities(k, sl, func(c CardID, ab uint8) {
		pos := uint8(255)
		for oi := 0; oi < no; oi++ {
			if s.g.grantSrc != 0 && s.perms[order[oi]].obj+1 == s.g.grantSrc {
				pos = uint8(oi)
				break
			}
		}
		for i, r := range g.grantOf {
			if r.card != c || r.ab != ab {
				continue
			}
			if n == maxGrants {
				continue // the real path stalls here: the list is the same skip
			}
			if g.grantSelf[i] && s.selfColMask(sl) == 0 {
				continue // a colourless grantee reflects nothing: gorge lists no such ability
			}
			j := n
			for j > 0 && key[j-1] > pos {
				out[j], key[j] = out[j-1], key[j-1]
				j--
			}
			out[j], key[j] = int16(i), pos
			n++
		}
	})
	if g.uses.has(useGainedMana) {
		n = s.gainedIdxs(k, sl, out, n)
	}
	return n
}

// grantOrderRef is grantOrder's reference construction: listenerOrder's full
// statM walk, restricted in place to the grant-capable slots.
func (s *State) grantOrderRef(k *prep, ord *[MaxPerm]Slot) int {
	var order [MaxPerm]Slot
	n := s.listenerOrder(s.statM, &order)
	m := 0
	for i := 0; i < n; i++ {
		if k.cards[s.cardAt(order[i])].grantStat {
			ord[m] = order[i]
			m++
		}
	}
	return m
}

// compareGrantOrder compares grantOrder's list with its reference at the
// decision's state and reports whether the order matters here (two entries).
func compareGrantOrder(t *testing.T, k *prep, s *State, where string) (multi bool) {
	t.Helper()
	var a, b [MaxPerm]Slot
	na := s.grantOrder(k, &a)
	nb := s.grantOrderRef(k, &b)
	if na != nb {
		t.Fatalf("%s: grantOrder %d slots, reference %d", where, na, nb)
	}
	for i := 0; i < na; i++ {
		if a[i] != b[i] {
			t.Fatalf("%s: grantOrder[%d] = slot %d, reference %d", where, i, a[i], b[i])
		}
	}
	return na >= 2
}

type grantCbk struct {
	c CardID
	a uint8
	n int
}

func collectGrants(dst *[]grantCbk) func(c CardID, ab uint8, n int) {
	return func(c CardID, ab uint8, n int) { *dst = append(*dst, grantCbk{c, ab, n}) }
}

// compareGrantsAt runs the real grantIdxs and contGrants beside their
// references at sl and fails on the first difference. It restores over and
// stalled, so the probes' capacity stalls (none on these boards) leave the
// played game as it was. It reports whether the slot held a grant, so a pool
// run that never reached one cannot pass vacuously, and whether the order
// the walk reads matters at this state (two grant-capable statics in play,
// or two granted abilities on the slot): a comparison that only ever saw one
// cannot tell an order from its reverse.
func compareGrantsAt(t *testing.T, k *prep, s *State, sl Slot, where string) (hit, multi bool) {
	t.Helper()
	over, stalled := s.over, s.g.stalled
	defer func() { s.over, s.g.stalled = over, stalled }()
	var a, b [maxGrants]int16
	na := s.grantIdxs(k, sl, &a)
	nb := s.grantIdxsRef(k, sl, &b)
	multi = na >= 2
	if na != nb {
		t.Fatalf("%s: grantIdxs count %d != reference %d", where, na, nb)
	}
	for i := 0; i < na; i++ {
		if a[i] != b[i] {
			t.Fatalf("%s: grantIdxs[%d] = %d, reference %d", where, i, a[i], b[i])
		}
	}
	lts, lost := s.lossAt(k, sl)
	var ca, cb []grantCbk
	s.contGrants(k, sl, lost, lts, collectGrants(&ca))
	s.contGrantsRef(k, sl, lost, lts, collectGrants(&cb))
	if len(ca) != len(cb) {
		t.Fatalf("%s: contGrants %d callbacks, reference %d", where, len(ca), len(cb))
	}
	for i := range ca {
		if ca[i] != cb[i] {
			t.Fatalf("%s: contGrants[%d] = %+v, reference %+v", where, i, ca[i], cb[i])
		}
	}
	return na > 0 || len(ca) > 0, multi
}

// grantSweepDecks is the measurement harness's deck source: the committed
// slice when the pool has one, else the harness's random two-colour decks
// (seed 7), so the boards are the profile's.
func grantSweepDecks(t *testing.T, name string, set cardset.Set, seats int) [][]cardset.Entry {
	t.Helper()
	if es := grantSliceDecks(t, name); len(es) >= seats {
		return es
	}
	return grantRandomDecks(set, 64, 7)
}

// grantSliceDecks reads dirac/oracle/testdata/slices/<name>/decks.tar.gz,
// the measurement harness's swSliceDecks.
func grantSliceDecks(t *testing.T, name string) [][]cardset.Entry {
	t.Helper()
	f, err := os.Open("oracle/testdata/slices/" + name + "/decks.tar.gz")
	if err != nil {
		return nil
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil
	}
	type deck struct {
		Cards []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"cards"`
	}
	byName := map[string][]cardset.Entry{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}
		if !strings.HasSuffix(h.Name, ".json") {
			continue
		}
		var d deck
		if err := json.NewDecoder(tr).Decode(&d); err != nil {
			continue
		}
		var es []cardset.Entry
		for _, c := range d.Cards {
			es = append(es, cardset.Entry{Name: c.Name, Count: c.Count})
		}
		if len(es) > 0 {
			byName[strings.TrimSuffix(h.Name, ".json")] = es
		}
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([][]cardset.Entry, len(names))
	for i, n := range names {
		out[i] = byName[n]
	}
	return out
}

// grantRandomDecks is the measurement harness's swRandomDecks: two-colour
// 40-card decks, 23 spells (<=2 copies) + 17 basics.
func grantRandomDecks(set cardset.Set, n int, seed uint64) [][]cardset.Entry {
	r := rand.New(rand.NewPCG(seed, 99))
	sup := set.Supported()
	basics := [5]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}
	cols := [5]kop.ColorMask{kop.ColW, kop.ColU, kop.ColB, kop.ColR, kop.ColG}
	var spells []int
	for i := 0; i < set.NumCards() && i < len(sup); i++ {
		if !sup[i] || set.Inert(i) {
			continue
		}
		d := set.Def(i)
		if d.Types&kop.TLand != 0 {
			continue
		}
		spells = append(spells, i)
	}
	var out [][]cardset.Entry
	for len(out) < n {
		a, b := r.IntN(5), r.IntN(5)
		for b == a {
			b = r.IntN(5)
		}
		m := cols[a] | cols[b]
		var cand []int
		for _, i := range spells {
			if set.Def(i).Colors&^m == 0 {
				cand = append(cand, i)
			}
		}
		if len(cand) < 12 {
			continue
		}
		cnt := map[string]int{}
		for k := 0; k < 23; k++ {
			cnt[set.Names()[cand[r.IntN(len(cand))]]]++
		}
		var es []cardset.Entry
		keys := make([]string, 0, len(cnt))
		for k := range cnt {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			es = append(es, cardset.Entry{Name: k, Count: cnt[k]})
		}
		es = append(es, cardset.Entry{Name: basics[a], Count: 9}, cardset.Entry{Name: basics[b], Count: 8})
		out = append(out, es)
	}
	return out
}

// grantBoostNames lists the pool's supported cards whose Continuous static
// grants a mana ability (prepTokMana's contGrant scan), so a crafted deck can
// put a granting static in each seat's board.
func grantBoostNames(set cardset.Set) []string {
	sup := set.Supported()
	var out []string
	for i := 0; i < set.NumCards() && i < len(sup); i++ {
		if !sup[i] || set.Inert(i) || set.Names()[i] == "" {
			continue
		}
		cd := set.Def(i)
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Grant != 0 &&
				int(a.Static.Grant-1) < len(cd.Abilities) && cd.Abilities[a.Static.Grant-1].Kind == kop.AbMana {
				out = append(out, set.Names()[i])
				break
			}
		}
	}
	return out
}

// grantBoostDecks are two 40-card decks around the pool's first grant card:
// three copies each and basics of its colours, so each seat's line casts one
// and the board holds two grant-capable statics. nil when the pool has none.
func grantBoostDecks(set cardset.Set) [][]cardset.Entry {
	names := grantBoostNames(set)
	if len(names) == 0 {
		return nil
	}
	sup := set.Supported()
	var cols []string
	for i := 0; i < set.NumCards() && i < len(sup); i++ {
		if sup[i] && set.Names()[i] == names[0] {
			m := set.Def(i).Colors
			for _, c := range []struct {
				b kop.ColorMask
				n string
			}{{kop.ColW, "Plains"}, {kop.ColU, "Island"}, {kop.ColB, "Swamp"}, {kop.ColR, "Mountain"}, {kop.ColG, "Forest"}} {
				if m&c.b != 0 {
					cols = append(cols, c.n)
				}
			}
		}
	}
	if len(cols) == 0 {
		cols = []string{"Plains", "Island"}
	}
	if len(cols) == 1 {
		cols = append(cols, cols[0])
	}
	one := []cardset.Entry{{Name: names[0], Count: 3}, {Name: cols[0], Count: 19}, {Name: cols[1], Count: 18}}
	return [][]cardset.Entry{one, one}
}

// grantBoostSliceDecks moves the slice decks that play a grant card first, so
// the first pairing seats four of them (edh's commander pools).
func grantBoostSliceDecks(t *testing.T, name string, names []string, decks [][]cardset.Entry) [][]cardset.Entry {
	t.Helper()
	has := func(es []cardset.Entry) bool {
		for _, e := range es {
			for _, n := range names {
				if e.Name == n {
					return true
				}
			}
		}
		return false
	}
	out := make([][]cardset.Entry, 0, len(decks))
	for _, d := range decks {
		if has(d) {
			out = append(out, d)
		}
	}
	for _, d := range decks {
		if !has(d) {
			out = append(out, d)
		}
	}
	return out
}

// TestGrantWalkOrderEquivalence plays each pool with the random policy and
// compares the real grant walk beside its listenerOrder reference at every
// occupied slot of every decision (the queries' states; the reference is the
// pre-grantOrder body). maxDecisions bounds each seed; a pool the corpus
// cannot build skips.
func TestGrantWalkOrderEquivalence(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"std-big", "edh", "std-tla"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			set := cardset.AllowUnverified(grantSweepPool(t, name))
			seats := 2
			if name == "edh" {
				seats = 4
			}
			decks := grantSweepDecks(t, name, set, seats)
			if strings.HasPrefix(name, "std-") {
				// the synthetic pools: two crafted decks around the pool's grant
				// card pair first, so the board holds two grant-capable statics
				if boost := grantBoostDecks(set); len(boost) > 0 {
					decks = append(boost, decks...)
				}
			} else if names := grantBoostNames(set); len(names) > 0 {
				// the commander pool: the slice decks that play a grant card first
				decks = grantBoostSliceDecks(t, name, names, decks)
			}
			// the measurement harness's pairing loop: the first pick can be a
			// refused interaction, later ones build; one game per pairing,
			// played in turn like the harness's rotation
			type grantGame struct {
				tbl *kop.Table
				dk  [MaxSeats][]CardID
				n   uint8
			}
			var games []grantGame
			refused := 0
			for c := 0; len(games) < 8 && c < 4*len(decks)+64; c++ {
				pick := make([][]cardset.Entry, seats)
				for j := range pick {
					if c == 0 {
						pick[j] = decks[j%len(decks)] // the crafted (or grant-heavy) decks pair first
					} else {
						pick[j] = decks[(c*seats+j*7+j)%len(decks)]
					}
				}
				tbl, ids, err := set.MatchupN(pick)
				if err != nil {
					t.Logf("%s: pairing %d refused: %v", name, c, err)
					refused++
					continue
				}
				Register(tbl)
				t.Cleanup(func() { ReleaseTable(tbl) })
				g := grantGame{tbl: tbl, n: uint8(seats)}
				copy(g.dk[:], ids)
				games = append(games, g)
			}
			if len(games) == 0 {
				t.Fatalf("%s: no game builds (%d refused)", name, refused)
			}
			var s State
			var d Decision
			const maxDecisions = 3000
			total, mtotal := 0, 0
			for gi, g := range games {
				for seed := uint64(1); seed <= 2; seed++ {
					sd := uint64(gi)*2 + seed
					s.InitN(g.tbl, g.dk, g.n, sd, Config{Mulligans: 2})
					pol := NewRandomPolicy(sd)
					hits, multis, played := 0, 0, 0
					func() {
						defer func() {
							if recover() != nil {
								t.Logf("%s game %d seed %d: ended on a panic (an unsupported shape's play)", name, gi, sd)
							}
						}()
						for i := 0; i < maxDecisions; i++ {
							s.StepInto(&d)
							if d.Kind == DkNone {
								played = i
								return
							}
							k := s.k()
							if k.gen != nil {
								if compareGrantOrder(t, k, &s, fmt.Sprintf("%s game %d seed %d decision %d", name, gi, sd, i)) {
									multis++
								}
								for m := s.occ; m.any(); m = m.rest() {
									if h, _ := compareGrantsAt(t, k, &s, m.first(), fmt.Sprintf("%s game %d seed %d decision %d slot %d", name, gi, sd, i, m.first())); h {
										hits++
									}
								}
							}
							s.Apply(pol.Pick(&d))
						}
					}()
					if hits == 0 {
						t.Logf("%s game %d seed %d: no slot held a granted mana ability (%d decisions)", name, gi, sd, played)
					} else {
						t.Logf("%s game %d seed %d: %d slot states held grants, %d ordered (%d decisions)", name, gi, sd, hits, multis, played)
					}
					total += hits
					mtotal += multis
				}
			}
			if total == 0 {
				t.Fatalf("%s: no slot ever held a granted mana ability: the comparison ran on none", name)
			}
			if mtotal == 0 {
				t.Fatalf("%s: the walk's order never mattered (mtotal 0): no board held two grant-capable statics", name)
			}
			t.Logf("%s: %d ordered states", name, mtotal)
		})
	}
}

package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Restricted mana floating beside a player's pool (gorge's
// Players[p].RestrictedMana, CR 106.6). gorge keeps one batch per
// activation, in the order they were added, and a payment carves the
// batches its descriptor admits in that order before the ordinary mana of
// their colour (pay.emitRestrictedManaSpend); a batch it does not admit stays
// hidden from the payment (pay.AvailableFor). State.rb[p] holds those batches
// merged only where that order cannot tell them apart: a unit joins the latest
// batch of its restriction and chosen type unless a later batch holds its
// colour. Helga, Skittish Seer's restricted mana beside a Village's floats in
// two batches. Past maxRB batches the mana floats unrestricted and the caller
// stalls (StallRestrSrc).

// maxRB is how many restricted batches one player's pool holds.
const maxRB = 4

// rbatch is one restricted batch: its units per colour, its restriction
// (genPrep.restrF index plus one; 0: the entry is free) and the creature type
// its source chose (Secluded Courtyard's "Spell.Creature+ChosenType").
type rbatch struct {
	n      [kop.NumManaKinds]uint8
	kind   uint8
	chosen uint8
}

// units is the batch's mana.
func (b *rbatch) units() int {
	t := 0
	for _, n := range b.n {
		t += int(n)
	}
	return t
}

// rbAny: p has restricted mana floating.
func (s *State) rbAny(p uint8) bool {
	for i := range s.rb[p&1] {
		if s.rb[p&1][i].kind == 0 {
			return false
		}
		if s.rb[p&1][i].units() != 0 {
			return true
		}
	}
	return false
}

// rbAll is the mask of p's batches holding mana.
func (s *State) rbAll(p uint8) uint8 {
	var m uint8
	for i := range s.rb[SeatIx(p)] {
		if b := &s.rb[SeatIx(p)][i]; b.kind != 0 && b.units() != 0 {
			m |= 1 << i
		}
	}
	return m
}

// rbClear empties p's restricted mana.
func (s *State) rbClear(p uint8) {
	s.rb[SeatIx(p)] = [maxRB]rbatch{}
}

// rbCompact drops p's emptied batches, keeping the order of the rest.
func (s *State) rbCompact(p uint8) {
	rb := &s.rb[SeatIx(p)]
	j := 0
	for i := range rb {
		if rb[i].kind != 0 && rb[i].units() != 0 {
			rb[j] = rb[i]
			j++
		}
	}
	for ; j < maxRB; j++ {
		rb[j] = rbatch{}
	}
}

// rbUnits adds the units of p's batches in mask m to t.
func (s *State) rbUnits(p, m uint8, t *[kop.NumManaKinds]uint8) {
	for i := range s.rb[SeatIx(p)] {
		if m&(1<<i) != 0 {
			for c, n := range s.rb[SeatIx(p)][i].n {
				t[c] += n
			}
		}
	}
}

// rbTotal is how much restricted mana p's batches in mask m hold.
func (s *State) rbTotal(p, m uint8) int {
	t := 0
	for i := range s.rb[p&1] {
		if m&(1<<i) != 0 {
			t += s.rb[p&1][i].units()
		}
	}
	return t
}

// rbPut floats n restricted mana of colour col, of restriction kind (restrF
// index plus one) and chosen type ch, beside p's pool: into the latest batch
// of that restriction and type when no later batch holds colour col, else a new
// one. False when every batch is taken.
func (s *State) rbPut(p uint8, kind, ch uint8, col int, n uint8) bool {
	rb := &s.rb[p&1]
	free := maxRB
	for i := maxRB - 1; i >= 0; i-- {
		b := &rb[i]
		if b.kind == 0 {
			free = i
			continue
		}
		if b.kind == kind && b.chosen == ch {
			if int(b.n[col])+int(n) > 255 {
				return false
			}
			b.n[col] += n
			return true
		}
		if b.n[col] != 0 {
			break // a later batch of another restriction holds the colour: gorge spends it after this one
		}
	}
	if free == maxRB {
		return false
	}
	rb[free] = rbatch{kind: kind, chosen: ch}
	rb[free].n[col] = n
	return true
}

// rbTake moves the units of p's batches in mask m into p's pool, where the
// payment may spend them; tk records what each batch gave. The batches stay
// in place, emptied, so what the payment leaves floats back where it was
// (rbSettle).
func (s *State) rbTake(p, m uint8, tk *[maxRB][kop.NumManaKinds]uint8) {
	pl := &s.pl[p&1]
	for i := range s.rb[p&1] {
		if m&(1<<i) == 0 {
			continue
		}
		b := &s.rb[p&1][i]
		for c, n := range b.n {
			tk[i][c] += n
			pl.pool[c] += n
		}
		b.n = [kop.NumManaKinds]uint8{}
	}
}

// rbTakeNew moves into p's pool the restricted mana p's batches gained since
// sv (a plan's own restricted taps: restrPairUnit), recording it in tk.
func (s *State) rbTakeNew(p uint8, sv *[maxRB]rbatch, tk *[maxRB][kop.NumManaKinds]uint8) {
	pl := &s.pl[p&1]
	for i := range s.rb[p&1] {
		b := &s.rb[p&1][i]
		for c := range b.n {
			if d := b.n[c] - sv[i].n[c]; d != 0 && b.n[c] > sv[i].n[c] {
				tk[i][c] += d
				pl.pool[c] += d
				b.n[c] = sv[i].n[c]
			}
		}
	}
}

// rbSettle floats again what the payment left of the restricted mana tk (per
// batch, as rbTake took it): per colour that mana is spent before the
// ordinary mana of its colour, ord of which was there before (a typed unit
// after it: pay.takeUnit), batch by batch in order (gorge
// emitRestrictedManaSpend); pre is the pool before the payment spent. typed:
// the restricted mana is a typed source's (rbTyped).
func (s *State) rbSettle(p uint8, tk *[maxRB][kop.NumManaKinds]uint8, ord, pre *[kop.NumManaKinds]uint8, typed bool) {
	pl := &s.pl[p&1]
	for c := 0; c < kop.NumManaKinds; c++ {
		spent := pre[c] - pl.pool[c]
		if typed {
			spent -= min(spent, ord[c]) // a typed unit is spent after the plain ones of its colour (pay.takeUnit)
		}
		for i := range tk {
			n := tk[i][c]
			if n == 0 {
				continue
			}
			d := min(spent, n)
			spent -= d
			if left := n - d; left != 0 {
				pl.pool[c] -= left
				s.rb[p&1][i].n[c] += left
			}
		}
	}
	s.rbCompact(p)
	if !s.rbAny(p) {
		s.g.poolArt[p&1] &^= poolArtRestr
	}
}

// rbTyped: p's restricted mana came from a typed source (poolArtRestr), so
// gorge spends the plain units of its colour first.
func (s *State) rbTyped(p uint8) bool {
	return s.g.poolArt[p&1]&poolArtRestr != 0 && !DebugRestrTypedFirst
}

// rbSpendN spends up to n of p's restricted mana in the batches of mask m, in
// batch order and colour order (a fee gorge carves from the batches it admits
// before ordinary mana), returning how much it spent.
func (s *State) rbSpendN(p, m uint8, n int) int {
	spent := 0
	for i := range s.rb[p&1] {
		if m&(1<<i) == 0 {
			continue
		}
		b := &s.rb[p&1][i]
		for c := range b.n {
			d := min(n-spent, int(b.n[c]))
			b.n[c] -= uint8(d)
			spent += d
		}
	}
	s.rbCompact(p)
	if !s.rbAny(p) {
		s.g.poolArt[p&1] &^= poolArtRestr
	}
	return spent
}

// rbHash folds p's restricted batches into a word for the state hash.
func (s *State) rbHash(p uint8) (h uint64, ok bool) {
	for i := range s.rb[p&1] {
		b := &s.rb[p&1][i]
		if b.kind == 0 {
			break
		}
		ok = true
		var w uint64
		for c, n := range b.n {
			w |= uint64(n) << (8 * c)
		}
		w |= uint64(b.kind)<<48 | uint64(b.chosen)<<56
		h = h*0x9e3779b97f4a7c15 + w
	}
	return h, ok
}

// rbColours is how many colours p's restricted mana in the batches of mask m holds.
func (s *State) rbColours(p, m uint8) int {
	var t [kop.NumManaKinds]uint8
	s.rbUnits(p, m, &t)
	n := 0
	for _, u := range t {
		if u != 0 {
			n++
		}
	}
	return n
}

// restrActNow is restrPoolAct for the payment being made (payDescNow).
func (s *State) restrActNow(k *prep, p uint8) uint8 {
	if !s.rbAny(p) {
		return 0
	}
	d := s.payDescNow(NoObj)
	return s.restrPoolAct(k, p, &d)
}

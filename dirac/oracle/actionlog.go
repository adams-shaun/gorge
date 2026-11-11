package oracle

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// RecordKernelActions turns a game a self-play runner played on the kernel alone
// (its kernel-native action log, decks and seed, and play, the kernel
// configuration it played with) into the KernelGame record Replay
// consumes: it re-plays the actions through the oracle's kernel adapter
// (with chance recording on) and records the canonical actions, the
// projection hash at every decision and the chance transcript. decks are
// the kernel decks self-play played (they must match the matchup's, mapped
// through the table).
//
// the kernel's default configuration and the adapter's pose different decision surfaces (the
// adapter's is gorge's: GorgeMustAttack, GorgeAltAdd, GorgeLowering), and
// each answers its own single-option decisions in place, so self-play's log
// alone cannot say which of its actions a decision the adapter elides
// would have consumed. The actions are therefore first expanded to one per
// decision the kernel's default configuration posed (playActions), and the adapter poses every
// decision too, eliding the single-option ones itself.
func RecordKernelActions(m *Matchup, tbl *kop.Table, names map[string]string, decks [2][]kop.CardID, seed uint64, cfg Config, play dirac.Config, acts []dirac.Action) (*KernelGame, error) {
	full, subs, err := playActions(tbl, decks, seed, play, acts)
	if err != nil {
		return nil, err
	}
	mk := RealKernel(tbl, names, cfg)
	if names == nil && tbl.TypeVocab != nil {
		mk = FDNSide(tbl, cfg) // a generated table: its adapter hooks
	}
	side, err := mk(m, seed)
	if err != nil {
		return nil, err
	}
	r := side.(*realSide)
	// RealKernel's config, every decision posed
	r.kcfg = dirac.LockstepConfig(cfg.surface())
	r.kcfg.NoElide = true
	r.kcfg.Mulligans = uint8(m.Mulligans)
	r.kcfg.Commander, r.kcfg.Commanders = m.kernelCommanders()
	r.kcfg.RecordChance = true
	r.kcfg.AutoDivide = play.AutoDivide
	r.s.Init(tbl, decks[0], decks[1], seed, r.kcfg)
	elide := cfg.PassElision
	g := &KernelGame{Format: gameFormat, Decks: m.Names, Seed: seed, Mulligans: m.Mulligans, PassElision: cfg.PassElision,
		AutopaySurface: cfg.GorgeAutopaySurface, Winner: -1}
	g.Commander, g.Commanders, g.Partners = m.commanderFields()
	for s := 0; s < 2; s++ {
		for _, c := range m.Decks[s] {
			g.Cards[s] = append(g.Cards[s], cardName(c))
		}
	}
	var p Projection
	i := 0
	for {
		d := r.Step()
		r.Project(&p)
		if d.Kind == DNone || i >= len(full) {
			g.Hashes = append(g.Hashes, compareHash(&p))
			break
		}
		// rec answers the decision with option j, recording it unless the
		// adapter's elision answers it in place
		rec := func(j int) {
			if !elide || r.d.N != 1 {
				g.Hashes = append(g.Hashes, compareHash(&p))
				g.Actions = append(g.Actions, r.opts[j])
			}
			r.Apply(r.opts[j])
		}
		a := full[i]
		found := -1
		for j := 0; j < int(r.d.N); j++ {
			if r.d.Options[j] == a {
				found = j
				break
			}
		}
		if found >= 0 {
			rec(found)
			i++
			continue
		}
		if a.Kind() == dirac.AkDone {
			// a declaration self-play closed that the adapter has already
			// closed (its last pick left Done alone)
			i++
			continue
		}
		if r.d.Kind == dirac.DkAttackers {
			// the kernel's default configuration (GorgeMustAttack off) declared the creatures
			// that must attack before the declaration; gorge poses them
			// as picks: pick each at the defending player first
			must := -1
			for j := 0; j < int(r.d.N); j++ {
				if o := r.d.Options[j]; o.Kind() == dirac.AkAttack && o.Target() >= dirac.PlayerTarget0 && r.s.MustAttack(dirac.Obj(o.Source())) {
					must = j
					break
				}
			}
			if must >= 0 {
				rec(must)
				continue
			}
		}
		if r.d.Kind == dirac.DkChoose && r.d.Sub == dirac.ChAltAdd {
			// the kernel's default configuration (Config.GorgeAltAdd off) lists only the
			// payable parts; gorge lists the unpayable one too: answer
			// the payable part, listed first (self-play's answer to the same
			// ask is consumed)
			rec(0)
			if subs[i] == dirac.ChAltAdd {
				i++
			}
			continue
		}
		if r.d.N == 1 {
			// a decision only the adapter poses (a must-attack pick's
			// Done, an ask the default surface answers in place)
			rec(0)
			continue
		}
		return nil, fmt.Errorf("action %d (%08x) is not offered: decision kind %d sub %d player %d options %08x", i, uint32(a), r.d.Kind, r.d.Sub, r.d.Player, r.d.Options[:r.d.N])
	}
	g.Over, g.Winner = r.Terminal()
	if tr, ok := r.s.ChanceTranscript(); ok {
		for _, c := range tr {
			g.Chance = append(g.Chance, ChanceDraw{Bound: c.Bound, Value: c.Value})
		}
	}
	return g, nil
}

// playActions replays a self-play action log on a kernel with its own
// configuration and every decision posed, and returns one action per
// decision the kernel's default configuration posed: the logged action, or the lone option
// its elision answered in place (with each decision's choice kind).
func playActions(tbl *kop.Table, decks [2][]kop.CardID, seed uint64, play dirac.Config, acts []dirac.Action) ([]dirac.Action, []dirac.ChoiceKind, error) {
	elide := !play.NoElide
	play.NoElide = true
	s := new(dirac.State)
	s.Init(tbl, decks[0], decks[1], seed, play)
	var full []dirac.Action
	var subs []dirac.ChoiceKind
	var d dirac.Decision
	for i := 0; ; {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		var a dirac.Action
		switch {
		case elide && d.N == 1:
			a = d.Options[0]
		case i < len(acts):
			a = acts[i]
			i++
			ok := false
			for _, o := range d.Options[:d.N] {
				ok = ok || o == a
			}
			if !ok {
				return nil, nil, fmt.Errorf("self-play action %d (%08x) is not offered on replay: decision kind %d player %d options %08x", i-1, uint32(a), d.Kind, d.Player, d.Options[:d.N])
			}
		default:
			return full, subs, nil
		}
		full = append(full, a)
		subs = append(subs, d.Sub)
		s.Apply(a)
	}
	return full, subs, nil
}

package hbot

import "github.com/adams-shaun/gorge/dirac"

// Declaration appends to out the bot's whole answer to an attackers or
// blockers decision (every pair of the declaration, declared ones
// included, in the answer's order) and returns it; Pick serves it one pick
// at a time. Other kinds append nothing. For tests and the parity harness.
func (bt *Bot) Declaration(s *dirac.State, d *dirac.Decision, out []dirac.Action) []dirac.Action {
	b := &bt.b
	cs := &b.cs
	switch d.Kind {
	case dirac.DkAttackers:
		b.fillCombat(s, bt.f, d.Player)
		cs.buildAttackOpts(b, s, d)
		cs.chooseAttackers(b, d.Player)
		for j := 0; j < cs.nCh; j++ {
			o := cs.opts[cs.chosen[j]]
			out = append(out, dirac.MakeAction(dirac.AkAttack, dirac.TCode(o.src), dirac.TCode(o.tgt), 0))
		}
	case dirac.DkBlockers:
		b.fillCombat(s, bt.f, d.Player)
		cs.buildBlockOpts(b, s, d.Player)
		cs.chooseBlockers(b, d.Player)
		for j := 0; j < cs.nCh; j++ {
			o := cs.opts[cs.chosen[j]]
			out = append(out, dirac.MakeAction(dirac.AkBlock, dirac.TCode(o.src), dirac.TCode(o.tgt), 0))
		}
	}
	return out
}

package kgen

import "strings"

func init() { kwHandlers["Mentor"] = (*lowerCtx).lowerMentorKw }

// lowerMentorKw lowers a printed bare K:Mentor (CR 702.134): gorge's kwMentor
// (cards/kw_mentor.go) expands it into one Mode$ Attacks self-trigger whose
// body is a targeted DB$ PutCounter carrying Mentor$ True, lowered from that
// line (mentorLesserPower). A face whose expansion is absent refuses.
func (lc *lowerCtx) lowerMentorKw(node, param string) {
	if param != "" {
		lc.refuseNode(node, "mentor with a parameter %q", param)
		return
	}
	line := kwLineOf(node)
	for _, t := range lc.face.Triggers {
		if t.Params["KeywordLine"] == line {
			return
		}
	}
	lc.refuseNode(node, "keyword %q without its trigger", "Mentor")
}

// mentorPutCounter lowers PutCounter with Mentor$ True: the counter goes on
// "target attacking creature with lesser power", the strict comparison
// gorge's rules mentorAdmits makes at the target offer and at the CR 608.2b
// recheck: the candidate's current power < the resolving source's. It is
// the body's ordinary targeted PutCounter with one more conjunct on each
// target clause, power < Count$CardPower (the source's own power; kernel
// PredPowerGEV PredN 2 bound by power - 1). Only the expansion's own shape
// (one battlefield target, Creature.attacking) lowers.
func (lc *lowerCtx) mentorPutCounter(ps *params) error {
	ps.consume("Mentor")
	if ps.str("ValidTgts") != "Creature.attacking" {
		return refuse("PutCounter Mentor$ with ValidTgts$ %q", ps.str("ValidTgts"))
	}
	n := len(lc.ab.Targets)
	if err := lc.putCounter(ps); err != nil {
		return err
	}
	if len(lc.ab.Targets) != n+1 {
		return refuse("PutCounter Mentor$ without its one target")
	}
	t := &lc.ab.Targets[n]
	if t.F == nil || t.F.Players != 0 || len(t.F.Clauses) != 1 {
		return refuse("PutCounter Mentor$ target filter is not one creature clause")
	}
	c := &t.F.Clauses[0]
	if c.CmpStat != cmpNone {
		return refuse("PutCounter Mentor$ target already compares %s", strings.TrimSpace(cmpStatName(c.CmpStat)))
	}
	v, err := lc.svarValue("Count$CardPower", 0)
	if err != nil {
		return err
	}
	c.CmpStat, c.CmpOp, c.CmpV = cmpPower, opLT, &v
	return nil
}

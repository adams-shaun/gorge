package kgen

import "testing"

// TestCloneEnterGrantLowers: Mercurial Pretender's as-enters copy with an
// AddAbilities$ grant lowers (it refused "Clone unread params
// [AddAbilities]"): its BecomeCopy names, in A, an override record of its
// own that grants one latent activated ability of the card, apart from the
// copy-exception record in C (lower_cloneentergrant.go).
func TestCloneEnterGrantLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Mercurial Pretender"})
	if len(cs) != 1 || cs[0].Name != "Mercurial Pretender" {
		t.Fatalf("lowered %d cards, want Mercurial Pretender", len(cs))
	}
	c := cs[0]
	for _, r := range c.Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	var bc *lInstr
	progs := [][]lInstr{}
	for _, a := range c.Abilities {
		progs = append(progs, a.Prog)
	}
	for i := range c.Repls {
		progs = append(progs, c.Repls[i].Prog)
	}
	for _, p := range progs {
		for i := range p {
			if p[i].Op == "BecomeCopy" {
				bc = &p[i]
			}
		}
	}
	if bc == nil {
		t.Fatal("no BecomeCopy op")
	}
	if bc.C&1 == 0 {
		t.Error("BecomeCopy is not optional (the keyword line's :Optional)")
	}
	if bc.C>>1 != 0 {
		t.Errorf("BecomeCopy names copy exceptions %d: the grant is not one", bc.C>>1)
	}
	if !bc.A.isConst() || bc.A.N < 1 || bc.A.N > len(c.overrides) {
		t.Fatalf("BecomeCopy A = %v, want a grant override index+1 of %d", bc.A, len(c.overrides))
	}
	o := c.overrides[bc.A.N-1]
	if len(o.GrantAbility) != 1 {
		t.Fatalf("grant override grants %v, want one ability", o.GrantAbility)
	}
	ab := c.Abilities[o.GrantAbility[0]]
	if ab.Kind != akActivated || !ab.Latent {
		t.Errorf("granted ability kind %d latent %v, want a latent activated ability", ab.Kind, ab.Latent)
	}
}

package cards

import (
	"path/filepath"
	"testing"
)

func fixtureRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	for _, src := range []string{
		"Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n",
		"Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n",
		"Name:Delver of Secrets\nManaCost:U\nTypes:Creature Human Wizard\nPT:1/1\nOracle:x\nALTERNATE\nName:Insectile Aberration\nTypes:Creature Human Insect\nPT:3/2\nK:Flying\nOracle:x\n",
	} {
		c, _ := ParseBytes("fixture.txt", []byte(src))
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		r.Add(c)
	}
	return r
}

func TestRegistryLookupNormalisation(t *testing.T) {
	r := fixtureRegistry(t)
	for _, name := range []string{
		"Lightning Bolt", "lightning bolt", "  Lightning  Bolt ",
		"Delver of Secrets", "Delver of Secrets // Insectile Aberration",
	} {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("Lookup(%q) missed", name)
		}
	}
	if _, ok := r.Lookup("Not A Card"); ok {
		t.Error("Lookup matched a card that does not exist")
	}
}

func TestRegistryCacheRoundTrip(t *testing.T) {
	r := fixtureRegistry(t)
	path := filepath.Join(t.TempDir(), "ir.gob.gz")
	if err := r.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(back.Cards) != len(r.Cards) {
		t.Fatalf("cards = %d, want %d", len(back.Cards), len(r.Cards))
	}
	// Structure must survive, not just names: the sub-ability tree and the
	// intrinsic mana ability are what the engine actually consumes.
	bolt, ok := back.Lookup("Lightning Bolt")
	if !ok {
		t.Fatal("bolt missing after round trip")
	}
	if sa := bolt.Faces[0].SpellAbility(); sa == nil || sa.Params["NumDmg"] != "3" {
		t.Fatalf("bolt spell ability = %+v", sa)
	}
	mtn, _ := back.Lookup("Mountain")
	if len(mtn.Faces[0].ManaAbilities()) != 1 {
		t.Fatal("intrinsic mana ability lost in round trip")
	}
}

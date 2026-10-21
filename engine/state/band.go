package state

// BandMates returns the other creatures in id's attacking band (CR 702.22c):
// battlefield attackers under id's controller whose Band equals id's, in
// that controller's battlefield order, id excluded. nil when id is in no band.
func (g *Game) BandMates(id ObjID) []ObjID {
	o := g.Obj(id)
	if o == nil || o.Band == 0 || !o.IsAttacking || o.Zone != ZBattlefield {
		return nil
	}
	var out []ObjID
	for _, m := range g.Zone(ZBattlefield, o.Controller) {
		if m == id {
			continue
		}
		if mo := g.Obj(m); mo != nil && mo.IsAttacking && mo.Band == o.Band {
			out = append(out, m)
		}
	}
	return out
}

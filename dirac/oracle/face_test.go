package oracle

// kernelFaceRefs returns the source refs of the kernel's posed face casts of
// the given face code at the decision the lockstep just compared. The
// lockstep has already proved this list is gorge's (offered options plus the
// potential planner's lowered plays), so the tallies read the union here.
func kernelFaceRefs(k Side, code uint8) []Ref {
	r, ok := k.(*realSide)
	if !ok {
		return nil
	}
	var out []Ref
	for _, o := range r.opts {
		if o.Kind() == ActCastFace && o.Mode()&0x0f == code {
			out = append(out, o.Src())
		}
	}
	return out
}

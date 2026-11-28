package ir

import "fmt"

// valueArgs is the number of Args each ValueKind takes; a kind not listed
// takes none.
var valueArgs = [valueKindCount]uint8{
	VAdd: 2, VSub: 2, VMul: 2, VMin: 2, VMax: 2, VIf: 2,
	VHalfDown: 1, VHalfUp: 1,
}

// ValidateValue checks v's shape: a named kind, the Args count its kind
// takes, a Filter for a count, a Test on VIf and nowhere else, origin Zones on
// VEnteredThisTurn only, and every reachable filter valid.
func ValidateValue(v *Value) error {
	if v == nil {
		return fmt.Errorf("ir: nil value")
	}
	if v.Kind == VNone || v.Kind >= valueKindCount {
		return fmt.Errorf("ir: value: no kind (%s)", v.Kind)
	}
	label := v.Kind.String()
	if want := int(valueArgs[v.Kind]); len(v.Args) != want {
		return fmt.Errorf("ir: %s: needs %d args, has %d", label, want, len(v.Args))
	}
	if (v.Kind == VCount || v.Kind == VCastThisTurn || v.Kind == VEnteredThisTurn) && v.Filter == nil {
		return fmt.Errorf("ir: %s: needs a filter", label)
	}
	if (v.Kind == VIf) != (v.Test != nil) {
		return fmt.Errorf("ir: %s: a test goes with if and only if", label)
	}
	if v.Zones != 0 && v.Kind != VEnteredThisTurn {
		return fmt.Errorf("ir: %s: takes no origin zones", label)
	}
	for _, a := range v.Args {
		if err := ValidateValue(a); err != nil {
			return err
		}
	}
	if v.Filter != nil {
		if err := ValidateFilter(v.Filter); err != nil {
			return err
		}
	}
	for _, r := range []*Ref{v.Ref, v.Ref2} {
		if err := validateRefFilters(r); err != nil {
			return err
		}
	}
	return validateTest(v.Test)
}

// validateTest checks the values and filters a VIf condition reaches.
func validateTest(c *Cond) error {
	if c == nil {
		return nil
	}
	if c.Kind == CNone {
		return fmt.Errorf("ir: if: test has no kind")
	}
	for _, x := range []*Value{c.L, c.R} {
		if x == nil {
			continue
		}
		if err := ValidateValue(x); err != nil {
			return err
		}
	}
	if err := validateRefFilters(c.Ref); err != nil {
		return err
	}
	if c.Kind == CCompare && (c.L == nil || c.R == nil || c.Cmp == CmpNone) {
		return fmt.Errorf("ir: if: a compare test needs l, r and cmp")
	}
	for _, a := range c.Args {
		if err := validateTest(a); err != nil {
			return err
		}
	}
	if c.Filter != nil {
		return ValidateFilter(c.Filter)
	}
	return nil
}

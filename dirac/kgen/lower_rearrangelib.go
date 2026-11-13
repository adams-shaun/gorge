package kgen

// rearrangeWholeLibrary reports a RearrangeTopOfLibrary NumCards$ that is the
// controller's whole library (Doomsday's Count$ValidLibrary Card.YouOwn,
// read at resolution after its sweep left only the found cards there). gorge
// effRearrangeTopOfLibrary clamps the count to the library and orders that
// many; the kernel's OpRearrange reads A at resolution with the same clamp,
// and a library past its eight-card look window stalls loudly (StallTables),
// never a silent short order.
func rearrangeWholeLibrary(n lValue) bool {
	return n.Kind == vLibrarySizeOf && n.Ref.Kind == rYou && n.Mul == 0 && n.Div == 0 && n.Add == 0 && !n.Neg && len(n.Sub) == 0
}

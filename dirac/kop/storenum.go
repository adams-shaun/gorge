package kop

// StoreSVar's number store (ExtStoreNum writes it, ExSVarNum reads it; both
// are documented in their manifest entries). The SVar names a table's cards
// write are Table.SVarNames, at most 15 of them: a record's name is that
// index, kept in the high nibble of the object-set record's kind.
//
// ExtStoreNum C: the name index in the low 4 bits, bit 4 set when the write
// of the printed default deletes the record, the printed default (a signed
// 11-bit number, -1024..1023) from bit 5 up. kgen sets the delete bit only for
// a printed default that is a literal in that range (StoreNumDefFits). The
// widths are literals in the functions below, not package constants: this
// package's constants are pinned ordinals (testdata/ordinals.tsv).

// SVarNamesFull reports whether a table already holds its 15 names.
func SVarNamesFull(n int) bool { return n >= 15 }

// StoreNumDefFits reports whether a printed default fits ExtStoreNum C's
// default field (the delete-on-default bit is set only for one that does).
func StoreNumDefFits(def int) bool { return def >= -1024 && def <= 1023 }

// StoreNumC packs ExtStoreNum's C: name is the Table.SVarNames index and
// def the printed default, deleted-on-write when del.
func StoreNumC(name int, del bool, def int) int16 {
	c := name & 0x0f
	if del {
		c |= 1<<4 | def<<5
	}
	return int16(c)
}

// StoreNumUnpack is StoreNumC's inverse.
func StoreNumUnpack(c int16) (name int, del bool, def int) {
	name = int(c) & 0x0f
	if int(c)&(1<<4) != 0 {
		del, def = true, int(c)>>5
	}
	return
}

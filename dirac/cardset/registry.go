package cardset

// ByName returns the set named name from this binary's binary tables. Any error
// (ErrNoCorpus, ErrNoTables, a damaged file, an unknown pool) is returned as it is.
func ByName(name string) (Set, error) { return load(name) }

// MustByName is ByName for a helper with no way to return an error or reach a
// testing.TB: it panics. Its callers guard with cardsettest.Require.
func MustByName(name string) Set {
	s, err := ByName(name)
	if err != nil {
		panic(err)
	}
	return s
}

// Names lists the names ByName serves, sorted: the manifest of this binary's
// tables. It has no error to return, so it is nil when the tables cannot be read,
// and an empty list does not mean there is nothing to check: a test calls
// cardsettest.Require first, which tells a missing corpus from missing tables.
func Names() []string {
	out, err := tableNames()
	if err != nil {
		return nil
	}
	return out
}

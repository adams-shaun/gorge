package spellbench

// knownUnsupportedFDN is TestFDNCoverage's ratchet table: the FDN Limited
// cards gorge does not fully support, with the primitives each is missing.
// Rows only ever go away (TestFDNCoverage logs a stale row). Empty: 281/281
// supported.
var knownUnsupportedFDN = map[string][]string{}

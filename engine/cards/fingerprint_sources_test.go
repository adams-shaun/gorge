package cards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// fingerprintFile is one engine/cards/*.go file's census entry: whether its source
// can change the IR a cache holds (and so belongs in CompilerFingerprint),
// and why.
type fingerprintFile struct {
	irAffecting bool
	why         string
}

// fingerprintCensus classifies EVERY non-test .go file in cards. A file is
// IR-affecting when anything CompileDir or Registry.Save runs can reach it --
// its code, or a type, const or var that code reads -- so a change to it could
// change the bytes of ir-<fingerprint>.gob.gz or its .seg sibling. Every
// other file only reads compiled IR at runtime (rules queries and coverage
// reports) or manages the cache file without shaping its
// content, and editing it must not cost every process on the box a corpus
// recompile (audit 2026-10-12: 73 engine/cards/*.go commits in 7 days, each a new
// cache, each a herd of recompiles).
//
// The split was measured, not guessed: a probe binary calling only CompileDir
// and Save, linked with the normal dead-code elimination, and its DWARF line
// table read per file; the files with no reachable function were then
// checked for types, consts and vars that reachable code or IR struct fields
// reference (go/ast). image.go and image_build.go are load-time only today
// but stay in because the pointer-free spec's S5 persists the Image itself. A file whose only reachable lines were package var
// initializers (fetch.go, mayask.go, breakdown.go) counts as unreachable.
//
// WHEN UNSURE, MARK IT IR-AFFECTING. A false "true" costs one extra
// recompile per edit; a false "false" serves a stale IR cache.
var fingerprintCensus = map[string]fingerprintFile{
	"ability_class.go":        {true, "IsManaAbilitySA/requiresTarget classify abilities during compile"},
	"aihints.go":              {true, "the PlayMain1 vocabulary shapes the canonical SVar value parsed into the IR"},
	"affected_defined.go":     {true, "NormalizeAffectedDefined rewrites static params during compile"},
	"ateot_values.go":         {true, "atEOTValues feeds Primitives, so which AtEOT$ values are unsupported is part of the compiled IR"},
	"breakdown.go":            {false, "coverage-report grouping (CoverageGroup); report time only, nothing compiled"},
	"cache_canon.go":          {true, "canonical gob encoding of the cache and segment bytes"},
	"cache_codec.go":          {true, "the face codec: the cache and segment bytes of every Face"},
	"cache_lock.go":           {false, "single-flight lock around compile+Save; never changes the bytes written"},
	"cache_lock_flock.go":     {false, "flock implementation of the single-flight lock"},
	"cache_lock_noflock.go":   {false, "no-op single-flight lock for platforms without flock"},
	"card_probes.go":          {false, "rules-side per-card pool probes, memoised at runtime"},
	"compiled_catalog.go":     {true, "CompileMetadata is reached from CompileDir; kept conservatively"},
	"compiled_codes.go":       {true, "dense kind codes the compiled catalog assigns"},
	"control_static_probe.go": {false, "runtime query MayCarryControlStatic over compiled faces"},
	"alone_keyword.go":        {true, "CR 506.5 alone-sentence keyword heads canonicalised during compile"},
	"counter_keyword.go":      {true, "keyword-counter table reached from compile"},
	"doc.go":                  {false, "package documentation only"},
	"equal.go":                {false, "typed deep equality for engine memo verification; runtime only"},
	"face.go":                 {true, "Face derive (mana cost, CMC, word sets) runs during compile"},
	"face_runtime.go":         {false, "runtime copy-basis setters events.Apply calls; compile never runs them"},
	"textchange.go":           {false, "CR 612 text-changed runtime faces events.Apply builds; compile never runs them"},
	"fetch.go":                {false, "git fetch of the corpus and its Lock record; corpus changes are caught by cacheFresh's mtimes"},
	"fingerprint.go":          {false, "the fingerprint hash itself"},
	"fingerprint_sources.go":  {false, "the embed list itself"},
	"gitenv.go":               {false, "git environment for fetch"},
	"hiddenkeyword.go":        {true, "CanonicalKeywordLine normalises keyword lines during compile"},
	"image.go":                {true, "Image row types; S5 of the 2026-10-06 pointer-free spec persists them as ir-<fp>.img, so kept"},
	"image_build.go":          {true, "builds the Image from IR; to be persisted under S5 (ir-<fp>.img), so kept"},
	"image_mat.go":            {true, "materialization behind Registry.AllCards, Save's input; kept conservatively"},
	"intrinsic.go":            {true, "ApplyIntrinsics adds basic-land mana abilities during compile"},
	"ir.go":                   {true, "the IR types themselves; their exported fields are what gob encodes"},
	"keywords.go":             {true, "expandKeywords turns printed keywords into abilities during compile"},
	"kw_affinity.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_afflict.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_afterlife.go":         {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_annihilator.go":       {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_backup.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_battle_cry.go":        {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_cipher.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_class.go":             {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_conspire.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_craft.go":             {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_crew.go":              {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_cumulativeupkeep.go":  {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_cycling.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_demonstrate.go":       {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_dethrone.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_devour.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_echo.go":              {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_embalm.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_enchant.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_encore.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_equip.go":             {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_etbcounter.go":        {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_etbreplacement.go":    {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_evolve.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_exalted.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_exploit.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_extort.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_fabricate.go":         {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_firebending.go":       {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_flanking.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_formirrodin.go":       {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_fortify.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_graft.go":             {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_granted.go":           {false, "runtime synthesizer for layer-6 GRANTED keywords; expandKeywords never calls it"},
	"kw_gravestorm.go":        {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_hideaway.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_increment.go":         {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_jobselect.go":         {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_levelup.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_livingweapon.go":      {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_melee.go":             {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_mentor.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_mobilize.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_myriad.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_ninjutsu.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_offspring.go":         {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_outlast.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_partner_with.go":      {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_persist.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_prevent.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_prowess.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_ravenous.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_reconfigure.go":       {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_renown.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_replicate.go":         {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_saddle.go":            {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_station.go":           {false, "runtime synthesizer for the Station push body; expandKeywords never calls it"},
	"kw_soulbond.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_squad.go":             {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_storm.go":             {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_sunburst.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_training.go":          {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_transmute.go":         {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_tribute.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_typecycling.go":       {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_undaunted.go":         {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_undying.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_unearth.go":           {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_vanishing.go":         {true, "keyword expansion registered at init; shapes compiled abilities"},
	"kw_ward.go":              {true, "keyword expansion registered at init; shapes compiled abilities"},
	"layout.go":               {false, "runtime predicates over AlternateMode"},
	"linemodes.go":            {true, "trigger/static/replacement mode codes assigned during compile"},
	"link.go":                 {true, "SVar and sub-ability linking during compile"},
	"mana_production.go":      {true, "ManaProduction summary derived during compile"},
	"mayask.go":               {false, "runtime may-ask queries for rules; compile reaches only its var tables"},
	"modes.go":                {false, "runtime SplitModeNames over Choices$"},
	"mustblock_shape.go":      {false, "runtime certification of MustBlock target shapes"},
	"name_choice.go":          {false, "runtime MentionsNameChoice text probe for the oracle runner"},
	"open.go":                 {false, "cache lookup, freshness, prune and path re-rooting; load side only"},
	"params.go":               {true, "compiled ParamKey vocabulary and ParamSet encoding"},
	"parse.go":                {true, "the script parser"},
	"keyword_refs.go":         {false, "runtime KeywordRef vars the cards package asks faces through"},
	"primitive.go":            {false, "coverage Primitives computed on demand, not stored"},
	"printed_facts.go":        {false, "view facts in an unexported Face field gob never encodes; derive recomputes them on load"},
	"registry.go":             {true, "Registry, CompileDir, Save/LoadRegistry and cacheVersion"},
	"registry_lazy.go":        {true, "AllCards (Save's input) dispatches through lazyCards; kept conservatively"},
	"rider_shape.go":          {true, "prevention rider param shape; kept in the compiler fingerprint"},
	"saga.go":                 {false, "runtime SagaChapters parse of K:Chapter"},
	"slot.go":                 {false, "runtime memo slots held in unexported fields gob never encodes"},
	"strtab.go":               {true, "interned string tables used by compiled params"},
	"subset.go":               {true, "writeSegments: the .seg file written beside the cache"},
	"svar_activator.go":       {false, "runtime query SVarGrantHasActivator"},
	"tokens.go":               {true, "token scripts are compiled into the cache by CompileDir"},
	"universe.go":             {false, "runtime name-universe view over a registry"},
	"validate.go":             {true, "Card.named is called by CompileDir"},
	"value_heads.go":          {false, "coverage value heads computed on demand, not stored"},
	"words.go":                {true, "interned type and keyword word sets derived during compile"},
}

// TestCompilerSourcesFollowTheCensus holds fingerprint_sources.go's embed list
// to fingerprintCensus: every non-test file must be classified (a new file
// fails here until someone decides, so nothing silently drops out of the
// fingerprint), and exactly the IR-affecting ones are embedded.
func TestCompilerSourcesFollowTheCensus(t *testing.T) {
	onDisk, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	present := map[string]bool{}
	for _, e := range onDisk {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		present[n] = true
		if _, ok := fingerprintCensus[n]; !ok {
			t.Errorf("engine/cards/%s is not in fingerprintCensus (fingerprint_sources_test.go): classify it "+
				"-- IR-affecting (also add `//go:embed %s` to fingerprint_sources.go) or not, with a "+
				"one-line reason. When unsure, it is IR-affecting.", n, n)
		}
	}
	embedded, err := compilerSources.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, e := range embedded {
		have[e.Name()] = true
	}
	var names []string
	for n := range fingerprintCensus {
		names = append(names, n)
	}
	for n := range have {
		if _, ok := fingerprintCensus[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		c, classified := fingerprintCensus[n]
		switch {
		case strings.HasSuffix(n, "_test.go"):
			t.Errorf("fingerprint_sources.go embeds test file %s: remove its //go:embed line", n)
		case !present[n] && classified:
			t.Errorf("fingerprintCensus lists %s, which is not a source file here: remove its entry", n)
		case !present[n]:
			t.Errorf("fingerprint_sources.go embeds %s, which is not a source file here: remove its line", n)
		case !classified:
			// reported above
		case c.why == "":
			t.Errorf("fingerprintCensus[%q] has no reason", n)
		case c.irAffecting && !have[n]:
			t.Errorf("engine/cards/%s is IR-affecting but not embedded: add `//go:embed %s` to fingerprint_sources.go (sorted)", n, n)
		case !c.irAffecting && have[n]:
			t.Errorf("engine/cards/%s is classified not IR-affecting but is embedded: remove its //go:embed line, "+
				"or reclassify it", n)
		}
	}
}

// TestFingerprintExcludedFilesRegisterNothing guards the census against its
// likeliest silent failure: an init() added to a file outside the fingerprint
// (a keyword expansion registered there, say) would shape compiled IR while
// edits to it no longer changed the cache name.
func TestFingerprintExcludedFilesRegisterNothing(t *testing.T) {
	fset := token.NewFileSet()
	for n, c := range fingerprintCensus {
		// fingerprint.go's init only asserts the embed is populated.
		if c.irAffecting || n == "fingerprint.go" {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "init" {
				t.Errorf("engine/cards/%s is outside CompilerFingerprint but declares init(): "+
					"mark it IR-affecting in fingerprintCensus or move the registration", n)
			}
		}
	}
}

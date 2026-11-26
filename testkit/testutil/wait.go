package testutil

// WaitBudget is the shared, generous ceiling for a test's wait on a server, a
// subprocess or a goroutine: the deadline of a poll loop, the time.After arm
// whose expiry fails the test, or a context.WithTimeout whose expiry is the
// failure arm. A green test returns as soon as its condition holds, so a
// generous ceiling costs nothing on a passing run; only a broken one waits it
// out. Per-site literals were tuned on idle boxes and ran out on loaded
// self-hosted runners with no bad commit (TestServesADiracGame across 3 CI
// runs, batch 45499867149, Mergify register cause 88), and nothing stopped a
// new short one. Replace them with this constant. The only lines that keep a
// literal carry a `// wait:ok <reason>` comment: a negative wait ("nothing
// happens within N s" — its expiry is the success arm) or a value that is
// itself under test. TestNoFixedLongWaitsInTests keeps new literals out.
//
// It is an untyped constant — two minutes in nanoseconds — so it converts to
// time.Duration at each use: testkit/testutil imports no time (D16,
// TestTimeIsImportedOnlyByTheHost).
const WaitBudget = 120_000_000_000

// basepath is the single value every same-origin request the client builds
// is prefixed with (Task U0 Part A). gorge ships as a library inside
// mtgserve (Ruling FL-81), whose router mounts it under a path prefix like
// /gorge; without this the client's absolute /api/... URLs silently force a
// whole-subdomain deployment.
//
// The value is read ONCE at startup, from the served page's
// <meta name="gorge-base" content="/gorge"> tag. A meta tag is the source of
// choice because it is inert (nothing else on a page reads it, unlike a
// <base href>, which re-resolves every relative URL in the document), needs
// no script tag (so it works under a strict CSP), and needs no build-time
// configuration (the same bundle serves at any mount point — the embedding
// server's template injects the tag). The default is "": byte-identical to
// the pre-base client, which is what keeps `cmd/gorged`, serving at the
// root, unchanged.
let base = detect();

// normalize strips trailing slashes so "/gorge/" and "/gorge" mount
// identically — the classic source of the double-slash "//api/..." break.
function normalize(raw: string): string {
  return raw.replace(/\/+$/, '');
}

function detect(): string {
  // vitest runs without a served page (and so without a DOM); in that
  // environment there is no meta tag to read, and the default "" is the
  // correct value.
  if (typeof document === 'undefined') return '';
  const el = document.querySelector('meta[name="gorge-base"]');
  return normalize(el?.getAttribute('content') ?? '');
}

/** withBase prefixes path with the base path; with an empty base it is identity, so every URL is byte-identical to the pre-base client. */
export const withBase = (path: string): string => base + path;

// setBasePathForTests is the test hook: the vitest spec files have no
// served page to read the meta tag from, so they set the mount root
// explicitly. Production code never calls it.
export function setBasePathForTests(b: string): void {
  base = normalize(b);
}

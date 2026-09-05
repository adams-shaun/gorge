export type Route =
  | { kind: 'overview' }
  | { kind: 'table'; table: string }
  | { kind: 'match'; table: string; match: number }
  | { kind: 'notfound' };

export function parseRoute(pathname: string): Route {
  if (pathname === '/') return { kind: 'overview' };
  const m = pathname.match(/^\/t\/([^/]+)(?:\/m\/(\d+))?$/);
  if (!m) return { kind: 'notfound' };
  const table = decodeURIComponent(m[1]);
  if (m[2] === undefined) return { kind: 'table', table };
  return { kind: 'match', table, match: Number(m[2]) };
}

export function href(r: Route): string {
  switch (r.kind) {
    case 'overview': return '/';
    case 'table': return `/t/${encodeURIComponent(r.table)}`;
    case 'match': return `/t/${encodeURIComponent(r.table)}/m/${r.match}`;
    case 'notfound': return '/';
  }
}

/** navigate pushes a route and notifies App via popstate. */
export function navigate(r: Route): void {
  history.pushState(null, '', href(r));
  dispatchEvent(new PopStateEvent('popstate'));
}

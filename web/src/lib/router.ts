export type Route =
  | { kind: 'overview' }
  | { kind: 'table'; table: string }
  | { kind: 'match'; table: string; match: number }
  | { kind: 'newEvent' }
  | { kind: 'event'; event: string }
  | { kind: 'notfound' };

export function parseRoute(pathname: string): Route {
  if (pathname === '/') return { kind: 'overview' };
  // Draft events: /events/new creates a pod, /e/{id} is the event page.
  // The seat credentials ride the query (?seat=&token=), never the path, so
  // the route is the same for every viewer of one event.
  if (pathname === '/events/new' || pathname === '/events/new/') return { kind: 'newEvent' };
  const e = pathname.match(/^\/e\/([^/]+)\/?$/);
  if (e) return { kind: 'event', event: decodeURIComponent(e[1]) };
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
    case 'newEvent': return '/events/new';
    case 'event': return `/e/${encodeURIComponent(r.event)}`;
    case 'notfound': return '/';
  }
}

/** navigate pushes a route and notifies App via popstate. */
export function navigate(r: Route): void {
  navigateTo(href(r));
}

/**
 * navigateTo pushes a same-origin path that may carry a query (an event
 * invite's `/e/{id}?seat=N&token=T`) and notifies App via popstate, so the
 * new route mounts without a page load and reads its own query.
 */
export function navigateTo(path: string): void {
  history.pushState(null, '', path);
  dispatchEvent(new PopStateEvent('popstate'));
}

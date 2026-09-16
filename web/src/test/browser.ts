import { chromium, type Browser } from 'playwright';
import { inject } from 'vitest';

export const browserURL = inject('browserURL');

let browser: Promise<Browser> | undefined;

/**
 * Connect test workers to the one Chromium process globalSetup launched.
 * A worker's connection is cheap; launchServer owns the only browser process.
 */
export function sharedBrowser(): Promise<Browser> {
  browser ??= chromium.connect(inject('browserWSEndpoint'));
  return browser;
}

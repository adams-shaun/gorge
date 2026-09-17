import 'vitest';

declare module 'vitest' {
  interface ProvidedContext {
    browserURL: string;
    browserWSEndpoint: string;
  }
}

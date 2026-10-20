// Bundle the official SDK locally: phones never download JavaScript from a CDN.
import { build } from 'esbuild';
await build({
  entryPoints: ['scripts/diagnostics-entry.mjs'],
  outfile: 'internal/server/web/diagnostics-sdk.js',
  bundle: true, minify: true, format: 'iife', target: 'es2020',
  define: { __SENTRY_DEBUG__: 'false', __SENTRY_TRACING__: 'false', __RRWEB_EXCLUDE_IFRAME__: 'true', __RRWEB_EXCLUDE_SHADOW_DOM__: 'true' },
  legalComments: 'eof', sourcemap: false,
});

import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { readFileSync } from 'node:fs';
import { webcrypto } from 'node:crypto';
const source = readFileSync(new URL('../web/diagnostics.js', import.meta.url), 'utf8');
function harness(offline = false, cachedConfig = null) {
  const records = [], listeners = {}, values = new Map();
  if (cachedConfig) values.set('copyapp.diagnostic-config', JSON.stringify(cachedConfig));
  const context = { crypto: webcrypto, URL, Date, Uint8Array, AbortController, setTimeout, clearTimeout,
    localStorage: { getItem: k => values.get(k), setItem: (k,v) => values.set(k,v) },
    addEventListener: (name, fn) => { listeners[name] = fn; },
    fetch: async () => { if (offline) throw new Error('network offline secret URL'); return { ok: true, json: async () => ({dsn: 'https://abc@o1.ingest.sentry.io/123', version: 'test'}) }; },
    CopyAppDiagnosticsSDK: { start: (_config, identity) => ({ emit: (...args) => records.push({ args, identity }) }) },
  };
  context.window = context;
  vm.runInNewContext(source, context);
  return { context, records, listeners };
}
const settle = () => new Promise(resolve => setImmediate(resolve));
test('only allowlisted fields cross the privacy boundary', async () => {
  const { context, records, listeners } = harness(); await settle();
  context.CopyAppDiagnostics.support('a'.repeat(32));
  context.CopyAppDiagnostics.emit('transfer', 'http_result', 'failed', {
    operation_id: 'b'.repeat(32), http_status: 403, duration_ms: 25,
    text: 'SECRET CLIPBOARD', password: 'SECRET PASSWORD', url: '/pair?t=SECRET_TOKEN',
  });
  listeners.unhandledrejection({ reason: new Error('SECRET CLIPBOARD') });
  const serialized = JSON.stringify(records);
  assert.ok(!serialized.includes('SECRET'));
  const record = records.find(r => r.args[0] === 'transfer');
  assert.equal(record.args[3].http_status, 403);
  assert.equal(record.identity.support_id, 'a'.repeat(32));
  assert.equal(record.args[3].operation_id, 'b'.repeat(32));
});
test('remote stages capped at 60 per minute including launch', async () => {
  const { context, records } = harness(); await settle();
  for (let i = 0; i < 10000; i++) context.CopyAppDiagnostics.emit('transfer', 'start', 'ok');
  assert.equal(records.length, 60);
});
test('offline config is harmless and pre-init queue remains bounded', async () => {
  const { context, records } = harness(true);
  for (let i = 0; i < 1000; i++) context.CopyAppDiagnostics.emit('transfer', 'start', 'ok');
  await settle();
  assert.equal(records.length, 0);
  assert.doesNotThrow(() => context.CopyAppDiagnostics.emit('not a fixed code', 'start', 'ok', {text:'secret'}));
});

test('previously loaded phone can report directly when the PC is unreachable', async () => {
  const { context, records } = harness(true, { dsn: 'https://abc@o1.ingest.sentry.io/123', version: 'test' });
  await settle();
  context.CopyAppDiagnostics.emit('sse', 'disconnect', 'failed');
  assert.equal(records.length, 2);
  assert.equal(records[1].args[0], 'sse');
});

// Phone transfer behavior without a browser: execute the real app functions
// with transport/DOM boundaries replaced, including streaming backpressure.
// node testdata/phone_test.mjs web/app.js
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import assert from 'node:assert/strict';

const source = readFileSync(process.argv[2], 'utf8');
const captures = [];
let destination = 'auto';
let maxBytes = 2 * 1024 ** 3;
const ctx = vm.createContext({
  FormData, Blob, File, URL, Number, Date, DOMException, AbortController,
  diag: { emit() {}, operation() { return ""; }, support() {} },
  maxBytes, pref: () => destination, toast: (...args) => captures.push({ toast: args }),
  fmtSize: (n) => n + ' bytes',
  addOutgoing: (description) => ({ description }),
  transmit: (entry, body, type) => { const result = { entry, body: body(), type }; captures.push(result); return result; },
});
function load(name) {
  const match = source.match(new RegExp('^  (?:async )?function ' + name + '\\([^]*?^  }', 'm'));
  assert.ok(match, `function ${name} exists`);
  vm.runInContext(match[0], ctx);
}
for (const name of ['sendText', 'sendFiles', 'sendClipboardImage', 'tooBig', 'whereMsgs', 'streamToDisk', 'pref', 'checkStatus', 'unpairPhone']) load(name);

const stored = new Map();
ctx.store = { get: (k) => stored.get(k) };
const preferenceBlock = source.match(/  const prefs = (\{[^]*?^  });/m);
assert.ok(preferenceBlock);
ctx.prefs = vm.runInContext('(' + preferenceBlock[1] + ')', ctx);
assert.equal(ctx.pref('dest'), 'auto');
assert.equal(ctx.pref('rxText'), 'show');
assert.equal(ctx.pref('rxFile'), 'show');
stored.set('copyapp.dest', 'bad-value');
assert.equal(ctx.pref('dest'), 'auto', 'ignore invalid saved preference');
stored.set('copyapp.dest', 'file');
stored.set('copyapp.rxText', 'copy');
stored.set('copyapp.rxFile', 'download');
assert.equal(ctx.pref('dest'), 'file');
assert.equal(ctx.pref('rxText'), 'copy');
assert.equal(ctx.pref('rxFile'), 'download');
ctx.pref = () => destination;

ctx.showPcMode = () => {};
ctx.fetch = async () => ({ ok: true, json: async () => ({ maxBytes: 1234, receiveMode: 'folder' }) });
assert.equal(await ctx.checkStatus(), 'ok');
assert.equal(ctx.maxBytes, 1234, 'use the configured PC limit');
assert.equal(ctx.receiveMode, 'folder');
ctx.fetch = async () => ({ ok: true, json: async () => ({ maxBytes: -5 }) });
await ctx.checkStatus();
assert.equal(ctx.maxBytes, 1234, 'ignore invalid server limits');
ctx.fetch = async () => ({ status: 401 });
assert.equal(await ctx.checkStatus(), 'unpaired');
ctx.fetch = async () => ({ ok: false, status: 500 });
assert.equal(await ctx.checkStatus(), 'down');
ctx.maxBytes = maxBytes;

// Every destination works for both text and multipart. mode must precede any
// file, because the server consumes multipart incrementally.
for (destination of ['auto', 'clipboard', 'file']) {
  const text = JSON.parse(ctx.sendText('שלום').body);
  assert.equal(text.mode, destination);
  assert.equal(text.text, 'שלום');
  const file = ctx.sendFiles([new File(['sample'], 'note.txt', { type: 'text/plain' })]);
  assert.deepEqual([...file.body.keys()], ['mode', 'files']);
  assert.equal(file.body.get('mode'), destination);
  const image = ctx.sendClipboardImage(new Blob(['png'], { type: 'image/png' }));
  assert.deepEqual([...image.body.keys()], ['mode', 'file']);
  assert.equal(image.body.get('mode'), destination);
}

// The PC's dynamic limit is enforced against the entire selected transfer,
// without reading any file payload just to calculate its size.
ctx.maxBytes = 10;
const before = captures.length;
ctx.sendFiles([{ size: 6 }, { size: 5 }]);
assert.equal(captures.length, before + 1);
assert.equal(captures.at(-1).toast[1][0], 'err.too_large.t');
assert.equal(captures.at(-1).toast[2][1].limit, '10 bytes');
ctx.sendClipboardImage({ size: 11 });
assert.equal(captures.at(-1).toast[1][0], 'err.too_large.t');
ctx.maxBytes = maxBytes;
ctx.sendFiles([{ size: maxBytes + 1 }]);
assert.equal(captures.at(-1).toast[1][0], 'err.too_large.t');
const count = captures.length;
ctx.sendFiles([]);
assert.equal(captures.length, count, 'empty selection makes no request');

// Responses explain the actual destination chosen by the PC, including a
// custom folder, rather than assuming Downloads for every file.
assert.equal(ctx.whereMsgs('text', 'clipboard', [])[0][0], 'where.clip');
assert.equal(ctx.whereMsgs('image', 'clipboard', [])[0][0], 'where.clipImg');
assert.equal(ctx.whereMsgs('text', 'folder', ['text.txt'])[0][0], 'where.saved');
assert.equal(ctx.whereMsgs('file', 'folder', ['a', 'b'])[0][0], 'where.savedMore');
assert.equal(ctx.whereMsgs('file', 'folder', ['a'])[1][0], 'where.folder');

// A big download writes each chunk before reading another. It never calls
// response.blob()/arrayBuffer(), and cancellation leaves the file uncommitted.
const actions = [];
const chunks = [new Uint8Array([1, 2]), new Uint8Array([3, 4, 5])];
const reader = {
  async read() { actions.push('read'); return chunks.length ? { value: chunks.shift(), done: false } : { done: true }; },
  async cancel() { actions.push('cancel'); },
  releaseLock() { actions.push('release'); },
};
const response = { headers: new Headers({ 'Content-Length': '5' }), body: { getReader: () => reader } };
const writer = { async write(bytes) { actions.push('write:' + bytes.byteLength); }, async close() { actions.push('close'); } };
const progress = [];
await ctx.streamToDisk(response, writer, (loaded, total) => progress.push([loaded, total]), new AbortController().signal);
assert.deepEqual(progress, [[2, 5], [5, 5]]);
assert.deepEqual(actions, ['read', 'write:2', 'read', 'write:3', 'read', 'close', 'cancel', 'release']);
actions.length = 0;
const aborted = new AbortController(); aborted.abort();
await assert.rejects(ctx.streamToDisk(response, writer, () => {}, aborted.signal), { name: 'AbortError' });
assert.deepEqual(actions, ['cancel', 'release']);

// Auto actions wait for the server's replay boundary, never a timing guess.
assert.match(source, /addEventListener\('ready', \(\) => \{ initialSyncDone = true;/);
assert.match(source, /if \(initialSyncDone\) autoHandle\(item, node\)/);
assert.doesNotMatch(source, /firstSyncAt/);

// An offline/failed disconnect cannot clear the HttpOnly session cookie.
// Keep the current phone state and show failure until the PC acknowledges it.
let unpaired = 0;
ctx.$ = () => ({ close() {} });
ctx.goUnpaired = () => { unpaired++; };
ctx.errorText = (code) => [[code], ['detail']];
ctx.fetch = async () => { throw new Error('offline'); };
assert.equal(await ctx.unpairPhone(), false);
assert.equal(unpaired, 0);
assert.equal(captures.at(-1).toast[0], 'err');
ctx.fetch = async () => ({ ok: false, status: 500 });
assert.equal(await ctx.unpairPhone(), false);
assert.equal(unpaired, 0);
ctx.fetch = async () => ({ ok: true, status: 204 });
assert.equal(await ctx.unpairPhone(), true);
assert.equal(unpaired, 1);
ctx.fetch = async () => ({ ok: false, status: 500, json: async () => ({ code: 'session_save' }) });
assert.equal(await ctx.unpairPhone(), false);
assert.equal(unpaired, 2, 'persistent revocation failure disconnects current phone UI');
assert.equal(captures.at(-1).toast[1][0], 'session_save');
console.log('phone transfer behavior ok');

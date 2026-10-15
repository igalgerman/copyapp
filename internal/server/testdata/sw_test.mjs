// Unit check for web/sw.js, run by TestServiceWorker (go test) when node is
// installed: node sw_test.mjs path/to/sw.js
// Runs the worker in a vm with an in-memory Cache API and fake events.
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import assert from 'node:assert/strict';

const stores = new Map();
const keyOf = (k) => (typeof k === 'string' ? k : new URL(k.url).pathname);
function cache(name) {
  if (!stores.has(name)) stores.set(name, new Map());
  const m = stores.get(name);
  return {
    put: async (k, r) => { m.set(keyOf(k), r); },
    match: async (k) => m.get(keyOf(k)),
    addAll: async () => {},
  };
}
const caches = {
  open: async (n) => cache(n),
  delete: async (n) => stores.delete(n),
  keys: async () => [...stores.keys()],
  match: async () => undefined,
};
const handlers = {};
const origin = 'https://192.168.1.5:8443';
let network = async () => { throw new Error('offline'); };
const self = {
  location: new URL(origin + '/sw.js'),
  addEventListener: (t, f) => { handlers[t] = f; },
  skipWaiting: async () => {},
  clients: { claim: async () => {} },
};
vm.runInContext(readFileSync(process.argv[2], 'utf8'), vm.createContext({
  self, caches, Response, Request, URL, Set, Promise, Date, JSON,
  fetch: (...a) => network(...a),
}));

function fire(type, request) {
  const ev = { request, responded: null, waits: [] };
  ev.respondWith = (p) => { ev.responded = p; };
  ev.waitUntil = (p) => { ev.waits.push(p); };
  handlers[type](ev);
  return ev;
}

// 1. A share POST is stashed and answered with 303 /?share=1.
const fd = new FormData();
fd.append('title', 'T');
fd.append('text', 'hello');
fd.append('url', 'https://example.com/x');
fd.append('files', new File(['abc'], 'a.txt', { type: 'text/plain' }));
fd.append('files', new File([new Uint8Array([1, 2, 3])], 'b.bin'));
let ev = fire('fetch', new Request(origin + '/share', { method: 'POST', body: fd }));
assert.ok(ev.responded, 'share POST must be intercepted');
let res = await ev.responded;
assert.equal(res.status, 303);
assert.equal(res.headers.get('Location'), origin + '/?share=1');
const share = stores.get('copyapp-share-v1');
const meta = await share.get('/__share/meta').json();
assert.equal(meta.text, 'hello');
assert.equal(meta.title, 'T');
assert.equal(meta.url, 'https://example.com/x');
assert.equal(meta.files.length, 2);
assert.equal(meta.files[0].name, 'a.txt');
assert.equal(await share.get(meta.files[0].key).text(), 'abc');
assert.deepEqual([...new Uint8Array(await share.get(meta.files[1].key).arrayBuffer())], [1, 2, 3]);

// 2. A second share replaces the first.
const fd2 = new FormData();
fd2.append('text', 'second');
await fire('fetch', new Request(origin + '/share', { method: 'POST', body: fd2 })).responded;
const meta2 = await stores.get('copyapp-share-v1').get('/__share/meta').json();
assert.equal(meta2.text, 'second');
assert.equal(meta2.files.length, 0);
assert.equal(stores.get('copyapp-share-v1').size, 1, 'old files removed');

// 3. A broken body redirects with an error instead of failing the navigation.
ev = fire('fetch', new Request(origin + '/share', { method: 'POST', body: 'x', headers: { 'Content-Type': 'multipart/form-data; boundary=zz' } }));
res = await ev.responded;
assert.equal(res.headers.get('Location'), origin + '/?error=share');

// 4. Other POSTs (/send, /unpair) and other origins go to the network untouched.
assert.equal(fire('fetch', new Request(origin + '/send', { method: 'POST', body: 'x' })).responded, null);
assert.equal(fire('fetch', new Request('https://evil.example/share', { method: 'POST', body: 'x' })).responded, null);
assert.equal(fire('fetch', new Request(origin + '/events')).responded, null);

// 5. Shell responses are cached under e.waitUntil.
network = async () => ({ ok: true, type: 'basic', clone() { return 'copy'; } });
ev = fire('fetch', new Request(origin + '/app.js'));
await ev.responded;
assert.equal(ev.waits.length, 1, 'cache.put must be inside e.waitUntil');
await Promise.all(ev.waits);
assert.equal(stores.get('copyapp-shell-v5').get('/app.js'), 'copy');

// 6. Activating a new worker keeps a pending share.
stores.set('copyapp-shell-v0', new Map());
ev = fire('activate');
await Promise.all(ev.waits);
assert.ok(stores.has('copyapp-share-v1'), 'activate must not drop the share stash');
assert.ok(!stores.has('copyapp-shell-v0'), 'old shell caches are removed');

console.log('sw.js ok');

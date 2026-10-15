// CopyApp service worker: caches the app shell and receives Android shares.
// It never touches /events, /item/*, /send, /status, /pair, /ca.crt or /unpair:
// those requests go straight to the network (no respondWith).
const CACHE = 'copyapp-shell-v3'; // bump when the shell changes (v3: browser-tab first, optional app)
// Shared items wait here until the page sends them (see app.js sendShared).
const SHARE_CACHE = 'copyapp-share-v1';
const SHELL = [
  '/',
  '/index.html',
  '/i18n.js',
  '/app.js',
  '/style.css',
  '/manifest.json',
  '/icon.svg',
  '/icon-192.png',
  '/icon-512.png',
  '/icon-maskable-512.png',
  '/bricolage.woff2',
];
const SHELL_SET = new Set(SHELL);

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE && k !== SHARE_CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

// The share target (manifest "action": "/share") is a POST navigation that
// Android may start without the SameSite=Strict session cookie. Instead of
// posting to the PC, stash the shared data and open the app, which then
// sends it with a normal same-origin request (cookie included).
async function stashShare(req) {
  const back = (q) => Response.redirect(new URL('/?' + q, self.location.origin).href, 303);
  try {
    const fd = await req.formData();
    await caches.delete(SHARE_CACHE); // one share at a time
    const cache = await caches.open(SHARE_CACHE);
    const str = (k) => (typeof fd.get(k) === 'string' ? fd.get(k) : '');
    const meta = { at: Date.now(), title: str('title'), text: str('text'), url: str('url'), files: [] };
    let i = 0;
    for (const f of fd.getAll('files')) {
      if (typeof f === 'string') continue;
      const key = '/__share/file/' + i++;
      await cache.put(key, new Response(f, { headers: { 'Content-Type': f.type || 'application/octet-stream' } }));
      meta.files.push({ key, name: f.name, type: f.type });
    }
    await cache.put('/__share/meta', new Response(JSON.stringify(meta), { headers: { 'Content-Type': 'application/json' } }));
    return back('share=1');
  } catch {
    return back('error=share');
  }
}

self.addEventListener('fetch', (e) => {
  const req = e.request;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (req.method === 'POST' && url.pathname === '/share') {
    e.respondWith(stashShare(req));
    return;
  }
  if (req.method !== 'GET') return;
  // Only the shell. Query strings (/?sent=..., /?error=...) map to the page itself.
  const path = url.pathname;
  if (!SHELL_SET.has(path)) return;
  const key = path === '/index.html' ? '/' : path;
  // Network first so a new copyapp.exe updates the app; cache when the PC is unreachable.
  e.respondWith(
    fetch(req, { cache: 'no-cache' })
      .then((res) => {
        if (res.ok && res.type === 'basic') {
          const copy = res.clone();
          e.waitUntil(caches.open(CACHE).then((c) => c.put(key, copy)));
        }
        return res;
      })
      .catch(() => caches.match(key).then((hit) => hit || Response.error()))
  );
});

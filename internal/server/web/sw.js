// CopyApp service worker: caches the app shell only.
// It never touches /events, /item/*, /send, /status, /pair, /ca.crt or /unpair:
// those requests go straight to the network (no respondWith).
const CACHE = 'copyapp-shell-v1';
const SHELL = [
  '/',
  '/index.html',
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
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
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
          caches.open(CACHE).then((c) => c.put(key, copy));
        }
        return res;
      })
      .catch(() => caches.match(key).then((hit) => hit || Response.error()))
  );
});

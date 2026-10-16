// Real-browser integration against an already-running ISOLATED CopyApp.
// Parent owns process lifetime. This script never launches/stops the exe.
// Required: COPYAPP_DATA_DIR, COPYAPP_RECEIVE_DIR; NODE_PATH must expose
// playwright, pngjs and jsqr (the recording harness's modules are suitable).
// Run: node docs/e2e/browser.js. Changes isolated settings/clipboard and files.
'use strict';
const fs = require('fs');
const path = require('path');
const os = require('os');
const assert = require('node:assert/strict');
const Module = require('module');
process.env.NODE_PATH = [process.env.NODE_PATH,
  path.join(os.tmpdir(), 'copyapp-video', 'node_modules'),
  path.join(os.tmpdir(), 'copyapp-video-node', 'node_modules')].filter(Boolean).join(path.delimiter);
Module._initPaths();
const { chromium, request } = require('playwright');
const { PNG } = require('pngjs');
const jsQR = require('jsqr');
const dataDir = process.env.COPYAPP_DATA_DIR;
const receiveDir = process.env.COPYAPP_RECEIVE_DIR;
assert(dataDir && receiveDir, 'COPYAPP_DATA_DIR and COPYAPP_RECEIVE_DIR must identify the isolated running app');
const localOrigin = 'http://127.0.0.1:' + fs.readFileSync(path.join(dataDir, 'local-port.txt'), 'utf8').trim();
const key = fs.readFileSync(path.join(dataDir, 'local.key'), 'utf8').trim();
const failures = [];
const observed = [];
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'copyapp-browser-'));
let native, browser, initial;
function pass(message) { console.log('PASS ' + message); }
async function eventually(fn, message) {
  const end = Date.now() + 15000;
  while (Date.now() < end) {
    if (await fn()) return;
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error(message);
}
async function state() {
  const r = await native.get('/local/state');
  assert.equal(r.status(), 200, 'native control authorization');
  return r.json();
}
function watch(page, name) {
  page.on('pageerror', error => failures.push(name + ': ' + error.message));
  page.on('console', message => {
    if (message.type() !== 'error') return;
    // The PC can finish a QR fetch after pairing closes; only that route's
    // expected 410 is harmless (the paired UI hides the QR at that point).
    if (name === 'PC' && /Failed to load resource.*410/.test(message.text())) {
      try { if (new URL(message.location().url).pathname === '/qr') return; } catch { /* retain error */ }
    }
    // Deliberate failed-unpair test and expected unauthorized startup checks.
    if (/Failed to load resource.*(?:401|500|ERR_FAILED)/.test(message.text())) return;
    failures.push(name + ' console: ' + message.text());
  });
  page.on('response', response => {
    const url = new URL(response.url());
    observed.push({ name, path: url.pathname, status: response.status() });
    if (response.status() >= 400 && !(url.pathname === '/status' && response.status() === 401) &&
        !(url.pathname === '/unpair' && response.status() === 500) &&
        !(name === 'PC' && url.pathname === '/qr' && response.status() === 410)) {
      failures.push(name + ' HTTP ' + response.status() + ' ' + url.pathname);
    }
  });
  page.on('requestfailed', req => {
    const pathname = new URL(req.url()).pathname;
    // Navigating/closing a page intentionally ends long-lived SSE requests.
    if (pathname !== '/events' && !/ERR_ABORTED/.test(req.failure()?.errorText || '')) {
      failures.push(name + ' network ' + pathname + ': ' + req.failure()?.errorText);
    }
  });
}
async function layout(page, label, phone) {
  await page.evaluate(() => document.fonts.ready);
  const bad = await page.evaluate(isPhone => {
    const issues = [];
    if (document.documentElement.scrollWidth > innerWidth + 2) issues.push('horizontal overflow');
    for (const n of document.querySelectorAll('[data-i18n], [data-i18n-html], [data-k]')) {
      if (!n.getClientRects().length) continue;
      const key = n.dataset.i18n || n.dataset.i18nHtml || n.dataset.k;
      if (!n.textContent.trim() || n.textContent.trim() === key) issues.push('missing text: ' + key);
      if (isPhone && !I18N.has(key)) issues.push('missing translation: ' + key);
    }
    for (const n of document.images) {
      if (n.getClientRects().length && n.src && n.complete && !n.naturalWidth) issues.push('broken image');
    }
    return issues;
  }, phone);
  assert.deepEqual(bad, [], label);
  pass(label);
}
async function language(page, lang, phone) {
  await page.locator((phone ? '.view-main ' : '#langs ') + '[data-lang="' + lang + '"]').click();
  await eventually(() => page.locator('html').getAttribute('lang').then(v => v === lang), 'language did not change');
  if (!phone) await eventually(async () => (await state()).settings.language === lang, 'PC language was not saved');
  assert.equal(await page.locator('html').getAttribute('dir'), lang === 'he' ? 'rtl' : 'ltr');
  await layout(page, (phone ? 'phone' : 'PC') + ' ' + lang + ' translations/layout', phone);
}
async function main() {
  native = await request.newContext({ baseURL: localOrigin, extraHTTPHeaders: { 'X-CopyApp': key }, timeout: 15000 });
  initial = await state();
  assert.equal(initial.settings.language, 'he', 'use fresh isolated preferences to verify Hebrew default');
  assert.equal(path.resolve(initial.settings.saveDir).toLowerCase(), path.resolve(receiveDir).toLowerCase());
  const baseURL = initial.address;
  assert(/^https:\/\//.test(baseURL), 'phone origin comes from app state');
  // Test-only: context.ignoreHTTPSErrors covers page fetches but Chromium's
  // separate native downloader may still reject the isolated app's private CA.
  // Match the explicit certificate bypass used by the normal browser-tab flow.
  browser = await chromium.launch({ headless: true, args: ['--ignore-certificate-errors'] });
  const pcContext = await browser.newContext({ viewport: { width: 1280, height: 900 }, acceptDownloads: true });
  const unauth = await pcContext.request.get(localOrigin + '/local/state');
  assert.equal(unauth.status(), 403, 'bare browser must not access local state');
  const pc = await pcContext.newPage(); watch(pc, 'PC');
  await pc.goto(localOrigin + '/pairing?k=' + encodeURIComponent(key));
  await eventually(() => pc.evaluate(() => !!sessionStorage.getItem('copyapp.localSession')), 'bootstrap token missing');
  assert.equal(new URL(pc.url()).search, '', 'bootstrap secret removed from address bar');
  await eventually(() => pc.locator('#logPath').textContent().then(v => v.includes('copyapp.log')), 'control page state missing');
  await layout(pc, 'PC initial Hebrew desktop', false);
  await pc.reload();
  await eventually(() => pc.locator('#logPath').textContent().then(v => v.includes('copyapp.log')), 'reload lost control authorization');
  assert.equal(await pc.locator('body').evaluate(n => n.classList.contains('offline')), false);
  pass('PC bootstrap, secret stripping, sessionStorage reload and native/browser auth separation');
  if (!(await state()).pairing.open) {
    const r = await native.post('/pairing/open', { headers: { Origin: localOrigin } });
    assert.equal(r.status(), 204);
  }
  const qr = await native.get('/qr'); assert.equal(qr.status(), 200);
  const png = PNG.sync.read(await qr.body());
  const decoded = jsQR(new Uint8ClampedArray(png.data), png.width, png.height);
  assert(decoded && decoded.data.startsWith(baseURL + '/pair?t='), 'QR must encode advertised phone origin');
  const phoneContext = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true,
    deviceScaleFactor: 1, ignoreHTTPSErrors: true, acceptDownloads: true, serviceWorkers: 'block' });
  const phone = await phoneContext.newPage(); watch(phone, 'phone');
  await phone.goto(decoded.data);
  await phone.waitForSelector('body[data-view="main"]');
  await phone.waitForSelector('#conn[data-state="online"]');
  assert.equal(await phone.locator('html').getAttribute('lang'), 'he');
  pass('normal decoded QR navigation pairs phone and opens real SSE stream');
  for (const lang of ['he', 'en', 'ru']) {
    await language(pc, lang, false);
    await language(phone, lang, true);
  }
  await language(pc, 'en', false); await language(phone, 'en', true);
  await pc.setViewportSize({ width: 390, height: 844 });
  await layout(pc, 'PC control at 390px', false);
  await pc.setViewportSize({ width: 1280, height: 900 });
  await phone.setViewportSize({ width: 1280, height: 900 });
  await layout(phone, 'phone PWA desktop layout', true);
  await phone.setViewportSize({ width: 390, height: 844 });
  async function textSend(text, destination) {
    await phone.locator('[data-open="sheet-paste"]').click();
    await phone.locator('#paste-text').fill(text);
    await phone.locator('input[name="dest-paste"][value="' + destination + '"]').locator('..').click();
    const response = phone.waitForResponse(r => new URL(r.url()).pathname === '/send' && r.request().method() === 'POST');
    await phone.locator('#paste-form button[type="submit"]').click();
    const r = await response; assert.equal(r.status(), 200);
    const result = await r.json(); assert.equal(result.ok, true);
    return result;
  }
  const marker = 'browser clipboard ' + Date.now() + ' שלום Привет';
  assert.equal((await textSend(marker, 'clipboard')).where, 'clipboard');
  await pc.locator('#sendNow').click();
  await eventually(() => phone.locator('.card-text').allTextContents().then(t => t.includes(marker)), 'PC clipboard round trip missing');
  pass('phone text -> actual Windows clipboard -> PC send button -> phone SSE DOM');
  await pc.locator('input[name="mode"][value="folder"]').check();
  await eventually(async () => (await state()).settings.receiveMode === 'folder', 'PC receive mode not saved');
  const folderText = 'browser folder ' + Date.now();
  const saved = await textSend(folderText, 'auto');
  assert.equal(saved.where, 'folder');
  assert.equal(fs.readFileSync(path.join(receiveDir, saved.saved[0]), 'utf8'), folderText);
  assert.equal((await textSend('explicit override ' + Date.now(), 'clipboard')).where, 'clipboard');
  pass('real PC default folder, Auto destination and explicit clipboard override');
  await phone.locator('input[name="dest-dock"][value="file"]').locator('..').click();
  const payload = Buffer.alloc(16 * 1024 * 1024, 0x53);
  const filename = 'browser-upload-' + Date.now() + '.bin';
  const uploadResponse = phone.waitForResponse(r => new URL(r.url()).pathname === '/send');
  await phone.locator('#pick-file').setInputFiles({ name: filename, mimeType: 'application/octet-stream', buffer: payload });
  const uploaded = await uploadResponse; assert.equal(uploaded.status(), 200);
  const uploadResult = await uploaded.json();
  assert.deepEqual(fs.readFileSync(path.join(receiveDir, uploadResult.saved[0])), payload);
  pass('16 MiB real DOM file upload and byte-identical disk output');
  const outgoing = path.join(scratch, 'browser-download.bin'); fs.writeFileSync(outgoing, payload.subarray(0, 65536));
  const publish = await native.post('/local/send-file?path=' + encodeURIComponent(outgoing));
  assert.equal(publish.status(), 204);
  const link = phone.locator('a[download="browser-download.bin"]'); await link.waitFor();
  // Android uses ordinary download links; desktop Chrome's save picker is optional.
  // Disable only that optional capability to exercise the real native link path.
  await phone.evaluate(() => { window.showSaveFilePicker = undefined; });
  const downloadPromise = phone.waitForEvent('download'); await link.click();
  const download = await downloadPromise;
  const downloadFailure = await download.failure();
  if (downloadFailure) {
    // Distinguish a browser downloader refusal from an endpoint/auth failure.
    const probe = await phoneContext.request.get(download.url());
    const probeBytes = await probe.body();
    throw new Error('Native download failed (' + downloadFailure + '); authenticated endpoint probe: HTTP ' +
      probe.status() + ', bytes=' + probeBytes.length + ', expected=' + Math.min(payload.length, 65536) +
      ', matching=' + probeBytes.equals(payload.subarray(0, 65536)));
  }
  assert.deepEqual(fs.readFileSync(await download.path()), payload.subarray(0, 65536));
  pass('PC file publish, SSE card, native browser download and matching bytes');
  await pc.locator('#maxSel').selectOption(String(200 << 20));
  await eventually(async () => (await state()).settings.maxBytes === (200 << 20), 'size setting not saved');
  // Sparse fixture: metadata exceeds the real minimum limit, no 200MiB payload allocation.
  const oversized = path.join(scratch, 'too-large.bin');
  const fd = fs.openSync(oversized, 'w'); fs.ftruncateSync(fd, (200 << 20) + 1); fs.closeSync(fd);
  const freshStatus = phone.waitForResponse(r => new URL(r.url()).pathname === '/status');
  await phone.locator('[data-open="sheet-settings"]').click();
  await freshStatus;
  await phone.locator('#sheet-settings [data-close]').click();
  let sends = 0; const countSend = req => { if (new URL(req.url()).pathname === '/send') sends++; };
  phone.on('request', countSend);
  await phone.locator('#pick-file').setInputFiles(oversized);
  await eventually(() => phone.locator('.toast-err').count().then(n => n > 0), 'oversize rejection toast missing');
  assert.equal(sends, 0, 'frontend rejects oversize before upload'); phone.off('request', countSend);
  pass('PC size setting propagates and frontend rejects oversized file before network');
  async function clickUnpair() {
    await phone.locator('[data-open="sheet-settings"]').click();
    await phone.locator('#unpair-btn').click();
    const response = phone.waitForResponse(r => new URL(r.url()).pathname === '/unpair');
    await phone.locator('#confirm-unpair').click();
    return response;
  }
  await phone.route('**/unpair', route => route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"test failure"}' }));
  assert.equal((await clickUnpair()).status(), 500);
  await phone.waitForSelector('.toast-err');
  assert.equal(await phone.locator('body').getAttribute('data-view'), 'main');
  await phone.reload(); await phone.waitForSelector('body[data-view="main"]');
  assert.equal((await state()).phone.paired, true);
  pass('failed unpair keeps paired UI and reload retains actual server session');
  await phone.unroute('**/unpair');
  assert.equal((await clickUnpair()).status(), 204); await phone.waitForSelector('body[data-view="pair"]');
  assert.equal((await state()).phone.paired, false);
  await phone.reload(); await phone.waitForSelector('body[data-view="pair"]');
  pass('successful unpair revokes server session and remains disconnected on reload');
  assert.deepEqual(failures, [], 'browser runtime/console/network errors');
  assert(observed.some(r => r.name === 'phone' && r.path === '/events' && r.status === 200));
  pass('all observed real browser requests/runtime checks');
}
main().catch(error => { console.error(error.stack || error); process.exitCode = 1; }).finally(async () => {
  if (native && initial) {
    try {
      const { language, receiveMode, maxBytes } = initial.settings;
      const r = await native.post('/local/settings', { data: { language, receiveMode, maxBytes } });
      if (!r.ok()) throw new Error('settings restore HTTP ' + r.status());
    } catch (error) { console.error('Settings restore failed: ' + error.message); process.exitCode = 1; }
  }
  if (browser) await browser.close();
  if (native) await native.dispose();
  fs.rmSync(scratch, { recursive: true, force: true });
});

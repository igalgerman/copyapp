// Unit check for web/i18n.js, run by TestI18nJS (go test) when node is
// installed: node i18n_test.mjs web/i18n.js web/index.html web/app.js
// Same key sets in he/ru/en, same {placeholders}, plural forms for every
// category the language has, and every key the page and app.js use exists.
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import assert from 'node:assert/strict';

const [src, htmlPath, appPath] = process.argv.slice(2);
const ctx = vm.createContext({});
vm.runInContext(readFileSync(src, 'utf8'), ctx);
const I = ctx.I18N;
const { STR, LANGS } = I;
const strip = (s) => s.replace(/[⁨⁩]/g, '');

assert.deepEqual([...LANGS], ['he', 'ru', 'en']);
assert.equal(I.DEFAULT, 'he');
assert.equal(I.lang, 'he', 'Hebrew is the default');
assert.deepEqual(Object.keys(STR).sort(), ['en', 'he', 'ru']);

const holes = (v) => {
  const all = typeof v === 'string' ? [v] : Object.values(v);
  const set = new Set();
  for (const s of all) for (const m of s.matchAll(/\{(\w+)\}/g)) set.add(m[1]);
  set.delete('n'); // a plural form may spell the number out ("שני קבצים")
  return [...set].sort();
};
const tags = (v) => (typeof v === 'string' ? [v] : Object.values(v)).join(' ').match(/<\/?[a-z]+/g) || [];

const keys = Object.keys(STR.en).sort();
for (const l of LANGS) {
  assert.deepEqual(Object.keys(STR[l]).sort(), keys, `${l}: key set differs from en`);
  const cats = new Intl.PluralRules(l).resolvedOptions().pluralCategories;
  for (const k of keys) {
    const v = STR[l][k];
    const en = STR.en[k];
    assert.equal(typeof v, typeof en, `${l}.${k}: plural vs plain mismatch`);
    assert.deepEqual(holes(v), holes(en), `${l}.${k}: placeholders differ`);
    for (const s of typeof v === 'string' ? [v] : Object.values(v)) {
      assert.ok(s.trim(), `${l}.${k}: empty`);
      assert.ok(!/<script|\son\w+=|javascript:/i.test(s), `${l}.${k}: unsafe markup`);
    }
    const allowed = new Set(['<b', '</b', '<bdi', '</bdi', '<kbd', '</kbd']);
    for (const tg of tags(v)) assert.ok(allowed.has(tg), `${l}.${k}: tag ${tg} not allowed`);
    if (typeof v === 'object') {
      for (const c of cats) assert.ok(c in v, `${l}.${k}: missing plural form "${c}"`);
      for (const c of Object.keys(v)) assert.ok(cats.includes(c), `${l}.${k}: "${c}" is not a ${l} plural category`);
    }
  }
}

// Plurals and interpolation.
const t = (k, v, l) => strip(I.t(k, v, l));
assert.equal(t('done.files', { n: 1 }, 'en'), '1 file in Downloads › CopyApp');
assert.equal(t('done.files', { n: 3 }, 'en'), '3 files in Downloads › CopyApp');
assert.match(t('done.files', { n: 1 }, 'ru'), /^1 файл /);
assert.match(t('done.files', { n: 3 }, 'ru'), /^3 файла /);
assert.match(t('done.files', { n: 5 }, 'ru'), /^5 файлов /);
assert.match(t('done.files', { n: 21 }, 'ru'), /^21 файл /);
assert.match(t('done.files', { n: 1 }, 'he'), /^קובץ אחד/);
assert.match(t('done.files', { n: 2 }, 'he'), /^שני קבצים/);
assert.match(t('done.files', { n: 7 }, 'he'), /^7 קבצים/);
assert.equal(t('out.more', { name: 'a.txt', n: 2 }, 'he'), 'a.txt ועוד שני קבצים');
// Values are isolated so file names keep their direction in RTL text.
assert.equal(I.t('toast.downloading', { name: 'x.pdf' }, 'he'), 'מוריד את ⁨x.pdf⁩');
assert.equal(I.t('no.such.key'), 'no.such.key');

// Sizes and relative times.
assert.equal(strip(I.size(1536 * 1024, 'en')), '1.5\u00a0MB'); // no-break space: never wraps between number and unit
assert.equal(strip(I.size(1536 * 1024, 'ru')), '1,5\u00a0МБ');
assert.equal(I.size(10, 'he'), '⁨10\u00a0B⁩');
const now = Date.UTC(2026, 0, 1, 12, 0, 0);
assert.equal(I.relTime(now - 5000, now, 'en'), 'just now');
assert.equal(I.relTime(now - 5 * 60000, now, 'en'), '5 minutes ago');
assert.equal(I.relTime(now - 5 * 60000, now, 'ru'), '5 минут назад');
assert.equal(I.relTime(now - 5 * 60000, now, 'he'), 'לפני 5 דקות');

// Every key the page and the app reference exists.
const html = readFileSync(htmlPath, 'utf8');
const used = new Set();
for (const m of html.matchAll(/data-i18n(?:-html)?="([^"]+)"/g)) used.add(m[1]);
for (const m of html.matchAll(/data-i18n-attr="([^"]+)"/g)) for (const p of m[1].split(';')) used.add(p.split(':')[1]);
const app = readFileSync(appPath, 'utf8');
for (const m of app.matchAll(/\[\s*'([a-z][\w.]*)'/g)) used.add(m[1]); // ['key', vars] messages
for (const m of app.matchAll(/\bT\('([\w.]+)'/g)) used.add(m[1]);
for (const m of app.matchAll(/\?\s*'([a-z]+\.[\w.]+)'\s*:\s*'([a-z]+\.[\w.]+)'/g)) { used.add(m[1]); used.add(m[2]); }
for (const k of used) if (!k.startsWith('err.')) assert.ok(k in STR.en, `used but not defined: ${k}`);
assert.ok(used.size > 60, `expected many keys in use, found ${used.size}`);
for (const code of ['empty', 'not_png', 'too_large', 'too_many_pixels', 'unauthorized', 'unsupported', 'bad_request', 'bad_text', 'clipboard', 'save', 'network', 'share', 'copy']) {
  assert.ok(I.has(`err.${code}.t`) && I.has(`err.${code}.d`), `error ${code}`);
}

// Browser-tab first: no certificate step in the main flow; the app upgrade is optional (Settings + one tip).
assert.ok(!/setup-card|cert-teaser/.test(html), 'the certificate must not be a main onboarding step');
for (const id of ['tip-card', 'tip-dismiss', 'verify-row', 'app-row', 'sheet-cert', 'cert-download']) assert.ok(html.includes(`id="${id}"`), `index.html lacks #${id}`);
assert.ok(/id="app-row"[^>]*data-open="sheet-cert"/.test(html), 'Settings "Make it a real app" opens the certificate sheet');
assert.ok(/id="verify-row"[\s\S]{0,400}data-fp-short/.test(html), 'Verify connection shows the short certificate code');
for (const k of ['setup.title', 'pair.certB', 'settings.cert']) assert.ok(!(k in STR.en), `stale key ${k}`);
assert.match(STR.en['pair.s3'], /Proceed/, 'pairing steps explain Advanced > Proceed');
assert.ok(html.includes('id="banner-help"') && /banner-help[^>]*hidden/.test(html), 'reconnect banner has a hidden Wi-Fi help line');
for (const l of LANGS) assert.match(STR[l]['banner.help'], /Wi‑Fi/, `${l}.banner.help mentions Wi-Fi`);
for (const l of LANGS) assert.ok(!/mobile|סלולר|мобильн/i.test(STR[l]['banner.help']), `${l}.banner.help must not ask to turn mobile data off`);
assert.ok(/HELP_AFTER = 10000/.test(app) && /bannerHelp\.hidden = !\(offlineSince/.test(app), 'help line appears after 10 s offline');
assert.ok(/writeText[\s\S]*legacyCopy/.test(app) && /isSecureContext/.test(app), 'clipboard fallbacks');

console.log(`i18n.js ok (${keys.length} keys, ${used.size} used)`);

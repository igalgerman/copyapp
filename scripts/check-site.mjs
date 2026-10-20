// Offline checks for the site's language and release-availability contract.
// Published asset existence is checked separately against GitHub before release.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = new URL('../', import.meta.url);
const html = readFileSync(new URL('site/index.html', root), 'utf8');
const source = readFileSync(new URL('site/app.js', root), 'utf8');
const begin = source.indexOf('var DICT = ');
const end = source.indexOf('/* Strings used only', begin);
assert(begin >= 0 && end > begin, 'Cannot locate website translation dictionary');
const literal = source.slice(begin + 'var DICT = '.length, end).trim().replace(/;\s*$/, '');
const dictionaries = vm.runInNewContext(`(${literal})`, {}, { timeout: 1000 });

const ids = [...html.matchAll(/\bid="([^"]+)"/g)].map(match => match[1]);
assert.equal(new Set(ids).size, ids.length, 'Duplicate HTML id');
const keys = new Set([...html.matchAll(/\bdata-i18n(?:-aria|-content|-svg)?="([^"]+)"/g)].map(match => match[1]));
assert(keys.size > 100, 'Website translation scan unexpectedly incomplete');
for (const lang of ['en', 'ru']) {
  for (const key of keys) {
    assert.equal(typeof dictionaries[lang]?.[key], 'string', `${lang}: missing ${key}`);
    assert(dictionaries[lang][key].trim(), `${lang}: empty ${key}`);
  }
  assert.match(dictionaries[lang].browserBadge, /1\.0\.2/, `${lang}: published PWA version missing`);
  assert.match(dictionaries[lang].companionStep1, /1\.0\.2/, `${lang}: Windows compatibility limit missing`);
  assert.match(dictionaries[lang].v1cap, /1\.0\.2/, `${lang}: install video version missing`);
  assert.match(dictionaries[lang].v2cap, /1\.0\.2/, `${lang}: use video version missing`);
  assert.match(dictionaries[lang].vNote, /APK/, `${lang}: old videos need APK distinction`);
  assert.match(dictionaries[lang].a8, /Sentry/, `${lang}: diagnostics privacy disclosure missing`);
}

for (const match of html.matchAll(/\b(?:href|src)="([^"]+)"/g)) {
  const value = match[1];
  if (value.startsWith('#')) assert(ids.includes(value.slice(1)), `Broken local anchor: ${value}`);
  assert(!/\.apk(?:[?#]|$)/i.test(value), 'No published stable APK exists: link the development workflow, not a guessed APK');
  if (/releases\/download\//.test(value)) {
    assert.match(value, /^https:\/\/github\.com\/igalgerman\/copyapp\/releases\/download\/v1\.0\.2\/(copyapp\.exe|install\.mp4|use\.mp4|demo-copyapp-[a-z-]+\.mp4)$/, `Unverified release asset: ${value}`);
  }
}
const expectedLinks = [
  /actions\/(?:workflows\/android\.yml|runs\/\d+)" data-i18n="companionAction"/,
  /actions\/(?:workflows\/validate\.yml|runs\/\d+)" data-i18n="companionWindowsAction"/,
  /blob\/master\/docs\/ANDROID\.md" data-i18n="companionGuide"/,
];
for (const pattern of expectedLinks) assert.match(html, pattern, 'Missing companion preview action');
assert.match(html, /data-i18n="companionBadge">[^<]*פיתוח/, 'Hebrew APK must be labeled development');
assert.match(html, /data-i18n="companionStep1">[^<]*1\.0\.2/, 'Hebrew Windows compatibility limit missing');
assert.match(html, /data-i18n="v1cap">[^<]*1\.0\.2/, 'Hebrew install video version missing');
assert.match(html, /data-i18n="v2cap">[^<]*1\.0\.2/, 'Hebrew use video version missing');
assert.match(html, /data-i18n="a8">[^<]*Sentry/, 'Hebrew diagnostics privacy disclosure missing');
console.log(`Website checks passed: ${ids.length} unique ids, ${keys.size} keys in English/Russian, release and preview links consistent (${fileURLToPath(root)}).`);

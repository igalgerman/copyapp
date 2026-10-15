// CopyApp phone UI. Vanilla JS, no build step. Strings live in i18n.js.
(() => {
  'use strict';

  const MAX_BYTES = 200 * 1024 * 1024;
  const EXPIRY_MS = 10 * 60 * 1000;
  const T = I18N.t;

  const $ = (sel, root = document) => root.querySelector(sel);
  const body = document.body;
  const feed = $('#feed');
  const empty = $('#empty');
  const conn = $('#conn');
  const connLabel = $('#conn-label');
  const banner = $('#banner');
  const bannerSub = $('#banner-sub');
  const bannerHelp = $('#banner-help');
  const sendBtn = $('#send-clip');
  const toasts = $('#toasts');
  const alerts = $('#alerts');

  const store = {
    get(k) { try { return localStorage.getItem(k); } catch { return null; } },
    set(k, v) { try { localStorage.setItem(k, v); } catch { /* private mode */ } },
    del(k) { try { localStorage.removeItem(k); } catch { /* private mode */ } },
  };

  const reduceMotion = matchMedia('(prefers-reduced-motion: reduce)');

  // ---------- small helpers ----------
  // A message is [key, vars?]. Nodes made from one keep the key in
  // data-i18n, so a language switch re-renders them in place.
  function setMsg(n, m) {
    n.dataset.i18n = m[0];
    if (m[1]) n.dataset.i18nVars = JSON.stringify(m[1]); else delete n.dataset.i18nVars;
    n.textContent = T(m[0], m[1]);
  }
  function el(tag, attrs = {}, ...kids) {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) {
      if (v == null || v === false) continue;
      if (k === 'class') n.className = v;
      else if (k === 'text') n.textContent = v;
      else if (k === 'msg') setMsg(n, v);
      else if (k === 'attrMsg') { // {attr: [key, vars]}
        const pairs = [];
        for (const [a, m] of Object.entries(v)) { pairs.push(a + ':' + m[0]); n.setAttribute(a, T(m[0], m[1])); if (m[1]) n.dataset.i18nVars = JSON.stringify(m[1]); }
        n.dataset.i18nAttr = pairs.join(';');
      }
      else if (k.startsWith('on')) n.addEventListener(k.slice(2), v);
      else n.setAttribute(k, v === true ? '' : v);
    }
    for (const kid of kids) if (kid != null) n.append(kid);
    return n;
  }
  function icon(name) {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('class', 'ic');
    svg.setAttribute('aria-hidden', 'true');
    const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
    use.setAttribute('href', '#i-' + name);
    svg.append(use);
    return svg;
  }
  const fmtSize = (n) => I18N.size(n);
  const relTime = (t) => I18N.relTime(t);
  function extOf(name, mime) {
    const m = /\.([a-z0-9]{1,5})$/i.exec(name || '');
    if (m) return m[1].toUpperCase();
    if (mime && mime.includes('/')) return mime.split('/')[1].split(/[+;.-]/)[0].slice(0, 4).toUpperCase();
    return 'FILE';
  }
  function buzz(pattern) {
    const ua = navigator.userActivation;
    if (!navigator.vibrate || (ua && !ua.hasBeenActive)) return; // Chrome blocks it before the first tap
    try { navigator.vibrate(pattern); } catch { /* unsupported */ }
  }

  // ---------- toasts ----------
  // title/detail are messages ([key, vars]).
  function toast(kind, title, detail, ms) {
    const t = el('button', { type: 'button', class: 'toast toast-' + kind },
      icon(kind === 'ok' ? 'check' : 'alert'),
      el('div', {}, el('b', { msg: title }), detail ? el('span', { msg: detail }) : null));
    const remove = () => {
      if (!t.isConnected) return;
      t.classList.add('is-leaving');
      setTimeout(() => t.remove(), reduceMotion.matches ? 0 : 250);
    };
    t.addEventListener('click', remove);
    while (toasts.children.length >= 2) toasts.firstElementChild.remove();
    toasts.append(t);
    // Re-enter the top layer so toasts show above open sheets.
    if (toasts.showPopover) {
      try { if (toasts.matches(':popover-open')) toasts.hidePopover(); toasts.showPopover(); } catch { /* old browser */ }
    }
    // Screen readers: polite for success, assertive for errors.
    if (kind === 'err') { alerts.textContent = ''; setTimeout(() => { alerts.textContent = T(...title) + '. ' + (detail ? T(...detail) : ''); }, 50); }
    else { t.setAttribute('role', 'status'); }
    setTimeout(remove, ms || (kind === 'err' ? 6500 : 3800));
    if (kind === 'ok') buzz(18); else buzz([30, 60, 30]);
  }

  // Server and local error codes -> [title, detail] messages (err.<code>.t/.d in i18n.js).
  function errorText(code) {
    const c = I18N.has('err.' + code + '.t') ? code : 'bad_request';
    return [['err.' + c + '.t'], ['err.' + c + '.d']];
  }

  // ---------- views ----------
  function setView(v) {
    body.dataset.view = v;
    if (v === 'pair') buildQR();
    if (v === 'main') requestAnimationFrame(measureDock);
  }
  function measureDock() {
    const dock = $('.dock');
    if (!dock) return;
    const h = dock.getBoundingClientRect().height;
    document.documentElement.style.setProperty('--dock-h', h + 'px');
    document.documentElement.style.setProperty('--toast-bottom', (body.dataset.view === 'main' ? h : 24) + 'px');
  }
  addEventListener('resize', measureDock);

  let qrBuilt = false;
  function buildQR() { // decorative pseudo-QR for the pairing illustration
    if (qrBuilt) return;
    qrBuilt = true;
    const box = $('#pair-qr');
    const N = 11;
    let seed = 7;
    const rnd = () => (seed = (seed * 9301 + 49297) % 233280) / 233280;
    const finder = (x, y) => {
      for (const [fx, fy] of [[0, 0], [N - 3, 0], [0, N - 3]]) {
        if (x >= fx && x < fx + 3 && y >= fy && y < fy + 3) return (x === fx + 1 && y === fy + 1) ? 'c' : '';
      }
      return null;
    };
    const frag = document.createDocumentFragment();
    for (let y = 0; y < N; y++) for (let x = 0; x < N; x++) {
      const f = finder(x, y);
      const cls = f !== null ? f : (rnd() > 0.52 ? '' : 'o');
      frag.append(el('i', { class: cls || null }));
    }
    box.append(frag);
  }

  function setConn(state) {
    conn.dataset.state = state;
    setMsg(connLabel, [state === 'online' ? 'conn.online' : state === 'offline' ? 'conn.offline' : 'conn.connecting']);
  }

  // ---------- feed ----------
  const received = new Map(); // id -> {item, node}
  let streamOpenedAt = 0;

  function updateEmpty() {
    empty.hidden = feed.children.length > 0;
  }
  function nearBottom() {
    return innerHeight + scrollY >= document.documentElement.scrollHeight - 160;
  }
  function scrollToEnd(smooth) {
    requestAnimationFrame(() => scrollTo({ top: document.documentElement.scrollHeight, behavior: smooth && !reduceMotion.matches ? 'smooth' : 'auto' }));
  }
  function insertByTime(node, t) {
    node.dataset.t = t;
    let before = null;
    for (const c of feed.children) { if (+c.dataset.t > t) { before = c; break; } }
    feed.insertBefore(node, before);
  }
  function stamp(node, t) {
    const time = $('.msg-time', node);
    time.dateTime = new Date(t).toISOString();
    time.textContent = relTime(t);
    time.dataset.t = t;
  }
  function refreshTimes() {
    for (const time of feed.querySelectorAll('.msg-time')) time.textContent = relTime(+time.dataset.t);
  }
  setInterval(() => {
    refreshTimes();
    for (const { item, node } of received.values()) markExpired(item, node);
  }, 20000);

  function markExpired(item, node) {
    if (node.classList.contains('is-expired')) return;
    if (Date.now() - item.t < EXPIRY_MS) return;
    if (item.kind === 'text' && typeof item.text === 'string') return; // already on the phone
    node.classList.add('is-expired');
    const card = $('.card', node);
    for (const b of card.querySelectorAll('.card-actions .btn')) {
      b.setAttribute('aria-disabled', 'true');
      b.tabIndex = -1;
      if (b.tagName === 'A') b.removeAttribute('href');
    }
    card.append(el('p', { class: 'expired-note', msg: ['item.expired'] }));
  }

  function addReceived(item) {
    if (received.has(item.id)) return;
    const t = Date.parse(item.created) || Date.now();
    item.t = t;
    const node = $('#tpl-in').content.firstElementChild.cloneNode(true);
    const card = $('.card', node);
    const url = '/item/' + encodeURIComponent(item.id);
    const label = item.kind === 'image' ? 'item.image' : item.kind === 'file' ? 'item.file' : 'item.text';
    node.setAttribute('aria-label', T(label));
    node.dataset.i18nAttr = 'aria-label:' + label;

    if (item.kind === 'text') {
      const hasText = typeof item.text === 'string';
      // The PC's own text: dir="auto" lets Hebrew/Russian/English each read naturally.
      const txt = hasText
        ? el('p', { class: 'card-text clamp', dir: 'auto', text: item.text })
        : el('p', { class: 'card-text clamp', msg: ['item.long', { size: fmtSize(item.size) }] });
      card.append(txt);
      if (hasText && (item.text.length > 220 || item.text.split('\n').length > 6)) {
        const more = el('button', { type: 'button', class: 'more-btn', msg: ['item.showAll'], 'aria-expanded': 'false' });
        more.addEventListener('click', () => {
          const open = txt.classList.toggle('clamp');
          setMsg(more, [open ? 'item.showAll' : 'item.showLess']);
          more.setAttribute('aria-expanded', String(!open));
        });
        card.append(more);
      }
      const copy = el('button', { type: 'button', class: 'btn btn-primary' }, icon('copy'), el('span', { msg: ['copy'] }));
      copy.addEventListener('click', () => copyText(item, copy));
      card.append(el('div', { class: 'card-actions' }, copy));
    } else if (item.kind === 'image') {
      const img = el('img', { class: 'card-thumb', src: url, attrMsg: { alt: item.name ? ['item.imgAlt', { name: item.name }] : ['item.image'] }, loading: 'lazy', decoding: 'async' });
      img.addEventListener('load', () => { if (nearBottom()) scrollToEnd(false); }, { once: true });
      img.addEventListener('error', () => img.replaceWith(el('div', { class: 'thumb-missing' }, icon('photo'))), { once: true });
      card.append(img);
      const copy = el('button', { type: 'button', class: 'btn btn-primary' }, icon('copy'), el('span', { msg: ['copyImage'] }));
      copy.addEventListener('click', () => copyImage(item, copy));
      const dl = el('a', { class: 'btn btn-quiet', href: url, download: item.name || 'image.png' }, icon('download'), el('span', { msg: ['save'] }));
      dl.addEventListener('click', () => { toast('ok', ['toast.saving'], ['toast.inDownloads']); noteSuccess(); });
      card.append(el('div', { class: 'card-actions' }, copy, dl));
    } else {
      const name = item.name || T('dock.file');
      card.append(el('div', { class: 'file-row' },
        el('span', { class: 'file-tile', 'aria-hidden': 'true', text: extOf(name, item.mime) }),
        el('div', {}, el('div', { class: 'file-name' }, el('bdi', { text: name })), el('div', { class: 'file-sub', text: fmtSize(item.size), 'data-size': item.size }))));
      const dl = el('a', { class: 'btn btn-primary', href: url, download: name }, icon('download'), el('span', { msg: ['download'] }));
      dl.addEventListener('click', () => { toast('ok', ['toast.downloading', { name }], ['toast.inDownloads']); noteSuccess(); });
      card.append(el('div', { class: 'card-actions' }, dl));
    }

    stamp(node, t);
    const live = streamOpenedAt && Date.now() - streamOpenedAt > 1200;
    if (live) node.classList.add('is-new');
    const stick = nearBottom();
    insertByTime(node, t);
    received.set(item.id, { item, node });
    markExpired(item, node);
    updateEmpty();
    if (live) {
      buzz(12);
      if (stick || node === feed.lastElementChild) scrollToEnd(true);
    } else if (stick) scrollToEnd(false);
  }

  // ---------- copy (PC -> phone) ----------
  function flashDone(btn) {
    noteSuccess();
    const label = $('span', btn);
    const old = label.dataset.i18n;
    const use = $('use', btn);
    const oldIcon = use.getAttribute('href');
    btn.classList.add('is-done');
    setMsg(label, ['copied']);
    use.setAttribute('href', '#i-check');
    buzz(18);
    clearTimeout(btn._t);
    btn._t = setTimeout(() => {
      btn.classList.remove('is-done');
      setMsg(label, [old]);
      use.setAttribute('href', oldIcon);
    }, 2200);
  }
  // Fallback for pages where navigator.clipboard is missing or refuses
  // (e.g. the certificate warning was bypassed and the browser is strict).
  function legacyCopy(text) {
    const ta = el('textarea', { readonly: true, 'aria-hidden': 'true', style: 'position:fixed;top:0;left:0;width:1px;height:1px;opacity:0' });
    ta.value = text;
    body.append(ta);
    ta.select();
    try { ta.setSelectionRange(0, text.length); } catch { /* old browser */ }
    let ok = false;
    try { ok = document.execCommand('copy'); } catch { ok = false; }
    ta.remove();
    return ok;
  }
  async function writeText(text) {
    const cb = navigator.clipboard;
    if (cb && cb.writeText) {
      try { await cb.writeText(text); return; } catch { /* blocked: try the old way */ }
    }
    if (!legacyCopy(text)) throw new Error('copy');
  }
  async function fetchItem(item) {
    const r = await fetch('/item/' + encodeURIComponent(item.id), { cache: 'no-store' });
    if (r.status === 401) { onUnauthorized(); throw new Error('unauthorized'); }
    if (!r.ok) throw new Error('gone');
    return r;
  }
  async function copyText(item, btn) {
    try {
      if (typeof item.text === 'string') {
        await writeText(item.text);
      } else if (window.ClipboardItem && navigator.clipboard && navigator.clipboard.write) {
        // Pass a promise so the copy keeps the tap's user activation.
        let text = null;
        const blob = fetchItem(item).then((r) => r.text()).then((s) => { text = s; return new Blob([s], { type: 'text/plain' }); });
        try {
          await navigator.clipboard.write([new ClipboardItem({ 'text/plain': blob })]);
        } catch (e) {
          if (text === null) await blob; // rethrows 'gone' / 'unauthorized'
          if (!legacyCopy(text)) throw e;
        }
      } else {
        const s = await (await fetchItem(item)).text();
        if (!legacyCopy(s)) throw new Error('copy');
      }
      flashDone(btn);
    } catch (e) {
      if (e && e.message === 'unauthorized') return;
      if (e && e.message === 'gone') return toast('err', ['gone.item'], ['gone.detail']);
      toast('err', ...errorText('copy'));
    }
  }
  function toPng(blob) {
    if (blob.type === 'image/png') return Promise.resolve(blob);
    return createImageBitmap(blob).then((bmp) => {
      const c = document.createElement('canvas');
      c.width = bmp.width; c.height = bmp.height;
      c.getContext('2d').drawImage(bmp, 0, 0);
      return new Promise((res, rej) => c.toBlob((b) => (b ? res(b) : rej(new Error('png'))), 'image/png'));
    });
  }
  async function copyImage(item, btn) {
    if (!(window.ClipboardItem && navigator.clipboard && navigator.clipboard.write)) {
      return toast('err', ['img.noCopy'], ['img.saveHint']);
    }
    try {
      const png = fetchItem(item).then((r) => r.blob()).then(toPng);
      await navigator.clipboard.write([new ClipboardItem({ 'image/png': png })]);
      flashDone(btn);
    } catch (e) {
      if (e && e.message === 'unauthorized') return;
      if (e && e.message === 'gone') return toast('err', ['gone.image'], ['gone.detail']);
      toast('err', ['img.copyFail'], ['img.saveHint']);
    }
  }

  // ---------- send (phone -> PC) ----------
  const DONE_TEXT = {
    text: [['done.text'], ['done.paste']],
    image: [['done.image'], ['done.paste']],
    file: [['done.file'], ['done.fileWhere']],
  };

  function addOutgoing(desc) {
    const node = $('#tpl-out').content.firstElementChild.cloneNode(true);
    node.classList.add('is-new');
    const card = $('.card', node);
    const t = Date.now();
    stamp(node, t);
    node.dataset.t = t;
    feed.append(node);
    updateEmpty();
    scrollToEnd(true);

    if (desc.thumb) {
      const img = el('img', { class: 'out-thumb', src: desc.thumb, attrMsg: { alt: ['out.imgAlt'] } });
      img.addEventListener('load', () => scrollToEnd(false), { once: true });
      img.addEventListener('error', () => img.remove(), { once: true });
      card.append(img);
    }
    const bodyEl = el('div', { class: 'out-body' });
    if (desc.text != null) bodyEl.append(el('p', { class: 'out-text', dir: 'auto', text: desc.text }));
    if (desc.files) {
      const f = desc.files;
      const name = f.length === 1 ? el('bdi', { text: f[0].name }) : el('span', { msg: ['out.more', { name: f[0].name, n: f.length - 1 }] });
      const total = f.reduce((a, b) => a + b.size, 0);
      bodyEl.append(el('div', { class: 'file-row file-row-flush' },
        el('span', { class: 'file-tile', 'aria-hidden': 'true', dir: 'ltr', text: f.length === 1 ? extOf(f[0].name, f[0].type) : f.length + '×' }),
        el('div', {}, el('div', { class: 'file-name' }, name), el('div', { class: 'file-sub', text: fmtSize(total), 'data-size': total }))));
    }
    const statusSlot = el('div', { class: 'out-slot' });
    bodyEl.append(statusSlot);
    card.append(bodyEl);
    node.setAttribute('aria-label', T('out.aria'));
    node.dataset.i18nAttr = 'aria-label:out.aria';
    return { node, statusSlot, total: desc.total || 0 };
  }

  function showProgress(entry, xhr) {
    const fill = el('div', { class: 'progress-fill' });
    const pct = el('span', { class: 'progress-pct', dir: 'ltr', text: '0%' });
    const bytes = el('span', { msg: [entry.total > 0 ? 'prog.sending' : 'prog.sendingPc'] });
    const cancel = el('button', { type: 'button', class: 'out-btn' }, icon('x'), el('span', { msg: ['cancel'] }));
    cancel.addEventListener('click', () => xhr.abort());
    const bar = el('div', { class: 'progress', role: 'progressbar', attrMsg: { 'aria-label': ['prog.aria'] }, 'aria-valuemin': '0', 'aria-valuemax': '100', 'aria-valuenow': '0' },
      el('div', { class: 'progress-track' }, fill),
      el('div', { class: 'progress-row' }, el('span', {}, pct, ' ', bytes), cancel));
    entry.statusSlot.replaceChildren(bar);
    let last = 0;
    return (loaded, total) => {
      const p = total ? Math.min(100, Math.floor((loaded / total) * 100)) : 0;
      if (p === last && p !== 0) return;
      last = p;
      fill.style.width = p + '%';
      pct.textContent = p + '%';
      bar.setAttribute('aria-valuenow', String(p));
      setMsg(bytes, p >= 100 ? ['prog.finishing'] : ['prog.of', { a: fmtSize(loaded), b: fmtSize(total) }]);
      if (p >= 100) cancel.hidden = true;
    };
  }

  // a and b are messages; b may be null.
  function showResult(entry, ok, a, b, retry, canceled) {
    entry.node.classList.toggle('is-error', !ok && !canceled);
    entry.node.classList.toggle('is-canceled', !!canceled);
    entry.node.classList.remove('is-busy');
    const status = el('p', { class: 'out-status' }, icon(ok ? 'check' : canceled ? 'x' : 'alert'),
      el('span', {}, el('b', { msg: a }), b ? el('br') : null, b ? el('span', { msg: b }) : null));
    if (!ok && retry) {
      const again = el('button', { type: 'button', class: 'out-btn' }, icon('retry'), el('span', { msg: ['tryAgain'] }));
      again.addEventListener('click', () => { entry.node.classList.remove('is-error', 'is-canceled'); retry(); });
      entry.statusSlot.replaceChildren(el('div', { class: 'out-row' }, status, again));
    } else {
      entry.statusSlot.replaceChildren(status);
    }
  }

  function transmit(entry, makeBody, contentType) {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/send');
    xhr.responseType = 'json';
    xhr.setRequestHeader('Accept', 'application/json');
    if (contentType) xhr.setRequestHeader('Content-Type', contentType);
    const update = showProgress(entry, xhr);
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) update(e.loaded, e.total); };
    const retry = () => transmit(entry, makeBody, contentType);
    return new Promise((resolve) => {
      xhr.onload = () => {
        const res = xhr.response || {};
        if (xhr.status === 200 && res.ok) {
          const [a, b] = DONE_TEXT[res.kind] || DONE_TEXT.file;
          let detail = b;
          if (res.kind === 'file' && res.saved && res.saved.length > 1) detail = ['done.files', { n: res.saved.length }];
          showResult(entry, true, a, detail);
          noteSuccess();
          toast('ok', [res.kind === 'file' ? 'toast.sentPc' : 'toast.copiedPc'], detail);
          resolve(true);
          return;
        }
        const code = res.code || (xhr.status === 413 ? 'too_large' : xhr.status === 401 ? 'unauthorized' : 'bad_request');
        let [a, b] = errorText(code);
        const partial = Array.isArray(res.saved) && res.saved.length > 0;
        if (partial) { // some files made it; say which, and don't resend them
          a = ['partial.title'];
          b = ['partial.detail', { files: res.saved.join(', '), reason: T(...b) }];
        }
        showResult(entry, false, a, b, partial || code === 'unauthorized' || code === 'too_large' || code === 'too_many_pixels' || code === 'empty' ? null : retry);
        toast('err', a, b);
        if (code === 'unauthorized') onUnauthorized(true);
        resolve(false);
      };
      xhr.onerror = () => {
        const [a, b] = errorText('network');
        showResult(entry, false, a, b, retry);
        toast('err', a, b);
        kickReconnect();
        resolve(false);
      };
      xhr.onabort = () => {
        showResult(entry, false, ['canceled'], null, retry, true);
        resolve(false);
      };
      xhr.send(makeBody());
    });
  }

  function sendText(text) {
    const entry = addOutgoing({ text });
    return transmit(entry, () => JSON.stringify({ text }), 'application/json');
  }
  function sendClipboardImage(blob) {
    const thumb = URL.createObjectURL(blob);
    const entry = addOutgoing({ thumb, total: blob.size });
    return transmit(entry, () => {
      const fd = new FormData();
      fd.append('mode', 'clipboard'); // must precede the file
      fd.append('file', blob, 'clipboard.png');
      return fd;
    });
  }
  function sendFiles(list) {
    const files = Array.from(list);
    if (!files.length) return;
    const total = files.reduce((a, f) => a + f.size, 0);
    if (total > MAX_BYTES) return toast('err', ...errorText('too_large'));
    let thumb = null;
    if (files.length === 1 && /^image\/(png|jpe?g|gif|webp)$/.test(files[0].type)) thumb = URL.createObjectURL(files[0]);
    const entry = addOutgoing({ files, thumb, total });
    return transmit(entry, () => {
      const fd = new FormData();
      for (const f of files) fd.append('files', f, f.name);
      return fd;
    });
  }

  let busy = false;
  async function sendClipboard() {
    if (busy) return;
    busy = true;
    sendBtn.classList.add('is-busy');
    try {
      const got = await readClipboard();
      if (got === null) { openSheet('sheet-paste', true); return; }
      if (got instanceof Blob) { await sendClipboardImage(got); }
      else if (!got.trim()) { toast('err', ...errorText('empty')); return; }
      else { await sendText(got); }
      sendBtn.classList.remove('is-sent'); void sendBtn.offsetWidth; sendBtn.classList.add('is-sent');
    } finally {
      busy = false;
      sendBtn.classList.remove('is-busy');
    }
  }
  // Returns a PNG Blob, a string, or null when the browser won't let us read
  // (no secure context, no API, or permission denied): the caller then opens
  // the paste box instead.
  async function readClipboard() {
    const cb = window.isSecureContext ? navigator.clipboard : null;
    if (!cb || !(cb.read || cb.readText)) return null;
    if (cb.read) {
      try {
        const items = await cb.read();
        for (const it of items) {
          if (it.types.includes('image/png')) return await it.getType('image/png');
        }
        for (const it of items) {
          if (it.types.includes('text/plain')) return await (await it.getType('text/plain')).text();
        }
        return '';
      } catch { /* fall back to readText */ }
    }
    if (cb.readText) {
      try { return await cb.readText(); } catch { return null; }
    }
    return null;
  }

  sendBtn.addEventListener('click', sendClipboard);
  for (const id of ['pick-photo', 'pick-file']) {
    const input = document.getElementById(id);
    input.addEventListener('change', () => { sendFiles(input.files); input.value = ''; });
  }

  // ---------- sheets ----------
  function openSheet(id, fromClipboard) {
    const d = document.getElementById(id);
    if (!d || d.open) return;
    for (const other of document.querySelectorAll('dialog[open]')) other.close();
    if (id === 'sheet-paste') setMsg($('#paste-sub'), [fromClipboard ? 'paste.subClip' : 'paste.sub']);
    d.showModal();
    if (id === 'sheet-paste') setTimeout(() => $('#paste-text').focus(), 60);
    if (id === 'sheet-cert' || id === 'sheet-settings') loadCertFp();
  }
  document.addEventListener('click', (e) => {
    const opener = e.target.closest('[data-open]');
    if (opener) { e.preventDefault(); openSheet(opener.dataset.open); return; }
    const closer = e.target.closest('[data-close]');
    if (closer) { closer.closest('dialog').close(); return; }
    if (e.target instanceof HTMLDialogElement) e.target.close(); // tap on backdrop
  });
  $('#paste-form').addEventListener('submit', (e) => {
    e.preventDefault();
    const ta = $('#paste-text');
    const text = ta.value;
    if (!text.trim()) { ta.focus(); return; }
    $('#sheet-paste').close();
    ta.value = '';
    sendText(text);
  });

  $('#cert-download').addEventListener('click', async (e) => {
    // Check first: the PC only hands out the certificate while pairing is open or once paired.
    e.preventDefault();
    try {
      const r = await fetch('/ca.crt', { method: 'GET', cache: 'no-store' });
      if (!r.ok) throw new Error('closed');
      const blob = await r.blob();
      await learnShortFp(await blob.text());
      const a = el('a', { href: URL.createObjectURL(blob), download: 'copyapp-ca.crt' });
      body.append(a); a.click(); a.remove();
      setTimeout(() => URL.revokeObjectURL(a.href), 10000);
      toast('ok', ['cert.ok'], ['cert.okSub']);
    } catch {
      toast('err', ['cert.fail'], ['cert.failSub']);
    }
  });

  $('#unpair-btn').addEventListener('click', () => openSheet('sheet-confirm'));
  $('#confirm-unpair').addEventListener('click', async () => {
    $('#sheet-confirm').close();
    try { await fetch('/unpair', { method: 'POST' }); } catch { /* still forget locally */ }
    goUnpaired('unpair.done');
  });

  // ---------- optional upgrade ----------
  // The app works fully in a plain browser tab (certificate warning bypassed).
  // Installing the CA and the app is optional; it adds the Share-menu entry.
  const standalone = () => matchMedia('(display-mode: standalone)').matches || navigator.standalone === true;
  let swActive = false; // a service worker registered: the phone trusts the certificate
  const tipCard = $('#tip-card');
  // After the first transfer that worked, show one tip about keeping the page handy.
  function noteSuccess() {
    if (store.get('firstSuccess') === '1') return;
    store.set('firstSuccess', '1');
    maybeShowTip();
  }
  function maybeShowTip() {
    tipCard.hidden = standalone() || store.get('firstSuccess') !== '1' || store.get('tipDismissed') === '1';
  }
  function dismissTip() { store.set('tipDismissed', '1'); tipCard.hidden = true; }
  $('#tip-dismiss').addEventListener('click', dismissTip);
  tipCard.querySelector('[data-open="sheet-cert"]').addEventListener('click', dismissTip);
  function updateAppRow() {
    $('#app-row').hidden = standalone();
    setMsg($('#app-row-sub'), [swActive ? 'settings.appSubTrusted' : 'settings.appSub']);
  }

  // ---------- connection ----------
  let es = null;
  let retryTimer = 0;
  let countdownTimer = 0;
  let attempt = 0;
  let paired = false;
  let bannerTimer = 0;
  let offlineSince = 0; // when the current outage began (0: online)
  const HELP_AFTER = 10000; // show the same-Wi-Fi hint after 10 s offline


  function onUnauthorized(silent) {
    if (!paired) return;
    goUnpaired(silent ? null : 'unpair.remote');
  }
  // msgKey: i18n key for the lead line (null: the default).
  function goUnpaired(msgKey) {
    paired = false;
    store.del('paired');
    closeStream();
    clearTimeout(retryTimer); clearInterval(countdownTimer); clearTimeout(bannerTimer);
    banner.hidden = true;
    for (const d of document.querySelectorAll('dialog[open]')) d.close();
    setMsg($('#pair-lead'), [msgKey || 'pair.lead']);
    setView('pair');
  }
  function closeStream() { if (es) { es.close(); es = null; } }

  async function checkStatus() {
    try {
      const r = await fetch('/status', { cache: 'no-store', headers: { Accept: 'application/json' } });
      if (r.status === 401) return 'unpaired';
      if (!r.ok) return 'down';
      const j = await r.json();
      if (j.fingerprint) $('#fingerprint').textContent = formatFingerprint(j.fingerprint);
      if (j.shortFingerprint && !shortFp) showShortFp(j.shortFingerprint);
      return 'ok';
    } catch { return 'down'; }
  }
  // The short code ("AB12 CD34 EF56 7890") is the first 16 hex digits of the
  // CA's SHA-256 fingerprint. Computed here from the certificate file itself
  // when possible, so it describes exactly what gets installed.
  let shortFp = '';
  function showShortFp(s) {
    for (const n of document.querySelectorAll('[data-fp-short]')) n.textContent = s;
  }
  async function shortFpOfPEM(pem) {
    const m = /-----BEGIN CERTIFICATE-----([\s\S]+?)-----END CERTIFICATE-----/.exec(pem);
    if (!m || !(window.crypto && crypto.subtle)) return '';
    const der = Uint8Array.from(atob(m[1].replace(/\s+/g, '')), (c) => c.charCodeAt(0));
    const h = new Uint8Array(await crypto.subtle.digest('SHA-256', der));
    const hex = Array.from(h.slice(0, 8), (x) => x.toString(16).padStart(2, '0')).join('').toUpperCase();
    return hex.match(/.{4}/g).join(' ');
  }
  async function learnShortFp(pemText) {
    try {
      const s = await shortFpOfPEM(pemText);
      if (s) { shortFp = s; showShortFp(s); }
    } catch { /* keep the server's value */ }
  }
  async function loadCertFp() {
    if (shortFp) return;
    try {
      const r = await fetch('/ca.crt', { cache: 'no-store' });
      if (r.ok) await learnShortFp(await r.text());
    } catch { /* PC not sharing it right now */ }
  }
  function formatFingerprint(fp) {
    // Same AB:CD:… form as the PC's pairing window; allow line breaks after colons.
    const hex = String(fp).replace(/[^0-9a-f]/gi, '').toUpperCase();
    return (hex.match(/.{1,2}/g) || []).join(':​') || fp;
  }

  function connect() {
    closeStream();
    clearTimeout(retryTimer);
    setConn('connecting');
    es = new EventSource('/events');
    es.addEventListener('open', () => {
      attempt = 0;
      streamOpenedAt = Date.now();
      setConn('online');
      hideBanner();
    });
    es.addEventListener('item', (ev) => {
      try { addReceived(JSON.parse(ev.data)); } catch { /* ignore malformed */ }
    });
    es.addEventListener('error', async () => {
      closeStream();
      streamOpenedAt = 0;
      const s = await checkStatus();
      if (s === 'unpaired') return onUnauthorized();
      if (s === 'ok' && attempt === 0) { attempt = 1; retryTimer = setTimeout(connect, 600); return; }
      scheduleReconnect();
    });
  }

  function scheduleReconnect() {
    setConn('offline');
    if (!offlineSince) offlineSince = Date.now();
    attempt++;
    const delay = Math.min(30000, 1000 * 2 ** Math.min(attempt - 1, 5)) * (0.85 + Math.random() * 0.3);
    const due = Date.now() + delay;
    clearTimeout(retryTimer);
    retryTimer = setTimeout(retryNow, delay);
    showBanner(due);
  }
  function showBanner(due) {
    const render = () => {
      const s = Math.max(0, Math.ceil((due - Date.now()) / 1000));
      setMsg(bannerSub, s <= 0 ? ['banner.now'] : [attempt >= 4 ? 'banner.hint' : 'banner.in', { n: s }]);
      bannerHelp.hidden = !(offlineSince && Date.now() - offlineSince > HELP_AFTER);
    };
    render();
    clearInterval(countdownTimer);
    countdownTimer = setInterval(render, 1000);
    // Don't flash the banner for a blip that fixes itself.
    if (banner.hidden && !bannerTimer) {
      bannerTimer = setTimeout(() => { bannerTimer = 0; if (conn.dataset.state !== 'online' && paired) banner.hidden = false; }, attempt > 1 ? 0 : 1200);
    }
  }
  function hideBanner() {
    offlineSince = 0;
    bannerHelp.hidden = true;
    clearInterval(countdownTimer);
    clearTimeout(bannerTimer); bannerTimer = 0;
    if (!banner.hidden) toast('ok', ['online.t'], ['online.d'], 2500);
    banner.hidden = true;
  }
  async function retryNow() {
    clearTimeout(retryTimer);
    clearInterval(countdownTimer);
    setMsg(bannerSub, ['banner.now']);
    setConn('connecting');
    const s = await checkStatus();
    if (s === 'unpaired') return onUnauthorized();
    if (s === 'ok') return connect();
    scheduleReconnect();
  }
  function kickReconnect() { if (paired && !es) retryNow(); }
  $('#banner-retry').addEventListener('click', retryNow);
  addEventListener('online', () => { if (paired && conn.dataset.state !== 'online') retryNow(); });
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible' && paired && conn.dataset.state !== 'online') retryNow();
  });

  // Language switched (i18n.js already re-rendered every data-i18n node):
  // refresh what is computed rather than looked up.
  document.addEventListener('i18n:change', () => {
    refreshTimes();
    for (const n of feed.querySelectorAll('.file-sub[data-size]')) n.textContent = fmtSize(+n.dataset.size);
    requestAnimationFrame(measureDock);
  });

  // ---------- start ----------
  function readQuery() {
    const q = new URLSearchParams(location.search);
    const sent = q.get('sent');
    const err = q.get('error');
    const share = q.get('share') === '1';
    if (sent || err || share) history.replaceState(null, '', '/');
    return { sent, err, share };
  }

  // Android shares land in sw.js, which stashes them in the Cache API and
  // opens /?share=1. Read the stash once (then delete it) and send it like a
  // normal pick, so the request is same-origin and carries the session cookie.
  const SHARE_CACHE = 'copyapp-share-v1';
  async function takeShared() {
    if (!('caches' in window)) return null;
    try {
      const cache = await caches.open(SHARE_CACHE);
      const m = await cache.match('/__share/meta');
      if (!m) return null;
      const meta = await m.json();
      const files = [];
      for (const f of meta.files || []) {
        const r = await cache.match(f.key);
        if (r) files.push(new File([await r.blob()], f.name || 'file', { type: f.type || '' }));
      }
      return { ...meta, files };
    } catch {
      return null;
    } finally {
      try { await caches.delete(SHARE_CACHE); } catch { /* nothing stashed */ }
    }
  }
  async function sendShared() {
    const s = await takeShared();
    if (!s || Date.now() - (s.at || 0) > 10 * 60 * 1000) {
      toast('err', ...errorText('share'));
      return;
    }
    if (s.files.length) return sendFiles(s.files);
    // Same rules as the PC's own share handling: text, then the link, else the title.
    let t = s.text || '';
    if (s.url && !t.includes(s.url)) t = (t.trim() ? t + '\n' : '') + s.url;
    if (!t.trim()) t = s.title || '';
    if (!t.trim()) return toast('err', ...errorText('empty'));
    return sendText(t);
  }

  let shareWaiting = false;
  async function start() {
    const q = readQuery();
    const { sent, err } = q;
    if (q.share) shareWaiting = true; // survives the retry below, which re-reads a clean URL
    const share = shareWaiting;
    const s = await checkStatus();
    if (share && s === 'unpaired') { shareWaiting = false; takeShared(); } // drop it: nowhere to send
    if (s === 'unpaired') {
      goUnpaired(err === 'unauthorized' || share ? 'start.shareUnpaired' : null);
      return;
    }
    if (s === 'down' && store.get('paired') !== '1') {
      goUnpaired('start.down');
      setTimeout(start, 5000);
      return;
    }
    paired = true;
    store.set('paired', '1');
    setView('main');
    maybeShowTip();
    updateAppRow();
    updateEmpty();
    if (s === 'ok') connect(); else scheduleReconnect();
    if (sent) {
      const [a, b] = DONE_TEXT[sent] || DONE_TEXT.file;
      toast('ok', [sent === 'file' ? 'toast.sentPc' : 'toast.copiedPc'], sent === 'file' ? b : a);
    } else if (err) {
      toast('err', ...errorText(err));
    }
    if (share) { shareWaiting = false; sendShared(); } // offline is fine: the card offers Retry
  }

  // Registration fails while the certificate isn't trusted (a tab where the
  // warning was bypassed). That's the normal case, so stay silent: the app
  // works without it, and the share target / offline shell come with the
  // optional upgrade.
  if ('serviceWorker' in navigator && window.isSecureContext) {
    addEventListener('load', () => {
      let reg;
      try { reg = navigator.serviceWorker.register('/sw.js'); } catch { return; }
      Promise.resolve(reg).then(() => { swActive = true; updateAppRow(); }, () => { /* untrusted cert: no SW */ });
    });
  }

  start();
})();

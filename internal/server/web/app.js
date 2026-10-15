// CopyApp phone UI. Vanilla JS, no build step.
(() => {
  'use strict';

  const MAX_BYTES = 200 * 1024 * 1024;
  const EXPIRY_MS = 10 * 60 * 1000;

  const $ = (sel, root = document) => root.querySelector(sel);
  const body = document.body;
  const feed = $('#feed');
  const empty = $('#empty');
  const conn = $('#conn');
  const connLabel = $('#conn-label');
  const banner = $('#banner');
  const bannerTitle = $('#banner-title');
  const bannerSub = $('#banner-sub');
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
  function el(tag, attrs = {}, ...kids) {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) {
      if (v == null || v === false) continue;
      if (k === 'class') n.className = v;
      else if (k === 'text') n.textContent = v;
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
  function fmtSize(n) {
    if (n < 1024) return n + ' B';
    if (n < 1024 * 1024) return Math.max(1, Math.round(n / 1024)) + ' KB';
    if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(n < 10 * 1024 * 1024 ? 1 : 0) + ' MB';
    return (n / 1024 / 1024 / 1024).toFixed(1) + ' GB';
  }
  function relTime(t) {
    const s = Math.round((Date.now() - t) / 1000);
    if (s < 45) return 'just now';
    const m = Math.round(s / 60);
    if (m < 60) return m + ' min ago';
    return new Date(t).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }
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
  function toast(kind, title, detail, ms) {
    const t = el('button', { type: 'button', class: 'toast toast-' + kind },
      icon(kind === 'ok' ? 'check' : 'alert'),
      el('div', {}, el('b', { text: title }), detail ? el('span', { text: detail }) : null));
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
    if (kind === 'err') { alerts.textContent = ''; setTimeout(() => { alerts.textContent = title + '. ' + (detail || ''); }, 50); }
    else { t.setAttribute('role', 'status'); }
    setTimeout(remove, ms || (kind === 'err' ? 6500 : 3800));
    if (kind === 'ok') buzz(18); else buzz([30, 60, 30]);
  }

  const ERRORS = {
    empty: ['Nothing to send', 'Your clipboard is empty. Copy something first.'],
    not_png: ['Your PC couldn’t paste that image', 'Only PNG images can go to the clipboard. Send it with File instead.'],
    too_large: ['That’s too big', 'The limit is 200 MB per transfer. Try a smaller file.'],
    unauthorized: ['This phone isn’t paired anymore', 'Scan the QR code on your PC to connect again.'],
    unsupported: ['Your PC didn’t accept that', 'Try sending it as a file instead.'],
    bad_request: ['Your PC couldn’t read that', 'Try sending it again.'],
    bad_text: ['That text can’t be sent', 'It contains characters your PC can’t use.'],
    clipboard: ['Your PC’s clipboard is busy', 'Another app is using it. Try again in a moment.'],
    save: ['Your PC couldn’t save the file', 'Check that the PC has enough free space.'],
    network: ['Can’t reach your PC', 'Make sure both are on the same Wi‑Fi and CopyApp is running.'],
    copy: ['Couldn’t copy on this phone', 'Your browser blocked the clipboard. Try again, or long-press to copy.'],
  };
  function errorText(code) { return ERRORS[code] || ERRORS.bad_request; }

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
    connLabel.textContent = state === 'online' ? 'Connected' : state === 'offline' ? 'Offline' : 'Connecting';
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
  setInterval(() => {
    for (const time of feed.querySelectorAll('.msg-time')) time.textContent = relTime(+time.dataset.t);
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
    card.append(el('p', { class: 'expired-note', text: 'Expired. Send it again from your PC.' }));
  }

  function addReceived(item) {
    if (received.has(item.id)) return;
    const t = Date.parse(item.created) || Date.now();
    item.t = t;
    const node = $('#tpl-in').content.firstElementChild.cloneNode(true);
    const card = $('.card', node);
    const url = '/item/' + encodeURIComponent(item.id);
    const label = item.kind === 'image' ? 'Image from PC' : item.kind === 'file' ? 'File from PC' : 'Text from PC';
    node.setAttribute('aria-label', label);

    if (item.kind === 'text') {
      const hasText = typeof item.text === 'string';
      const txt = el('p', { class: 'card-text clamp', text: hasText ? item.text : 'Long text (' + fmtSize(item.size) + '). Tap Copy to get all of it.' });
      card.append(txt);
      if (hasText && (item.text.length > 220 || item.text.split('\n').length > 6)) {
        const more = el('button', { type: 'button', class: 'more-btn', text: 'Show all', 'aria-expanded': 'false' });
        more.addEventListener('click', () => {
          const open = txt.classList.toggle('clamp');
          more.textContent = open ? 'Show all' : 'Show less';
          more.setAttribute('aria-expanded', String(!open));
        });
        card.append(more);
      }
      const copy = el('button', { type: 'button', class: 'btn btn-primary' }, icon('copy'), el('span', { text: 'Copy' }));
      copy.addEventListener('click', () => copyText(item, copy));
      card.append(el('div', { class: 'card-actions' }, copy));
    } else if (item.kind === 'image') {
      const img = el('img', { class: 'card-thumb', src: url, alt: item.name ? 'Image: ' + item.name : 'Image from your PC', loading: 'lazy', decoding: 'async' });
      img.addEventListener('load', () => { if (nearBottom()) scrollToEnd(false); }, { once: true });
      img.addEventListener('error', () => img.replaceWith(el('div', { class: 'thumb-missing' }, icon('photo'))), { once: true });
      card.append(img);
      const copy = el('button', { type: 'button', class: 'btn btn-primary' }, icon('copy'), el('span', { text: 'Copy image' }));
      copy.addEventListener('click', () => copyImage(item, copy));
      const dl = el('a', { class: 'btn btn-quiet', href: url, download: item.name || 'image.png' }, icon('download'), el('span', { text: 'Save' }));
      dl.addEventListener('click', () => toast('ok', 'Saving to your phone', 'Find it in Downloads.'));
      card.append(el('div', { class: 'card-actions' }, copy, dl));
    } else {
      const name = item.name || 'File';
      card.append(el('div', { class: 'file-row' },
        el('span', { class: 'file-tile', 'aria-hidden': 'true', text: extOf(name, item.mime) }),
        el('div', {}, el('div', { class: 'file-name', text: name }), el('div', { class: 'file-sub', text: fmtSize(item.size) }))));
      const dl = el('a', { class: 'btn btn-primary', href: url, download: name }, icon('download'), el('span', { text: 'Download' }));
      dl.addEventListener('click', () => toast('ok', 'Downloading ' + name, 'Find it in Downloads.'));
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
  function flashDone(btn, text) {
    const label = $('span', btn);
    const old = label.textContent;
    const use = $('use', btn);
    const oldIcon = use.getAttribute('href');
    btn.classList.add('is-done');
    label.textContent = text;
    use.setAttribute('href', '#i-check');
    buzz(18);
    clearTimeout(btn._t);
    btn._t = setTimeout(() => {
      btn.classList.remove('is-done');
      label.textContent = old;
      use.setAttribute('href', oldIcon);
    }, 2200);
  }
  function legacyCopy(text) {
    const ta = el('textarea', { readonly: true, style: 'position:fixed;top:0;left:0;opacity:0' });
    ta.value = text;
    body.append(ta);
    ta.select();
    let ok = false;
    try { ok = document.execCommand('copy'); } catch { ok = false; }
    ta.remove();
    return ok;
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
        if (navigator.clipboard && navigator.clipboard.writeText) await navigator.clipboard.writeText(item.text);
        else if (!legacyCopy(item.text)) throw new Error('copy');
      } else if (window.ClipboardItem && navigator.clipboard && navigator.clipboard.write) {
        // Pass a promise so the copy keeps the tap's user activation.
        const blob = fetchItem(item).then((r) => r.text()).then((s) => new Blob([s], { type: 'text/plain' }));
        await navigator.clipboard.write([new ClipboardItem({ 'text/plain': blob })]);
      } else {
        const s = await (await fetchItem(item)).text();
        if (!legacyCopy(s)) throw new Error('copy');
      }
      flashDone(btn, 'Copied');
    } catch (e) {
      if (e && e.message === 'unauthorized') return;
      if (e && e.message === 'gone') return toast('err', 'That item is gone', 'Items stay on your PC for 10 minutes. Send it again from the PC.');
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
      return toast('err', 'This browser can’t copy images', 'Tap Save instead, then share it from your gallery.');
    }
    try {
      const png = fetchItem(item).then((r) => r.blob()).then(toPng);
      await navigator.clipboard.write([new ClipboardItem({ 'image/png': png })]);
      flashDone(btn, 'Copied');
    } catch (e) {
      if (e && e.message === 'unauthorized') return;
      if (e && e.message === 'gone') return toast('err', 'That image is gone', 'Items stay on your PC for 10 minutes. Send it again from the PC.');
      toast('err', 'Couldn’t copy the image', 'Tap Save instead, then share it from your gallery.');
    }
  }

  // ---------- send (phone -> PC) ----------
  const DONE_TEXT = {
    text: ['On your PC’s clipboard', 'Paste with Ctrl+V'],
    image: ['Image on your PC’s clipboard', 'Paste with Ctrl+V'],
    file: ['Saved on your PC', 'In Downloads › CopyApp'],
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
      const img = el('img', { class: 'out-thumb', src: desc.thumb, alt: 'Image you sent' });
      img.addEventListener('load', () => scrollToEnd(false), { once: true });
      img.addEventListener('error', () => img.remove(), { once: true });
      card.append(img);
    }
    const bodyEl = el('div', { class: 'out-body' });
    if (desc.text != null) bodyEl.append(el('p', { class: 'out-text', text: desc.text }));
    if (desc.files) {
      const f = desc.files;
      const name = f.length === 1 ? f[0].name : f[0].name + ' and ' + (f.length - 1) + ' more';
      const total = f.reduce((a, b) => a + b.size, 0);
      bodyEl.append(el('div', { class: 'file-row', style: 'padding:0' },
        el('span', { class: 'file-tile', 'aria-hidden': 'true', text: f.length === 1 ? extOf(f[0].name, f[0].type) : f.length + '×' }),
        el('div', {}, el('div', { class: 'file-name', text: name }), el('div', { class: 'file-sub', text: fmtSize(total) }))));
    }
    const statusSlot = el('div', { class: 'out-slot' });
    bodyEl.append(statusSlot);
    card.append(bodyEl);
    node.setAttribute('aria-label', 'Sent to PC');
    return { node, statusSlot, total: desc.total || 0 };
  }

  function showProgress(entry, xhr) {
    const fill = el('div', { class: 'progress-fill' });
    const pct = el('span', { class: 'progress-pct', text: '0%' });
    const bytes = el('span', { text: entry.total > 0 ? 'Sending…' : 'Sending to your PC…' });
    const cancel = el('button', { type: 'button', class: 'out-btn' }, icon('x'), el('span', { text: 'Cancel' }));
    cancel.addEventListener('click', () => xhr.abort());
    const bar = el('div', { class: 'progress', role: 'progressbar', 'aria-label': 'Upload progress', 'aria-valuemin': '0', 'aria-valuemax': '100', 'aria-valuenow': '0' },
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
      bytes.textContent = p >= 100 ? 'Finishing up…' : fmtSize(loaded) + ' of ' + fmtSize(total);
      if (p >= 100) cancel.hidden = true;
    };
  }

  function showResult(entry, ok, a, b, retry, canceled) {
    entry.node.classList.toggle('is-error', !ok && !canceled);
    entry.node.classList.toggle('is-canceled', !!canceled);
    entry.node.classList.remove('is-busy');
    const status = el('p', { class: 'out-status' }, icon(ok ? 'check' : canceled ? 'x' : 'alert'),el('span', {}, el('b', { text: a }), b ? el('br') : null, b || null));
    if (!ok && retry) {
      const again = el('button', { type: 'button', class: 'out-btn' }, icon('retry'), el('span', { text: 'Try again' }));
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
          if (res.kind === 'file' && res.saved && res.saved.length > 1) detail = res.saved.length + ' files in Downloads › CopyApp';
          showResult(entry, true, a, detail);
          toast('ok', res.kind === 'file' ? 'Sent to your PC' : 'Copied to your PC', detail);
          resolve(true);
          return;
        }
        const code = res.code || (xhr.status === 413 ? 'too_large' : xhr.status === 401 ? 'unauthorized' : 'bad_request');
        const [a, b] = errorText(code);
        showResult(entry, false, a, b, code === 'unauthorized' || code === 'too_large' || code === 'empty' ? null : retry);
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
        showResult(entry, false, 'Canceled', null, retry, true);
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
  // Returns a PNG Blob, a string, or null when the browser won't let us read.
  async function readClipboard() {
    const cb = navigator.clipboard;
    if (!cb) return null;
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
    if (id === 'sheet-paste') {
      $('#paste-sub').textContent = fromClipboard
        ? 'This browser didn’t share your clipboard. Paste it here instead.'
        : 'Type or paste below. It goes straight to your PC’s clipboard.';
    }
    d.showModal();
    if (id === 'sheet-paste') setTimeout(() => $('#paste-text').focus(), 60);
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
      const a = el('a', { href: URL.createObjectURL(blob), download: 'copyapp-ca.crt' });
      body.append(a); a.click(); a.remove();
      setTimeout(() => URL.revokeObjectURL(a.href), 10000);
      toast('ok', 'Certificate downloaded', 'Now follow the steps below.');
      store.set('setupDismissed', '1');
      setupCard.hidden = true;
    } catch {
      toast('err', 'Your PC isn’t sharing the certificate right now', 'On your PC, choose Pair phone, then tap Download again.');
    }
  });

  $('#unpair-btn').addEventListener('click', () => openSheet('sheet-confirm'));
  $('#confirm-unpair').addEventListener('click', async () => {
    $('#sheet-confirm').close();
    try { await fetch('/unpair', { method: 'POST' }); } catch { /* still forget locally */ }
    goUnpaired('This phone is now unpaired. You can pair it again any time.');
  });

  const setupCard = $('#setup-card');
  $('#setup-dismiss').addEventListener('click', () => { store.set('setupDismissed', '1'); setupCard.hidden = true; });
  function maybeShowSetup() {
    const standalone = matchMedia('(display-mode: standalone)').matches;
    setupCard.hidden = standalone || store.get('setupDismissed') === '1';
  }

  // ---------- connection ----------
  let es = null;
  let retryTimer = 0;
  let countdownTimer = 0;
  let attempt = 0;
  let paired = false;
  let bannerTimer = 0;

  function onUnauthorized(silent) {
    if (!paired) return;
    goUnpaired(silent ? null : 'This phone was unpaired. Scan the QR code on your PC to connect again.');
  }
  function goUnpaired(msg) {
    paired = false;
    store.del('paired');
    closeStream();
    clearTimeout(retryTimer); clearInterval(countdownTimer); clearTimeout(bannerTimer);
    banner.hidden = true;
    for (const d of document.querySelectorAll('dialog[open]')) d.close();
    $('#pair-lead').textContent = msg || 'This phone isn’t connected to a PC yet. It takes one scan.';
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
      return 'ok';
    } catch { return 'down'; }
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
      bannerTitle.textContent = 'Can’t reach your PC';
      bannerSub.textContent = attempt >= 4
        ? (s > 0 ? 'Is the PC on and on this Wi‑Fi? Retrying in ' + s + ' s' : 'Reconnecting…')
        : (s > 0 ? 'Reconnecting in ' + s + ' s' : 'Reconnecting…');
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
    clearInterval(countdownTimer);
    clearTimeout(bannerTimer); bannerTimer = 0;
    if (!banner.hidden) toast('ok', 'Back online', 'Connected to your PC again.', 2500);
    banner.hidden = true;
  }
  async function retryNow() {
    clearTimeout(retryTimer);
    clearInterval(countdownTimer);
    bannerSub.textContent = 'Reconnecting…';
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

  // ---------- start ----------
  function readQuery() {
    const q = new URLSearchParams(location.search);
    const sent = q.get('sent');
    const err = q.get('error');
    if (sent || err) history.replaceState(null, '', '/');
    return { sent, err };
  }

  async function start() {
    const { sent, err } = readQuery();
    const s = await checkStatus();
    if (s === 'unpaired') {
      goUnpaired(err === 'unauthorized' ? 'Sharing didn’t work because this phone isn’t paired. Scan the QR code on your PC first.' : null);
      return;
    }
    if (s === 'down' && store.get('paired') !== '1') {
      goUnpaired('Can’t reach your PC right now. Make sure CopyApp is running and you’re on the same Wi‑Fi.');
      setTimeout(start, 5000);
      return;
    }
    paired = true;
    store.set('paired', '1');
    setView('main');
    maybeShowSetup();
    updateEmpty();
    if (s === 'ok') connect(); else scheduleReconnect();
    if (sent) {
      const [a, b] = DONE_TEXT[sent] || DONE_TEXT.file;
      toast('ok', sent === 'file' ? 'Sent to your PC' : 'Copied to your PC', sent === 'file' ? b : a + '. ' + b);
    } else if (err) {
      toast('err', ...errorText(err));
    }
  }

  if ('serviceWorker' in navigator && window.isSecureContext) {
    addEventListener('load', () => navigator.serviceWorker.register('/sw.js').catch(() => { /* untrusted cert: no SW */ }));
  }

  start();
})();

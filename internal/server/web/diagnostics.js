// Privacy boundary for diagnostics. It emits codes only, never exception strings,
// URLs, clipboard values, file names, cookies or request bodies.
(() => {
  'use strict';
  const id = () => {
    const b = new Uint8Array(16); crypto.getRandomValues(b);
    return Array.from(b, v => v.toString(16).padStart(2, '0')).join('');
  };
  let installation;
  try {
    installation = localStorage.getItem('copyapp.diagnostic-id');
    if (!/^[a-f0-9]{32}$/.test(installation || '')) { installation = id(); localStorage.setItem('copyapp.diagnostic-id', installation); }
  } catch { installation = id(); }
  const identity = { installation_id: installation, session_id: id() };
  const codes = /^[a-zA-Z0-9_.-]{1,64}$/;
  let client, config, count = 0, period = Date.now();
  const pending = [];
  function validConfig(c) {
    if (!c || typeof c.dsn !== 'string' || typeof c.version !== 'string' || !/^[a-zA-Z0-9.+_-]{0,64}$/.test(c.version)) return null;
    if (c.dsn === '') return { dsn: '', version: c.version };
    try {
      const u = new URL(c.dsn);
      if (u.protocol !== 'https:' || !u.username || u.password || u.search || u.hash || !/^\/[0-9]+$/.test(u.pathname) || (u.port && u.port !== '443')) return null;
      if (!/\.ingest(?:\.(?:us|de))?\.sentry\.io$/.test(u.hostname)) return null;
      return { dsn: c.dsn, version: c.version };
    } catch { return null; }
  }
  try {
    config = validConfig(JSON.parse(localStorage.getItem('copyapp.diagnostic-config') || 'null'));
    const previousSupport = localStorage.getItem('copyapp.support-id');
    if (/^[a-f0-9]{32}$/.test(previousSupport || '')) identity.support_id = previousSupport;
  } catch { /* no cached configuration */ }

  function emit(action, stage, result, fields = {}) {
    if (![action, stage, result].every(v => typeof v === 'string' && codes.test(v))) return;
    if (Date.now() - period >= 60000) { period = Date.now(); count = 0; }
    if (count >= 60) return;
    count++;
    const safe = {};
    if (/^[a-f0-9]{32}$/.test(fields.operation_id || '')) safe.operation_id = fields.operation_id;
    if (Number.isInteger(fields.http_status) && fields.http_status >= 100 && fields.http_status <= 599) safe.http_status = fields.http_status;
    if (Number.isFinite(fields.duration_ms)) safe.duration_ms = Math.max(0, Math.min(600000, Math.round(fields.duration_ms)));
    if (client) { try { client.emit(action, stage, result, safe); } catch { /* telemetry cannot break transfers */ } }
    else if (pending.length < 16) pending.push([action, stage, result, safe]);
  }
  function start() {
    if (client || !config || !config.dsn || !window.CopyAppDiagnosticsSDK) return;
    try { client = window.CopyAppDiagnosticsSDK.start(config, identity); for (const args of pending.splice(0)) client.emit(...args); } catch { /* diagnostic failure must not affect transfers */ }
  }
  window.CopyAppDiagnostics = {
    emit, operation: id,
    support(id) { if (/^[a-f0-9]{32}$/.test(id || '')) { identity.support_id = id; try { localStorage.setItem('copyapp.support-id', id); } catch {} } },
  };
  addEventListener('copyapp-diagnostics-ready', start);
  start();
  addEventListener('error', () => emit('javascript', 'unhandled_error', 'failed'));
  addEventListener('unhandledrejection', () => emit('javascript', 'unhandled_rejection', 'failed'));
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 3000);
  fetch('/diagnostics/config', { cache: 'no-store', credentials: 'omit', signal: controller.signal })
    .then(r => r.ok ? r.json() : null)
    .then(c => { const next = validConfig(c); if (!next) return; config = next; try { localStorage.setItem('copyapp.diagnostic-config', JSON.stringify(config)); } catch {} if (!config.dsn) { client = null; pending.length = 0; } start(); })
    .catch(() => {})
    .finally(() => clearTimeout(timer));
  emit('app', 'launch', 'ok');
})();

import { init, logger, captureMessage, makeFetchTransport } from '@sentry/browser';

// No automatic integrations: no console, URL, breadcrumb, DOM, network body,
// session replay, performance tracing or clipboard data is collected.
window.CopyAppDiagnosticsSDK = {
  start(config, identity) {
    init({
      dsn: config.dsn, release: 'copyapp@' + config.version,
      environment: 'production', defaultIntegrations: false,
      sendDefaultPii: false, enableLogs: true,
      transport: options => makeFetchTransport({ ...options, bufferSize: 8 }, (url, options) => {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), 3000);
        return fetch(url, { ...options, credentials: 'omit', referrerPolicy: 'no-referrer', signal: controller.signal }).finally(() => clearTimeout(timer));
      }),
      beforeSend(event) {
        delete event.request; delete event.user; delete event.breadcrumbs;
        delete event.exception; delete event.extra; delete event.contexts;
        return event;
      },
      beforeSendLog(log) {
        // Only explicit, fixed stage codes emitted below are permitted.
        const attributes = {};
        for (const [key, value] of Object.entries(log.attributes || {})) {
          if (['app','side','version','installation_id','session_id','support_id','operation_id','action','stage','result','http_status','duration_ms'].includes(key)) attributes[key] = value;
        }
        return { ...log, attributes };
      },
    });
    return {
      emit(action, stage, result, fields) {
        const attrs = { ...identity, app: 'copyapp', side: 'android', version: config.version, action, stage, result, ...fields };
        logger[result === 'ok' || result === 'start' ? 'info' : 'warn'](action + '.' + stage + '.' + result, attrs);
        if (action === 'javascript') captureMessage('javascript.' + stage, { level: 'error', tags: { ...identity, app: 'copyapp', side: 'android', version: config.version } });
      },
    };
  },
};
window.dispatchEvent(new Event('copyapp-diagnostics-ready'));

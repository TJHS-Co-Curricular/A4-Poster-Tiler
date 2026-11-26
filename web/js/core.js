/* core.js —— 共用小工具：$、esc、api（带超时、自动重试、中文错误）、log（写进程序日志）。
 * 提供：App.$、App.esc、App.api、App.log、App.mb。需要：无。 */
(function (App) {
  'use strict';

  const $ = id => document.getElementById(id);
  const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const mb = n => (n / 1048576).toFixed(1) + ' MB';
  const wait = ms => new Promise(ok => setTimeout(ok, ms));

  /* 所有请求都走这里。opt = { method, body, headers, timeout(ms), retries }。
   * 回传解析过的 JSON；服务端回错误时 throw Error（message 是中文，err.data 是回传内容）。
   * 只有「连不上」才重试：服务端已经回答的错误重送也一样。 */
  async function api(path, opt) {
    const o = Object.assign({ method: 'GET', timeout: 15000, retries: 2 }, opt || {});
    let last;
    for (let i = 0; i <= o.retries; i++) {
      const ctl = new AbortController();
      const timer = setTimeout(() => ctl.abort(), o.timeout);
      try {
        const res = await fetch(path, {
          method: o.method, body: o.body, signal: ctl.signal, cache: 'no-store',
          headers: Object.assign(o.method === 'GET' ? {} : { 'X-App': '1' }, o.headers || {}),
        });
        let data = null;
        try { data = await res.json(); } catch (e) { data = null; }
        if (!res.ok) {
          const err = new Error((data && data.error) || `服务端回传 ${res.status}`);
          err.data = data || {}; err.status = res.status;
          throw err;
        }
        return data;
      } catch (e) {
        if (e.status) throw e;
        last = e.name === 'AbortError' ? new Error('等太久没有回应') : new Error('连不上程序（黑色窗口是不是关掉了？）');
        last.offline = true;
        if (i < o.retries) await wait(600 * (i + 1));
      } finally {
        clearTimeout(timer);
      }
    }
    throw last;
  }

  /* 写进程序的日志（失败就算了，不影响使用） */
  function log(level, msg) {
    api('/api/log', { method: 'POST', body: JSON.stringify({ level, msg }), headers: { 'Content-Type': 'application/json' }, retries: 1 })
      .catch(() => {});
  }

  App.$ = $; App.esc = esc; App.api = api; App.log = log; App.mb = mb; App.wait = wait;
})((window.App = window.App || {}));

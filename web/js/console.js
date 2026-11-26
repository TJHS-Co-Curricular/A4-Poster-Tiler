/* console.js —— 程序没有黑色窗口了（维护者定 2026-11-26），这里补上它的功能：
 *   - 「控制台」面板：看程序窗口的内容、打开 logs / output 文件夹、结束程序（只有这台电脑看得到）
 *   - 心跳：每 15 秒告诉程序「网页还开着」；窗口关掉时送一个「关掉了」，程序约 20 秒后自动结束
 * 提供：App.console。需要：core.js。 */
(function (App) {
  'use strict';
  const { $, api } = App;
  const PING_MS = 15000;
  let timer = null, quitting = false;

  function heartbeat() {
    const ping = () => { if (!quitting) fetch('/__ping', { cache: 'no-store' }).catch(() => {}); };
    ping();
    setInterval(ping, PING_MS);
    document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') ping(); });
    // 关掉窗口（或重新整理）时说一声；重新整理的话新页面会马上再来心跳，程序不会结束
    window.addEventListener('pagehide', () => { if (!quitting) navigator.sendBeacon('/api/bye', ''); });
  }

  async function load() {
    try {
      const d = await api('/api/console', { retries: 0, timeout: 5000 });
      const box = $('consoleLines');
      const atEnd = box.scrollTop + box.clientHeight >= box.scrollHeight - 8;
      box.textContent = d.lines.join('\n');
      if (atEnd) box.scrollTop = box.scrollHeight;
    } catch (e) {
      $('consoleLines').textContent = quitting ? '程序已结束。' : '读不到程序的情况：' + e.message;
    }
  }

  function open() {
    $('consoleDlg').showModal();
    load().then(() => { const b = $('consoleLines'); b.scrollTop = b.scrollHeight; });
    timer = setInterval(load, 2000);
  }
  function close() {
    clearInterval(timer); timer = null;
    $('consoleDlg').close();
  }

  async function quit() {
    if (!confirm('确定要结束程序吗？\n结束后这个窗口就不能用了，要用时再双击 A4-Poster-Tiler.exe。')) return;
    try {
      await api('/api/quit', { method: 'POST', retries: 0 });
    } catch (e) {
      if (!e.offline) { alert(e.message); return; }
    }
    quitting = true;
    clearInterval(timer);
    document.body.innerHTML = '<main class="bye"><h1>程序已结束</h1><p>可以关掉这个窗口了。要再用时，双击 A4-Poster-Tiler.exe。</p></main>';
    setTimeout(() => window.close(), 800);   // 独立窗口可以自己关掉；一般浏览器分页关不掉就留着这句话
  }

  function action(path) {
    return () => api(path, { method: 'POST', retries: 0 }).catch(e => alert(e.message));
  }

  /* info = /api/info 的回传；只有这台电脑才显示「控制台」 */
  function init(info) {
    heartbeat();
    if (!info || !info.local) return;
    $('consoleBtn').hidden = false;
    $('consoleBtn').onclick = open;
    $('consoleClose').onclick = close;
    $('consoleDlg').addEventListener('close', () => { clearInterval(timer); timer = null; });
    $('openLogs').onclick = action('/api/open-logs');
    $('openOutput').onclick = action('/api/open-output');
    $('quitApp').onclick = quit;
  }

  App.console = { init };
})((window.App = window.App || {}));

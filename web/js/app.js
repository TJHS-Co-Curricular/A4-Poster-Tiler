/* app.js —— 页面（F9）：选图片 → 填尺寸 → 马上显示要几张、示意图 → 导出 PDF（F10）存进 output/。
 * 一次只建一张纸的画布，切完压成 JPG 就丢；原图只读一次。
 * 提供：无（页面入口）。需要：core.js、tiler.js、source.js、render.js。 */
(function (App) {
  'use strict';
  const { $, esc, api, mb, tiler } = App;

  const LOW_DPI = 100;   // 低于这个 dpi 提示「印出来会糊」
  const st = {
    info: { marginMm: 8, overlapMm: 10, canSave: false, version: '' },
    src: null, unit: 'in', paper: 'A4', mode: 'overlap', busy: false,
  };

  /* ---------- 设置值 ---------- */
  function sizeMm() {
    const w = parseFloat($('w').value), h = parseFloat($('h').value);
    return { W: tiler.toMm(w, st.unit), H: tiler.toMm(h, st.unit), ok: w > 0 && h > 0 };
  }
  function currentPlan() {
    const z = sizeMm();
    if (!z.ok) return null;
    return tiler.plan(z.W, z.H, { paper: st.paper, mode: st.mode, margin: st.info.marginMm, overlap: st.info.overlapMm });
  }
  const fmt = (v, d) => String(+v.toFixed(d == null ? 2 : d));
  const unitName = u => (u === 'cm' ? '厘米' : '英寸');

  function css(name) { return getComputedStyle(document.documentElement).getPropertyValue(name).trim(); }

  /* ---------- 画面更新 ---------- */
  function refresh() {
    let T = null, err = '';
    try { T = currentPlan(); } catch (e) { err = e.message; }
    const P = tiler.PAPERS[st.paper];
    document.querySelectorAll('[data-paper-label]').forEach(n => { n.textContent = P.label; });
    $('modeHint').textContent = st.mode === 'butt'
      ? '对边拼：每张剪掉四边白边，边对边对齐，背面用胶带贴。不重叠，但对不准时会看到缝。'
      : `重叠粘贴：相邻两张重叠 ${fmt(st.info.overlapMm, 1)} mm，剪掉左边、上面的白边，压在前一张的虚线上粘。比较容易对齐。`;
    $('fig').innerHTML = App.render.figure(st.mode);
    $('howPaste').innerHTML = st.mode === 'butt'
      ? '<b>从第 1 排第 1 张开始，边对边贴。</b>每张剪好四边后，和左边、上面那张对齐，在背面用胶带贴。'
      : '<b>从第 1 排第 1 张开始，往右、往下贴。</b>后一张剪好的左边（或上边）对准前一张的虚线，压在上面粘。虚线会被盖住。';

    if (!T) {
      $('summary').innerHTML = `<p class="${err ? 'bad' : 'muted'}">${esc(err || '请填成品的宽和高。')}</p>`;
      $('export').disabled = true; $('legend').textContent = ''; $('warn').innerHTML = '';
      $('measure').textContent = '—';
      return;
    }
    const z = sizeMm();
    const other = st.unit === 'in' ? `${fmt(z.W / 10, 1)} × ${fmt(z.H / 10, 1)} 厘米` : `${fmt(z.W / 25.4)} × ${fmt(z.H / 25.4)} 英寸`;
    $('summary').innerHTML = `
      <div class="count"><b>${T.n}</b> 张 ${esc(P.label)}</div>
      <div class="facts">
        <span class="pill">${T.land ? '横放' : '直放'}</span>
        <span class="pill">${T.rows} 排 × ${T.cols} 张</span>
        <span class="pill">${esc(tiler.MODES[T.mode])}</span>
      </div>
      <p class="muted">成品 ${fmt(+$('w').value)} × ${fmt(+$('h').value)} ${unitName(st.unit)}（${other}）·
        每张白边 ${fmt(T.M, 1)} mm${T.mode === 'butt' ? '' : `，重叠 ${fmt(T.O, 1)} mm`}</p>`;
    $('legend').textContent = T.mode === 'butt'
      ? '示意图：每个框是一张纸印的范围，编号 = 第几排-第几张。'
      : '示意图：每个框是一张纸印的范围，编号 = 第几排-第几张；颜色深的是两张重叠的地方。';
    // 第 1 张：重叠粘贴时右边 / 下面是虚线（还有下一张），否则是短黑线
    const t1 = tiler.tiles(T)[0], ov = T.mode !== 'butt';
    const toX = ov && !t1.lastCol ? '虚线' : '右边短黑线', toY = ov && !t1.lastRow ? '虚线' : '下面短黑线';
    const dx = ov && !t1.lastCol ? T.sx : t1.w, dy = ov && !t1.lastRow ? T.sy : t1.h;
    $('measure').textContent = `左边短黑线到${toX} ${fmt(dx, 1)} mm，上面短黑线到${toY} ${fmt(dy, 1)} mm`;

    const warns = [];
    if (st.src) {
      if (st.src.kind === 'image') {
        const r = tiler.resolution(st.src.pxW, st.src.pxH, z.W, z.H);
        const msg = `图片 ${st.src.pxW} × ${st.src.pxH} 像素，印成这个尺寸约 <b>${Math.round(r.dpi)} dpi</b>`;
        warns.push(r.dpi < LOW_DPI
          ? `<p class="note warn">${msg}：低于 ${LOW_DPI} dpi，印出来会糊。可以改用更大的图片，或把尺寸调小。</p>`
          : `<p class="note">${msg}。</p>`);
      } else {
        const zz = tiler.pdfRenderSize(z.W, z.H, st.src.ratio);
        warns.push(`<p class="note">PDF 会按 ${zz.dpi} dpi 转成图片再切（${zz.w} × ${zz.h} 像素）。${st.src.pages > 1 ? `这个 PDF 有 ${st.src.pages} 页，只用第 1 页。` : ''}</p>`);
      }
      const diff = (st.src.ratio / (z.W / z.H)) - 1;
      if (Math.abs(diff) > 0.01) {
        warns.push(`<p class="note warn">图片比例和成品尺寸不一样，图片会被${diff > 0 ? '左右压扁' : '上下压扁'}约 ${Math.round(Math.abs(diff) * 100)}%。
          要照图片比例，就勾「宽、高按图片比例锁在一起」再改宽或高。</p>`);
      }
    }
    $('warn').innerHTML = warns.join('');
    App.render.preview($('preview'), st.src && st.src.thumb, T, {
      fill: hexA(css('--accent'), 0.16), line: css('--accent-ink'), text: css('--accent-ink'),
      halo: css('--surface'), edge: css('--text-2'), empty: css('--surface-2'),
    });
    $('export').disabled = !st.src || st.busy;
    $('export').textContent = `导出 ${P.label} 拼贴 PDF（${T.n} 张）`;
  }

  function hexA(hex, a) {
    const m = /^#?([0-9a-f]{6})$/i.exec(hex);
    if (!m) return `rgba(42,120,214,${a})`;
    const n = parseInt(m[1], 16);
    return `rgba(${n >> 16},${(n >> 8) & 255},${n & 255},${a})`;
  }

  /* ---------- 输入 ---------- */
  function lockFrom(which) {
    if (!$('lock').checked || !st.src) return;
    const w = parseFloat($('w').value), h = parseFloat($('h').value);
    if (which === 'w' && w > 0) $('h').value = fmt(w / st.src.ratio);
    if (which === 'h' && h > 0) $('w').value = fmt(h * st.src.ratio);
  }

  function seg(id, key, after) {
    $(id).querySelectorAll('button').forEach(b => {
      b.onclick = () => {
        if (st.busy) return;
        const prev = st[key];
        st[key] = b.dataset.v;
        $(id).querySelectorAll('button').forEach(x => x.classList.toggle('on', x === b));
        if (after) after(prev);
        refresh();
      };
    });
  }

  async function pick(file) {
    if (!file || st.busy) return;
    $('dropMain').textContent = `正在读取 ${file.name}…`;
    $('dropSub').textContent = App.source.isPdf(file) ? '第一次读 PDF 要几秒' : '大图片要几秒';
    try {
      const src = await App.source.open(file);
      if (st.src) st.src.release();
      st.src = src;
      $('dropMain').textContent = src.fileName;
      $('dropSub').textContent = src.kind === 'image'
        ? `${src.pxW} × ${src.pxH} 像素 · ${mb(file.size)} · 点这里换一张`
        : `PDF 第 1 页 ${fmt(src.ptW / 72)} × ${fmt(src.ptH / 72)} 英寸 · ${mb(file.size)} · 点这里换一张`;
      if (!$('name').value.trim()) $('name').value = file.name.replace(/\.[^.]+$/, '').slice(0, 40);
      if ($('lock').checked) lockFrom('w');
      $('done').hidden = true;
      App.log('info', `选了文件 ${file.name}（${src.kind === 'image' ? `${src.pxW}×${src.pxH} 像素` : `PDF ${src.pages} 页`}，${mb(file.size)}）`);
    } catch (e) {
      $('dropMain').textContent = '点这里选图片，或把文件拖进来';
      $('dropSub').innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      App.log('warn', `文件打不开：${file.name}（${e.message}）`);
    }
    refresh();
  }

  /* ---------- 导出 ---------- */
  function jpg(cv) {
    return new Promise((ok, bad) => cv.toBlob(b => (b ? ok(b) : bad(new Error('压缩 JPG 失败（可能是电脑内存不够）'))), 'image/jpeg', 0.92));
  }

  function download(blob, name) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = name; a.style.display = 'none';
    document.body.append(a); a.click();
    setTimeout(() => { a.remove(); URL.revokeObjectURL(url); }, 60000);
  }

  async function exportPdf() {
    const T = currentPlan();
    if (!T || !st.src || st.busy) return;
    st.busy = true; refresh();
    const prog = $('prog'), name = $('name').value.trim() || '海报';
    const file = tiler.fileName(name, T, new Date());
    $('done').hidden = true;
    const t0 = performance.now();
    try {
      prog.textContent = st.src.kind === 'pdf' ? '把 PDF 转成图片…（大海报要十几秒）' : '准备中…';
      await App.wait(30);
      const src = await st.src.full(T.W, T.H);
      const L = tiler.layout(T, tiler.pageScale(src.dpi));
      const pages = [];
      for (const t of L.tiles) {
        prog.textContent = `切第 ${pages.length + 1} / ${T.n} 张…`;
        let cv = App.render.page(src, T, L, t, name);
        const blob = await jpg(cv);
        cv.width = cv.height = 0; cv = null;   // 切完就丢
        pages.push({ bytes: new Uint8Array(await blob.arrayBuffer()), w: L.pageW, h: L.pageH });
      }
      st.src.release();
      prog.textContent = '组合 PDF…';
      const out = tiler.pdf(pages, `${name}（${T.paperLabel} ${tiler.MODES[T.mode]}）`, tiler.pt(T.pw), tiler.pt(T.ph));
      pages.length = 0;
      const secs = ((performance.now() - t0) / 1000).toFixed(1);
      const what = `${T.n} 张 ${T.paperLabel}（${T.land ? '横放' : '直放'}，${T.rows} 排 × ${T.cols} 张），${mb(out.size)}`;
      let saved = null, why = '';
      if (st.info.canSave) {
        prog.textContent = '保存到 output 文件夹…';
        try {
          saved = await api('/api/save?name=' + encodeURIComponent(file), {
            method: 'POST', body: out, headers: { 'Content-Type': 'application/pdf' }, timeout: 180000, retries: 2,
          });
        } catch (e) { why = e.message; }
      }
      if (!saved) download(out, file);
      prog.textContent = '';
      showDone(saved, file, what, why);
      App.log('info', `导出完成：${file}，${what}，用时 ${secs} 秒${saved ? '' : '（浏览器下载）'}`);
    } catch (e) {
      prog.innerHTML = `<span class="bad">导出失败：${esc(e.message)}</span><br><span class="muted">详细情况看 logs 文件夹里的日志，发给维护的人。</span>`;
      App.log('error', `导出失败：${file}（${e && e.message}）`);
      try { st.src.release(); } catch (x) { /* 已经放掉了 */ }
    } finally {
      st.busy = false; refresh();
    }
  }

  function showDone(saved, file, what, why) {
    const box = $('done');
    box.hidden = false;
    if (saved) {
      box.innerHTML = `<p class="good"><b>已保存：</b>${esc(saved.path)}</p><p>${esc(what)}</p>
        ${st.info.local ? '<button type="button" id="openOut">打开 output 文件夹</button>' : ''}
        <p class="big-note">打印时选「实际大小（Actual Size）/ 100%」，不要选「适合页面（Fit）」。</p>`;
      const b = $('openOut');
      if (b) b.onclick = () => api('/api/open-output', { method: 'POST', retries: 0 }).catch(e => alert(e.message));
    } else {
      box.innerHTML = `<p class="good"><b>已下载：</b>${esc(file)}（在浏览器的「下载」文件夹）</p><p>${esc(what)}</p>
        ${why ? `<p class="muted">没有存进 output 文件夹：${esc(why)}</p>` : ''}
        <p class="big-note">打印时选「实际大小（Actual Size）/ 100%」，不要选「适合页面（Fit）」。</p>`;
    }
  }

  /* ---------- 开始 ---------- */
  async function init() {
    seg('unit', 'unit', prev => {
      if (prev === st.unit) return;
      for (const id of ['w', 'h']) {
        const v = parseFloat($(id).value);
        if (v > 0) $(id).value = fmt(tiler.fromMm(tiler.toMm(v, prev), st.unit));
      }
      $('unitLabel').textContent = unitName(st.unit);
    });
    seg('paper', 'paper');
    seg('mode', 'mode');
    $('w').oninput = () => { lockFrom('w'); refresh(); };
    $('h').oninput = () => { lockFrom('h'); refresh(); };
    $('lock').onchange = () => { lockFrom('w'); refresh(); };
    document.querySelectorAll('.presets button').forEach(b => {
      b.onclick = () => {
        if (st.busy) return;
        $('lock').checked = false;   // 常用尺寸照填，不跟图片比例
        $('w').value = fmt(tiler.fromMm(tiler.toMm(+b.dataset.w, 'in'), st.unit));
        $('h').value = fmt(tiler.fromMm(tiler.toMm(+b.dataset.h, 'in'), st.unit));
        refresh();
      };
    });
    $('file').onchange = e => { pick(e.target.files[0]); e.target.value = ''; };
    const drop = $('drop');
    ['dragenter', 'dragover'].forEach(ev => drop.addEventListener(ev, e => { e.preventDefault(); drop.classList.add('over'); }));
    ['dragleave', 'drop'].forEach(ev => drop.addEventListener(ev, e => { e.preventDefault(); drop.classList.remove('over'); }));
    drop.addEventListener('drop', e => pick(e.dataTransfer.files[0]));
    $('export').onclick = exportPdf;
    window.addEventListener('resize', () => refresh());
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => refresh());
    new MutationObserver(() => refresh()).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });

    refresh();
    try {
      st.info = Object.assign(st.info, await api('/api/info'));
      $('ver').textContent = 'v' + st.info.version;
    } catch (e) {
      $('ver').textContent = 'v?';
      $('prog').innerHTML = `<span class="bad">${esc(e.message)}</span> 导出的 PDF 会改成下载到浏览器的「下载」文件夹。`;
    }
    App.console.init(st.info);
    refresh();
  }

  document.addEventListener('DOMContentLoaded', init);
})((window.App = window.App || {}));

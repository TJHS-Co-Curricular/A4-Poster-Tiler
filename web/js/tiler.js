/* tiler.js —— 切图规则（F6）：要几张纸、每张印海报的哪一块、剪裁线和对齐虚线画在哪、
 * 每张上方的文字、导出文件名、组 PDF。只算数字，不碰网页，所以 Node 测试直接载入这个文件。
 *
 * 规则来源（不要改，改要先问维护者）：
 *   - 设定的尺寸 = 拼好、剪好之后的成品尺寸（维护者定 2026-11-25）
 *   - 拼法：重叠粘贴（维护者定 2026-11-25）；另有「对边拼」选项（维护者定 2026-11-26）
 *   - 白边 8 mm、重叠 10 mm（维护者定 2026-11-25）；可在 config.ini [tile] 改（维护者定 2026-11-26）
 *   - 尺寸照设定，最后一排 / 一列只印用得到的部分，不放大缩小去凑整张（维护者定 2026-11-25）
 *   - 直放、横放两种都算，张数少的那个；一样多用直放（维护者定 2026-11-25）
 *   - 纸张除了 A4 也可以选 A3、Letter（维护者定 2026-11-26）
 * 算法搬自 Award Ceremony public/js/control/poster.js 的 poA4Plan / poA4Export、output.js 的 outPdf。
 *
 * 单位：mm；画到纸上时用「页面像素」（s = 每 mm 几个像素）。
 * 提供：App.tiler。需要：无。
 */
(function (App) {
  'use strict';

  const PAPERS = {
    A4: { key: 'A4', label: 'A4', w: 210, h: 297 },
    A3: { key: 'A3', label: 'A3', w: 297, h: 420 },
    Letter: { key: 'Letter', label: 'Letter', w: 215.9, h: 279.4 },
  };
  const MODES = { overlap: '重叠粘贴', butt: '对边拼' };
  const MM_PER = { in: 25.4, cm: 10 };

  /* 要几张纸。Wmm、Hmm = 成品宽、高；opt = { paper, mode, margin, overlap } */
  function plan(Wmm, Hmm, opt) {
    const o = opt || {};
    const paper = PAPERS[o.paper] || PAPERS.A4;
    const mode = o.mode === 'butt' ? 'butt' : 'overlap';
    const M = o.margin == null ? 8 : +o.margin;
    const O = mode === 'butt' ? 0 : (o.overlap == null ? 10 : +o.overlap);
    if (!(Wmm > 0 && Hmm > 0)) throw new Error('成品尺寸要大于 0');
    const one = (pw, ph) => {
      const cw = pw - 2 * M, ch = ph - 2 * M;   // 一张纸能印海报的区域
      const sx = cw - O, sy = ch - O;           // 每多一张，海报往前推进多少
      if (sx <= 0 || sy <= 0) throw new Error('白边或重叠太大，一张纸放不下');
      const cols = Wmm <= cw ? 1 : Math.ceil((Wmm - cw) / sx - 1e-6) + 1;
      const rows = Hmm <= ch ? 1 : Math.ceil((Hmm - ch) / sy - 1e-6) + 1;
      return { pw, ph, cw, ch, sx, sy, cols, rows, n: cols * rows, land: pw > ph };
    };
    const a = one(paper.w, paper.h), b = one(paper.h, paper.w);
    const T = b.n < a.n ? b : a;                // 张数少的；一样多用直放
    return Object.assign(T, { W: Wmm, H: Hmm, M, O, paper: paper.key, paperLabel: paper.label, mode });
  }

  /* 每张在海报上的位置（mm），从第 0 排第 0 张开始，一排一排往下 */
  function tiles(T) {
    const out = [];
    for (let r = 0; r < T.rows; r++) {
      for (let c = 0; c < T.cols; c++) {
        const x = c * T.sx, y = r * T.sy;
        out.push({
          r, c, x, y, w: Math.min(T.cw, T.W - x), h: Math.min(T.ch, T.H - y),
          first: r === 0 && c === 0, lastCol: c === T.cols - 1, lastRow: r === T.rows - 1,
        });
      }
    }
    return out;
  }

  /* 页面像素的排版。s = 每 mm 几个像素。
   * 整张海报先对齐到像素格（PW × PH），每张从格子上的整数位置开始切：
   * 这样相邻两张重叠的那一条，用的是海报上同一批像素，画面完全一样。 */
  function layout(T, s) {
    const R = Math.round;
    const PW = R(T.W * s), PH = R(T.H * s);
    const px0 = c => R(c * T.sx * s), py0 = r => R(r * T.sy * s);
    const list = tiles(T).map(t => {
      const x0 = px0(t.c), y0 = py0(t.r);
      // 结尾也对齐到像素格：对边拼时刚好接上下一张，不会多一条或少一条像素
      return Object.assign({}, t, {
        px0: x0, py0: y0,
        pwp: Math.min(R((t.c * T.sx + T.cw) * s), PW) - x0,
        php: Math.min(R((t.r * T.sy + T.ch) * s), PH) - y0,
        stepX: t.lastCol ? 0 : px0(t.c + 1) - x0,   // 下一张从这张的哪里开始（对齐虚线的位置）
        stepY: t.lastRow ? 0 : py0(t.r + 1) - y0,
      });
    });
    return { s, PW, PH, pageW: R(T.pw * s), pageH: R(T.ph * s), dM: R(T.M * s), tiles: list };
  }

  /* 一张纸上要画的线（页面像素）：crop = 剪裁线（白边里的短黑线），guides = 对齐虚线。
   * 剪裁线：对准海报这一块的左边、上边；最后一列加右边、最后一排加下边（整张海报的外边）；
   *         对边拼每张四边都要剪，所以四边都画。
   *         只画在纸四周那一圈白边里：从离纸边 1 mm 画到离白边内缘 1.5 mm（和 Award Ceremony 一样）。
   *         最后一排 / 一列海报比较短时，线不会穿过下面 / 右边那片空白，免得被当成剪裁位置。
   * 对齐虚线：重叠粘贴、右边 / 下面还有下一张时，画在重叠那一条的起点（粘好后被下一张盖住）。 */
  function marks(T, L, t) {
    const s = L.s, a = 1 * s, b = 1.5 * s;
    const left = L.dM, top = L.dM, right = L.dM + t.pwp, bottom = L.dM + t.php;
    const butt = T.mode === 'butt';
    const xs = [left].concat(t.lastCol || butt ? [right] : []);
    const ys = [top].concat(t.lastRow || butt ? [bottom] : []);
    const crop = [];
    for (const x of xs) {
      crop.push([x, a, x, top - b]);
      crop.push([x, L.pageH - a, x, Math.max(bottom, L.pageH - L.dM) + b]);
    }
    for (const y of ys) {
      crop.push([a, y, left - b, y]);
      crop.push([L.pageW - a, y, Math.max(right, L.pageW - L.dM) + b, y]);
    }
    const guides = [];
    if (!butt && !t.lastCol) guides.push([left + t.stepX, top, left + t.stepX, bottom]);
    if (!butt && !t.lastRow) guides.push([left, top + t.stepY, right, top + t.stepY]);
    return { crop, guides };
  }

  function joinZh(list) {
    return list.length < 2 ? list.join('') : list.slice(0, -1).join('、') + '和' + list[list.length - 1];
  }

  /* 每张上方白边的文字：tag（粗体编号）+ count、name、instr（灰色小字） */
  function label(T, t, name) {
    const tag = `第 ${t.r + 1} 排 · 第 ${t.c + 1} 张`;
    const count = `（共 ${T.rows} 排 × ${T.cols} 张）`;
    let instr;
    if (T.mode === 'butt') {
      instr = '沿短黑线剪掉四边的白边，和相邻那张边对边贴（背面用胶带）';
    } else {
      const cut = ['左边', '上面'].concat(t.lastCol ? ['右边'] : [], t.lastRow ? ['下面'] : []);
      instr = `沿短黑线剪掉${joinZh(cut)}的白边`;
      if (t.first) instr = '第一张：' + instr;
      else instr += `，对准${t.c && t.r ? '左边和上面' : t.c ? '左边' : '上面'}那张的虚线粘`;
    }
    return { tag, count, name: String(name || '').trim(), instr };
  }

  /* 文件名里不能有的字换成 _（Windows 也能保存） */
  function safeFile(t) {
    return String(t || '').replace(/[\\/:*?"<>|\u0000-\u001f]/g, '_').replace(/\.+$/, '').trim().slice(0, 60) || '海报';
  }
  const pad = n => String(n).padStart(2, '0');

  /* 海报名称-A4拼贴7x4_20261126_1530.pdf（7x4 = 每排 7 张 × 4 排） */
  function fileName(name, T, now) {
    const d = now || new Date();
    const stamp = `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}_${pad(d.getHours())}${pad(d.getMinutes())}`;
    const kind = T.mode === 'butt' ? '对边拼' : '拼贴';
    return `${safeFile(name)}-${T.paperLabel}${kind}${T.cols}x${T.rows}_${stamp}.pdf`;
  }

  /* 图片解析度：实际 dpi（宽、高取小的）和比例差多少（> 0 = 图片比较宽，会被横向压扁） */
  function resolution(srcW, srcH, Wmm, Hmm) {
    const dpi = Math.min(srcW / (Wmm / 25.4), srcH / (Hmm / 25.4));
    return { dpi, ratioDiff: (srcW / srcH) / (Wmm / Hmm) - 1 };
  }

  /* 切出来每张纸用几 dpi：照图片的解析度，最低 150（线和字才清楚）、最高 300（PDF 才不会太大） */
  function pageScale(dpi) {
    return Math.min(300, Math.max(150, dpi || 150)) / 25.4;
  }

  /* PDF 输入转成图片的大小：成品 200 dpi；浏览器画布有上限，太大就降低 dpi */
  function pdfRenderSize(Wmm, Hmm, pageRatio) {
    const MAX_SIDE = 16384, MAX_AREA = 120e6;
    let dpi = 200;
    const size = d => {
      const w = Math.round(Wmm / 25.4 * d);
      return { w, h: Math.round(w / pageRatio) };
    };
    let z = size(dpi);
    const k = Math.min(1, MAX_SIDE / z.w, MAX_SIDE / z.h, Math.sqrt(MAX_AREA / (z.w * z.h)));
    if (k < 1) { dpi = Math.floor(dpi * k); z = size(dpi); }
    return { dpi, w: z.w, h: z.h };
  }

  function toMm(v, unit) { return +v * (MM_PER[unit] || 25.4); }
  function fromMm(mm, unit) { return mm / (MM_PER[unit] || 25.4); }

  /* 最简单的 PDF：每页一张 JPG（DCTDecode，原样放进去，不重新压缩）。
   * pages = [{ bytes: Uint8Array, w, h }]；W、H = 页面大小（pt）。回传 Blob。 */
  function pdf(pages, title, W, H) {
    const enc = new TextEncoder(), parts = [], xref = [];
    let off = 0;
    const push = x => { const b = typeof x === 'string' ? enc.encode(x) : x; parts.push(b); off += b.length; };
    const obj = (n, ...body) => { xref[n] = off; push(`${n} 0 obj\n`); body.forEach(push); push('\nendobj\n'); };
    const N = pages.length;
    const num = v => (Math.round(v * 100) / 100).toString();
    const utf16 = t => '<FEFF' + Array.from(String(t)).map(ch => {
      const u = ch.codePointAt(0);
      if (u < 0x10000) return u.toString(16).padStart(4, '0');
      const v = u - 0x10000;
      return ((v >> 10) + 0xd800).toString(16) + ((v & 0x3ff) + 0xdc00).toString(16);
    }).join('').toUpperCase() + '>';
    push('%PDF-1.4\n'); push(new Uint8Array([37, 226, 227, 207, 211, 10]));   // 二进制标记
    obj(1, '<< /Type /Catalog /Pages 2 0 R >>');
    obj(2, `<< /Type /Pages /Count ${N} /Kids [${pages.map((_, i) => `${4 + i * 3} 0 R`).join(' ')}] >>`);
    obj(3, `<< /Title ${utf16(title)} /Producer (A4-Poster-Tiler) >>`);
    pages.forEach((p, i) => {
      const pg = 4 + i * 3, ct = pg + 1, im = pg + 2, draw = `q ${num(W)} 0 0 ${num(H)} 0 0 cm /Im0 Do Q`;
      obj(pg, `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${num(W)} ${num(H)}] /Resources << /XObject << /Im0 ${im} 0 R >> >> /Contents ${ct} 0 R >>`);
      obj(ct, `<< /Length ${draw.length} >>\nstream\n${draw}\nendstream`);
      obj(im, `<< /Type /XObject /Subtype /Image /Width ${p.w} /Height ${p.h} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${p.bytes.length} >>\nstream\n`, p.bytes, '\nendstream');
    });
    const start = off, size = 4 + N * 3;
    push(`xref\n0 ${size}\n0000000000 65535 f \n` + Array.from({ length: size - 1 }, (_, i) => String(xref[i + 1]).padStart(10, '0') + ' 00000 n \n').join(''));
    push(`trailer\n<< /Size ${size} /Root 1 0 R /Info 3 0 R >>\nstartxref\n${start}\n%%EOF\n`);
    return new Blob(parts, { type: 'application/pdf' });
  }

  const pt = mm => mm / 25.4 * 72;

  App.tiler = { PAPERS, MODES, plan, tiles, layout, marks, label, safeFile, fileName, resolution, pageScale, pdfRenderSize, toMm, fromMm, pdf, pt };
})((typeof window !== 'undefined' ? (window.App = window.App || {}) : (globalThis.App = globalThis.App || {})));

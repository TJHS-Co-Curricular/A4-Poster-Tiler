/* render.js —— 画图（F10 导出用）：画一张纸、画示意图、画「怎么贴」的小图。
 * 数字都来自 tiler.js；这里只负责把它们画到画布上。
 * 提供：App.render。需要：tiler.js。 */
(function (App) {
  'use strict';

  const FONT = '"Microsoft YaHei", "PingFang SC", "Noto Sans SC", system-ui, sans-serif';
  // 打印用的颜色：纸是白的、线是黑的（不跟网页主题变）
  const PAPER = '#ffffff', INK = '#000000', INK_2 = '#444444';

  /* 画一张纸。src = { img, w, h }（整张海报的原图），回传画布 */
  function page(src, T, L, t, name) {
    const s = L.s, mm = v => v * s;
    const cv = document.createElement('canvas');
    cv.width = L.pageW; cv.height = L.pageH;
    const g = cv.getContext('2d');
    g.fillStyle = PAPER; g.fillRect(0, 0, cv.width, cv.height);

    // 海报这一块：海报像素格 → 原图像素（宽、高各自换算，比例不同时会拉伸）
    const rx = src.w / L.PW, ry = src.h / L.PH;
    g.imageSmoothingEnabled = true; g.imageSmoothingQuality = 'high';
    g.drawImage(src.img, t.px0 * rx, t.py0 * ry, t.pwp * rx, t.php * ry, L.dM, L.dM, t.pwp, t.php);

    const m = App.tiler.marks(T, L, t);
    // 剪裁线：白边里的短黑线
    g.strokeStyle = INK; g.lineWidth = Math.max(1, mm(0.25)); g.lineCap = 'butt';
    g.beginPath();
    for (const [x1, y1, x2, y2] of m.crop) { g.moveTo(x1, y1); g.lineTo(x2, y2); }
    g.stroke();
    // 对齐虚线：黑白相间，先白后黑错开半格，深色、浅色背景都看得到
    if (m.guides.length) {
      g.save();
      g.setLineDash([mm(2), mm(1.5)]); g.lineWidth = Math.max(1, mm(0.35));
      g.beginPath();
      for (const [x1, y1, x2, y2] of m.guides) { g.moveTo(x1, y1); g.lineTo(x2, y2); }
      g.strokeStyle = 'rgba(255,255,255,.9)'; g.stroke();
      g.strokeStyle = 'rgba(0,0,0,.9)'; g.lineDashOffset = mm(1.75); g.stroke();
      g.restore();
    }
    text(g, T, L, t, name);
    return cv;
  }

  /* 上方白边的文字：粗体「第 r 排 · 第 c 张」+ 灰色小字（共几排几张、海报名称、怎么剪怎么贴） */
  function text(g, T, L, t, name) {
    const s = L.s, lab = App.tiler.label(T, t, name);
    const x0 = L.dM + 3 * s, y = L.dM / 2;
    // 右边有剪裁线时，文字尽量不要压到它
    const rightMark = (t.lastCol || T.mode === 'butt') ? L.dM + t.pwp - 2 * s : Infinity;
    const full = L.pageW - L.dM - x0;
    const maxW = rightMark - x0 >= 70 * s ? Math.min(full, rightMark - x0) : full;
    g.textBaseline = 'middle'; g.textAlign = 'left';
    g.fillStyle = INK; g.font = `700 ${3.2 * s}px ${FONT}`;
    g.fillText(lab.tag, x0, y);
    const tagW = g.measureText(lab.tag).width + 2.5 * s;
    g.fillStyle = INK_2; g.font = `${2.6 * s}px ${FONT}`;
    const room = maxW - tagW;
    let note = `${lab.count}${lab.name}　${lab.instr}`;
    if (g.measureText(note).width > room) note = `${lab.count}${lab.instr}`;   // 放不下就先不写海报名称
    g.fillText(note, x0 + tagW, y, Math.max(room, 10 * s));
  }

  /* 示意图：海报缩小，画出每张纸印的范围和编号；重叠的地方颜色深一点 */
  function preview(cv, thumb, T, colors) {
    const box = cv.parentElement.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    const maxW = Math.max(200, box.width), maxH = Math.max(200, Math.min(560, window.innerHeight * 0.6));
    const k = Math.min(maxW / T.W, maxH / T.H);
    const cssW = Math.round(T.W * k), cssH = Math.round(T.H * k);
    cv.style.width = cssW + 'px'; cv.style.height = cssH + 'px';
    cv.width = Math.round(cssW * dpr); cv.height = Math.round(cssH * dpr);
    const g = cv.getContext('2d');
    const z = k * dpr;   // 每 mm 几个画布像素
    g.clearRect(0, 0, cv.width, cv.height);
    if (thumb) { g.imageSmoothingQuality = 'high'; g.drawImage(thumb, 0, 0, cv.width, cv.height); }
    else { g.fillStyle = colors.empty; g.fillRect(0, 0, cv.width, cv.height); }
    const list = App.tiler.tiles(T);
    g.fillStyle = colors.fill;
    for (const t of list) g.fillRect(t.x * z, t.y * z, t.w * z, t.h * z);   // 半透明叠起来：重叠处较深
    g.strokeStyle = colors.line; g.lineWidth = Math.max(1, dpr);
    for (const t of list) g.strokeRect(t.x * z + 0.5, t.y * z + 0.5, t.w * z - 1, t.h * z - 1);
    const fs = Math.max(10 * dpr, Math.min(T.sx, T.sy) * z * 0.16);
    g.font = `700 ${fs}px ${FONT}`; g.textAlign = 'center'; g.textBaseline = 'middle';
    for (const t of list) {
      const cx = (t.x + t.w / 2) * z, cy = (t.y + t.h / 2) * z, label = `${t.r + 1}-${t.c + 1}`;
      g.lineWidth = fs / 4; g.strokeStyle = colors.halo; g.strokeText(label, cx, cy);
      g.fillStyle = colors.text; g.fillText(label, cx, cy);
    }
    g.strokeStyle = colors.edge; g.lineWidth = 2 * dpr; g.strokeRect(dpr, dpr, cv.width - 2 * dpr, cv.height - 2 * dpr);
  }

  /* 「怎么剪、怎么贴」的小图（SVG 字串；颜色用 CSS 变量） */
  function figure(mode) {
    if (mode === 'butt') {
      return `<svg viewBox="0 0 300 150" role="img" aria-label="对边拼：剪掉四边白边，边对边贴">
  <rect class="sheet" x="10" y="25" width="120" height="100"/><rect class="art" x="20" y="35" width="100" height="80"/>
  <rect class="sheet" x="170" y="25" width="120" height="100"/><rect class="art" x="180" y="35" width="100" height="80"/>
  <path class="cut" d="M20 27v6M120 27v6M20 117v6M120 117v6M12 35h6M12 115h6M122 35h6M122 115h6"/>
  <path class="cut" d="M180 27v6M280 27v6M180 117v6M280 117v6M172 35h6M172 115h6M282 35h6M282 115h6"/>
  <path class="arrow" d="M135 75h30m-8-6 8 6-8 6"/>
  <text x="70" y="16">剪掉四边白边</text><text x="230" y="16">边对边贴</text>
  <text x="150" y="145">背面用胶带贴牢</text></svg>`;
    }
    return `<svg viewBox="0 0 300 150" role="img" aria-label="重叠粘贴：剪掉左边和上面的白边，压在前一张的虚线上">
  <rect class="sheet" x="10" y="25" width="150" height="110"/><rect class="art" x="20" y="35" width="130" height="90"/>
  <line class="dash" x1="132" y1="35" x2="132" y2="125"/>
  <rect class="overlap" x="132" y="35" width="18" height="90"/>
  <rect class="art top" x="132" y="35" width="130" height="90"/>
  <path class="cut" d="M20 27v6M12 35h6M12 125h6"/>
  <path class="arrow" d="M200 140h-60m8-6-8 6 8 6"/>
  <text x="85" y="16">第 1 张</text><text x="210" y="16">第 2 张（剪掉左边白边）</text>
  <text x="225" y="147">左边对准虚线，压上去粘</text></svg>`;
  }

  App.render = { page, preview, figure };
})((window.App = window.App || {}));

'use strict';
// 切图规则测试（node --test tests/）。直接载入网页用的 web/js/tiler.js，确保测的就是浏览器跑的那份。
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

require(path.join(__dirname, '..', 'web', 'js', 'tiler.js'));
const T = globalThis.App.tiler;
const IN = 25.4;
const opt = { paper: 'A4', mode: 'overlap', margin: 8, overlap: 10 };

// 检查数字：docs/a4-tile-prompt.md 第 4 节（维护者 2026-11-25，Award Ceremony 实际在用、验证过）
const CASES = [
  { w: 24, h: 43, n: 16, land: false, rows: 4, cols: 4 },
  { w: 51, h: 43, n: 28, land: false, rows: 4, cols: 7 },
  { w: 11, h: 43, n: 6, land: true, rows: 6, cols: 1 },
  { w: 8, h: 10, n: 2, land: false, rows: 1, cols: 2 },   // 直放、横放都是 2 张，取直放
];

for (const c of CASES) {
  test(`检查数字：${c.w} × ${c.h} 英寸 → ${c.n} 张，${c.land ? '横放' : '直放'}，${c.rows} 排 × ${c.cols} 张`, () => {
    const p = T.plan(c.w * IN, c.h * IN, opt);
    assert.equal(p.n, c.n);
    assert.equal(p.land, c.land);
    assert.equal(p.rows, c.rows);
    assert.equal(p.cols, c.cols);
  });
}

test('一样多张时用直放（维护者定 2026-11-25）', () => {
  // 8 × 10 英寸：直放 1 排 × 2 张、横放 2 排 × 1 张，都是 2 张
  const p = T.plan(8 * IN, 10 * IN, opt);
  assert.equal(p.land, false);
  assert.equal(p.pw, 210);
});

// 很多尺寸、三种纸、两种拼法都要符合的性质
function* sizes() {
  for (const paper of ['A4', 'A3', 'Letter']) {
    for (const mode of ['overlap', 'butt']) {
      for (const w of [5, 8, 11, 17.3, 24, 36, 51, 72]) {
        for (const h of [6, 10, 23.4, 43, 60]) yield { paper, mode, w, h };
      }
    }
  }
}

test('拼回去的总尺寸 = 设定尺寸（mm 和像素都对）', () => {
  for (const z of sizes()) {
    const p = T.plan(z.w * IN, z.h * IN, Object.assign({}, opt, z));
    const list = T.tiles(p);
    const lastC = list.filter(t => t.lastCol)[0], lastR = list.filter(t => t.lastRow)[0];
    assert.ok(Math.abs(lastC.x + lastC.w - p.W) < 1e-9, `宽 ${JSON.stringify(z)}`);
    assert.ok(Math.abs(lastR.y + lastR.h - p.H) < 1e-9, `高 ${JSON.stringify(z)}`);
    const L = T.layout(p, T.pageScale(200));
    for (const t of L.tiles) {
      if (t.lastCol) assert.equal(t.px0 + t.pwp, L.PW);
      if (t.lastRow) assert.equal(t.py0 + t.php, L.PH);
    }
  }
});

test('最后一排 / 一列不超出海报，也不超出纸能印的区域', () => {
  for (const z of sizes()) {
    const p = T.plan(z.w * IN, z.h * IN, Object.assign({}, opt, z));
    for (const t of T.tiles(p)) {
      assert.ok(t.w > 0 && t.h > 0);
      assert.ok(t.x + t.w <= p.W + 1e-9 && t.y + t.h <= p.H + 1e-9);
      assert.ok(t.w <= p.cw + 1e-9 && t.h <= p.ch + 1e-9);
    }
    const L = T.layout(p, T.pageScale(150));
    for (const t of L.tiles) {
      assert.ok(t.px0 + t.pwp <= L.PW && t.py0 + t.php <= L.PH);
      assert.ok(L.dM + t.pwp <= L.pageW - L.dM + 1, '右边白边至少还在');
      assert.ok(L.dM + t.php <= L.pageH - L.dM + 1, '下面白边至少还在');
    }
  }
});

test('张数是最少的：少一排或少一列就拼不满', () => {
  for (const z of sizes()) {
    const p = T.plan(z.w * IN, z.h * IN, Object.assign({}, opt, z));
    if (p.cols > 1) assert.ok((p.cols - 2) * p.sx + p.cw < p.W - 1e-6);
    if (p.rows > 1) assert.ok((p.rows - 2) * p.sy + p.ch < p.H - 1e-6);
  }
});

test('相邻两张重叠的那一条，用的是海报上同一批像素（画面完全一样）', () => {
  for (const dpi of [72, 150, 203, 300]) {
    const p = T.plan(51 * IN, 43 * IN, opt);
    const L = T.layout(p, T.pageScale(dpi));
    const at = (r, c) => L.tiles.find(t => t.r === r && t.c === c);
    for (const a of L.tiles) {
      const right = !a.lastCol && at(a.r, a.c + 1), down = !a.lastRow && at(a.r + 1, a.c);
      if (right) {
        // 重叠宽度 ≈ 10 mm（像素格取整，误差 1 像素以内）
        const band = a.px0 + a.pwp - right.px0;
        assert.ok(Math.abs(band - 10 * L.s) <= 1, `重叠 ${band} px`);
        // a 的第 (stepX + i) 列 和 right 的第 i 列，是海报上同一列像素
        for (let i = 0; i < band; i++) assert.equal(a.px0 + a.stepX + i, right.px0 + i);
        assert.equal(a.py0, right.py0); assert.equal(a.php, right.php);
      }
      if (down) {
        const band = a.py0 + a.php - down.py0;
        assert.ok(Math.abs(band - 10 * L.s) <= 1);
        for (let i = 0; i < band; i++) assert.equal(a.py0 + a.stepY + i, down.py0 + i);
        assert.equal(a.px0, down.px0); assert.equal(a.pwp, down.pwp);
      }
    }
  }
});

test('对边拼：不重叠，每张接在上一张的结尾', () => {
  const p = T.plan(51 * IN, 43 * IN, Object.assign({}, opt, { mode: 'butt' }));
  assert.equal(p.O, 0);
  const L = T.layout(p, T.pageScale(200));
  for (const t of L.tiles) {
    const right = L.tiles.find(u => u.r === t.r && u.c === t.c + 1);
    if (right) assert.equal(t.px0 + t.pwp, right.px0);
  }
});

test('剪裁线只画在白边里，离纸边 1 mm、离海报边 1.5 mm', () => {
  for (const mode of ['overlap', 'butt']) {
    const p = T.plan(51 * IN, 43 * IN, Object.assign({}, opt, { mode }));
    const L = T.layout(p, T.pageScale(200));
    const s = L.s, eps = 1e-6;
    for (const t of L.tiles) {
      const box = { l: L.dM, t: L.dM, r: L.dM + t.pwp, b: L.dM + t.php };
      const m = T.marks(p, L, t);
      for (const [x1, y1, x2, y2] of m.crop) {
        // 线段不进入海报（往外留 1.5 mm）
        const inX = Math.max(x1, x2) > box.l - 1.5 * s + eps && Math.min(x1, x2) < box.r + 1.5 * s - eps;
        const inY = Math.max(y1, y2) > box.t - 1.5 * s + eps && Math.min(y1, y2) < box.b + 1.5 * s - eps;
        assert.ok(!(inX && inY), `剪裁线进了海报：${[x1, y1, x2, y2]}`);
        // 只在纸四周那一圈白边里（最后一排 / 一列较短时，不穿过空白）
        if (x1 === x2) assert.ok(Math.max(y1, y2) <= L.dM + eps || Math.min(y1, y2) >= L.pageH - L.dM - eps);
        else assert.ok(Math.max(x1, x2) <= L.dM + eps || Math.min(x1, x2) >= L.pageW - L.dM - eps);
        for (const v of [x1, x2]) assert.ok(v >= 1 * s - eps && v <= L.pageW - 1 * s + eps);
        for (const v of [y1, y2]) assert.ok(v >= 1 * s - eps && v <= L.pageH - 1 * s + eps);
      }
      const xs = new Set(m.crop.filter(l => l[0] === l[2]).map(l => l[0]));
      const ys = new Set(m.crop.filter(l => l[1] === l[3]).map(l => l[1]));
      assert.ok(xs.has(box.l) && ys.has(box.t), '每张都有左边、上边');
      assert.equal(xs.has(box.r), t.lastCol || mode === 'butt', '右边：最后一列 / 对边拼才画');
      assert.equal(ys.has(box.b), t.lastRow || mode === 'butt', '下边：最后一排 / 对边拼才画');
    }
  }
});

test('对齐虚线：重叠粘贴画在下一张开始的地方；对边拼不画', () => {
  const p = T.plan(51 * IN, 43 * IN, opt);
  const L = T.layout(p, T.pageScale(200));
  for (const t of L.tiles) {
    const g = T.marks(p, L, t).guides;
    assert.equal(g.length, (t.lastCol ? 0 : 1) + (t.lastRow ? 0 : 1));
    for (const [x1, y1, x2, y2] of g) {
      if (x1 === x2) { assert.equal(x1, L.dM + t.stepX); assert.ok(Math.abs(t.stepX - p.sx * L.s) <= 1); }
      else { assert.equal(y1, L.dM + t.stepY); assert.equal(y1, y2); }
    }
  }
  const b = T.plan(51 * IN, 43 * IN, Object.assign({}, opt, { mode: 'butt' }));
  const Lb = T.layout(b, T.pageScale(200));
  for (const t of Lb.tiles) assert.equal(T.marks(b, Lb, t).guides.length, 0);
});

test('每张上方的文字', () => {
  const p = T.plan(51 * IN, 43 * IN, opt);
  const list = T.tiles(p);
  const first = T.label(p, list[0], '颁奖典礼');
  assert.equal(first.tag, '第 1 排 · 第 1 张');
  assert.equal(first.count, '（共 4 排 × 7 张）');
  assert.equal(first.instr, '第一张：沿短黑线剪掉左边和上面的白边');
  const second = T.label(p, list[1], '颁奖典礼');
  assert.equal(second.instr, '沿短黑线剪掉左边和上面的白边，对准左边那张的虚线粘');
  const last = T.label(p, list[list.length - 1], '');
  assert.equal(last.tag, '第 4 排 · 第 7 张');
  assert.equal(last.instr, '沿短黑线剪掉左边、上面、右边和下面的白边，对准左边和上面那张的虚线粘');
  const b = T.plan(51 * IN, 43 * IN, Object.assign({}, opt, { mode: 'butt' }));
  assert.match(T.label(b, T.tiles(b)[3], '').instr, /四边/);
});

test('文件名：海报名称-A4拼贴7x4_20261126_1530.pdf', () => {
  const p = T.plan(51 * IN, 43 * IN, opt);
  const d = new Date(2026, 10, 26, 15, 30);
  assert.equal(T.fileName('2026 颁奖典礼', p, d), '2026 颁奖典礼-A4拼贴7x4_20261126_1530.pdf');
  assert.equal(T.fileName('a/b:c', p, d), 'a_b_c-A4拼贴7x4_20261126_1530.pdf');
  assert.equal(T.fileName('', p, d), '海报-A4拼贴7x4_20261126_1530.pdf');
  const b = T.plan(51 * IN, 43 * IN, { paper: 'A3', mode: 'butt', margin: 8, overlap: 10 });
  assert.match(T.fileName('x', b, d), /^x-A3对边拼\d+x\d+_20261126_1530\.pdf$/);
});

test('白边、重叠可以改（config.ini [tile]）', () => {
  const a = T.plan(51 * IN, 43 * IN, { margin: 12, overlap: 15 });
  assert.equal(a.M, 12); assert.equal(a.O, 15);
  assert.equal(a.cw, a.pw - 24); assert.equal(a.sx, a.pw - 24 - 15);
  assert.throws(() => T.plan(51 * IN, 43 * IN, { margin: 100, overlap: 10 }), /放不下/);
});

test('解析度：像素 ÷ 成品英寸 = dpi', () => {
  const r = T.resolution(10200, 8600, 51 * IN, 43 * IN);
  assert.equal(Math.round(r.dpi), 200);
  assert.ok(Math.abs(r.ratioDiff) < 0.001);
  assert.equal(T.pageScale(50), 150 / 25.4);
  assert.equal(T.pageScale(600), 300 / 25.4);
});

test('PDF 转图片：一般 200 dpi，太大时降低，不超过画布上限', () => {
  assert.equal(T.pdfRenderSize(51 * IN, 43 * IN, 51 / 43).dpi, 200);
  const big = T.pdfRenderSize(120 * IN, 80 * IN, 1.5);
  assert.ok(big.dpi < 200 && big.w <= 16384 && big.h <= 16384 && big.w * big.h <= 120e6);
});

test('PDF：页数、页面大小、xref 位置都正确', async () => {
  const fake = n => ({ bytes: new Uint8Array([0xff, 0xd8, n, 0xff, 0xd9]), w: 10, h: 14 });
  const blob = T.pdf([fake(1), fake(2), fake(3)], '测试甲 海报', T.pt(210), T.pt(297));
  const buf = Buffer.from(await blob.arrayBuffer());
  const s = buf.toString('latin1');
  assert.ok(s.startsWith('%PDF-1.4'));
  assert.match(s, /\/Count 3 /);
  assert.equal((s.match(/\/MediaBox \[0 0 595\.28 841\.89\]/g) || []).length, 3);
  const start = +/startxref\n(\d+)/.exec(s)[1];
  assert.ok(s.slice(start).startsWith('xref'));
  const rows = s.slice(start).split('\n').slice(3, 3 + 12);
  rows.forEach((row, i) => {
    const at = +row.slice(0, 10);
    assert.ok(s.slice(at).startsWith(`${i + 1} 0 obj`), `第 ${i + 1} 个物件的位置`);
  });
  assert.ok(buf.includes(Buffer.from([0xff, 0xd8, 2, 0xff, 0xd9])), 'JPG 原样放进去');
});

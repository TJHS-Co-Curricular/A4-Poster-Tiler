/* source.js —— 读取老师选的文件（F5）：JPG / PNG 用浏览器解码一次；PDF 用 pdf.js 读第 1 页。
 * 回传的 Source：
 *   { kind: 'image' | 'pdf', fileName, pxW, pxH, ratio, pages, thumb(画布，预览用),
 *     full(Wmm, Hmm, onProg) → Promise<{ img, w, h, dpi }>  导出时用的整张图（图片：原图；PDF：照成品尺寸转出来）,
 *     release() 导出后放掉 PDF 转出来的大画布 }
 * 提供：App.source。需要：tiler.js（pdfRenderSize）；web/vendor/pdfjs/（只在选 PDF 时才载入）。 */
(function (App) {
  'use strict';

  const THUMB = 1600;   // 预览图最长边（像素）
  let pdfjs = null;

  function makeThumb(img, w, h) {
    const k = Math.min(1, THUMB / Math.max(w, h));
    const cv = document.createElement('canvas');
    cv.width = Math.max(1, Math.round(w * k)); cv.height = Math.max(1, Math.round(h * k));
    const g = cv.getContext('2d');
    g.imageSmoothingQuality = 'high';
    g.drawImage(img, 0, 0, cv.width, cv.height);
    return cv;
  }

  function isPdf(file) {
    return /\.pdf$/i.test(file.name) || file.type === 'application/pdf';
  }

  async function open(file) {
    return isPdf(file) ? openPdf(file) : openImage(file);
  }

  async function openImage(file) {
    if (!/^image\/(jpeg|png)$/.test(file.type) && !/\.(jpe?g|png)$/i.test(file.name)) {
      throw new Error('只收 JPG、PNG 或 PDF 文件');
    }
    let bmp;
    try {
      bmp = await createImageBitmap(file);   // 原图只解码这一次
    } catch (e) {
      throw new Error('图片打不开：文件可能坏了，或图片太大（超过浏览器能处理的大小）');
    }
    const w = bmp.width, h = bmp.height;
    return {
      kind: 'image', fileName: file.name, pxW: w, pxH: h, ratio: w / h, pages: 1,
      thumb: makeThumb(bmp, w, h),
      full: async (Wmm, Hmm) => ({ img: bmp, w, h, dpi: App.tiler.resolution(w, h, Wmm, Hmm).dpi }),
      release() {},
    };
  }

  async function loadPdfjs() {
    if (pdfjs) return pdfjs;
    try {
      pdfjs = await import('/vendor/pdfjs/pdf.min.mjs');
    } catch (e) {
      throw new Error('PDF 工具载入失败，请按 Ctrl+F5 重新整理再试（或改选 JPG / PNG）');
    }
    pdfjs.GlobalWorkerOptions.workerSrc = '/vendor/pdfjs/pdf.worker.min.mjs';
    return pdfjs;
  }

  async function renderPage(page, w, h) {
    const vp0 = page.getViewport({ scale: 1 });
    const vp = page.getViewport({ scale: w / vp0.width });
    const cv = document.createElement('canvas');
    cv.width = w; cv.height = h;
    const g = cv.getContext('2d');
    if (!g) throw new Error('浏览器画布不够大，请把成品尺寸调小或改用图片');
    g.fillStyle = '#fff'; g.fillRect(0, 0, w, h);   // PDF 透明的地方印成白色
    await page.render({ canvasContext: g, viewport: vp, background: 'rgb(255,255,255)' }).promise;
    return cv;
  }

  async function openPdf(file) {
    const lib = await loadPdfjs();
    let doc;
    try {
      doc = await lib.getDocument({ data: new Uint8Array(await file.arrayBuffer()), isOffscreenCanvasSupported: true }).promise;
    } catch (e) {
      if (e && e.name === 'PasswordException') throw new Error('这个 PDF 有密码，请先另存为没有密码的 PDF');
      throw new Error('PDF 打不开：文件可能坏了');
    }
    const page = await doc.getPage(1);
    const vp = page.getViewport({ scale: 1 });   // pt
    const ratio = vp.width / vp.height;
    const tw = Math.round(ratio >= 1 ? THUMB : THUMB * ratio);
    const thumb = await renderPage(page, tw, Math.round(tw / ratio));
    let big = null;
    return {
      kind: 'pdf', fileName: file.name, pxW: 0, pxH: 0, ptW: vp.width, ptH: vp.height, ratio, pages: doc.numPages,
      thumb,
      async full(Wmm, Hmm) {
        const z = App.tiler.pdfRenderSize(Wmm, Hmm, ratio);
        big = await renderPage(page, z.w, z.h);
        return { img: big, w: z.w, h: z.h, dpi: z.dpi };
      },
      release() { if (big) { big.width = big.height = 0; big = null; } },
    };
  }

  App.source = { open, isPdf };
})((window.App = window.App || {}));

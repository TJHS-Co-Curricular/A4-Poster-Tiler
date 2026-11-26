# CLAUDE.md — A4 拼贴海报（A4-Poster-Tiler）

> 通用规范在全局规则（`~/.claude/rules/zhijie-standard/`），这里**只写本项目特有、或和全局不同的内容**。
> 版本记录见 `CHANGELOG.md`；当前版本见根目录 `VERSION`。原始需求：@docs/a4-tile-prompt.md

## 1. 项目一句话

把一张大图片（JPG / PNG / PDF 第 1 页）按成品尺寸切成很多张 A4（或 A3 / Letter），画好剪裁线、对齐虚线和编号，组成 PDF 存进 `output/`，打包成单一 exe 给老师用。

## 2. 技术栈

| 项目 | 内容 |
|---|---|
| 服务端 | Go 1.22+，只用标准库（`cmd/a4-poster-tiler` + `internal/`） |
| 前端 | 原生 HTML / CSS / JS，命名空间 `App` |
| 第三方 | pdf.js 6.5.438 legacy build（`web/vendor/pdfjs/`，Apache-2.0，维护者同意 2026-11-26） |
| 测试 | `go test ./...` + `node --test tests/`（`build.bat` 两个都跑） |
| 默认端口 | 5030 |
| exe 大小 | 约 9 MB |

## 3. 目录结构

```
cmd/a4-poster-tiler/main.go   F1 启动：横幅、端口顺延、单一执行个体、独立窗口、关窗自动结束
internal/settings/            F3 读 config.ini（只补不改）
internal/server/              F9/F11 路由：网页、/api/save（存 PDF）、/api/log
web/js/tiler.js               ★ F6 切图规则：张数、位置、剪裁线、文字、文件名、PDF（Node 测试直接载入）
web/js/render.js              F10 画一张纸、示意图
web/js/source.js              F5 读图片 / PDF
web/js/app.js                 页面
web/js/console.js             「控制台」面板、心跳
tests/tiler.test.js           ★ 规则测试（检查数字表）
tests/structure_test.go       结构测试
```

**切图、画线、组 PDF 都在浏览器做**；服务端只提供网页、存 PDF、记日志。

## 4. 领域知识

- 坐标单位 mm；画到纸上用「页面像素」s = 每 mm 几个像素（照图片 dpi，夹在 150 ~ 300 dpi）。
- 整张海报先对齐到像素格，每张从整数像素开始切 → 相邻两张重叠那一条是同一批像素（测试锁住）。
- 图片比例和成品尺寸不同时会拉伸（页面会提示压扁多少%）。
- PDF 输入：第 1 页按成品 200 dpi 转成图片（太大时降低，画布上限 16384 边 / 1.2 亿像素）。没有附 cmaps / 标准字体：没内嵌字体的 PDF 文字可能显示不对。

## 5. 已确认的规则

| 项目 | 决定 | 来源 |
|---|---|---|
| 设定尺寸 = 拼好剪好后的成品尺寸 | — | 维护者定（2026-11-25） |
| 拼法 | 重叠粘贴（默认）；另有「对边拼」选项 | 维护者定（2026-11-25、2026-11-26） |
| 白边 / 重叠 | 8 mm / 10 mm，可在 config.ini `[tile]` 改 | 维护者定（2026-11-25、2026-11-26） |
| 最后一排 / 一列 | 只印用到的部分，不放大缩小 | 维护者定（2026-11-25） |
| 直放 / 横放 | 张数少的；一样多用直放 | 维护者定（2026-11-25） |
| 纸张 | A4、A3、Letter | 维护者定（2026-11-26） |
| 输入 | JPG、PNG、PDF（第 1 页） | 维护者定（2026-11-26） |
| 交付 | 单一 exe + 浏览器 | 维护者定（2026-11-26） |

**还没得到答复的：**
- 剪裁线的长度：现在只画在纸四周那圈白边里（和 Award Ceremony 一样）。docs 写「画到离海报边 1.5 mm」，
  照字面做时最后一排 / 一列较短的那张，线会穿过下面 / 右边整片空白，所以先照 Award Ceremony（2026-11-26 实作时的解读）。
- 每张上方的剪法文字：每张都写要剪「左边和上面」（加上最后一列的右边、最后一排的下面），
  因为每张都有这几条短黑线；Award Ceremony 原本第 1 排其他张只写「剪掉左边」。
- Award Ceremony 里的「导出 A4 拼贴 PDF」按钮要保留，还是改成只导出 JPG 再用本程序切？

## 6. 本项目例外

- 没有 `Result/`、`data/`：程序不读名单，也不保存手动调整。
- **没有黑色命令窗口、不用一般浏览器分页**（维护者定 2026-11-26，全局 `40-build-deploy.md` 的例外）：
  - 用 `-H windowsgui` 打包；页面用 Edge / Chrome 的 `--app` 应用模式开独立窗口（找不到就用默认浏览器）。
  - 窗口内容改在网页右上角「控制台」看（`/api/console`，logx 留最近 500 行）；无法启动时用对话框（`platform.MessageBox`）。
  - 结束：只开本机时，网页每 15 秒心跳 `/__ping`，关窗送 `/api/bye`，20 秒内没有新心跳就结束；3 分钟没心跳也结束。
    access = lan 时不会自动结束，要在「控制台」按「结束程序」。
  - `go run` 时仍有命令窗口，内容照样印出来。
- 网页页脚只写 `A4 拼贴海报 · v版本号`，不写「© 年份 循人中学」（维护者定 2026-11-26）。
- 浏览器端用 ES module：只有 pdf.js（`import('/vendor/pdfjs/pdf.min.mjs')`，选 PDF 时才载入）；自己的代码仍是普通脚本。服务端对 `.mjs` 回 `text/javascript`。
- exe「属性」里还没有版本信息（Go 需要另外的资源工具）；版本号显示在「控制台」的横幅、网页页脚、日志第一行（go run 时也在命令窗口标题）。

## 7. 绝对不能改

1. `config.ini` 已交付的键名：`[server]` `[paths]` `[log]` `[tile] margin_mm / overlap_mm`
2. 导出文件名格式：`海报名称-A4拼贴7x4_YYYYMMDD_HHMM.pdf`（7x4 = 每排张数 x 排数）

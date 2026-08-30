---
name: epub-image-layout-optimizer
description: 优化 EPUB 图片版式、figure 环绕、图注、栅格格式、封面声明与阅读器兼容性。用于图片过小、裁切、Kindle 空白、图文环绕差、缺图注，或封面/海报图片处理需要 EPUB 安全规则时。
---

# EPUB 图片版式优化

## 何时用

- 处理可重排 EPUB 正文中的图片：figure 环绕、图注、封面声明、Kindle 格式风险。封面式全页海报布局使用 `epub-alite-converter`。
- 固定目标：JPEG / PNG 作为主要交付格式；`figure` 包裹浮动图片和图注；百分比宽度挂在 `figure` 上，不直接 float `img`；wrapped figure 内部 `img { width: 100%; height: auto; }`；封面图在 OPF 中完整声明；周围有足够正文来证明环绕行为。
- 左/右环绕模式（`figure.img-left`，`img-right` 对称）：

  ```html
  <figure class="img-left">
    <img src="../Images/example.png" alt="图示说明"/>
    <figcaption>图注文字。</figcaption>
  </figure>
  ```

  ```css
  .img-left {
    float: left;
    width: 30%;
    margin: 0.2em 1em 0.6em 0;
  }

  .img-left img,
  .img-right img {
    display: block;
    width: 100%;
    height: auto;
  }
  ```

- 封面规则：manifest 封面 item 带 `properties="cover-image"`；metadata 同步 `<meta name="cover" content="..."/>` 兼容 Kindle；Kindle 交付优先 JPEG/PNG，不依赖 SVG-only 封面声明。
- 图片压缩边界：压缩、色彩空间转换和有损质量参数不在本仓实现。本 skill 只判断图片是否适合 EPUB/Kindle 主路径、检查 OPF manifest/封面/figure/图注/alt、并在外部压缩或转码后复查。用户要求压缩图片时，先说明需要外部工具完成，再把压缩后的资源带回本 skill 校验。
- 禁止事项：
  - 不在可重排主路径固定图片高度；不依赖 `aspect-ratio` 作为环绕主路径；Kindle 主路径不用 `em` 宽度控制 figure float。
  - 不为了视觉整齐删除图注或 alt；除非用户明确选择，不用截图替代真实文本。
  - 不用短段落直接判定 float 失败；需要长正文 fixture，或标记为阈值反例。

## 调什么

机器判断入口（只读扫描，不改文件；列出逐图候选与可追溯风险）：

```sh
epub run epub.image.layout.optimize --input <书> --json
```

无额外 KEY=VALUE 参数；不需要 `--output`。需要旧报告形状的逐图明细（`findings` 数组：`file`、`image`、`selector`、`finding` 与 `warnings`）时加 `legacy_report=true`。

祖先为 `.noteref-icon` 或 `a[epub:type~=noteref]` 的图片是注释交互控件，能力已自动排除，不生成 figure/浮动/图注/alt 候选。修复动作按第四段由 AI 在新候选 EPUB 上执行，改后跑：

```sh
epub redline --check all <before.epub> <after.epub>
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：本能力只产 `warn`（候选与风险，不是错误）；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- facts 键前缀 `epub.image.layout.optimize.`：`findings`（候选条数）、`warnings`（扫描警告条数，如 spine XHTML 缺失、XML 解析失败——非零说明结论不完整）。
- findings 的 ID 即候选类别：
  - `lone-image-no-figure`：图片未包 figure。
  - `caption-detached`：疑似图注落在 figure 外。
  - `float-width-risk`：img 上直接 float 或百分比宽度，阅读器脆弱。
  - `missing-alt`：缺 alt。
  - `chapter-head-image-candidate`：疑似章首图。
  - `fullpage-image-alite-candidate`：疑似整页海报图（转 `epub-alite-converter`）。
  - `detail` 是 `文件 · 图片`，`location` 是 selector。
- `legacy_report=true` 时 `facts` 额外含 `legacyReport`（逐图明细数组）。

## 依据返回怎么判断

- 按 findings 分类图片角色：封面图、正文内联图、浮动 figure、通栏 figure、图标/注释标记、海报背景、公式或图表 fallback；只有确认角色后才动结构。
- 修复规则：
  - 把 direct floated `img` 转成 `figure.img-left` / `figure.img-right`；对尚未人工确认图文关系的插图，只规范为居中的 `figure.illustration`，不自动分配左/右浮动类，把候选写入决策记录留待逐图 review。
  - figure 宽度从 `30%` 起步，正式默认保持 `25%`–`35%`，除非阅读器测试证明需要调整。
  - 保留图注和 alt；只有图片角色明确时才补 alt。
  - 图片环绕规则写进 `media.css`；通用 `figure/img` 默认写进 `base.css`。
  - 新增或删除图片时同步 OPF manifest。
  - 面向 Kindle 时用 JPEG/PNG 替换 WebP 主路径；必要时预栅格化风险 SVG。
- 验证 fixture（改 demo 模板时对照）：`Text/01-body.xhtml` 基础 figure、`Text/03c-poster-contain.xhtml` 单图卷封 contain + fallback、`Text/09-kindle-risk.xhtml` Kindle 图片风险页、`Text/17-image-layout.xhtml` figure 环绕与大字号回归；demo 构建与验证由 `epub-style-demo-maintainer` 处理。
- `facts` 的 `warnings` 非零 → 先修对应文件或如实标注结论不完整；`status == failed` → 修输入后重跑；`status == approval-required` → 停下来问人；写型改动后 `epub redline` 未通过 → 回滚或修复后重跑。

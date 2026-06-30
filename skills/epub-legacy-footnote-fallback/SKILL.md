---
name: epub-legacy-footnote-fallback
description: 在项目标准 EPUB 3 grouped footnote 结构上叠加多看旧版 popup note 兼容 fallback。用于必须兼容 Duokan legacy readers，且仍需满足 SPEC §1 fallback 约束时。
---

# EPUB 旧版弹注 Fallback

只有目标 EPUB 必须保留多看旧版 popup-note 兼容性时才使用这个 skill。它是兼容层，不是项目默认 note 模式。

## 固定目标

权威结构源是 `docs/final/SPEC-实现约束.md` §1；以下内容保持 skill 可独立执行，并不得与该节分叉。

保留项目标准结构作为主形态：

- XHTML 根 `<html>` 声明 `xmlns:epub="http://www.idpf.org/2007/ops"`。
- noteref 是带 `epub:type="noteref"` 和 `role="doc-noteref"` 的 `<a>`。
- noteref `href` 指向同一 XHTML 文件内的 note `li`。
- 每个 XHTML 文件只有一个 grouped note body：`<aside epub:type="footnote" role="doc-footnote">`。
- 所有本地 notes 放在 `ol.footnote-list`。
- 每条 note target 是 `li.footnote-item`。
- backlink 是 `◎`。

在同一结构上叠加 legacy hooks：

- noteref anchor 增加 `duokan-footnote`。
- grouped `ol.footnote-list` 增加 `duokan-footnote-content`。
- 每个 note `li` 增加 `duokan-footnote-item`。
- noteref anchor 内放 note icon 图片。

不要为 fallback 创建第二份 note body。

## XHTML 模式

```html
<p>
  正文文字
  <sup>
    <a id="note-1"
       class="noteref-icon duokan-footnote"
       epub:type="noteref"
       role="doc-noteref"
       href="#footnote-1">
      <img alt="注" src="../Images/note.png"/>
    </a>
  </sup>
  继续正文。
</p>

<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line"/></div>
  <ol class="footnote-list duokan-footnote-content">
    <li class="footnote-item duokan-footnote-item" id="footnote-1">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-1">◎</a>
        富文本注释内容。
      </p>
    </li>
  </ol>
</aside>
```

## 转换流程

1. 从已经符合，或可先转换为，标准 `aside > ol.footnote-list > li.footnote-item` 模式的文件开始。
2. 确保 XHTML 根 `<html>` 声明 `xmlns:epub="http://www.idpf.org/2007/ops"`。
3. 尽量保留 id。确保 noteref id 和 note target id 在当前 XHTML 内唯一。
4. 给每个 noteref anchor 增加 `duokan-footnote`，但不要删除 `epub:type`、`role`、`id` 或 `href`。
5. 确保 noteref anchor 包含图片图标。已有本地图标时保留原 `img src`；只有缺少图片热区且需要新增 legacy fallback 资源时，才使用 `../Images/note.png`。
6. 给 grouped `ol.footnote-list` 增加 `duokan-footnote-content`；不要把它放到 `li` 上。
7. 给每个 `li.footnote-item` 增加 `duokan-footnote-item`。
8. 确保 noteref 实际引用的图标资源已在 OPF manifest 声明且文件存在；只有新增默认图标时才补 `Images/note.png`。
9. 验证所有 href/backlink target 都解析到同一 XHTML 文件内。

## CSS

如果 EPUB 还没有样式化这些类，把以下规则合并到活动 note CSS：

```css
.noteref-icon,
a.duokan-footnote {
  text-decoration: none;
}

.noteref-icon img,
a.duokan-footnote img {
  width: auto;
  height: 1em;
  vertical-align: baseline;
}

ol.duokan-footnote-content {
  list-style-type: none;
  padding: 0;
  margin: 0;
}

.footnote-item.duokan-footnote-item {
  list-style-type: none;
}
```

如果源文件没有 `.footnote-line`，可以添加可见分隔线：要么使用 `footnote-line` 规则，要么给 `.duokan-footnote-content` 加 border；同一路径不要两者都用。

## 禁止事项

- 普通项目输出不要使用这个 skill；默认用 `epub-popup-footnote-converter`。
- 不删除 `footnote-list`、`footnote-item` 等中性类。
- 不只保留 `duokan-*` 类。
- 不复制一份第二个可见 note list。
- 不把 `duokan-footnote-content` 放在单个 `li` 上；旧多看兼容验证的是 grouped `ol` 上的类。
- 不使用 JavaScript 或 `display:none` note body。
- 不在多看 fallback 范围外添加阅读器私有 note 属性。

## 验证 fixture

使用这些本地参考：

- `templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml`：单条兼容样例。
- `templates/epub-style-demo/retired/06-multi-legacy-note-fallback.xhtml`：历史多条 fallback notes 对照，不进入默认 demo 构建。

应用 fallback 后运行 stdlib-only validator：

```sh
scripts/validate-popup-notes.sh
```

多条 note 验证：

- 同一 XHTML 文件内有多条 notes 时，每个 trigger 只能打开它指向的 `li` 内容。
- 标准 EPUB 路径必须能通过 href -> target id 精确解析。

## Dry-run 约定

本 skill 默认 dry-run。直接调用只输出预期改动 JSON；加 `--commit` 才真正改。

代理协议示例（注释说明代理动作，不是独立 shell 命令）：

```sh
# 代理调用当前 skill，并将 dry-run JSON 写入 work/dry-run.json

# 人工审查
cat work/dry-run.json | jq

# 用户确认后，由映射 provider 写出新的 EPUB 产物
```

dry-run 输出格式见 [docs/pipeline/cleanup-flow.md](../../docs/pipeline/cleanup-flow.md)。

# 多看弹注 fallback 修正说明

> 状态：诊断已完成；本说明用于把变更交给另一模型/协作者执行。  
> 范围：影响 `SPEC-实现约束.md §1`、终极手册 §7.3、`skills/epub-legacy-footnote-fallback/SKILL.md`，以及 `templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml`、`06-multi-legacy-note-fallback.xhtml`。

## 1. 现象

多看阅读器（以及使用 Duokan 弹注协议的复刻 / 兼容版本）在以下结构下**容易**出现「无反应」「弹出整段空白」「弹出整页 / 整个列表」三种之一（具体阅读器版本表现可能略有差异，需以多看实测为准）：

```html
<aside epub:type="footnote" role="doc-footnote">
  <ol class="footnote-list">                                      <!-- 没有 duokan 类 -->
    <li class="footnote-item duokan-footnote-item
               duokan-footnote-content"                            <!-- 错误：content 挂在 li -->
        id="footnote-1">
      <p class="footnote">…</p>
    </li>
  </ol>
</aside>
```

推断（基于社区可见的 Duokan 协议历史样本，未做完整版本回归）应替换为以下结构，预期**只弹出当前 `<li>`** 的内容；实际是否达成此效果以多看实测为准（验证步骤见 §5）：

```html
<aside epub:type="footnote" role="doc-footnote">
  <hr class="footnote-line"/>
  <ol class="footnote-list duokan-footnote-content">              <!-- 正确：content 挂在 ol -->
    <li class="footnote-item duokan-footnote-item"                 <!-- li 仅持 item 标记 -->
        id="footnote-1">
      <p class="footnote">…</p>
    </li>
  </ol>
</aside>
```

## 2. 根因

多看私有弹注协议的语义实际是：

| 类名 | 语义 |
|---|---|
| `a.duokan-footnote` | 触发器：携带 `href="#xxx"` 指向某条注释 |
| `ol.duokan-footnote-content` | **容器标记**：整组弹注的「源」；阅读器从 href 里的 id 出发，向上找到最近的 `duokan-footnote-content` 祖先 |
| `li.duokan-footnote-item` | 单条注释：弹窗实际显示的就是被命中的 `id` 所在的 `li` 内容 |

弹窗命中规则（社区实测的近似还原）：

1. 用户点 `a.duokan-footnote` → 读 href → 找到目标节点 N（带相应 `id`）；
2. 沿 N 向上查找 `duokan-footnote-content`；
3. 找到后，把 N 的内容渲染进弹层。

因此：

- `duokan-footnote-content` 必须出现在祖先（`<ol>`）上，否则向上查找失败，多看可能不弹（旧版直接跳页）。
- 当 `duokan-footnote-content` 误挂在 `<li>` 上、且 `<li>` 同时是 N 自身时，部分版本会把整段 `<li>` 当作"容器+目标"重复识别，弹层可能空白或弹出整个 `<ol>` 的兜底渲染。
- `duokan-footnote-item` 是"这是一条单注"的标识，仅作用于 `<li>`。

这与当前 `SPEC-实现约束.md §1` 写的「`duokan-footnote-content` 必须挂在 `<li>` 上，不允许挂在 `<ol>` 上」恰恰相反，需要倒过来写。

## 3. 修正后的目标结构（项目落地版）

保留 EPUB 3 标准弹注（noteref/aside/`ol.footnote-list`/`li.footnote-item`/`◎` 回跳），在此之上叠加多看 fallback：

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
        注释正文。允许 <strong>富文本</strong>、书名号《XXX》、行内引用 <q>…</q> 等。
      </p>
    </li>
    <!-- 同一 XHTML 文件内的所有注释都聚合在这同一个 ol 内 -->
  </ol>
</aside>
```

不变量（务必保持）：

- 同一 XHTML 文件 **只有一个** `<aside epub:type="footnote">` 注释容器；
- 同一 XHTML 文件的所有注释**共享同一个** `<ol class="footnote-list duokan-footnote-content">`；不要为每条注释单独生成 `<ol>`，也不要复制第二份注释容器；
- `<li>` 自己持 `id`，noteref 的 `href` 直接指向该 `id`；
- `<li>` 不再带 `duokan-footnote-content` 类；
- 标准回跳与多看弹注共用同一份 `<li>` 内容，**不要复制一份纯文本兜底**到别处；
- noteref 锚里必须有 `<img>` 图标（多看老版本依赖图片作为可见触发热区，纯文字 noteref 会被忽略）。

## 4. 项目内需要同步修改的位置

请按下列顺序改：

### 4.1 `docs/final/SPEC-实现约束.md §1`

替换"当需要兼容多看旧版本时"那一段，新文本：

```
- 当需要兼容多看旧版本时，必须在标准结构基础上同步：
  - noteref 锚 `<a>` 增加 `class="duokan-footnote"`，且锚内放注释图标 `<img>`；
  - 注释聚合容器 `<ol class="footnote-list">` 同时挂 `duokan-footnote-content` 类；
  - 每条 `<li class="footnote-item">` 同时挂 `duokan-footnote-item` 类；
  - `duokan-footnote-content` 必须挂在 `<ol>` 上，不允许挂在 `<li>` 上；
  - `<li>` 不重复持有 `duokan-footnote-content`。
- fallback 为次路径，禁止创建第二份注释容器。
```

### 4.2 `docs/final/EPUB 3 终极实践手册.md §7.3`

把现有 fallback XHTML 示例中的：

```html
<li class="footnote-item duokan-footnote-item duokan-footnote-content" id="footnote-legacy-1">
```

改为容器在 `<ol>` 上：

```html
<ol class="footnote-list duokan-footnote-content">
  <li class="footnote-item duokan-footnote-item" id="footnote-legacy-1">
```

并把 §十二的"自检补充"两行：

```
- [ ] 每条 `li.footnote-item` 同时挂 `duokan-footnote-item` 与 `duokan-footnote-content`。
- [ ] `duokan-footnote-content` 不出现在 `<ol>`。
```

替换为：

```
- [ ] 容器 `ol.footnote-list` 同时挂 `duokan-footnote-content`。
- [ ] 每条 `li.footnote-item` 同时挂 `duokan-footnote-item`。
- [ ] `duokan-footnote-content` 不出现在 `<li>`；只挂在 `<ol>`。
```

### 4.3 `skills/epub-legacy-footnote-fallback/SKILL.md`

- "Fixed Target" 段：把 `add duokan-footnote-content to each note <li> (not the grouped ol)` 改成 `add duokan-footnote-content to the grouped <ol> (not on each <li>)`。
- "XHTML Pattern" 段：示例改成上面 §3 的形态。
- "Conversion Workflow" 第 5 步：`Add duokan-footnote-content to each li.footnote-item; do not put it on ol` 改成 `Add duokan-footnote-content to the grouped <ol> (footnote-list); do not put it on individual <li>`。
- CSS 段：**删除** 旧规则 `.duokan-footnote-content { margin-top: 0; }` 与 `.footnote-item.duokan-footnote-content { margin: 0; }`（前者会被新规则的 `margin: 0` 覆盖、后者按新结构已不会命中）。**新增**：`ol.duokan-footnote-content { list-style-type: none; padding: 0; margin: 0; }`。
- "Guardrails" 段：`Do not place duokan-footnote-content on grouped ol` → 反转为 `Do not place duokan-footnote-content on individual <li>; it must mark the grouped <ol>`。

### 4.4 `templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml` 与 `06-multi-legacy-note-fallback.xhtml`

把 `<ol class="footnote-list">` → `<ol class="footnote-list duokan-footnote-content">`；把每条 `<li class="footnote-item duokan-footnote-item duokan-footnote-content" id="…">` → `<li class="footnote-item duokan-footnote-item" id="…">`。其余结构（noteref、回跳、`◎`、`epub:type/role`、id）保持不变。

> 本说明只描述目标结构与改动清单；具体编辑工作交给执行模型完成。05/06 demo 必须先改、打包、多看实测通过，再回写 §4.1–4.3 的 SPEC/手册/skill。

### 4.5 弹注 CSS 落点

按 [css-layering-plan.md §2.1](./css-layering-plan.md)，弹注样式应统一迁到 `notes.css`。本节的新增规则也跟随迁过去：

新增（合并到 `notes.css`）：

```css
ol.duokan-footnote-content {
  list-style-type: none;
  padding: 0;
  margin: 0;
}
```

删除（若 base.css 或 notes.css 仍有这一段）：

```css
.footnote-item.duokan-footnote-content { margin: 0; }   /* 已不会被命中，移除 */
```

> 若执行顺序是「先 CSS 分层迁移、再多看 fix」（README 推荐顺序），落 notes.css 即可；若仍在 base.css 上做，请同步处理后续按 css-layering-plan §6 的迁移步骤。

## 5. 验证步骤

1. 打包：`sh templates/epub-style-demo/build.sh`，记下时间戳 EPUB。
2. 多看（或多看复刻版本，如「多看阅读」「读读」「阅读 App 兼容 Duokan 协议」）打开 dist EPUB：
   - 翻到第 5 / 6 页，点正文里的注释图标 → 应弹出**仅一条** `<li>` 内容；
   - 第 6 页连续点 3 条注释 → 每次弹出的内容互不相同；
   - 弹层关闭后正文不跳转、不复位章节顶部。
3. 同包内 Apple Books / Thorium / Kindle Previewer 复测 02、05、06 三页，确认标准弹注路径（noteref→`◎`回跳）仍可用。
4. 把多看复测结果写入 `docs/final/reader-matrix.yaml`：新增 reader id `duokan`（或 `duokan_legacy`），加 expectations 行，附 reader version、artifact 路径、现象、状态。

## 6. 不变量自检（给执行模型一份 grep 清单）

完成上述修改后，仓库内应全部满足：

```bash
# 1) duokan-footnote-content 只允许出现在带 footnote-list 类的 <ol> 上
#    （即同时挂在 footnote-list 类的元素上才合规）
rg -nP 'class="[^"]*\bduokan-footnote-content\b' templates docs skills \
  | rg -v 'class="[^"]*\bfootnote-list\b[^"]*duokan-footnote-content|duokan-footnote-content[^"]*\bfootnote-list\b'

# 2) duokan-footnote-item 只允许出现在 <li> 上（不允许在 <ol> / <p> / <div> 等）
rg -nP '<(?!li\b)[a-z]+[^>]*\bduokan-footnote-item\b' templates docs skills

# 3) noteref 锚（class="duokan-footnote"，**精确匹配** 而非前缀）内必须有 <img>
#    用负向预查避免误匹配 duokan-footnote-content / duokan-footnote-item
rg -nP 'class="[^"]*\bduokan-footnote(?!-)\b[^"]*"[^>]*>(?!\s*<img)' templates docs skills

# 4) 同文件不允许出现两个 <aside epub:type="footnote">
for f in templates/epub-style-demo/OEBPS/Text/*.xhtml; do
  c=$(rg -c 'epub:type="footnote"' "$f")
  [ "$c" -gt 1 ] && echo "MULTIPLE FOOTNOTE ASIDE in $f"
done
```

前三条若有输出说明 fallback 规则被破坏；第四条若有输出说明出现了第二份注释容器。

## 7. 与项目大原则的关系

- 仍然遵守"demo 先行、文档后补"：本文档**只描述**目标结构与改动清单，并未直接修改 05/06 demo；待执行模型按本文 §4.4 改写 05/06、跑构建并在多看实测稳定后，再统一推进 SPEC/手册/Skill。
- 仍然遵守 SPEC §1 的"同一 XHTML 文件只有一个 footnote 容器"，本次只是把多看私有标记从错误位置（`<li>`）搬到正确位置（`<ol>`）。
- 标准 EPUB 3 路径完全不变，noteref→`◎`回跳依然有效，Apple Books / Thorium / Kindle Previewer 不受影响。

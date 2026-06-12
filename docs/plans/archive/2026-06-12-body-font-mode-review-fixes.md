# 正文字体模式（v0.2.1）review 修复清单

> 手动执行版。每个任务给出精确的「现状 → 改为」文本，按引用文本定位（行号是写作时 HEAD=5c4ecd7 的参考值，编辑后会漂移）。
> 若由 AI 代理执行：建议配合 superpowers:executing-plans 按任务推进，复选框用于跟踪。

**目标：** 修复 v0.2.1「正文字体自由化」改造遗留的 15 项 review 发现——核心是把 `ibooks:specified-fonts` 的「仅锁定时添加」新规则贯彻到 SPEC、手册、入门教程、skill、demo 注释和转换工具，并补齐 AGENTS.md 要求的阅读器实测追溯记录。

**两个前置决策（已在 review 中确定，执行时不再讨论）：**

1. **正文字体模式是全书级决策。** `ibooks:specified-fonts` 是书级 meta、`body-font-locked` 是页级 class，混用无意义。真实书籍全书统一模式；demo 演示书因包含锁定演示页（场景 07），整书按锁定书处理，**保留** `package.opf` 里的 meta，不改 OPF，只补文档说明。
2. **条件规则的完整表述是三分支**（与手册 §一 blockquote 一致）：自由模式不加；锁定模式加；启用嵌入字体时加。SPEC §8 现行表述漏了嵌入分支，本次一并补上。
   其中「嵌入字体 + 自由正文」组合（如只嵌标题字体 / 生僻字子集、正文不锁）是**待实测假设**：暂按保守口径**加** meta。依据是旧实测前提——此 meta 是 Apple Books 启用书内嵌入字体的开关，不加时用户切换字体可能整体覆盖书内 `font-family`（含挂在专用类上的嵌入字体）。两个待验证问题登记进 reader-matrix（见 Task 10）：
   - (a) 此 meta 是否真的**阻止**读者切换字体，还是只把「原版」设为默认、读者仍可切换；
   - (b) 无此 meta 时，挂专用类的嵌入字体（`.title-kai` / `.rare` 等）在用户切换字体后是否被覆盖或忽略。
   实测结论与保守口径冲突时，以实测为准回修 SPEC §8 与本清单（(a) 若证伪，「锁定模式」措辞需弱化为「默认使用书内字体链」）。

**验证总开关（最后统一跑，单项任务里也标了各自要跑的）：**

```sh
git diff --check
python3 scripts/validate_ai_entrypoints.py
python3 scripts/validate_skills_basic.py
python3 scripts/test_epub3_oneclick_converter.py
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<最新产物>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<最新产物>.epub
```

---

## Task 1：SPEC-实现约束.md——消除 §3 与 §8 自相矛盾，补全书级口径

**文件：** `docs/final/SPEC-实现约束.md`

- [ ] **1.1 修正 §3 第一条（约 L40）**

现状：

```markdown
- 无论是否启用书内字体嵌入，OPF `<package>` 都必须在 `prefix` 声明 ibooks 命名空间，并保留：`<meta property="ibooks:specified-fonts">true</meta>`。
```

改为：

```markdown
- `<meta property="ibooks:specified-fonts">true</meta>` 仅在正文字体锁定（`body.body-font-locked`）或启用嵌入字体时添加；添加时 OPF `<package>` 必须同步在 `prefix` 声明 ibooks 命名空间。自由模式（默认）两者都不需要。判定规则见 §8。
```

- [ ] **1.2 修正 §8 条件规则并补嵌入分支与全书级口径（约 L205）**

现状：

```markdown
- `<meta property="ibooks:specified-fonts">true</meta>` 仅当正文字体锁定（`body.body-font-locked`）时添加；自由模式下不设置此 meta，允许 Apple Books 读者正常切换字体。
```

改为（一条改两条）：

```markdown
- `<meta property="ibooks:specified-fonts">true</meta>` 仅当正文字体锁定（`body.body-font-locked`）或启用嵌入字体时添加；自由模式下不设置此 meta，允许 Apple Books 读者正常切换字体。「嵌入字体 + 自由正文」组合暂按此保守口径添加；其实际行为是待实测假设，见 reader-matrix `07-font-family-order` 待测条目，实测后以结果为准回修本条。
- 正文字体模式是**全书级决策**：同一本书要么全书自由（所有正文页 body 都不带 `body-font-locked`、OPF 无此 meta），要么全书锁定（所有正文页 body 都带 class、OPF 加一份 meta）。不按页混用；演示/测试用书（如 epub-style-demo 含锁定演示页）按锁定书处理，并在其场景矩阵中注明。
```

- [ ] **1.3 提交**

```sh
git add docs/final/SPEC-实现约束.md
git commit -m "fix(spec): 统一 ibooks:specified-fonts 条件规则，补全书级模式口径"
```

---

## Task 2：终极实践手册——§一 OPF 区两处旧表述

**文件：** `docs/final/EPUB 3 终极实践手册.md`

- [ ] **2.1 OPF 模板代码块（约 L86）**

现状（模板内一行）：

```xml
    <meta property="ibooks:specified-fonts">true</meta>
```

改为：

```xml
    <!-- 仅锁定模式 / 嵌入字体时添加：<meta property="ibooks:specified-fonts">true</meta> -->
```

- [ ] **2.2 规则列表旧条目（约 L125，紧邻新 blockquote 上方）**

现状：

```markdown
- 无论是否嵌入字体，Apple Books 路径都保留 `ibooks:specified-fonts=true`，避免用户偏好字体覆盖 CSS 字体链。
```

改为：

```markdown
- `ibooks:specified-fonts=true` 仅在正文字体锁定或启用嵌入字体时添加；自由模式（默认）不加，见下方说明与 SPEC §8。嵌入分支尚未实测，暂按保守口径添加，待 Apple Books 实测后修订（见 reader-matrix `07-font-family-order` 待测条目）。
```

其下方的 blockquote（「`ibooks:specified-fonts=true` 仅在正文字体锁定……」三分支）是 v0.2.1 新写的，保持不动。

- [ ] **2.3 提交**

```sh
git add 'docs/final/EPUB 3 终极实践手册.md'
git commit -m "docs(handbook): §一 OPF 模板与规则条目改为条件式 ibooks:specified-fonts"
```

---

## Task 3：终极实践手册——§4.2 正文字体重写 + L855 注释

**文件：** `docs/final/EPUB 3 终极实践手册.md`

- [ ] **3.1 重写 §4.2（约 L168–L180，从 `### 4.2 正文字体` 到「例外路径」段落末尾）**

现状：

````markdown
### 4.2 正文字体

```css
body {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

> 反例：上面的长链别名堆叠（如 `STSongti-*` / `NSimSun` / `宋体`）违反 SPEC §8，仅用于说明 anti-pattern。

默认路径：正文走各平台系统中文字体链（Apple `Songti SC` + Windows `SimSun` + Android / 跨平台开源 `Noto Serif CJK SC` + `serif`）。iOS / Apple Books 对 `Songti SC` 命中稳定；Android 系统已预装 `Noto Serif CJK SC`；Windows 走 `SimSun` 兜底。

例外路径：当全书含生僻字、且选择嵌入"全字符集"字体（非子集）时，按模式 C1-body 把嵌入字体放在链首：`"BookSongFull", "Songti SC", "SimSun", "Noto Serif CJK SC", serif`。`fontspec` 同步切到 `forceAll`，OPF manifest 挂对应字体 item。详见本节"含生僻字的全字符集方案（模式 C1-body）"。
````

改为（与速查表 §4.1 的自由/锁定结构对齐）：

````markdown
### 4.2 正文字体

正文分自由 / 锁定两种模式（规则见 SPEC §8）：

```css
/* 自由模式（默认）：body 不设 font-family，读者可自由切换字体 */

/* 锁定模式：给 body 加 class="body-font-locked"，并同步 OPF 加 ibooks:specified-fonts=true */
.body-font-locked {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

> 反例：不要把链写成同平台别名堆叠（如追加 `STSongti-*` / `NSimSun` / `宋体`），违反 SPEC §8。

锁定模式的字体链走各平台系统中文字体链（Apple `Songti SC` + Windows `SimSun` + Android / 跨平台开源 `Noto Serif CJK SC` + `serif`）。iOS / Apple Books 对 `Songti SC` 命中稳定；Android 系统已预装 `Noto Serif CJK SC`；Windows 走 `SimSun` 兜底。

例外路径：当全书含生僻字、且选择嵌入"全字符集"字体（非子集）时，按模式 C1-body 把嵌入字体放在锁定链链首：`body.body-font-locked { font-family: "BookSongFull", "Songti SC", "SimSun", "Noto Serif CJK SC", serif; }`。`fontspec` 同步切到 `forceAll`，OPF manifest 挂对应字体 item。详见本节"含生僻字的全字符集方案（模式 C1-body）"。
````

- [ ] **3.2 修正脚注样式注释（约 L855）**

现状：

```css
  /* font-family 继承 body：默认系统宋体链；C1-body 模式下继承嵌入全字符集字体 */
```

改为：

```css
  /* font-family 继承 body：自由模式下为阅读器默认字体；锁定 / C1-body 模式下继承对应字体链 */
```

（L960 的「注音跟正文同字体」两种模式下都成立，**不改**。）

- [ ] **3.3 提交**

```sh
git add 'docs/final/EPUB 3 终极实践手册.md'
git commit -m "docs(handbook): §4.2 正文字体改写为自由/锁定双模式"
```

---

## Task 4：epub-typography-optimizer skill——旧条目 + 模式保持规则

**文件：** `skills/epub-typography-optimizer/SKILL.md`

- [ ] **4.1 修正「固定目标」旧条目（约 L20）**

现状：

```markdown
- 即使没有嵌入字体，OPF 也保留 `ibooks:specified-fonts` metadata。
```

改为：

```markdown
- `ibooks:specified-fonts` 仅在正文锁定（`body-font-locked`）或启用嵌入字体时写入 OPF；自由模式（默认）不加。
```

- [ ] **4.2 在「工作流」一节末尾新增一步（编号顺延）**

```markdown
N. 保持既有书的字体模式：body 无 `font-family` 视为自由模式，不要替它加 `body-font-locked` 或 `ibooks:specified-fonts`；已锁定的书保持锁定，并检查 class 与 OPF meta 成对出现。
```

- [ ] **4.3 验证 + 提交**

```sh
python3 scripts/validate_skills_basic.py
git add skills/epub-typography-optimizer/SKILL.md
git commit -m "fix(skill): typography-optimizer 同步条件规则并新增模式保持步骤"
```

---

## Task 5：demo fonts.css——注释 typo、第二处旧注释、选择器合并

**文件：** `templates/epub-style-demo/OEBPS/Styles/fonts.css`

- [ ] **5.1 合并 `.body-font-locked` 进宋体组并顺带修掉 typo（约 L154–L165）**

现状：

```css
/* ===== 正文锁定模式（body.font-body-locked 专用） ===== */
.body-font-locked {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

/* ===== 宋体 / serif（正文衬线） ===== */
.book-song,
.song,
.book-body {                            /* .book-body 保留为向后兼容别名 */
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

改为（两段合一；typo `font-body-locked` 随旧注释一起消失）：

```css
/* ===== 宋体 / serif（正文衬线；.body-font-locked 为正文锁定模式入口，加在 body 上） ===== */
.body-font-locked,
.book-song,
.song,
.book-body {                            /* .book-body 保留为向后兼容别名 */
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

- [ ] **5.2 修正嵌入字体区残留的旧注释（约 L202–L205）**

现状：

```css
 * 只在有嵌入字体 (@font-face 已取消注释) 且 OPF manifest 已加 font item 时，
 * 才取消下面对应类的注释。（OPF metadata 的 ibooks:specified-fonts=true
 * 是通用预防默认，与是否嵌字体无关，参见配套文档 §4。）
 */
```

改为：

```css
 * 只在有嵌入字体 (@font-face 已取消注释) 且 OPF manifest 已加 font item 时，
 * 才取消下面对应类的注释。（嵌入字体启用后，OPF 需同步加
 * ibooks:specified-fonts=true，规则见 SPEC §8。）
 */
```

- [ ] **5.3 三个 preset 的 fonts.css 做同样合并（链逐字节相同，已核实）**

`templates/style-presets/{academic-cn,classical-annotated-cn,literary-cn}/Styles/fonts.css` 文件头部：

现状（三个文件相同）：

```css
.body-font-locked {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

.book-song,
.song,
.book-body {
```

改为：

```css
.body-font-locked,
.book-song,
.song,
.book-body {
```

（删除独立的 `.body-font-locked` 块，把它并入下方分组的首位；组内 `font-family` 声明不变。）

- [ ] **5.4 重建 demo 并验证、提交**

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<最新产物>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<最新产物>.epub
git add templates/epub-style-demo/OEBPS/Styles/fonts.css templates/style-presets/*/Styles/fonts.css
git commit -m "fix(fonts): 修正锁定模式注释并将 .body-font-locked 并入宋体选择器组"
```

---

## Task 6：demo 场景文档——场景 07 描述 + demo 书锁定口径

- [ ] **6.1 `templates/epub-style-demo/SCENE_MATRIX.md`（约 L16）**

现状：

```markdown
| font-family 顺序 | `Text/07-font-family-order.xhtml` | 系统优先、书内优先、楷体混合链、生僻字 fallback | Apple Books / Windows 阅读器 |
```

改为：

```markdown
| font-family 顺序 + 正文锁定 | `Text/07-font-family-order.xhtml` | 正文锁定模式（`body class="body-font-locked"` + OPF `ibooks:specified-fonts=true`）、系统优先、书内优先、楷体混合链、生僻字 fallback | Apple Books / Windows 阅读器 |
```

- [ ] **6.2 `templates/epub-style-demo/README.md` 场景列表第 10 条（约 L28）**

现状：

```markdown
10. `07-font-family-order.xhtml`：系统优先 / 书内优先 / 混合链的 font-family 顺序验证。
```

改为：

```markdown
10. `07-font-family-order.xhtml`：系统优先 / 书内优先 / 混合链的 font-family 顺序验证；同时演示正文锁定模式（`body class="body-font-locked"`）。
```

- [ ] **6.3 在 README.md 场景列表结束后新增一段说明（demo 书为何保留书级 meta）**

```markdown
> 注：本 demo 因包含正文锁定演示页（场景 07），`package.opf` 保留书级
> `<meta property="ibooks:specified-fonts">true</meta>`，整书按锁定模式书处理。
> 真实书籍应全书统一自由或锁定模式，见 `docs/final/SPEC-实现约束.md` §8。
```

- [ ] **6.4 提交**

```sh
git add templates/epub-style-demo/SCENE_MATRIX.md templates/epub-style-demo/README.md
git commit -m "docs(demo): 场景 07 描述与 demo 书锁定口径说明"
```

---

## Task 7：三个 style preset README——正文字体表述

- [ ] **7.1 `templates/style-presets/academic-cn/README.md`（L3 起的段落）**

现状句：`正文使用系统优先宋体链，标题使用黑体链，`
改为：`正文默认自由模式（不锁字体，跟随阅读器设置；需要锁定时给 body 加 class="body-font-locked"），标题使用黑体链，`

- [ ] **7.2 `templates/style-presets/literary-cn/README.md`**

现状句：`正文采用系统优先宋体链，章首留白舒展，`
改为：`正文默认自由模式（需要锁定时给 body 加 class="body-font-locked"），章首留白舒展，`

- [ ] **7.3 `templates/style-presets/classical-annotated-cn/README.md`**

现状句：`原文使用系统优先宋体链，译文和注释可挂 \`.book-kai\`，`
改为：`原文默认自由模式（需要锁定时给 body 加 class="body-font-locked"），译文和注释可挂 \`.book-kai\`，`

（三个文件都是两行折行段落，替换后按原有列宽重新折行即可。）

- [ ] **7.4 提交**

```sh
git add templates/style-presets/*/README.md
git commit -m "docs(presets): README 正文字体表述同步自由模式默认值"
```

---

## Task 8：入门教程 02-anatomy——meta 解释与溯源

**文件：** `docs/getting-started/02-anatomy.md`

- [ ] **8.1 修正 meta 条目（约 L114）**

现状：

```markdown
- **`meta property="ibooks:specified-fonts">true</meta>`：** Apple Books 专用，告诉系统"书里有嵌入字体"。需在 `<package>` 声明 `prefix="ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks-vocabulary-1.0/"`。
```

改为：

```markdown
- **`<meta property="ibooks:specified-fonts">true</meta>`：** Apple Books 专用，声明"按书内指定的字体渲染"，会阻止读者切换字体。仅在正文锁定（`body.body-font-locked`）或启用嵌入字体时添加；自由模式（默认）不加。嵌入字体分支为暂定保守规则，尚未实测，实测后可能调整。添加时需在 `<package>` 声明 `prefix="ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks-vocabulary-1.0/"`。
```

- [ ] **8.2 其下「溯源」行补 §8**

现状：`> 溯源：SPEC §3、§5；reader-matrix case 00-cover-metadata；demo \`package.opf\`。`
改为：`> 溯源：SPEC §3、§5、§8；reader-matrix case 00-cover-metadata；demo \`package.opf\`。`

- [ ] **8.3 提交**

```sh
git add docs/getting-started/02-anatomy.md
git commit -m "docs(getting-started): ibooks:specified-fonts 解释改为条件式"
```

---

## Task 9：epub3_oneclick_converter.py——条件注入 + 回归测试（TDD）

**文件：**
- 修改：`scripts/epub3_oneclick_converter.py`（`normalize_metadata` 约 L253–L296，`convert_epub` 约 L961）
- 测试：`scripts/test_epub3_oneclick_converter.py`

- [ ] **9.1 先写失败断言（自由模式不得注入 meta）**

`scripts/test_epub3_oneclick_converter.py` 的 `main()` 中，`assert opf.attrib.get("version") == "3.0"` 之后插入：

```python
      metas = opf.findall("opf:metadata/opf:meta", OPF_NS)
      assert not any(
        m.attrib.get("property") == "ibooks:specified-fonts" for m in metas
      ), "free-mode book must not get ibooks:specified-fonts"
```

- [ ] **9.2 跑测试确认失败**

```sh
python3 scripts/test_epub3_oneclick_converter.py
```

预期：AssertionError `free-mode book must not get ibooks:specified-fonts`（现行代码无条件注入）。

- [ ] **9.3 实现：检测 body-font-locked 后按需注入**

`scripts/epub3_oneclick_converter.py`（`re` 已 import）。在 `normalize_metadata` 定义之前加：

```python
BODY_FONT_LOCKED_RE = re.compile(
  rb"<body[^>]*\bclass\s*=\s*(['\"])[^'\"]*\bbody-font-locked\b[^'\"]*\1"
)


def has_body_font_locked(files: dict[str, bytes]) -> bool:
  return any(
    name.lower().endswith((".xhtml", ".html", ".htm")) and BODY_FONT_LOCKED_RE.search(data)
    for name, data in files.items()
  )
```

`normalize_metadata` 签名改为：

```python
def normalize_metadata(root: ET.Element, report: ConversionReport, body_font_locked: bool = False) -> None:
```

函数末尾的无条件注入：

```python
  if specified_fonts is None:
    specified_fonts = ET.SubElement(meta, q(OPF_URI, "meta"), {"property": "ibooks:specified-fonts"})
    specified_fonts.text = "true"
    report.metadata_updates.append("added ibooks:specified-fonts")
```

改为（已有 meta 不静默删除，符合 SPEC §10 保守改动边界，仅提示人工复核）：

```python
  if specified_fonts is None:
    if body_font_locked:
      specified_fonts = ET.SubElement(meta, q(OPF_URI, "meta"), {"property": "ibooks:specified-fonts"})
      specified_fonts.text = "true"
      report.metadata_updates.append("added ibooks:specified-fonts (body-font-locked detected)")
  elif not body_font_locked:
    report.metadata_updates.append("kept existing ibooks:specified-fonts (no body-font-locked page; review manually)")
```

`convert_epub` 中的调用（约 L961）：

```python
  normalize_metadata(root, report)
```

改为：

```python
  normalize_metadata(root, report, body_font_locked=has_body_font_locked(files))
```

- [ ] **9.4 跑测试确认通过**

```sh
python3 -m py_compile scripts/epub3_oneclick_converter.py
python3 scripts/test_epub3_oneclick_converter.py
```

预期：`epub3 oneclick converter tests ok`

- [ ] **9.5 补锁定模式正向用例**

`scripts/test_epub3_oneclick_converter.py`：给 `write_legacy_epub` 加参数，在 `with zipfile.ZipFile(path, "w") as zf:` 之前插入替换逻辑：

```python
def write_legacy_epub(path: Path, body_class: str = "") -> None:
  ...（files 字典原样）...
  if body_class:
    chapter = files["OEBPS/Text/chapter.xhtml"]
    assert "<body>" in chapter
    files["OEBPS/Text/chapter.xhtml"] = chapter.replace("<body>", f'<body class="{body_class}">', 1)
```

新增函数（放在 `main()` 之前），并在 `main()` 的 `print(...)` 之前调用 `locked_mode_case()`：

```python
def locked_mode_case() -> None:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "legacy-locked.epub"
    output = Path(raw) / "converted-locked.epub"
    write_legacy_epub(source, body_class="body-font-locked")
    report = C.convert_epub(source, output)
    with zipfile.ZipFile(output) as zf:
      opf = ET.fromstring(zf.read("OEBPS/content.opf"))
      metas = opf.findall("opf:metadata/opf:meta", OPF_NS)
      locked = [m for m in metas if m.attrib.get("property") == "ibooks:specified-fonts"]
      assert len(locked) == 1 and locked[0].text == "true", "locked book must get ibooks:specified-fonts=true"
    assert "added ibooks:specified-fonts (body-font-locked detected)" in report.metadata_updates, report
```

- [ ] **9.6 跑测试确认通过、提交**

```sh
python3 scripts/test_epub3_oneclick_converter.py
git add scripts/epub3_oneclick_converter.py scripts/test_epub3_oneclick_converter.py
git commit -m "fix(converter): ibooks:specified-fonts 改为检测 body-font-locked 后按需注入"
```

---

## Task 10：reader-matrix——补待实测条目（AGENTS.md 阅读器实测闭环）

**文件：** `docs/final/reader-matrix.yaml`

- [ ] **10.1 更新 `untested_cases` 里 07 的 note（约 L405）**

现状：

```yaml
  - case: 07-font-family-order
    fixture: templates/epub-style-demo/OEBPS/Text/07-font-family-order.xhtml
    note: "字体链顺序；尚未实测。"
```

改为：

```yaml
  - case: 07-font-family-order
    fixture: templates/epub-style-demo/OEBPS/Text/07-font-family-order.xhtml
    note: "字体链顺序 + 正文锁定模式（body-font-locked + OPF ibooks:specified-fonts=true）；尚未实测。待验证：(a) 此 meta 是否真的阻止 Apple Books 读者切换字体，还是只把'原版'设为默认；(b) 自由模式（body 无 font-family、无此 meta）能否正常切换；(c) 嵌入字体 + 自由正文且无此 meta 时，挂专用类的嵌入字体在用户切换字体后是否被覆盖或忽略。"
```

> 说明：SPEC §8 的「自由可切换 / 锁定不可切换」以及「嵌入字体需要此 meta」目前都是假设。按 AGENTS.md，Apple Books 实测（记录 reader 版本 + artifact + 现象）完成前，这条 note 就是合规的「待验证假设」记录；实测后把结论写进 `expectations` 并视结果回修 SPEC。
>
> 建议实测方案（Apple Books iOS / macOS 各跑一遍，记录版本号与产物文件名）：
> 1. **现 demo 产物**（OPF 含 meta + 场景 07 锁定页）：观察默认字体；尝试在字体菜单切换；切换后场景 07 正文、`.book-kai` 楷体类、生僻字 fallback 的表现。
> 2. **对照产物**：临时构建一版去掉 OPF meta 的 demo，重复同样三项观察。
> 3. 若 demo 当前没有真正嵌入的字体文件，需临时取消一段 `@font-face` 注释并挂到专用类，否则 (c) 测不到。
>
> 两版对照可同时回答 (a)(b)(c)。

- [ ] **10.2 提交**

```sh
git add docs/final/reader-matrix.yaml
git commit -m "docs(reader-matrix): 字体锁定/自由模式标记为待实测假设"
```

---

## Task 11（可选）：归档 plan 顶部加「已被取代」指针

`docs/plans/fonts-css-expansion-plan.md` 属历史存档（AGENTS.md：不为统一措辞重写历史），正文不动；仅在标题下加一行防误导指针：

- [ ] **11.1**

```markdown
> 2026-06-12 注：本文 §4「ibooks:specified-fonts 始终保留」已被 v0.2.1 条件规则取代，现行规则见 `docs/final/SPEC-实现约束.md` §8。其余内容为历史设计记录。
```

- [ ] **11.2 提交**

```sh
git add docs/plans/fonts-css-expansion-plan.md
git commit -m "docs(plans): fonts-css-expansion-plan 标注 §4 已被取代"
```

---

## Task 12：CHANGELOG + 收尾验证

- [ ] **12.1 `CHANGELOG.md` 顶部新增**

```markdown
## v0.2.2 - 2026-06-12

### Fixed

- 统一 `ibooks:specified-fonts` 条件规则：修正 SPEC §3、手册 §一 / §4.2、demo fonts.css 注释、typography skill、入门教程中残留的「始终保留」旧表述。
- `epub3_oneclick_converter.py` 不再无条件注入 `ibooks:specified-fonts=true`，改为检测 `body-font-locked` 后按需添加，并补充自由 / 锁定两个回归用例。
- SPEC §8 补全嵌入字体分支（未实测，暂按保守口径添加，待 Apple Books 实测后修订，见 reader-matrix 待测条目），明确正文字体模式为全书级决策；demo 演示书的混合页面口径写入 demo README。
- demo SCENE_MATRIX / README、三个 style preset README 同步新规则；`.body-font-locked` 并入宋体选择器组；reader-matrix 将字体模式行为登记为待实测假设。
```

- [ ] **12.2 全量验证（预期全部通过 / ok）**

```sh
git diff --check
python3 scripts/validate_ai_entrypoints.py
python3 scripts/validate_skills_basic.py
python3 scripts/test_epub3_oneclick_converter.py
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<最新产物>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<最新产物>.epub
```

- [ ] **12.3 提交**

```sh
git add CHANGELOG.md
git commit -m "docs: add v0.2.2 changelog"
```

---

## Review 发现 → 任务映射（自查用）

| # | 发现 | 任务 |
| --- | --- | --- |
| 1 | converter 无条件注入 meta | Task 9 |
| 2 | SPEC §3 vs §8 矛盾 | Task 1.1 |
| 3 | 手册 L125 旧条目 | Task 2.2 |
| 4 | demo OPF 书级 meta / 混合书口径空白 | Task 1.2 + 6.3（OPF 本身不改） |
| 5 | 手册 L86 OPF 模板 | Task 2.1 |
| 6 | 手册 §4.2 旧默认示例 | Task 3.1 |
| 7 | SKILL.md L20 矛盾 | Task 4.1 |
| 8 | reader-matrix 无支撑条目 / 自由模式无场景标注 | Task 10 + 6.1 |
| 9 | fonts.css L154 注释 typo | Task 5.1 |
| 10 | fonts.css L204 第二处旧注释 | Task 5.2 |
| 11 | SCENE_MATRIX / README 场景 07 过时 | Task 6.1–6.2 |
| 12 | 三个 preset README 过时 | Task 7 |
| 13 | 02-anatomy L114 过时 | Task 8 |
| 14 | 手册 L855 注释过时 | Task 3.2 |
| 15 | `.body-font-locked` 与宋体组重复 | Task 5.1 / 5.3 |

另：skill 未定义「既有书模式怎么处理」（review 中标记为 PLAUSIBLE）由 Task 4.2 关闭；归档 plan 误导风险由 Task 11（可选）关闭。

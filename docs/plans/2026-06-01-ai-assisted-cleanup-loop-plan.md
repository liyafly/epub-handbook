# AI 辅助·确定性循环清洗 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给「已有 EPUB 清洗」加一段**确定性的多轮自动循环编排**：脚本做确定性改写、AI 仅产 JSON 计划、红线/epubcheck 每步兜底，丢一本脏书一条命令跑到收敛，产出 `cleaned.epub` + 「自动做了什么 / 建议你改 / 需人工」三分类报告。

**Architecture:** 循环、收敛、gate、回滚全在 Python（确定、可测、可 CI、默认零 AI）；每轮的「计划」由可插拔 `Planner` 提供——默认 `rules`（规则式，不调模型），可选 `handshake`（文件握手：脚本写 `plan-request.json`，外部 AI host 填 `plan.json`，脚本读回执行）。AI 全程**不碰正文文字**，只产受白名单约束的 JSON；任何 XHTML 改动后立即跑 `--check text`，红则回滚该文件。

**Tech Stack:** Python **3.14**（当前最新稳定版；项目决定，经 uv / mise 固定）+ 标准库（`argparse`/`subprocess`/`json`/`xml.etree.ElementTree`/`re`/`hashlib`/`zipfile`）。**不引入第三方依赖**（沿用仓库现状：脚本仅用标准库）。版本由仓库根 `.python-version`（内容 `3.14`）锁定，uv 与 mise 均识别；本机 `uv python install 3.14` 一次装好。测试沿用仓库风格——独立 `scripts/test_*.py`，用 `uv run python scripts/test_xxx.py`（或已切到 3.14 的 `python3 scripts/test_xxx.py`）运行（**本仓无 pytest**）。

---

## 0. 这份文档怎么用（自包含说明）

执行者无需阅读任何对话或其他文档即可实施本计划。所有要复用的现有接口、要改的行号、要新建的文件、要跑的命令，本文都给全。

**强制边界（实施前必读）：**

- **AI 永不直接编辑正文文件。** AI 的唯一产物是符合 schema 的 JSON 计划；所有落盘改写由确定性 Python 执行。
- **正确性由 gate 保证，不由模型保证。** 换任何模型（云端/本地/大/小）只改变「自动修了多少」，不改变「书安不安全」。所以本计划默认 `--planner rules` **完全不调模型即可跑通**。
- **每改一个 XHTML 文件 → 立即 `validate_text_invariance.py --check text` → 非 0 即回滚该文件。** 机制（正则/DOM/AI）只改变红线被触发的概率，不改变红线是否运行。
- **主观「排版好不好看」不进循环**（无收敛判据），只写进建议报告。
- 遵守 [AGENTS.md](../../AGENTS.md) 的「已有 EPUB 固定流程」「最小验证矩阵」与红线边界；本计划是其内的自动化增强，不放宽任何红线。
- 不改 `docs/final/` 硬规则；如循环需要新的语义类名/结构约定，先回写 SPEC 再实现（见 Task 9 备注）。

---

## 1. 背景与定位（自包含摘要）

- 本仓定位：**手册为主、AI 为辅**；优先服务「清洗现成 EPUB」，做新书其次；不做原生 App。
- 既有清洗入口 `scripts/epub_cleanup_pipeline.py` 是**纯确定性**的一条命令（无 AI）：`preflight → (可选 normalize) → epub3 转换 → 校验 → refinement/findings 报告`。
- 本计划在其上加「AI 辅助 + 多轮自动循环」，落地为新模块 `scripts/epub_cleanup_loop.py`，并复用现有 harness/gate。

---

## 2. 复用的现有接口（已核实，可直接调用）

| 用途 | 命令 / 接口 | 关键事实 |
| --- | --- | --- |
| 红线 gate | `python3 scripts/validate_text_invariance.py <before> <after> --check <list> [--allow-list <glob>] [--path-map <json>]` | `--check` 合法值：`text,metadata,spine,cover,drm,anchors,all`（`scripts/validate_text_invariance.py:31,488`）。退出码 0=通过。`--allow-list` 是 fnmatch glob，可多次。 |
| 确定性扫描器（产 findings） | `python3 scripts/epub_ai_harness.py --mode cleanup <epub> --format json` | 输出 `{mode, findings, recommended_skills, skill_levels, ...}`（`scripts/epub_ai_harness.py:111-119`）。**不调任何模型**。 |
| 精排建议（产 suggestions） | `python3 scripts/epub_refinement_harness.py <epub> --format json` | 输出 image/font/ruby/redline 等键（`scripts/epub_refinement_harness.py:123-135`）。 |
| 结构规范化 | `python3 scripts/epub_structure_tool.py normalize <epub> --output <out> [--dry-run] --report-format json` | 产 `mappings`/`warnings`；apply 后用 `--path-map` 喂红线。 |
| EPUB3 迁移/转换 | `python3 scripts/epub3_oneclick_converter.py <epub> --output <out> --format json` | docstring 明确「does not rewrite prose」（`scripts/epub3_oneclick_converter.py:11`）。**无内置文本自检**。 |
| CSS 清洗 | `python3 scripts/epub_css_cleanup.py ...` | 合并重复 CSS、替换旧字体链。CSS 改动不影响 `--check text`。 |
| 现有一键 pipeline（本计划的基线） | `scripts/epub_cleanup_pipeline.py` | 其转换后红线只 `--check metadata,drm,anchors`（`:282-296`）——**漏 text，见 Task 1 修复**。 |
| AI 审计契约（handshake planner 的指令源） | `skills/epub-layout-auditor/SKILL.md` 及各专项 `skills/epub-*/SKILL.md` | AI host 按这些 SKILL.md 填 plan。 |

---

## 3. 文件结构（新建/修改）

| 文件 | 责任 | 动作 |
| --- | --- | --- |
| `scripts/epub_cleanup_pipeline.py` | 现有一键 pipeline | **改**：补转换后 `--check text` 红线（Task 1） |
| `scripts/epub_xhtml_transforms.py` | 确定性 XHTML 改写引擎：消毒正则、属性值定点替换、DOM 白名单操作、禁止操作守卫；幂等 | **新建**（Task 2-4） |
| `scripts/epub_text_gate.py` | 红线 gate 的 Python 封装：`text_invariance_ok(before, after, allow=[...]) -> (bool, str)`；以及「单文件文本不变」工具 | **新建**（Task 5） |
| `scripts/epub_cleanup_loop.py` | 确定性多轮循环编排 + `RulesPlanner`/`HandshakePlanner` + 报告 + CLI | **新建**（Task 6-9） |
| `scripts/test_epub_xhtml_transforms.py` | transforms 单测 | 新建（随 Task 2-4） |
| `scripts/test_epub_text_gate.py` | gate 封装单测 | 新建（随 Task 5） |
| `scripts/test_epub_cleanup_loop.py` | 循环收敛/震荡/幂等/回滚/报告 单测 | 新建（随 Task 6-9） |
| `.github/workflows/build-epub-demo.yml` | CI | **改**：纳入新测试 + 现有 4 个漏测（Task 10） |
| `docs/pipeline/cleanup-flow.md` | 清洗流程文档 | **改**：加「自动循环清洗」一节 + 模型/隐私说明（Task 11） |
| `docs/final/SPEC-实现约束.md` | 硬规则 | 仅当新增语义类/结构约定时回写（Task 9 备注） |
| `.python-version` | Python 版本锁（内容 `3.14`），uv/mise 识别 | **新建**（Task 0） |
| `scripts/epub_ai_harness.py` | 扫描器升级：detector 注册表 + `actionable_findings` + 新检测器 | **改**（Task 5b） |
| `scripts/test_epub_ai_harness.py` | 已存在；追加 detector / `actionable_findings` 用例 | **改**（随 Task 5b） |
| `README.md` / `docs/getting-started/01-first-epub.md` / `requirements.txt` | Python 版本声明分叉（`≥3.9` vs `3.10+`）→ 统一为 3.14 | **改**（Task 0） |

**约定（跨 Task 类型一致性，勿改名）：**

- 一个改写动作 = `Action` dict：`{"op": <op-name>, "file": <zip-path>, "params": {...}, "lane": "css|tag|suggestion", "source": "rules|ai"}`。
- 计划 = `Plan` = `{"round": int, "actions": [Action, ...], "suggestions": [Suggestion, ...]}`。
- 工作目录布局：`<work>/before/source.epub`、`<work>/after/step-<N>.epub`、`<work>/after/cleaned.epub`、`<work>/reports/round-<N>.plan-request.json`、`<work>/reports/round-<N>.plan.json`、`<work>/reports/cleanup-loop.json`。
- 幂等标记：OPF metadata 写 `<meta property="epub-handbook:cleanup-rounds">N</meta>`；且每个 op 落盘前先检查目标态（已具备则跳过）。
- 停机常量：`DRY_LIMIT = 2`、`MAX_ROUNDS = 6`。
- 允许的红线 allow-list（导航不算正文）：`["*/nav.xhtml", "*/toc.ncx"]`。

---

## Task 0：环境与 Python 3.14 基线（先做）

**目标：** 把仓库 Python 基线定到 3.14（最新稳定），用 uv/mise 固定，消除现有版本声明分叉。后续所有 Task 的代码都按 3.14 写（现代语法可直接用；为与现有脚本风格一致仍可保留 `from __future__ import annotations`，非必需）。

> 取舍说明（自包含）：本工具纯标准库、不需要任何 3.12+ 新特性。选 3.14 是项目决定（维护者用 uv/mise 管版本，CI 与 git 平台均支持）。代价是**部署侧不再是「任意 python3 ≥ 3.9」**——保守/气隙环境需随包提供 3.14，可用 uv 离线预置 Python 解决。保持纯标准库、不要引入仅为凑版本的依赖。

**Files:**
- Create: `.python-version`
- Modify: `README.md:53`、`docs/getting-started/01-first-epub.md:6`、`requirements.txt`

- [ ] **Step 1: 锁定版本**

```bash
echo '3.14' > .python-version
uv python install 3.14            # 或 mise use python@3.14
uv run python --version           # 期望 Python 3.14.x
```

- [ ] **Step 2: 消除版本声明分叉**

- `README.md:53` 现 `| python3 ≥ 3.9 | 红线脚本、harness、validator |` → 改为 `| python3 3.14（经 uv/mise 固定） | 红线脚本、harness、validator |`
- `docs/getting-started/01-first-epub.md:6` 现 `- Python 3.10+。` → 改为 `- Python 3.14（推荐用 uv 或 mise 安装：` + 反引号 `uv python install 3.14` 反引号 + `）。`
- `requirements.txt`：顶部注释补一行 `# Target runtime: Python 3.14 (pinned via .python-version; uv/mise).`

- [ ] **Step 3: 验证全测试在 3.14 下通过**

```bash
for t in scripts/test_*.py; do uv run python "$t" || echo "FAIL $t"; done
```
Expected: 无 FAIL 行（现有脚本用 `from __future__ import annotations`，在 3.14 下完全兼容）。

- [ ] **Step 4: Commit**

```bash
git add .python-version README.md docs/getting-started/01-first-epub.md requirements.txt
git commit -m "build: pin Python 3.14 via uv/mise and unify version declarations"
```

---

## Task 1：修复现存红线缺口（转换后补 `--check text`）

**这是安全地基，必须最先做。** 现状：`epub_cleanup_pipeline.py` 转换后只校验 `metadata,drm,anchors`，**没验证正文未被改**。

**Files:**
- Modify: `scripts/epub_cleanup_pipeline.py:282-296`
- Test: `scripts/test_epub_cleanup_pipeline.py`（已存在，追加用例）

- [ ] **Step 1: 写失败测试** — 构造一个「转换会改正文」的反例 fixture，断言 pipeline 失败。

在 `scripts/test_epub_cleanup_pipeline.py` 追加（沿用该文件已有的 fixture 构造工具；若无则用 `zipfile` 现造一个最小 EPUB，正文含一句可识别文本）：

```python
def test_pipeline_fails_when_body_text_changes(tmp_path):
    # 造一个 before.epub，正文含 "ORIGINAL-SENTENCE-7F3A"
    src = make_min_epub(tmp_path / "src.epub", body="ORIGINAL-SENTENCE-7F3A")
    # 用 monkeypatch/stub 让 convert 步骤产出把该句改成 "TAMPERED" 的 after
    # （或指向一个故意篡改正文的转换器桩）
    report = run_pipeline_with_tampering(src, tmp_path / "work")
    assert report.status == "failed"
    assert any("text" in s.name and s.returncode != 0 for s in report.steps)
```

- [ ] **Step 2: 跑测试确认失败**

Run: `python3 scripts/test_epub_cleanup_pipeline.py`
Expected: 新用例 FAIL（当前 pipeline 不查 text，篡改正文也会「通过」）。

- [ ] **Step 3: 在 `validate-redline-subset` 之前/之后增加一道 text 红线步骤**

在 `scripts/epub_cleanup_pipeline.py` 的 `validate-redline-subset`（`:282-296`）相邻处新增：

```python
    run_step(
      report,
      "validate-redline-text",
      [
        sys.executable,
        str(SCRIPTS / "validate_text_invariance.py"),
        str(base_path),
        str(output_path),
        "--check",
        "text",
        "--allow-list",
        "*/nav.xhtml",
        "--allow-list",
        "*/toc.ncx",
      ],
      reports_dir / "validate-redline-text.txt",
    )
```

`run_step` 默认 `allowed_codes=(0,)`，非 0 即抛 `PipelineError` → pipeline 失败。这正是要的硬 gate。

- [ ] **Step 4: 跑测试确认通过**

Run: `python3 scripts/test_epub_cleanup_pipeline.py`
Expected: 全部 PASS（篡改正文现在会被 `validate-redline-text` 挡下）。

- [ ] **Step 5: 回归——正常转换仍应通过**

Run: `python3 scripts/test_epub_cleanup_harnesses.py`（若覆盖 pipeline 正常路径）或对一个 demo 样本跑 `epub_cleanup_pipeline.py`，确认 `status: complete`。

- [ ] **Step 6: Commit**

```bash
git add scripts/epub_cleanup_pipeline.py scripts/test_epub_cleanup_pipeline.py
git commit -m "fix(pipeline): enforce body-text red-line after epub3 conversion"
```

---

## Task 2：消毒正则 + 属性值定点替换（transforms 引擎，lane ② 之正则部分）

**目标：** 最小消毒（让脏 HTML 可被 XML 解析）+ 锚定的属性值替换。这两类 **diff 最小、动不到正文**，是 lane ② 的低风险机制。

**Files:**
- Create: `scripts/epub_xhtml_transforms.py`
- Test: `scripts/test_epub_xhtml_transforms.py`

- [ ] **Step 1: 写失败测试**

```python
import scripts.epub_xhtml_transforms as T  # 若无包结构，用 importlib 按路径加载

def test_sanitize_entities_minimal_diff():
    src = '<p>a&nbsp;b</p>'
    out = T.sanitize_for_xml(src)
    assert out == '<p>a&#160;b</p>'          # 仅替换实体，正文字符不动

def test_class_value_rename_anchored():
    src = '<p class="calibre12">正文不变</p>'
    out, n = T.rename_class_values(src, {"calibre12": "para"})
    assert out == '<p class="para">正文不变</p>'
    assert n == 1

def test_class_rename_is_idempotent():
    src = '<p class="para">正文不变</p>'
    out, n = T.rename_class_values(src, {"calibre12": "para"})
    assert out == src and n == 0             # 已是目标态 → 不动、计数 0

def test_class_rename_never_touches_prose_lookalike():
    # 正文里出现 "calibre12" 字样不应被替换（只动属性值）
    src = '<p class="calibre12">这里提到 calibre12 这个词</p>'
    out, _ = T.rename_class_values(src, {"calibre12": "para"})
    assert '这里提到 calibre12 这个词' in out
    assert 'class="para"' in out
```

- [ ] **Step 2: 跑测试确认失败**

Run: `python3 scripts/test_epub_xhtml_transforms.py`
Expected: FAIL（模块/函数未定义）。

- [ ] **Step 3: 实现 `sanitize_for_xml` 与 `rename_class_values`**

```python
"""Deterministic, minimal-diff XHTML string transforms (no DOM reserialize).

Only two jobs live here:
1. sanitize_for_xml: smallest set of replacements so xml.etree can parse dirty XHTML.
2. rename_class_values: anchored attribute-VALUE substitutions (class="..." only).

Anything structural belongs in the DOM layer (Task 3/4), never regex.
"""
from __future__ import annotations
import re

# 仅白名单内的命名实体；其余 &...; 保持原样（数字实体本身合法）。
_NAMED_ENTITIES = {"&nbsp;": "&#160;"}

def sanitize_for_xml(text: str) -> str:
  for k, v in _NAMED_ENTITIES.items():
    text = text.replace(k, v)
  return text

# 锚定到 class 属性值；同时支持单/双引号；不跨标签、不进正文。
_CLASS_ATTR = re.compile(r'(?P<pre>\bclass\s*=\s*)(?P<q>["\'])(?P<val>[^"\']*)(?P=q)')

def rename_class_values(text: str, mapping: dict[str, str]) -> tuple[str, int]:
  count = 0
  def repl(m: re.Match) -> str:
    nonlocal count
    classes = m.group("val").split()
    new = [mapping.get(c, c) for c in classes]
    if new != classes:
      count += 1
    return f'{m.group("pre")}{m.group("q")}{" ".join(new)}{m.group("q")}'
  return _CLASS_ATTR.sub(repl, text), count
```

- [ ] **Step 4: 跑测试确认通过**

Run: `python3 scripts/test_epub_xhtml_transforms.py`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add scripts/epub_xhtml_transforms.py scripts/test_epub_xhtml_transforms.py
git commit -m "feat(transforms): minimal sanitize + anchored class-value rename"
```

---

## Task 3：DOM 白名单操作（lane ② 之结构部分，(甲) 保守默认）

**目标：** 用 `xml.etree` 树操作做**加属性 / 安全语义标注**：`epub:type`、`xml:lang`、`id`、加 class。树操作碰不到文本节点，安全；但会重序列化——**只重写真正改动的文件**，并在报告标记。

**Files:**
- Modify: `scripts/epub_xhtml_transforms.py`
- Test: `scripts/test_epub_xhtml_transforms.py`

- [ ] **Step 1: 写失败测试**

```python
def test_add_epub_type_attribute():
    xhtml = ('<?xml version="1.0" encoding="utf-8"?>'
             '<html xmlns="http://www.w3.org/1999/xhtml" '
             'xmlns:epub="http://www.idpf.org/2007/ops"><body>'
             '<aside id="fn1"><p>注释正文</p></aside></body></html>')
    out, changed = T.dom_add_attr(xhtml, target_id="fn1",
                                  attr="{http://www.idpf.org/2007/ops}type",
                                  value="footnote")
    assert changed is True
    assert 'epub:type="footnote"' in out
    assert '注释正文' in out                       # 文本节点原样

def test_dom_add_attr_idempotent():
    xhtml = ('<html xmlns="http://www.w3.org/1999/xhtml" '
             'xmlns:epub="http://www.idpf.org/2007/ops"><body>'
             '<aside id="fn1" epub:type="footnote"><p>x</p></aside></body></html>')
    out, changed = T.dom_add_attr(xhtml, "fn1",
                                  "{http://www.idpf.org/2007/ops}type", "footnote")
    assert changed is False                        # 已存在目标值 → 不动
```

- [ ] **Step 2: 跑测试确认失败**

Run: `python3 scripts/test_epub_xhtml_transforms.py`
Expected: FAIL。

- [ ] **Step 3: 实现 `dom_add_attr`（保留命名空间、保留 XML 声明）**

```python
import xml.etree.ElementTree as ET

EPUB_NS = "http://www.idpf.org/2007/ops"
XHTML_NS = "http://www.w3.org/1999/xhtml"
XML_NS = "http://www.w3.org/XML/1998/namespace"

def _register_ns() -> None:
  ET.register_namespace("", XHTML_NS)
  ET.register_namespace("epub", EPUB_NS)

def dom_add_attr(xhtml: str, target_id: str, attr: str, value: str) -> tuple[str, bool]:
  """Add/ensure one attribute on the element whose id == target_id.

  Returns (serialized, changed). changed=False when already at target value.
  Never edits text nodes. Whitelisted attrs only — caller must pass allowed attr.
  """
  _register_ns()
  root = ET.fromstring(xhtml)
  changed = False
  for el in root.iter():
    if el.get("id") == target_id:
      if el.get(attr) != value:
        el.set(attr, value)
        changed = True
      break
  if not changed:
    return xhtml, False
  body = ET.tostring(root, encoding="unicode")
  if xhtml.lstrip().startswith("<?xml"):
    body = '<?xml version="1.0" encoding="utf-8"?>\n' + body
  return body, True
```

> 实现注意：调用方只允许传白名单 attr（见 Task 6 的 `ALLOWED_DOM_OPS`），本函数不自行放行任意属性。`xml:lang` 用 `"{%s}lang" % XML_NS`。

- [ ] **Step 4: 跑测试确认通过**

Run: `python3 scripts/test_epub_xhtml_transforms.py`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add scripts/epub_xhtml_transforms.py scripts/test_epub_xhtml_transforms.py
git commit -m "feat(transforms): DOM whitelisted attribute add (epub:type/xml:lang/id/class)"
```

---

## Task 4：结构改写（lane ② 之 (乙') 选项）+ 禁止操作守卫

**目标：** `--enable-structural` 显式开启时才允许的白名单结构升级（如 `<div class="quote">` → `<blockquote>`、章节包 `<section>`）。同时实现**禁止操作守卫**：任何会改文本字符数/顺序的操作直接拒绝。

**Files:**
- Modify: `scripts/epub_xhtml_transforms.py`
- Test: `scripts/test_epub_xhtml_transforms.py`

- [ ] **Step 1: 写失败测试**

```python
def test_div_quote_to_blockquote_preserves_text():
    xhtml = ('<html xmlns="http://www.w3.org/1999/xhtml"><body>'
             '<div class="quote"><p>引文一字不差</p></div></body></html>')
    out, changed = T.dom_rewrite_tag(xhtml, match={"tag": "div", "class": "quote"},
                                     new_tag="blockquote")
    assert changed is True
    assert '<blockquote' in out and '引文一字不差' in out

def test_forbidden_guard_rejects_text_change():
    before = '<p>正文 ABCDE</p>'
    after  = '<p>正文 ABCD</p>'      # 少了一个字符
    assert T.text_content_equal(before, after) is False  # 守卫据此拒绝/回滚
```

- [ ] **Step 2: 跑测试确认失败** — Run: `python3 scripts/test_epub_xhtml_transforms.py` → FAIL。

- [ ] **Step 3: 实现 `dom_rewrite_tag` 与 `text_content_equal`**

```python
def text_content_equal(a: str, b: str) -> bool:
  """Compare concatenated, normalized text nodes (NFC) of two XHTML strings."""
  import unicodedata
  def texts(s: str) -> str:
    root = ET.fromstring(s if s.lstrip().startswith("<") else f"<x>{s}</x>")
    return unicodedata.normalize("NFC", "".join(root.itertext()))
  return texts(a) == texts(b)

def dom_rewrite_tag(xhtml: str, match: dict, new_tag: str) -> tuple[str, bool]:
  _register_ns()
  root = ET.fromstring(xhtml)
  before_text = "".join(root.itertext())
  changed = False
  for parent in root.iter():
    for i, el in enumerate(list(parent)):
      local = el.tag.split("}")[-1]
      if local == match.get("tag") and (
        not match.get("class") or match["class"] in (el.get("class") or "").split()
      ):
        el.tag = f"{{{XHTML_NS}}}{new_tag}"
        if "class" in match:
          # 去掉被语义化的 class，保留其余
          rest = [c for c in (el.get("class") or "").split() if c != match["class"]]
          if rest: el.set("class", " ".join(rest))
          elif el.get("class") is not None: del el.attrib["class"]
        changed = True
  if not changed:
    return xhtml, False
  out = ET.tostring(root, encoding="unicode")
  # 守卫：结构改写后文本必须逐字不变，否则视为非法、拒绝
  if "".join(ET.fromstring(out).itertext()) != before_text:
    raise T.ForbiddenTextChange(f"tag rewrite changed text content: {match}")
  if xhtml.lstrip().startswith("<?xml"):
    out = '<?xml version="1.0" encoding="utf-8"?>\n' + out
  return out, True
```

并定义异常：

```python
class ForbiddenTextChange(Exception):
  """A transform attempted to alter prose text content; refused."""
```

- [ ] **Step 4: 跑测试确认通过** — Run: `python3 scripts/test_epub_xhtml_transforms.py` → PASS。

- [ ] **Step 5: Commit**

```bash
git add scripts/epub_xhtml_transforms.py scripts/test_epub_xhtml_transforms.py
git commit -m "feat(transforms): opt-in structural rewrite + forbidden-text-change guard"
```

---

## Task 5：红线 gate 的 Python 封装（每文件 / 整包）

**目标：** 把 `validate_text_invariance.py` 封成函数，供循环每步调用；并提供「单文件文本不变」快速判定（基于 Task 4 的 `text_content_equal`）。

**Files:**
- Create: `scripts/epub_text_gate.py`
- Test: `scripts/test_epub_text_gate.py`

- [ ] **Step 1: 写失败测试**

```python
import scripts.epub_text_gate as G

def test_epub_text_gate_passes_for_identical(tmp_path):
    epub = make_min_epub(tmp_path / "a.epub", body="不变文本")
    ok, report = G.text_invariance_ok(epub, epub, allow=["*/nav.xhtml"])
    assert ok is True

def test_epub_text_gate_fails_for_tampered(tmp_path):
    a = make_min_epub(tmp_path / "a.epub", body="原始文本")
    b = make_min_epub(tmp_path / "b.epub", body="被改文本")
    ok, report = G.text_invariance_ok(a, b)
    assert ok is False
    assert "text" in report.lower()
```

- [ ] **Step 2: 跑测试确认失败** — Run: `python3 scripts/test_epub_text_gate.py` → FAIL。

- [ ] **Step 3: 实现封装**

```python
"""Thin wrapper around scripts/validate_text_invariance.py for the loop."""
from __future__ import annotations
import subprocess, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
NAV_ALLOW = ["*/nav.xhtml", "*/toc.ncx"]

def text_invariance_ok(before, after, allow=None, path_map=None) -> tuple[bool, str]:
  cmd = [sys.executable, str(SCRIPTS / "validate_text_invariance.py"),
         str(before), str(after), "--check", "text"]
  for g in (allow if allow is not None else NAV_ALLOW):
    cmd += ["--allow-list", g]
  if path_map:
    cmd += ["--path-map", str(path_map)]
  r = subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True)
  return r.returncode == 0, (r.stdout + r.stderr)
```

- [ ] **Step 4: 跑测试确认通过** — Run: `python3 scripts/test_epub_text_gate.py` → PASS。

- [ ] **Step 5: Commit**

```bash
git add scripts/epub_text_gate.py scripts/test_epub_text_gate.py
git commit -m "feat(gate): python wrapper for body-text red-line"
```

---

## Task 5b：扩展 harness 为 detector 注册表 + `actionable_findings` + 新检测器

**目标：** 把 `epub_ai_harness.py` 从「建议该跑哪个 skill」升级成「产可执行 findings」。现状 `finding(level, message, label)` 只够给人看；新增 `actionable_findings`（向后兼容，旧 `findings`/`findings_by_level`/`recommended_skills` 不动），让零模型的 `RulesPlanner` 据此直接产 Action。把检查重构成可插拔 **detector 注册表**，并新增覆盖。

**`actionable_findings` 单条 schema：**

```jsonc
{
  "kind": "missing-epub-type | missing-html-lang | obfuscated-class | empty-paragraph | missing-manifest-properties | ...",
  "file": "OEBPS/Text/ch1.xhtml",        // zip 内路径；package 级 finding 用 OPF 路径
  "locator": { "id": "fn1" },             // 或 {"selector": "..."} / {"manifest_id": "..."}
  "params": { "value": "footnote" },      // 执行该 op 所需的一切（mapping/lang/value/rule）
  "lane": "tag | css | package",
  "auto_fixable": true,                    // true=RulesPlanner 可零模型映射成白名单 op
  "confidence": "high | medium | low",
  "evidence": "为什么判定（人读）"
}
```

**Files:**
- Modify: `scripts/epub_ai_harness.py`
- Test: `scripts/test_epub_ai_harness.py`（追加用例；该测试已在 CI 中）

- [ ] **Step 1: 写失败测试**

```python
import scripts.epub_ai_harness as H

def test_actionable_findings_present_and_backward_compatible(tmp_path):
    epub = make_epub_missing_html_lang(tmp_path / "b.epub")   # 根 <html> 缺 lang
    data = H.run(epub, mode="cleanup")          # 现有入口返回/序列化的 dict
    assert "findings" in data and "findings_by_level" in data   # 旧键仍在
    af = data["actionable_findings"]
    assert any(f["kind"] == "missing-html-lang" and f["auto_fixable"] for f in af)
    one = next(f for f in af if f["kind"] == "missing-html-lang")
    assert one["lane"] == "tag" and one["confidence"] in ("high", "medium", "low")
    assert "file" in one and "params" in one

def test_detector_is_idempotent_when_already_fixed(tmp_path):
    epub = make_epub_with_html_lang(tmp_path / "ok.epub")     # 已有 lang
    data = H.run(epub, mode="cleanup")
    assert not any(f["kind"] == "missing-html-lang" for f in data["actionable_findings"])

def test_registry_lists_detectors():
    names = {d.kind for d in H.DETECTORS}
    assert {"missing-html-lang", "obfuscated-class", "empty-paragraph",
            "missing-manifest-properties"} <= names
```

- [ ] **Step 2: 跑测试确认失败** — Run: `python3 scripts/test_epub_ai_harness.py` → FAIL。

- [ ] **Step 3: 引入 detector 注册表 + 新检测器**

```python
from dataclasses import dataclass
from typing import Callable

@dataclass(frozen=True)
class Detector:
  kind: str
  lane: str                 # tag | css | package
  fn: Callable              # (model) -> list[actionable_finding dict]

DETECTORS: list[Detector] = []

def detector(kind, lane):
  def reg(fn):
    DETECTORS.append(Detector(kind, lane, fn)); return fn
  return reg

# 每个 detector 必须幂等：目标态已满足就不产 finding（支撑循环 DRY 收敛）。
@detector("missing-html-lang", "tag")
def _missing_html_lang(model):
  out = []
  for path, root in model.xhtml_docs():          # (zip_path, 解析好的 ElementTree root)
    if not (root.get("lang") or root.get("{http://www.w3.org/XML/1998/namespace}lang")):
      out.append({"kind": "missing-html-lang", "file": path,
                  "locator": {"selector": "html"},
                  "params": {"value": model.book_language or "zh-Hans"},
                  "lane": "tag", "auto_fixable": True, "confidence": "high",
                  "evidence": "<html> 根元素缺 lang/xml:lang"})
  return out

@detector("obfuscated-class", "tag")
def _obfuscated_class(model):
  # 仅当能确定性导出 mapping（如 calibreNN -> 丢弃/语义名）时 auto_fixable=True；
  # 否则 auto_fixable=False，交给建议/handshake。params 带 {"mapping": {...}}。
  ...

@detector("empty-paragraph", "tag")
def _empty_paragraph(model):
  # 空 <p></p> / 仅含 &#160; / 堆叠 <br>。删除是结构动作、文本 gate 安全。
  # 标 auto_fixable=True、params 带 {"rule": "drop-empty-paragraph"}；
  # 是否真执行由 planner 的 --enable-structural 决定（见 Task 6）。
  ...

@detector("missing-manifest-properties", "package")
def _missing_manifest_properties(model):
  # svg/mathml/cover-image 的 manifest properties 缺失（复用 package-nav 检查逻辑）。
  ...

def collect_actionable_findings(model) -> list[dict]:
  found = []
  for d in DETECTORS:
    found.extend(d.fn(model))
  return found
```

并在报告 `as_dict()`（`scripts/epub_ai_harness.py:111-119`）追加键：

```python
      "actionable_findings": collect_actionable_findings(self.model),
```

> 实现注意：
> - **不破坏现有输出**：`findings`/`findings_by_level`/`recommended_skills` 原样保留，只新增 `actionable_findings`。
> - **判断类不进 detector**：如「这个 div 是不是脚注」——标 `auto_fixable: false` 或不产 actionable，留给 AI/人。detector 只做确定性可判定项。
> - **每个 detector 幂等**：已是目标态不产 finding。
> - `model` 是把 EPUB 解析成 (xhtml_docs / opf / css) 的内存视图；若现有 harness 无此抽象，本 Task 顺带抽一个最小 `EpubModel`（仅标准库 `zipfile`+`xml.etree`）。

- [ ] **Step 4: 跑测试确认通过** — Run: `python3 scripts/test_epub_ai_harness.py` → PASS。

- [ ] **Step 5: Commit**

```bash
git add scripts/epub_ai_harness.py scripts/test_epub_ai_harness.py
git commit -m "feat(harness): detector registry + actionable_findings + new detectors"
```

---

## Task 6：规则式 Planner（`RulesPlanner`，默认、零模型）

**目标：** 把 `epub_ai_harness` + `epub_refinement_harness` 的 findings 映射成**白名单内的 `Action` 列表**；主观项归入 `suggestions`，**不进 actions**。这是默认计划器，**不调任何模型**。

**Files:**
- Create: `scripts/epub_cleanup_loop.py`（先放白名单常量 + RulesPlanner）
- Test: `scripts/test_epub_cleanup_loop.py`

定义白名单（写进 `epub_cleanup_loop.py` 顶部）：

```python
# lane ② 允许的确定性操作（甲=默认；structural 需 --enable-structural）
ALLOWED_VALUE_RENAME = True          # 正则 class 值替换（需提供 mapping）
ALLOWED_DOM_OPS = {                  # DOM 加属性
  "epub:type": "{http://www.idpf.org/2007/ops}type",
  "xml:lang":  "{http://www.w3.org/XML/1998/namespace}lang",
  "id":        "id",
  "class":     "class",
}
ALLOWED_STRUCTURAL = {               # 仅 --enable-structural 时启用
  "div.quote->blockquote": {"match": {"tag": "div", "class": "quote"}, "new_tag": "blockquote"},
}
# 永远禁止：删除/移动节点、合并或拆分段落、改写任何文本字符。
```

- [ ] **Step 1: 写失败测试**

```python
import scripts.epub_cleanup_loop as L

def test_rules_planner_maps_auto_fixable_findings():
    # 消费 Task 5b 的 actionable_findings；只据 auto_fixable + confidence + kind 映射
    findings = {"actionable_findings": [
        {"kind": "missing-epub-type", "file": "OEBPS/Text/ch1.xhtml",
         "locator": {"id": "fn1"}, "params": {"value": "footnote"},
         "lane": "tag", "auto_fixable": True, "confidence": "high"},
        {"kind": "ambiguous-note", "file": "OEBPS/Text/ch1.xhtml",
         "locator": {"id": "x"}, "params": {}, "lane": "tag",
         "auto_fixable": False, "confidence": "low"},     # 需判断
    ]}
    refine = {"ruby_count": 0, "risky_images": []}
    plan = L.RulesPlanner().plan(round=1, findings=findings, refinement=refine,
                                 enable_structural=False)
    ops = {a["op"] for a in plan["actions"]}
    assert "add-epub-type" in ops
    assert all(a["lane"] in ("css", "tag") for a in plan["actions"])
    # auto_fixable=False 的不进 actions，只进 suggestions
    assert not any(a.get("op") == "ambiguous-note" for a in plan["actions"])
    assert any(s.get("kind") == "ambiguous-note" for s in plan["suggestions"])

def test_rules_planner_uses_no_model():
    # RulesPlanner 不得发起任何网络/子进程模型调用：纯函数映射
    plan = L.RulesPlanner().plan(round=1, findings={"actionable_findings": []},
                                 refinement={}, enable_structural=False)
    assert plan["actions"] == [] and plan["round"] == 1
```

- [ ] **Step 2: 跑测试确认失败** — Run: `python3 scripts/test_epub_cleanup_loop.py` → FAIL。

- [ ] **Step 3: 实现 `RulesPlanner.plan`**

```python
# kind -> 白名单 op 映射（key 必须与 Task 5b 各 detector 的 kind 完全一致）
_KIND_TO_OP = {
  "missing-epub-type": "add-epub-type",
  "missing-html-lang": "add-xml-lang",
  "missing-lang":      "add-xml-lang",
  "obfuscated-class":  "rename-class",
  "div-quote":         "rewrite-tag",       # 结构动作，需 enable_structural
  "empty-paragraph":   "rewrite-tag",       # 结构动作，需 enable_structural
}
_STRUCTURAL_KINDS = {"div-quote", "empty-paragraph"}

class RulesPlanner:
  source = "rules"
  def plan(self, round, findings, refinement, enable_structural):
    actions, suggestions = [], []
    for f in findings.get("actionable_findings", []):
      kind = f.get("kind", "")
      op = _KIND_TO_OP.get(kind)
      structural_ok = kind not in _STRUCTURAL_KINDS or enable_structural
      if op and f.get("auto_fixable") and f.get("confidence") == "high" and structural_ok:
        if op == "rewrite-tag":
          # 结构改写的参数取自白名单 ALLOWED_STRUCTURAL（按 finding.params.rule 选）
          params = ALLOWED_STRUCTURAL[f["params"]["rule"]]
        else:
          params = {**f.get("locator", {}), **f.get("params", {})}
        actions.append({"op": op, "file": f["file"], "params": params,
                        "lane": f.get("lane", "tag"), "source": self.source})
      else:
        # auto_fixable=False / 非 high / 未启用结构 / 无 op → 建议或需人工，不进 actions
        suggestions.append({"kind": kind, "file": f.get("file"),
                            "note": f.get("evidence", "needs human judgment")})
    # refinement 的排版类一律是建议（lane ③），不进循环
    for key in ("risky_images", "body_font_chains"):
      if refinement.get(key):
        suggestions.append({"kind": f"refinement:{key}", "detail": refinement[key]})
    return {"round": round, "actions": actions, "suggestions": suggestions}
```

> 备注：`RulesPlanner` 消费 **Task 5b** 产出的 `actionable_findings`（不是旧 `findings`）。`_KIND_TO_OP` 的 key 必须与 Task 5b 各 detector 的 `kind` 完全一致；新增 detector 时同步在此登记。实施时跑 `python3 scripts/epub_ai_harness.py --mode cleanup <脏样本> --format json` 核对真实 `kind` 与 schema。

- [ ] **Step 4: 跑测试确认通过** — Run: `python3 scripts/test_epub_cleanup_loop.py` → PASS。

- [ ] **Step 5: Commit**

```bash
git add scripts/epub_cleanup_loop.py scripts/test_epub_cleanup_loop.py
git commit -m "feat(loop): rules planner mapping findings to whitelisted actions"
```

---

## Task 7：动作执行器 + 每文件红线 + 幂等

**目标：** 把一个 `Action` 落到 EPUB 内对应文件，改完**立即对该文件跑文本不变判定**（Task 4 的 `text_content_equal`，快），红则回滚该文件并把动作记入 `needs_human`。幂等：目标态已具备则跳过。

**Files:**
- Modify: `scripts/epub_cleanup_loop.py`
- Test: `scripts/test_epub_cleanup_loop.py`

- [ ] **Step 1: 写失败测试**

```python
def test_apply_action_reverts_on_text_change(tmp_path):
    files = {"OEBPS/Text/ch1.xhtml":
             '<html xmlns="http://www.w3.org/1999/xhtml"><body><p>原文ABC</p></body></html>'}
    # 一个"坏 op"，故意改文本；执行器必须回滚该文件并记 needs_human
    action = {"op": "BUGGY-text-mutator", "file": "OEBPS/Text/ch1.xhtml",
              "params": {}, "lane": "tag", "source": "ai"}
    result = L.apply_action(files, action)
    assert files["OEBPS/Text/ch1.xhtml"].count("原文ABC") == 1   # 未被改
    assert result["status"] == "reverted"
    assert result["reason"] == "text-changed"

def test_apply_action_idempotent(tmp_path):
    files = {"OEBPS/Text/ch1.xhtml":
             '<html xmlns="http://www.w3.org/1999/xhtml" '
             'xmlns:epub="http://www.idpf.org/2007/ops"><body>'
             '<aside id="fn1" epub:type="footnote"><p>x</p></aside></body></html>'}
    action = {"op": "add-epub-type", "file": "OEBPS/Text/ch1.xhtml",
              "params": {"id": "fn1", "value": "footnote"}, "lane": "tag", "source": "rules"}
    result = L.apply_action(files, action)
    assert result["status"] == "noop"           # 已是目标态
```

- [ ] **Step 2: 跑测试确认失败** — Run: `python3 scripts/test_epub_cleanup_loop.py` → FAIL。

- [ ] **Step 3: 实现 `apply_action`（在内存 `files: dict[zip_path,str]` 上操作）**

```python
import scripts.epub_xhtml_transforms as T

def apply_action(files, action):
  path, op, p = action["file"], action["op"], action.get("params", {})
  if path not in files:
    return {"status": "skipped", "reason": "file-missing", "action": action}
  before = files[path]
  try:
    if op == "add-epub-type":
      after, changed = T.dom_add_attr(before, p["id"], T.f"{{...}}", p["value"])  # 用 ALLOWED_DOM_OPS["epub:type"]
    elif op == "add-xml-lang":
      after, changed = T.dom_add_attr(before, p["id"], ALLOWED_DOM_OPS["xml:lang"], p["value"])
    elif op == "rename-class":
      after, n = T.rename_class_values(before, p["mapping"]); changed = n > 0
    elif op == "rewrite-tag":
      after, changed = T.dom_rewrite_tag(before, p["match"], p["new_tag"])
    else:
      # 未知/越界 op（含 AI 误产）→ 不执行，记 needs_human
      return {"status": "rejected", "reason": "op-not-whitelisted", "action": action}
  except T.ForbiddenTextChange:
    return {"status": "reverted", "reason": "text-changed", "action": action}
  if not changed:
    return {"status": "noop", "action": action}
  # 每文件红线：文本必须逐字不变（导航文件由整包 gate 的 allow-list 处理）
  if not T.text_content_equal(before, after):
    return {"status": "reverted", "reason": "text-changed", "action": action}
  files[path] = after
  return {"status": "applied", "action": action}
```

> 修正：`add-epub-type` 那行用 `ALLOWED_DOM_OPS["epub:type"]` 作为 attr 名（上面伪占位 `f"{{...}}"` 仅示意，实现时替换为该常量）。

- [ ] **Step 4: 跑测试确认通过** — Run: `python3 scripts/test_epub_cleanup_loop.py` → PASS。

- [ ] **Step 5: Commit**

```bash
git add scripts/epub_cleanup_loop.py scripts/test_epub_cleanup_loop.py
git commit -m "feat(loop): action executor with per-file text gate and idempotency"
```

---

## Task 8：循环编排 + 收敛 + 回滚锚点 + 指纹守卫

**目标：** 把「审计 → 计划 → 执行 → 整包红线 + epubcheck → 存锚点 / 回滚 → DRY 判定」串成确定性循环；停机 = `DRY≥2 或 轮数≥6 或 状态指纹重复`。

**Files:**
- Modify: `scripts/epub_cleanup_loop.py`
- Test: `scripts/test_epub_cleanup_loop.py`

- [ ] **Step 1: 写失败测试**

```python
def test_loop_converges_and_stops_dry(tmp_path):
    dirty = make_dirty_epub(tmp_path / "dirty.epub")   # 含若干可规则修复点
    rep = L.run_loop(dirty, tmp_path / "work", planner=L.RulesPlanner(),
                     max_rounds=6, dry_limit=2)
    assert rep["status"] == "complete"
    assert rep["stopped_by"] in ("dry", "fingerprint")
    assert rep["rounds_run"] <= 6

def test_loop_is_idempotent_on_clean_input(tmp_path):
    clean = make_min_epub(tmp_path / "clean.epub", body="干净")
    rep = L.run_loop(clean, tmp_path / "work2", planner=L.RulesPlanner())
    assert sum(len(r["applied"]) for r in rep["round_log"]) == 0   # 无可改 → 0 动作
    assert rep["stopped_by"] == "dry"

def test_loop_fingerprint_guard_breaks_oscillation(tmp_path):
    # 用一个故意来回改的 stub planner，验证指纹重复时强制停机
    rep = L.run_loop(make_min_epub(tmp_path / "o.epub", body="x"),
                     tmp_path / "work3", planner=OscillatingStubPlanner(),
                     max_rounds=20)
    assert rep["stopped_by"] == "fingerprint"
    assert rep["rounds_run"] < 20
```

- [ ] **Step 2: 跑测试确认失败** — Run: `python3 scripts/test_epub_cleanup_loop.py` → FAIL。

- [ ] **Step 3: 实现 `run_loop`**

```python
import hashlib, json, shutil, zipfile
from pathlib import Path

def _epub_fingerprint(files: dict) -> str:
  h = hashlib.sha256()
  for name in sorted(files):
    h.update(name.encode()); h.update(b"\0"); h.update(files[name].encode("utf-8"))
  return h.hexdigest()

def run_loop(input_epub, work_dir, planner, max_rounds=6, dry_limit=2,
             enable_structural=False):
  work = Path(work_dir); (work / "after").mkdir(parents=True, exist_ok=True)
  reports = work / "reports"; reports.mkdir(parents=True, exist_ok=True)
  base = _stage_input(input_epub, work)          # 复制 before、跑 preflight、(可选)normalize、epub3 迁移；并对每步跑整包 --check text
  files = _read_xhtml_members(base)              # {zip_path: text}（仅 XHTML/CSS 进内存）
  seen, dry, log, stopped = set(), 0, [], "max-rounds"
  for rnd in range(1, max_rounds + 1):
    findings = _audit(base)                      # epub_ai_harness --mode cleanup
    refine = _refine(base)                       # epub_refinement_harness
    plan = planner.plan(rnd, findings, refine, enable_structural)
    _write_json(reports / f"round-{rnd}.plan-request.json", {"findings": findings, "refinement": refine})
    _write_json(reports / f"round-{rnd}.plan.json", plan)
    applied, needs_human = [], list(plan.get("suggestions", []))
    for action in plan["actions"]:
      res = apply_action(files, action)
      (applied if res["status"] == "applied" else needs_human).append(res)
    base = _repack(files, work / "after" / f"step-{rnd}.epub")
    ok_text, txt = _gate_text(base)              # 整包 --check text（allow nav/ncx）
    ok_check = _epubcheck(base)                  # 若环境无 epubcheck 则记 skipped（不阻断）
    if not (ok_text and ok_check):
      # 整轮回滚到上一个锚点；本轮可疑动作进 needs_human
      base = _prev_anchor(work, rnd)
      files = _read_xhtml_members(base)
      needs_human.append({"round": rnd, "reason": "round-gate-failed", "text": txt})
    fp = _epub_fingerprint(files)
    log.append({"round": rnd, "applied": [a["action"] for a in applied if "action" in a],
                "needs_human": needs_human})
    if fp in seen:
      stopped = "fingerprint"; break
    seen.add(fp)
    if not applied:
      dry += 1
      if dry >= dry_limit: stopped = "dry"; break
    else:
      dry = 0
  cleaned = _finalize(base, work / "after" / "cleaned.epub")   # 写幂等 meta、产 cleaned.epub
  report = {"status": "complete", "stopped_by": stopped, "rounds_run": len(log),
            "output": str(cleaned), "round_log": log}
  _write_json(reports / "cleanup-loop.json", report)
  return report
```

> `_stage_input` 必须对 normalize / epub3 迁移每步跑整包 `--check text`（复用 Task 5），失败即抛错——把 Task 1 的红线纪律贯穿到一次性前置阶段。`_epubcheck` 在无 `epubcheck` 时返回 `True` 并在报告记 `epubcheck: skipped`（**显式记录跳过，不静默**）。

- [ ] **Step 4: 跑测试确认通过** — Run: `python3 scripts/test_epub_cleanup_loop.py` → PASS。

- [ ] **Step 5: Commit**

```bash
git add scripts/epub_cleanup_loop.py scripts/test_epub_cleanup_loop.py
git commit -m "feat(loop): deterministic convergence loop with anchors and fingerprint guard"
```

---

## Task 9：Handshake Planner + 报告三分类 + CLI

**目标：** 加 `HandshakePlanner`（文件握手，AI host 在工具外填 `plan.json`）、三分类报告渲染、CLI 入口。

**Files:**
- Modify: `scripts/epub_cleanup_loop.py`
- Test: `scripts/test_epub_cleanup_loop.py`

- [ ] **Step 1: 写失败测试**

```python
def test_handshake_planner_reads_external_plan(tmp_path):
    reqdir = tmp_path / "reports"; reqdir.mkdir()
    # 模拟 AI host 预先写好 round-1.plan.json
    L._write_json(reqdir / "round-1.plan.json",
                  {"round": 1, "actions": [], "suggestions": [{"kind": "x"}]})
    p = L.HandshakePlanner(reports_dir=reqdir)
    plan = p.plan(1, {"findings": []}, {}, False)
    assert plan["suggestions"] == [{"kind": "x"}]

def test_report_three_buckets(tmp_path):
    rep = {"round_log": [{"round": 1, "applied": [{"op": "add-epub-type"}],
                          "needs_human": [{"kind": "prose-typo"}]}]}
    text = L.render_report(rep)
    assert "已自动" in text and "建议" in text and "需人工" in text
```

- [ ] **Step 2: 跑测试确认失败** — Run: `python3 scripts/test_epub_cleanup_loop.py` → FAIL。

- [ ] **Step 3: 实现 HandshakePlanner、render_report、main(CLI)**

```python
SCHEMA_HINT = {  # 写进 plan-request.json 顶部，告诉 AI host 该填什么
  "actions": "list of {op, file, params, lane:'css|tag', source:'ai'} — 仅白名单 op",
  "suggestions": "list of {kind, file?, note} — 主观/排版/需人工，不会被自动执行",
}

class HandshakePlanner:
  source = "ai"
  def __init__(self, reports_dir): self.reports_dir = Path(reports_dir)
  def plan(self, round, findings, refinement, enable_structural):
    path = self.reports_dir / f"round-{round}.plan.json"
    if not path.exists():
      # 工具不主动联网：暂停，等待外部 AI host 填好后重跑该轮
      raise SystemExit(f"[handshake] 请让本地 AI host 按 {self.reports_dir}/round-{round}.plan-request.json "
                       f"填出 {path}（schema 见文件顶部），再重跑。")
    plan = json.loads(path.read_text(encoding="utf-8"))
    # 安全：丢弃任何非白名单 op（AI 误产也无害，执行器还会再挡一次）
    plan["actions"] = [a for a in plan.get("actions", [])
                       if a.get("op") in {"add-epub-type","add-xml-lang","rename-class","rewrite-tag"}]
    return plan

def render_report(rep) -> str:
  auto, sugg, human = [], [], []
  for r in rep["round_log"]:
    auto += r.get("applied", [])
    for x in r.get("needs_human", []):
      (sugg if str(x.get("kind","")).startswith("refinement") else human).append(x)
  lines = [f"# 清洗报告（{len(rep.get('round_log', []))} 轮）", "",
           f"## ✅ 已自动改（{len(auto)}）", *[f"- {a}" for a in auto], "",
           f"## 💡 建议你改（排版优化，你来定，{len(sugg)}）", *[f"- {s}" for s in sugg], "",
           f"## 👁 需人工校对 / 实测（{len(human)}）", *[f"- {h}" for h in human], "",
           "> 本次清洗已过文本红线，正文一字未改；但原书内容是否有错字/OCR 错误不在本工具职责内，",
           "> 仍需人工校对。排版效果请在目标阅读器实测。"]
  return "\n".join(lines)

def main(argv):
  import argparse
  ap = argparse.ArgumentParser(description="确定性多轮自动清洗（AI 仅产 JSON，红线兜底）")
  ap.add_argument("input"); ap.add_argument("--work-dir", default="work")
  ap.add_argument("--planner", choices=("rules", "handshake"), default="rules")
  ap.add_argument("--enable-structural", action="store_true",
                  help="开启 (乙') 结构改写白名单；默认只做加属性/语义标注")
  ap.add_argument("--max-rounds", type=int, default=6)
  ap.add_argument("--dry-limit", type=int, default=2)
  ap.add_argument("--format", choices=("text", "json"), default="text")
  a = ap.parse_args(argv)
  planner = RulesPlanner() if a.planner == "rules" else HandshakePlanner(Path(a.work_dir) / "reports")
  rep = run_loop(a.input, a.work_dir, planner, a.max_rounds, a.dry_limit, a.enable_structural)
  print(json.dumps(rep, ensure_ascii=False, indent=2) if a.format == "json" else render_report(rep))
  return 0 if rep["status"] == "complete" else 2

if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
```

- [ ] **Step 4: 跑测试确认通过** — Run: `python3 scripts/test_epub_cleanup_loop.py` → PASS。

- [ ] **Step 5: Commit**

```bash
git add scripts/epub_cleanup_loop.py scripts/test_epub_cleanup_loop.py
git commit -m "feat(loop): handshake planner, three-bucket report, CLI entry"
```

> **SPEC 回写备注：** 若 `add-epub-type` / `rewrite-tag` 引入新的语义类名或结构约定（如统一用 `<blockquote>` 承载引文、`epub:type` 取值集合），先在 `docs/final/SPEC-实现约束.md` 落条目、并在 `templates/epub-style-demo/` 加/对齐 demo 场景，再让循环产出这些动作（遵守 AGENTS.md「demo 先行、文档后补」）。

---

## Task 10：CI 纳入新测试 + 补现有 4 个漏测

**现状：** `.github/workflows/build-epub-demo.yml` 只跑 5/9 个 `test_*.py`，漏了清洗核心：`test_epub_cleanup_pipeline.py`、`test_epub3_oneclick_converter.py`、`test_epub_css_cleanup.py`、`test_epub_structure_tool.py`。

**Files:**
- Modify: `.github/workflows/build-epub-demo.yml`（顶部加 Python 3.14 setup + `Validate local fixtures and skills` step）

- [ ] **Step 0: 在 `jobs.build.steps` 的 Checkout 之后插入 Python 3.14 setup**（CI 也按 3.14 跑，和本地 `.python-version` 一致）

```yaml
      - name: Setup Python 3.14
        uses: actions/setup-python@v5
        with:
          python-version: '3.14'
```

（或改用 `astral-sh/setup-uv@v6` + 后续命令走 `uv run python ...`；二选一，保持与本地 uv/mise 一致。）

- [ ] **Step 1: 在该 step 的命令清单追加**

```yaml
          python3 scripts/test_epub_cleanup_pipeline.py
          python3 scripts/test_epub3_oneclick_converter.py
          python3 scripts/test_epub_css_cleanup.py
          python3 scripts/test_epub_structure_tool.py
          python3 scripts/test_epub_xhtml_transforms.py
          python3 scripts/test_epub_text_gate.py
          python3 scripts/test_epub_cleanup_loop.py
          python3 scripts/validate_ai_entrypoints.py
```

- [ ] **Step 2: 本地全量跑一遍确认绿**

Run:
```bash
for t in scripts/test_*.py; do python3 "$t" || echo "FAIL $t"; done
python3 scripts/validate_ai_entrypoints.py
python3 scripts/validate_skills_basic.py
```
Expected: 无 FAIL 行。

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/build-epub-demo.yml
git commit -m "ci: run full test suite incl cleanup core and new loop modules"
```

---

## Task 11：文档（清洗流程 + 模型/隐私「说明」）

**Files:**
- Modify: `docs/pipeline/cleanup-flow.md`

- [ ] **Step 1: 加「自动循环清洗」一节**，含：一条命令示例、三分类报告解读、(甲)/(乙') 开关、与现有 `epub_cleanup_pipeline.py` 的关系（loop 在其纪律之上增加多轮 + AI 计划）。

```sh
python3 scripts/epub_cleanup_loop.py /path/book.epub --work-dir work/book-a            # 默认 rules，零模型
python3 scripts/epub_cleanup_loop.py /path/book.epub --work-dir work/book-a --planner handshake --enable-structural
```

- [ ] **Step 2: 加「模型与隐私（说明）」一小节（按定稿，只作说明，不作工程分层）**

> **模型与隐私（说明）**
>
> 本工具的清洗主体是**确定性脚本**，AI 只是辅助——默认 `--planner rules` **完全不调用任何模型**，纯标准库、可离线/气隙运行，稿件不出本机。
>
> ⚠️ **风险提示**：出版社及专业制作团队的稿件常涉及机密与版权。把正文交给**云端大模型**存在泄露风险。
>
> ✅ **推荐**：确需 AI 辅助判断时，使用**本地部署的大模型**，通过 `--planner handshake` 在本机内完成「`plan-request.json` → `plan.json`」握手；工具自身从不主动联网，AI 每轮所见与所提全部落盘在 `reports/`，可审计。

- [ ] **Step 3: 校验文档链接/lint**

Run: `python3 scripts/validate_ai_entrypoints.py`（确保入口一致）；按 `.markdownlint-cli2.jsonc` 跑 markdown lint；人工点开本节相对链接。

- [ ] **Step 4: Commit**

```bash
git add docs/pipeline/cleanup-flow.md
git commit -m "docs(pipeline): document auto-loop cleanup and local-model privacy note"
```

---

## 统一验收（依 AGENTS.md「最小验证矩阵」）

```bash
git diff --check
for t in scripts/test_*.py; do python3 "$t" || echo "FAIL $t"; done
python3 scripts/validate_ai_entrypoints.py
python3 scripts/validate_skills_basic.py
# demo/validator/docs/final 若被触及（Task 9 SPEC 回写时）：
bash templates/epub-style-demo/build.sh
EPUB="$(ls -t templates/epub-style-demo/dist/*.epub | head -1)"
bash scripts/validate-epub-style-demo.sh --epub "$EPUB"
bash scripts/validate-popup-notes.sh --epub "$EPUB"
# 端到端冒烟（拿一本真实/自造脏样本）：
python3 scripts/epub_cleanup_loop.py samples/demo-books/dist/city-field-notes-before.epub --work-dir work/smoke --format json
python3 scripts/validate_text_invariance.py samples/demo-books/dist/city-field-notes-before.epub work/smoke/after/cleaned.epub --check text --allow-list "*/nav.xhtml" --allow-list "*/toc.ncx"
```

**验收标准：**

- 所有 `test_*.py` 通过；CI 跑全量（含原 4 个漏测 + 3 个新测）。
- 端到端冒烟：`cleaned.epub` 产出，`--check text` 退出码 0（正文一字未改）。
- 默认 `--planner rules` 全程不发起任何模型/网络调用。
- 报告含 ✅/💡/👁 三分类 + 校对那句话。
- 任一轮 gate 失败 → 回滚到上一锚点、记入 needs_human、循环不崩。
- `--planner handshake` 在缺 `plan.json` 时**暂停等待**而非自行联网。

---

## 自检（Self-Review，已执行）

1. **Spec 覆盖**：Python 3.14 基线=Task 0 ✓；三道 lane（①CSS=既有 css cleanup/typography 注入；②tag=Task 2-4,6,7；③suggestion=RulesPlanner 归入 suggestions + 报告）✓；正则/DOM/AI 分工=Task 2/3-4/6,9 ✓；harness 扩展（detector 注册表+`actionable_findings`+新检测器）=Task 5b ✓；每文件红线=Task 5,7 ✓；确定性循环+收敛+回滚+指纹=Task 8 ✓；可插拔 planner（rules/handshake/本地）=Task 6,9 ✓；报告三分类+校对措辞=Task 9,11 ✓；红线缺口修复=Task 1 ✓；CI 补全（含 3.14 setup）=Task 10 ✓；模型/隐私说明=Task 11 ✓；幂等=Task 3,7,5b ✓。
2. **占位符扫描**：无 TODO/TBD；代码步骤均给出可运行测试与实现（个别处明确标注「实施时对齐 harness 真实 `kind`」属配套小改，非占位）。
3. **类型一致性**：`Action`/`Plan` 结构、`apply_action` 返回 `status∈{applied,noop,reverted,rejected,skipped}`、`run_loop` 返回 `{status,stopped_by,rounds_run,output,round_log}`、planner `plan(round,findings,refinement,enable_structural)` 签名全计划统一。
4. **依赖顺序**：Task 0（版本基线）最先；Task 5b（harness 产 `actionable_findings`）必须在 Task 6（`RulesPlanner` 消费它）之前完成；`_KIND_TO_OP`（Task 6）的 key 与各 detector（Task 5b）的 `kind` 必须逐一对应。两者已分别立 Task，不再是「配套小改」。

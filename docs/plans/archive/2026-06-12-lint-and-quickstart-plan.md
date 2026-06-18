# 执行层 Phase 1：epub-lint v0 + book-starter 快速上手

> 手动 / 代理执行版。前置依赖：`codex/archive-completed-fix-plan-and-update-readme`（a86dfe3）已合入 main——本文假定 demo validator 已含 §8 守卫与 epubcheck 接线。
> 若由 AI 代理执行：建议配合 superpowers:executing-plans 按任务推进；每个任务自带验证命令。

**目标：** 把 SPEC 从「只能靠人读」升级为「机器可检」（通用 `epub_lint.py` v0，吃任意 EPUB），并补上「使用者拿到仓库十分钟出一本书」的路径（`templates/book-starter/`）。公版书语料、KP CLI 回归、a11y 等延后项见附录 B。

**两个已定决策：**

1. **lint 是通用工具，与 demo validator 互补**：validator 只校 demo fixture；lint 吃任意 EPUB、按 SPEC 条款输出 rule id + 证据。纯 Python 标准库，延续仓库约束。
2. **starter 固定预装 literary-cn preset**（六个 CSS 全部进 manifest，页面默认只 link `fonts.css` + `base.css`）；换 preset 是「复制 Styles + 不改 manifest」的零成本动作，因为三个 preset 的文件名集合一致（`base/fonts/notes/effects/media` + 各自主题层）——主题层文件名不同（`literary.css` / `academic.css` / `classical.css`），换 preset 时 manifest 中主题层一行需同步，README 写明。

---

## Task 1：`scripts/epub_lint.py` v0（10 条规则，TDD）

**文件：**
- 新建：`scripts/epub_lint.py`
- 新建：`scripts/test_epub_lint.py`

规则清单（完整分级表见附录 A）：

| 规则 | 严重度 | SPEC | 检查内容 |
| --- | --- | --- | --- |
| L-F01 | warn（>5 段升 error） | §8 | font-family 链超过 4 段 |
| L-F02 | error | §8 | 同平台别名堆叠（SimSun+宋体、Songti SC+STSongti-* 等） |
| L-F03 | error | §8 | 有 src 的 @font-face 无任何类引用 |
| L-F04 | error | §8 | 嵌入字体出现在裸元素选择器链（body/h1-6/p/code…无 class） |
| L-F05 | error | §8 | body-font-locked 页 ⇔ OPF `ibooks:specified-fonts` 配对（嵌入字体 item 存在时豁免「有 meta 无锁定页」） |
| L-O01 | error | §3 | 出现 `ibooks:*` property 但 package 未声明 ibooks prefix |
| L-N01 | error | §1 | `epub:type=noteref` 必须指向同文件内 `aside[epub:type~=footnote]` 的锚点 |
| L-M01 | error | §5.8 | 含 `<math>` 的 XHTML 其 manifest item 缺 `properties="mathml"` |
| L-T01 | error | §5.7 | 规则块写了 `text-decoration-style` 但没写基础 `text-decoration` |
| L-A01 | warn | §2 | CSS 中出现 `vh` / `vw` 单位 |
| L-X01 | warn | — | XHTML 解析失败，结构类规则对该文件跳过 |

- [ ] **1.1 先写测试** `scripts/test_epub_lint.py`：

```python
#!/usr/bin/env python3
"""Regression tests for epub_lint.py."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))
import epub_lint as L  # noqa: E402

CONTAINER = '''<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
'''

PAGE = '''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
  <head><title>t</title><link rel="stylesheet" type="text/css" href="../Styles/base.css"/></head>
  {body}
</html>
'''


def make_epub(
  path: Path,
  css: str = "body { margin: 0; }",
  body: str = "<body><p>正文。</p></body>",
  extra_meta: str = "",
  prefix: str = ' prefix="ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks-vocabulary-1.0/"',
  chapter_properties: str = "",
  extra_items: str = "",
) -> None:
  opf = f'''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bid"{prefix}>
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bid">urn:uuid:lint-fixture</dc:identifier>
    <dc:title>Lint Fixture</dc:title>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">2026-06-12T00:00:00Z</meta>
{extra_meta}  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"{chapter_properties}/>
    <item id="css" href="Styles/base.css" media-type="text/css"/>
{extra_items}  </manifest>
  <spine><itemref idref="chapter"/></spine>
</package>
'''
  nav = PAGE.format(body='<body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml">一</a></li></ol></nav></body>')
  files = {
    "META-INF/container.xml": CONTAINER,
    "OEBPS/package.opf": opf,
    "OEBPS/nav.xhtml": nav,
    "OEBPS/Text/chapter.xhtml": PAGE.format(body=body),
    "OEBPS/Styles/base.css": css,
  }
  with zipfile.ZipFile(path, "w") as zf:
    zf.writestr("mimetype", b"application/epub+zip")
    for name, data in files.items():
      zf.writestr(name, data)


def rules_of(findings: list[L.Finding]) -> set[str]:
  return {f.rule for f in findings}


def case(tmp: Path, name: str, **kwargs) -> set[str]:
  path = tmp / f"{name}.epub"
  make_epub(path, **kwargs)
  return rules_of(L.lint_epub(path))


def main() -> int:
  with TemporaryDirectory() as raw:
    tmp = Path(raw)

    clean = case(tmp, "clean")
    assert clean == set(), f"clean book must have no findings: {clean}"

    r = case(tmp, "f01", css='p { font-family: "A", "B", "C", "D", "E", serif; }')
    assert "L-F01" in r, r

    r = case(tmp, "f02", css='p { font-family: "SimSun", "宋体", serif; }')
    assert "L-F02" in r, r

    r = case(tmp, "f03", css='@font-face { font-family: "Ghost"; src: url("../Fonts/g.ttf"); }')
    assert "L-F03" in r, r

    r = case(tmp, "f04", css='@font-face { font-family: "Embed"; src: url("../Fonts/e.ttf"); }\nbody { font-family: "Embed", serif; }')
    assert "L-F04" in r, r
    assert "L-F03" not in r, r

    r = case(tmp, "f05-locked-no-meta", body='<body class="body-font-locked"><p>x</p></body>')
    assert "L-F05" in r, r

    r = case(tmp, "f05-meta-no-lock", extra_meta='    <meta property="ibooks:specified-fonts">true</meta>\n')
    assert "L-F05" in r, r

    r = case(
      tmp, "f05-meta-with-font-item",
      extra_meta='    <meta property="ibooks:specified-fonts">true</meta>\n',
      extra_items='    <item id="f1" href="Fonts/e.ttf" media-type="font/ttf"/>\n',
    )
    assert "L-F05" not in r, r

    r = case(tmp, "o01", extra_meta='    <meta property="ibooks:version">1.0</meta>\n', prefix="")
    assert "L-O01" in r, r

    r = case(tmp, "n01", body='<body><p><a epub:type="noteref" href="#fn-missing">注</a></p></body>')
    assert "L-N01" in r, r

    n01_ok = case(
      tmp, "n01-ok",
      body=(
        '<body><p><a epub:type="noteref" href="#fn1">注</a></p>'
        '<aside epub:type="footnote" id="fn1"><p>注文。</p></aside></body>'
      ),
    )
    assert "L-N01" not in n01_ok, n01_ok

    r = case(tmp, "m01", body='<body><p><math xmlns="http://www.w3.org/1998/Math/MathML"><mn>1</mn></math></p></body>')
    assert "L-M01" in r, r

    m01_ok = case(
      tmp, "m01-ok",
      body='<body><p><math xmlns="http://www.w3.org/1998/Math/MathML"><mn>1</mn></math></p></body>',
      chapter_properties=' properties="mathml"',
    )
    assert "L-M01" not in m01_ok, m01_ok

    r = case(tmp, "t01", css='.wavy { text-decoration-style: wavy; }')
    assert "L-T01" in r, r

    t01_ok = case(tmp, "t01-ok", css='.wavy { text-decoration: underline; text-decoration-style: wavy; }')
    assert "L-T01" not in t01_ok, t01_ok

    r = case(tmp, "a01", css='.poster { width: 50vw; }')
    assert "L-A01" in r, r

  print("epub lint tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
```

- [ ] **1.2 跑测试确认失败**

```sh
python3 scripts/test_epub_lint.py
```

预期：`ModuleNotFoundError: No module named 'epub_lint'`。

- [ ] **1.3 实现** `scripts/epub_lint.py`：

```python
#!/usr/bin/env python3
"""Generic SPEC-rule linter for any EPUB (stdlib only).

Rules map to docs/final/SPEC-实现约束.md clauses; see rule table in
docs/plans/2026-06-12-lint-and-quickstart-plan.md 附录 A.
Complements scripts/validate_epub_style_demo.py (demo-fixture-only checks).
"""

from __future__ import annotations

import argparse
import json
import posixpath
import re
import sys
import zipfile
from dataclasses import dataclass
from pathlib import Path
from xml.etree import ElementTree as ET

OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}
CONTAINER_NS = {"c": "urn:oasis:names:tc:opendocument:xmlns:container"}
EPUB_TYPE = "{http://www.idpf.org/2007/ops}type"
MATH_TAG = "{http://www.w3.org/1998/Math/MathML}math"

# 同平台别名组（SPEC §8）；`*` 结尾按前缀匹配。
ALIAS_GROUPS: list[list[str]] = [
  ["songti sc", "stsongti-*"],
  ["simsun", "nsimsun", "宋体"],
  ["microsoft yahei", "微软雅黑"],
  ["kaiti", "楷体"],
  ["fangsong", "仿宋"],
  ["noto serif cjk sc", "source han serif sc", "思源宋体"],
  ["noto sans cjk sc", "source han sans sc", "思源黑体"],
]

BARE_ELEMENT_RE = re.compile(r"^(body|h[1-6]|p|code|pre|blockquote|li|div|span)$")
BODY_FONT_LOCKED_RE = re.compile(
  r"<body[^>]*\bclass\s*=\s*(['\"])[^'\"]*\bbody-font-locked\b[^'\"]*\1", re.I
)
VH_VW_RE = re.compile(r"\b\d+(?:\.\d+)?(?:vh|vw)\b")


@dataclass
class Finding:
  rule: str
  severity: str  # error | warn
  location: str
  detail: str


@dataclass
class Book:
  opf_path: str
  opf_root: ET.Element
  prefix_attr: str
  manifest: list[dict]
  css_texts: dict[str, str]      # href -> comment-stripped css
  xhtml_texts: dict[str, str]    # href -> raw text
  xhtml_roots: dict[str, ET.Element | None]


def strip_css_comments(css: str) -> str:
  return re.sub(r"/\*.*?\*/", "", css, flags=re.S)


def iter_css_rules(css: str):
  """Yield (selector, body). @media 内层规则也会被捕获；@media 头部行被忽略。"""
  for match in re.finditer(r"([^{}]+)\{([^{}]*)\}", css):
    selector = match.group(1).strip()
    selector = selector.split("{")[-1].strip()
    if selector.startswith("@media") or selector.startswith("@supports"):
      continue
    yield selector, match.group(2)


def split_font_chain(value: str) -> list[str]:
  parts = re.findall(r'"[^"]*"|\'[^\']*\'|[^,]+', value)
  return [p.strip().strip("\"'").strip() for p in parts if p.strip().strip("\"'").strip()]


def font_family_decls(rule_body: str) -> list[str]:
  return [m.group(1).strip().rstrip(";").strip()
          for m in re.finditer(r"font-family\s*:\s*([^;}]+)", rule_body, re.I)]


def load_book(path: Path) -> tuple[Book | None, list[Finding]]:
  findings: list[Finding] = []
  with zipfile.ZipFile(path) as zf:
    names = set(zf.namelist())
    container = ET.fromstring(zf.read("META-INF/container.xml"))
    rootfile = container.find("c:rootfiles/c:rootfile", CONTAINER_NS)
    opf_path = rootfile.attrib["full-path"] if rootfile is not None else None
    if not opf_path or opf_path not in names:
      candidates = [n for n in names if n.endswith(".opf")]
      if not candidates:
        return None, [Finding("L-FATAL", "error", "META-INF/container.xml", "无法定位 OPF")]
      opf_path = candidates[0]
    opf_root = ET.fromstring(zf.read(opf_path))
    opf_dir = posixpath.dirname(opf_path)

    manifest: list[dict] = []
    css_texts: dict[str, str] = {}
    xhtml_texts: dict[str, str] = {}
    xhtml_roots: dict[str, ET.Element | None] = {}
    for item in opf_root.findall("opf:manifest/opf:item", OPF_NS):
      href = item.attrib.get("href", "")
      zip_path = posixpath.normpath(posixpath.join(opf_dir, href)) if href else ""
      entry = {
        "id": item.attrib.get("id", ""),
        "href": href,
        "media_type": item.attrib.get("media-type", ""),
        "properties": (item.attrib.get("properties") or "").split(),
        "zip_path": zip_path,
      }
      manifest.append(entry)
      if zip_path not in names:
        continue
      raw = zf.read(zip_path)
      if entry["media_type"] == "text/css":
        css_texts[href] = strip_css_comments(raw.decode("utf-8", errors="replace"))
      elif entry["media_type"] == "application/xhtml+xml":
        text = raw.decode("utf-8", errors="replace")
        xhtml_texts[href] = text
        try:
          xhtml_roots[href] = ET.fromstring(raw)
        except ET.ParseError as exc:
          xhtml_roots[href] = None
          findings.append(Finding("L-X01", "warn", href, f"XHTML 解析失败，结构类规则跳过：{exc}"))

  book = Book(
    opf_path=opf_path,
    opf_root=opf_root,
    prefix_attr=opf_root.attrib.get("prefix", ""),
    manifest=manifest,
    css_texts=css_texts,
    xhtml_texts=xhtml_texts,
    xhtml_roots=xhtml_roots,
  )
  return book, findings


def fontface_families(css: str) -> dict[str, bool]:
  """family(lower) -> has_src，仅统计 @font-face 块。"""
  result: dict[str, bool] = {}
  for match in re.finditer(r"@font-face\s*\{([^{}]*)\}", css, re.S | re.I):
    body = match.group(1)
    fams = font_family_decls(body)
    has_src = re.search(r"\bsrc\s*:", body, re.I) is not None
    for fam in fams:
      for name in split_font_chain(fam):
        result[name.lower()] = result.get(name.lower(), False) or has_src
  return result


def alias_matches(group: list[str], family: str) -> bool:
  fam = family.lower()
  for pattern in group:
    if pattern.endswith("*"):
      if fam.startswith(pattern[:-1]):
        return True
    elif fam == pattern:
      return True
  return False


def check_css_rules(book: Book) -> list[Finding]:
  findings: list[Finding] = []
  embedded = {fam for css in book.css_texts.values()
              for fam, has_src in fontface_families(css).items() if has_src}
  used_families: set[str] = set()

  for href, css in book.css_texts.items():
    body_no_fontface = re.sub(r"@font-face\s*\{[^{}]*\}", "", css, flags=re.S | re.I)
    for selector, rule_body in iter_css_rules(body_no_fontface):
      for decl in font_family_decls(rule_body):
        chain = split_font_chain(decl)
        used_families.update(f.lower() for f in chain)
        loc = f"{href} `{selector}`"
        if "inherit" in (f.lower() for f in chain):
          continue
        if len(chain) > 5:
          findings.append(Finding("L-F01", "error", loc, f"font-family 链 {len(chain)} 段（>5）：{decl}"))
        elif len(chain) > 4:
          findings.append(Finding("L-F01", "warn", loc, f"font-family 链 {len(chain)} 段（SPEC §8 默认 ≤4）：{decl}"))
        for group in ALIAS_GROUPS:
          hits = sorted({f for f in chain if alias_matches(group, f)})
          if len(hits) > 1:
            findings.append(Finding("L-F02", "error", loc, f"同平台别名堆叠：{hits}"))
        chain_embedded = [f for f in chain if f.lower() in embedded]
        if chain_embedded:
          for simple in selector.split(","):
            if BARE_ELEMENT_RE.fullmatch(simple.strip()):
              findings.append(Finding(
                "L-F04", "error", loc,
                f"嵌入字体 {chain_embedded} 出现在裸元素选择器 `{simple.strip()}`，须挂专用类（SPEC §8）",
              ))
      if re.search(r"text-decoration-style\s*:", rule_body, re.I) and not re.search(
        r"text-decoration(-line)?\s*:", rule_body, re.I
      ):
        findings.append(Finding("L-T01", "error", f"{href} `{selector}`",
                                "写了 text-decoration-style 但缺基础 text-decoration（SPEC §5.7）"))
    if VH_VW_RE.search(css):
      findings.append(Finding("L-A01", "warn", href, "CSS 使用了 vh/vw 单位（SPEC §2 A-lite 禁用）"))

  for fam in sorted(embedded):
    if fam not in used_families:
      findings.append(Finding("L-F03", "error", "fonts css", f"@font-face `{fam}` 有 src 但无任何规则引用（SPEC §8：删除或保持注释）"))
  return findings


def check_font_lock_pairing(book: Book) -> list[Finding]:
  locked = sorted(h for h, t in book.xhtml_texts.items() if BODY_FONT_LOCKED_RE.search(t))
  has_meta = any(
    m.attrib.get("property") == "ibooks:specified-fonts" and (m.text or "").strip().lower() == "true"
    for m in book.opf_root.findall("opf:metadata/opf:meta", OPF_NS)
  )
  has_font_items = any(
    e["media_type"].startswith("font/") or e["media_type"] in
    {"application/font-sfnt", "application/vnd.ms-opentype", "application/font-woff"}
    for e in book.manifest
  )
  findings: list[Finding] = []
  if locked and not has_meta:
    findings.append(Finding("L-F05", "error", book.opf_path,
                            f"存在 body-font-locked 页 {locked} 但 OPF 缺 ibooks:specified-fonts=true（SPEC §8）"))
  if has_meta and not locked and not has_font_items:
    findings.append(Finding("L-F05", "error", book.opf_path,
                            "OPF 有 ibooks:specified-fonts=true 但既无锁定页也无嵌入字体 item（SPEC §8 自由模式不加）"))
  return findings


def check_opf(book: Book) -> list[Finding]:
  findings: list[Finding] = []
  has_ibooks_meta = any(
    (m.attrib.get("property") or "").startswith("ibooks:")
    for m in book.opf_root.findall("opf:metadata/opf:meta", OPF_NS)
  )
  if has_ibooks_meta and "ibooks:" not in book.prefix_attr:
    findings.append(Finding("L-O01", "error", book.opf_path,
                            "存在 ibooks:* property 但 package 未声明 ibooks prefix（SPEC §3）"))
  return findings


def check_documents(book: Book) -> list[Finding]:
  findings: list[Finding] = []
  props_by_href = {e["href"]: e["properties"] for e in book.manifest}
  for href, root in book.xhtml_roots.items():
    if root is None:
      continue
    ids: dict[str, ET.Element] = {}
    asides: set[str] = set()
    noterefs: list[ET.Element] = []
    has_math = False
    for el in root.iter():
      el_id = el.attrib.get("id")
      if el_id:
        ids[el_id] = el
      types = (el.attrib.get(EPUB_TYPE) or "").split()
      if "footnote" in types and el_id:
        asides.add(el_id)
      if "noteref" in types:
        noterefs.append(el)
      if el.tag == MATH_TAG:
        has_math = True
    for ref in noterefs:
      target = ref.attrib.get("href", "")
      if not target.startswith("#"):
        findings.append(Finding("L-N01", "error", href,
                                f"noteref href `{target}` 不是同文件锚点（SPEC §1：本文件 aside 聚合）"))
        continue
      frag = target[1:]
      if frag not in ids:
        findings.append(Finding("L-N01", "error", href, f"noteref 目标 #{frag} 不存在"))
      elif frag not in asides:
        findings.append(Finding("L-N01", "error", href,
                                f"noteref 目标 #{frag} 不是 aside[epub:type~=footnote]（SPEC §1）"))
    if has_math and "mathml" not in props_by_href.get(href, []):
      findings.append(Finding("L-M01", "error", href,
                              "含 MathML 但 manifest item 缺 properties=\"mathml\"（SPEC §5.8）"))
  return findings


def lint_epub(path: Path) -> list[Finding]:
  book, findings = load_book(path)
  if book is None:
    return findings
  findings.extend(check_css_rules(book))
  findings.extend(check_font_lock_pairing(book))
  findings.extend(check_opf(book))
  findings.extend(check_documents(book))
  return findings


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Lint any EPUB against docs/final/SPEC-实现约束.md mechanical rules")
  parser.add_argument("epub", type=Path)
  parser.add_argument("--json", action="store_true", help="输出 JSON")
  parser.add_argument("--strict", action="store_true", help="warn 也按失败处理")
  args = parser.parse_args(argv)

  findings = lint_epub(args.epub)
  errors = [f for f in findings if f.severity == "error"]
  warns = [f for f in findings if f.severity == "warn"]
  if args.json:
    print(json.dumps([f.__dict__ for f in findings], ensure_ascii=False, indent=2))
  else:
    for f in findings:
      print(f"[{f.severity}] {f.rule} {f.location}: {f.detail}")
    print(f"epub-lint: {len(errors)} error(s), {len(warns)} warning(s) in {args.epub}")
  if errors or (args.strict and warns):
    return 1
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
```

- [ ] **1.4 跑测试确认通过**

```sh
python3 -m py_compile scripts/epub_lint.py && python3 scripts/test_epub_lint.py
```

预期：`epub lint tests ok`

- [ ] **1.5 在真实产物上自检**

```sh
sh templates/epub-style-demo/build.sh
python3 scripts/epub_lint.py templates/epub-style-demo/dist/<最新产物>.epub
```

预期：`0 error(s)`。若出现 error，逐条核实：是 demo 真实违规（修 demo）还是规则误报（修规则 + 在 test_epub_lint.py 加回归用例），不允许静默放宽。已知可接受输出：demo 含竖排/海报场景，可能出现少量 L-A01 warn（poster.css 若用 vh/vw），warn 不挡 exit 0；确属 A-lite 范围内的合规用法时在附录 A 给 L-A01 标注豁免计划（v1 改为只扫 A-lite 页引用的 CSS）。

- [ ] **1.6 提交**

```sh
git add scripts/epub_lint.py scripts/test_epub_lint.py
git commit -m "feat(lint): add generic SPEC-rule epub linter (v0, 10 rules)"
```

---

## Task 2：`templates/book-starter/` 最小成书骨架

**文件（全部新建）：**
- `templates/book-starter/mimetype`（内容恰好为 `application/epub+zip`，无换行）
- `templates/book-starter/META-INF/container.xml`
- `templates/book-starter/OEBPS/package.opf`
- `templates/book-starter/OEBPS/nav.xhtml`
- `templates/book-starter/OEBPS/toc.ncx`
- `templates/book-starter/OEBPS/Text/00-title.xhtml`
- `templates/book-starter/OEBPS/Text/01-chapter.xhtml`
- `templates/book-starter/OEBPS/Styles/`（从 literary-cn 复制 6 个 CSS）
- `templates/book-starter/build.sh`
- `templates/book-starter/README.md`

- [ ] **2.1 复制 preset CSS**

```sh
mkdir -p templates/book-starter/OEBPS/Styles
cp templates/style-presets/literary-cn/Styles/*.css templates/book-starter/OEBPS/Styles/
```

- [ ] **2.2 `META-INF/container.xml`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
```

- [ ] **2.3 `OEBPS/package.opf`**（自由模式：无 ibooks meta、body 不锁字体）

```xml
<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid"
         prefix="rendition: http://www.idpf.org/vocab/rendition/#">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">urn:uuid:00000000-0000-4000-8000-000000000001</dc:identifier>
    <dc:title>书名（请修改）</dc:title>
    <dc:creator>作者（请修改）</dc:creator>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">2026-06-12T00:00:00Z</meta>
    <meta property="rendition:layout">reflowable</meta>
  </metadata>
  <manifest>
    <item id="nav"        href="nav.xhtml"           media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx"        href="toc.ncx"             media-type="application/x-dtbncx+xml"/>
    <item id="title-page" href="Text/00-title.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter-1"  href="Text/01-chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="css-base"    href="Styles/base.css"     media-type="text/css"/>
    <item id="css-fonts"   href="Styles/fonts.css"    media-type="text/css"/>
    <item id="css-notes"   href="Styles/notes.css"    media-type="text/css"/>
    <item id="css-effects" href="Styles/effects.css"  media-type="text/css"/>
    <item id="css-media"   href="Styles/media.css"    media-type="text/css"/>
    <item id="css-theme"   href="Styles/literary.css" media-type="text/css"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="title-page"/>
    <itemref idref="chapter-1"/>
  </spine>
</package>
```

- [ ] **2.4 `OEBPS/nav.xhtml`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
<head>
  <title>目录</title>
  <link rel="stylesheet" type="text/css" href="Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="Styles/base.css"/>
</head>
<body>
  <nav epub:type="toc" id="toc">
    <h1>目录</h1>
    <ol>
      <li><a href="Text/00-title.xhtml">书名页</a></li>
      <li><a href="Text/01-chapter.xhtml">第一章</a></li>
    </ol>
  </nav>
</body>
</html>
```

- [ ] **2.5 `OEBPS/toc.ncx`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head>
    <meta name="dtb:uid" content="urn:uuid:00000000-0000-4000-8000-000000000001"/>
    <meta name="dtb:depth" content="1"/>
    <meta name="dtb:totalPageCount" content="0"/>
    <meta name="dtb:maxPageNumber" content="0"/>
  </head>
  <docTitle><text>书名（请修改）</text></docTitle>
  <navMap>
    <navPoint id="np-1" playOrder="1">
      <navLabel><text>书名页</text></navLabel>
      <content src="Text/00-title.xhtml"/>
    </navPoint>
    <navPoint id="np-2" playOrder="2">
      <navLabel><text>第一章</text></navLabel>
      <content src="Text/01-chapter.xhtml"/>
    </navPoint>
  </navMap>
</ncx>
```

- [ ] **2.6 `OEBPS/Text/00-title.xhtml` 与 `01-chapter.xhtml`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
<head>
  <title>书名页</title>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
</head>
<body>
  <section epub:type="titlepage">
    <h1>书名（请修改）</h1>
    <p>作者（请修改）</p>
  </section>
</body>
</html>
```

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
<head>
  <title>第一章</title>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
</head>
<body>
  <section epub:type="chapter">
    <h1>第一章</h1>
    <p>正文从这里开始。本骨架默认自由模式：body 不锁字体，读者可在阅读器中自由切换；需要锁定时按 SPEC §8 给 body 加 <code>class="body-font-locked"</code> 并在 OPF 加对应 meta。</p>
    <p>弹注、图文环绕、竖排等进阶场景请参考 <code>templates/epub-style-demo/</code> 对应页面，按需把结构复制进来并 link 对应 CSS 层（notes / effects / media）。</p>
  </section>
</body>
</html>
```

- [ ] **2.7 `build.sh`**（与 demo 同款打包配方）

```sh
#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
STAMP=$(date +%Y%m%d-%H%M%S)
OUT_DIR="$ROOT/dist"
OUT="${1:-$OUT_DIR/book-starter-$STAMP.epub}"
TMP="$OUT.tmp"

command -v zip >/dev/null 2>&1 || { echo "zip is required." >&2; exit 1; }
[ -e "$OUT" ] || [ -e "$TMP" ] && { echo "Output path already exists: $OUT" >&2; exit 1; } || true

mkdir -p "$(dirname "$OUT")"
(
  cd "$ROOT"
  zip -X -0 "$TMP" mimetype >/dev/null
  zip -X -r -9 "$TMP" META-INF OEBPS >/dev/null
)
mv "$TMP" "$OUT"
echo "$OUT"
```

- [ ] **2.8 `README.md`**

```markdown
# book-starter

最小可成书骨架：标题页 + 一章正文 + nav + NCX，预装 literary-cn preset（自由模式，
body 不锁字体）。用途是「十分钟出一本结构合规的书」，进阶场景从
`templates/epub-style-demo/` 按页复制。

## 用法

1. 复制本目录为你的书目录；改 `package.opf` 的 `dc:title` / `dc:creator` /
   `dc:identifier`（换一个新 UUID，并同步 `toc.ncx` 的 `dtb:uid`）。
2. 在 `Text/` 写章节，每加一页同步 `package.opf` manifest+spine、`nav.xhtml`、`toc.ncx`。
3. 构建并体检：

   ```sh
   sh build.sh
   python3 ../../scripts/epub_lint.py dist/<产物>.epub
   ```

## 换 preset

```sh
cp ../style-presets/academic-cn/Styles/*.css OEBPS/Styles/
```

三个 preset 的文件名只有主题层不同（`literary.css` / `academic.css` /
`classical.css`），换完后把 `package.opf` 里 `css-theme` 那一行的 href 改成对应
文件名，并删除旧主题层文件。页面默认只 link `fonts.css` + `base.css`；需要弹注 /
文字效果 / 图文混排时按 `docs/final/SPEC-实现约束.md` §7 的分层约定补 link。

## 模式说明

默认自由模式（SPEC §8）。整书锁定字体时：每个正文页 `<body class="body-font-locked">`，
`package.opf` metadata 加 `<meta property="ibooks:specified-fonts">true</meta>`
并在 `<package>` 声明 ibooks prefix。`epub_lint.py` 的 L-F05/L-O01 会替你检查配对。
```

- [ ] **2.9 dist 不入 git**：确认根 `.gitignore` 已覆盖 `templates/*/dist/`（demo 的 dist 已被忽略；若规则是写死的 `templates/epub-style-demo/dist/`，补一行 `templates/book-starter/dist/`）。

- [ ] **2.10 构建 + lint + 提交**

```sh
chmod +x templates/book-starter/build.sh
sh templates/book-starter/build.sh
python3 scripts/epub_lint.py templates/book-starter/dist/<产物>.epub
git add templates/book-starter
git commit -m "feat(templates): add book-starter minimal skeleton with literary-cn preset"
```

预期 lint：`0 error(s)`；如出现 L-A01 warn（preset 的 effects/media 层若用了 vh/vw），逐条核实是否 A-lite 合规用法，warn 不挡 exit 0。

---

## Task 3：文档接线（AGENTS 矩阵、入门页、遗留口径）

- [ ] **3.1 `AGENTS.md` 最小验证矩阵**加两行（表格末尾）：

```markdown
| 任意 EPUB 产物 | `python3 scripts/epub_lint.py <artifact>`，error 必须清零或逐条给出豁免理由 |
| demo / starter 构建产物 | epubcheck：PATH 有 `epubcheck` 或设 `EPUBCHECK_JAR` 时由 validator 自动运行；本机没有 Java 时记录跳过理由（`brew install epubcheck` 可一并安装 JDK） |
```

- [ ] **3.2 `skills/README.md` 推荐使用顺序第 5 步**改为：

```markdown
5. 修改模板或规则后，运行 demo build 和 validator；对任意 EPUB 产物运行 `python3 scripts/epub_lint.py` 做 SPEC 规则机检。
```

- [ ] **3.3 新增入门页 `docs/getting-started/09-make-a-book.md`**：

```markdown
# 九、用 book-starter 十分钟出一本书

前面章节教你**理解** EPUB；这一页教你**直接出一本**。

1. 复制骨架：`cp -r templates/book-starter ~/my-book && cd ~/my-book`
2. 改元数据：`OEBPS/package.opf` 的 `dc:title` / `dc:creator` / `dc:identifier`
   （新 UUID 同步到 `OEBPS/toc.ncx` 的 `dtb:uid`）。
3. 写正文：编辑 `OEBPS/Text/01-chapter.xhtml`；新增章节页时同步 manifest、spine、
   `nav.xhtml`、`toc.ncx` 四处。
4. 构建：`sh build.sh`，产物在 `dist/`。
5. 体检：`python3 <仓库路径>/scripts/epub_lint.py dist/<产物>.epub`，error 清零后
   再丢进 Apple Books / Kindle Previewer 实看。

骨架默认自由模式（读者可换字体）。弹注、图片环绕、竖排等场景去
`templates/epub-style-demo/` 找对应页面抄结构；规则依据查
`docs/final/EPUB 3 HTML CSS 属性速查表.md`。

> 溯源：templates/book-starter/README.md；SPEC §7 / §8。
```

并在 `docs/getting-started/README.md` 的目录列表加对应一行（紧随 08 之后，沿用现有格式）。

- [ ] **3.4 修分支遗留的旧口径**：`docs/plans/demo-scene-expansion-plan.md` 顶部状态区（`> 状态：…` 之后）加一行：

```markdown
> 2026-06-12 注：10–17 场景已落地，本文保留作推导与补测跟踪。文中「`ibooks:specified-fonts` 作为通用预防默认保留」的旧口径（§6 备注、§9 第 3 步）已被 SPEC §8 条件规则取代，以 SPEC 为准。
```

- [ ] **3.5 `run_epubcheck` 的 Java 探测加固**（`scripts/validate_epub_style_demo.py`）：macOS 自带 `/usr/bin/java` 占位程序，`shutil.which` 会误判已安装。把

```python
    java = shutil.which("java")
```

改为：

```python
    java = shutil.which("java")
    if java:
      probe = subprocess.run([java, "-version"], capture_output=True, check=False)
      if probe.returncode != 0:
        java = None
```

（后续 `elif not java:` 分支的提示随之变为准确的「java 不可用」。）

- [ ] **3.6 验证 + 提交**

```sh
python3 scripts/validate_ai_entrypoints.py
python3 scripts/validate_skills_basic.py
python3 scripts/test_validate_epub_style_demo.py
git add AGENTS.md skills/README.md docs/getting-started/ docs/plans/demo-scene-expansion-plan.md scripts/validate_epub_style_demo.py
git commit -m "docs: wire epub-lint and book-starter into AGENTS matrix and getting-started"
```

---

## Task 4：收尾（CHANGELOG、计划索引、全量验证）

- [ ] **4.1 `CHANGELOG.md` 顶部新增**

```markdown
## v0.2.3 - 2026-06-12

### Added

- `scripts/epub_lint.py`：通用 SPEC 规则机检（v0 共 10 条规则，覆盖 §1/§2/§3/§5.7/§5.8/§8），可对任意 EPUB 运行；配套回归测试。
- `templates/book-starter/`：最小成书骨架（标题页 + 一章 + nav + NCX，预装 literary-cn preset，自由模式），新增入门页 09 讲解十分钟出书路径。

### Changed

- AGENTS.md 最小验证矩阵纳入 epub-lint 与 epubcheck 运行政策；skills/README 推荐顺序同步。
- demo-scene-expansion-plan 标注旧 `ibooks:specified-fonts` 口径已被 SPEC §8 取代；demo validator 的 Java 探测对 macOS 占位 java 免疫。
```

- [ ] **4.2 `docs/plans/README.md`「当前计划」**加一行：

```markdown
- `2026-06-12-lint-and-quickstart-plan.md`：epub-lint v0 + book-starter 快速上手执行计划
```

- [ ] **4.3 全量验证（预期全绿）**

```sh
git diff --check
python3 scripts/validate_ai_entrypoints.py
python3 scripts/validate_skills_basic.py
python3 scripts/test_epub_lint.py
python3 scripts/test_validate_epub_style_demo.py
sh templates/epub-style-demo/build.sh && python3 scripts/epub_lint.py templates/epub-style-demo/dist/<最新>.epub
sh templates/book-starter/build.sh && python3 scripts/epub_lint.py templates/book-starter/dist/<最新>.epub
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<最新>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<最新>.epub
git add CHANGELOG.md docs/plans/README.md
git commit -m "docs: add v0.2.3 changelog"
```

---

## 附录 A：SPEC 规则机械化全量清单（「列规则」）

**v0 已实现（Task 1）**：见任务表（L-F01…L-A01、L-X01）。

**v1 候选（按价值排序，暂不实现）：**

| 候选 | SPEC | 检查内容 | 备注 |
| --- | --- | --- | --- |
| L-F06 | §8 | 模式 C 链中嵌入字体只出现一次，且位于链首或倒数第 2 位 | 需先识别「专用类」语义 |
| L-I01 | §5.6 | `figure.img-left/right` 的 `width` 在 25%–35% 区间、内部 img 为 `width:100%` | 区间值以 SPEC 当前文本为准 |
| L-A02 | §2 | A-lite 页（poster 类）禁 `position:absolute/fixed`、固定高度 | 需按页归属 CSS，解决 L-A01 的误报后一并做 |
| L-P01 | §1 | 弹注图标触发结构（`img.noteref-icon` + `role="doc-noteref"`）完整性 | demo validator 已有 fixture 版，泛化即可 |
| L-C01 | §5 | cover 双声明：`properties="cover-image"` 与 `<meta name="cover">` 同在 | 仅在存在封面图时检查 |
| L-S01 | §7 | 单个 CSS 文件 > 500 行报 warn（400 行提示） | 行数阈值取 §7 现文 |
| L-D01 | §1 | duokan fallback 结构（`ol.duokan-footnote-content` 配套）一致性 | 依赖多看实测结论稳定后 |
| L-L01 | §5.9 | 中英混排页缺 `xml:lang` 切换 | 误报率需评估 |
| L-K01 | — | NCX `dtb:uid` 与 `dc:identifier` 一致 | epubcheck 也查，但本地无 Java 时兜底 |

**不可机械化（留给 skills 的人审清单）**：正文节奏与留白、字号层级审美、图片选型与裁切、章首装饰取舍、文白对照的语义切分。lint 输出可作为这些 skills 的输入证据（v1：在 `epub-layout-auditor` 工作流第一步加「先跑 `epub_lint.py`，error 清单作为分派依据」）。

## 附录 B：新场景分析（延后项与触发条件）

| 方向 | 价值 | 前置 | 建议触发 |
| --- | --- | --- | --- |
| Apple Books 实测 (a)(b)(c) | 整个 §8 字体规则的证据闭环 | 一台 iOS/macOS 设备 + 一次 no-meta 对照构建 | 你有空的第一个实测时段；方案已写在 reader-matrix `07` 待测条目 |
| 字体子集化工具链 | 中文嵌字最大实操痛点（20MB→子集） | 标准库字符集扫描脚本 + 文档化 `pyftsubset`（fonttools 仅作文档推荐，不进依赖） | 第一本真书需要嵌字时 |
| Kindle Previewer CLI 回归 | 把两条 Kindle warn 复测脚本化 | 本机安装 Kindle Previewer（当前未装） | 装好 KP 的那台机器上做；先 `--help` 落参数再写脚本 |
| EPUB a11y 1.1 章节 + Ace 检查 | 国际出版硬趋势；SPEC 新大章 | Ace（Node CLI）可选集成，政策同 epubcheck | lint v0 稳定、实测闭环跑顺之后 |
| 诗词 / 剧本 / 对话体场景 | 扩 demo 覆盖面 | 各需 1 个 demo 页 + reader-matrix case | 有真书需求驱动时再做，避免无证据规则 |
| 暗色模式安全 | 小而实用（颜色 token 不写死黑白） | 1 条 SPEC 约定 + lint 规则（检查 `color:#000`/`background:#fff` 硬编码） | 可并入 lint v1 |
| 公版书语料 + golden 回归 | 全流程的真实检验 | 选书（你在找） | 选定即启动，沿用 work/<book>/ 约定 |

## 附录 C：两个问题的结论

**「我的例子能否让使用者快速使用？」——此前不能，Task 2/3 之后能。** 现状是：`epub-style-demo` 是 24 页的*测试书*（场景验证用，不适合当起点）；3 个 preset 是*纯 CSS*（没有书的骨架）；getting-started 01 教的是*手工理解性搭建*。也就是说「想直接排一本书的人」拿到仓库后没有可复制的起点——book-starter 正是补这个洞：骨架 + preset 预装 + lint 体检 + 入门页 09 的十分钟路径。

**「还需要细分 skill 吗？」——不需要，方向反过来。** 15 个 skill 已按问题域一一对应（字体、CSS 分层、图文、竖排、弹注、Kindle、OPF/nav…），且 `epub-layout-auditor` 已承担总入口/分派角色，再细分只会增加代理的路由成本。真正缺的是两样：① skill 引用机器证据——lint 落地后在 layout-auditor 等 skill 的工作流第一步接入 `epub_lint.py` 输出（v1，见附录 A 末段）；② skill 级场景测试——`validate_skills_basic.py` 只查结构契约，给每个 skill 配「输入 fixture → 期望判断」的用例属于后续增强，建议在第一本公版书走完全流程后、用真实案例反哺着做，避免空造 fixture。

# 2026-06-03 审计修复计划（remediation plan）

> 本文件是 `docs/plans/` 下的维护说明，不直接驱动行为（见 AGENTS.md §文档落点）。
> 它把 2026-06-03 仓库审计的发现整理成「人工照着改」的清单：每条给出精确位置、改前/改后、为什么、验证命令、AGENTS.md 合规注意、风险。
> 改任何 `docs/final/` 硬规则、跑任何验证，都按 AGENTS.md「最小验证矩阵」和「实测闭环」执行。

## 0. 范围与前提

- **不在本计划内**：`references/epubs/…(Z-Library).epub` 的合规/仓库膨胀问题。按你的决定，这一项**不改**，本文件完全不涉及。
- **CHANGELOG 策略**：你是「感觉有问题才故意没发新版本」，这是对的。本计划把 CHANGELOG 排成**收尾步骤**——所有真实 bug 修完、测试补齐、确认稳定后，再一次性发 `v0.2.0`（见 §7）。在那之前不要声称已发布。
- **本计划起草时已做对抗式复核**，并纠正了起草草稿里的 5 处错误，已直接写进下面对应条目：
  1. `bug-001` 草稿把源码里的 NBSP（U+00A0）误显示成普通空格——下面改法用 ` ` 转义，等价且不会误删。
  2. `bug-002` 草稿用 `.iter()` 建父映射且引用了不存在的函数，会让测试失败——下面改用 ElementTree 标准的 `manifest_elem.remove()`。
  3. `defensive-06` 草稿用了单数键 `plan["suggestion"]`，但代码用复数 `suggestions`——下面已修正。
  4. `defensive-07` 草稿的 before 块行号错乱，且重复 `import sys`（`sys` 已在顶部导入）——下面已修正。
  5. `skills/contract-2` 草稿有 Python 语法 bug：`line.split("|"[1]...`（括号位置错，且 `"|"[1]` 会 IndexError）——下面已修正为 `line.split("|")[1]`。
- **测试脚手架（§3）注意**：起草的测试文件里有些 CLI flag / 输出键是**猜的**，已确认与真实脚本不符的我已在条目里标出并给出正确写法。**粘贴测试前务必先看一眼对应脚本的 `argparse` 与真实 JSON 输出键**，脚手架只是起点，真正要保证的是「必测用例清单」。

执行顺序建议：**§1 关键 bug → §2 防御性 → §3 补测试（回归用例锁住 §1）→ §4 CI → §5 skills → §6 文档 → §7 发版**。

---

## 1. 关键代码 bug（已复核成立，优先）

### 1.1 红线 gate 缺 Unicode NFC 归一化 〔high〕

- **文件**：`scripts/validate_text_invariance.py`（`normalize_text`，约第 60–61 行）
- **现状**：`normalize_text` 只做空白和 NBSP 归一，没做 Unicode 形式归一；而同仓 `scripts/epub_xhtml_transforms.py:116` 的 `text_content_equal()` 用了 `unicodedata.normalize("NFC", ...)`。两个文本相等性 gate 口径分叉，导致 NFC/NFD 等价文本被红线误判为「正文被改」（已复现：café 的合成型 vs 分解型 → 退出码 1）。
- **改法**（纯加法：顶部加 import + 外层包一层 NFC；` ` 转义等价于原文里的 NBSP，可整行替换）：

  ```python
  # 文件顶部 import 区加：
  import unicodedata

  # normalize_text 改为：
  def normalize_text(value: str) -> str:
    return unicodedata.normalize("NFC", re.sub(r"\s+", " ", value.replace(" ", " ")).strip())
  ```

- **为什么**：红线 gate 是安全关键路径，必须和 `epub_xhtml_transforms` 同口径，避免无故卡流程的假阳性。
- **验证**：补完 §3.6 的 NFC 回归用例后跑 `python3 scripts/test_validate_text_invariance.py`；`git diff --check`。
- **合规**：AGENTS.md §最小验证矩阵（改 Python 脚本跑对应 `test_*.py`）。
- **风险**：低。`unicodedata` 是标准库；现有测试用 ASCII，不受影响。

### 1.2 EPUB3 迁移：多 nav 只加不减，产出不合规 〔high〕

- **文件**：`scripts/epub3_migration_harness.py`（约第 286–297 行）
- **现状**：`if len(nav_items(new_root)) != 1:` 时无条件**新增**一个 `properties="nav"` item，从不删旧的。输入若已有 >1 个 nav（如 NCX 被误标）→ 输出有更多 nav，违反 EPUB3「恰好一个 nav」，而这正是 `test_epub_cleanup_harnesses.py` 在强制检查的。
- **改法**（用 ElementTree 标准 `.remove()`；`nav_items()` 返回的就是 `<manifest>` 的直接子 `item`，可安全 remove）：

  ```python
      if len(nav_items(new_root)) != 1:
        # 先移除所有已有的 nav item，确保最终唯一
        manifest_elem = manifest(new_root)
        for old_nav in nav_items(new_root):
          manifest_elem.remove(old_nav)
        # 再生成唯一 nav
        generated_nav_href = choose_nav_href(opf_dir, names)
        entries = ncx_entries(zin, new_root, opf_dir) or spine_entries(new_root)
        if not entries:
          raise Epub3Error("cannot generate nav.xhtml: no NCX navPoint or spine entries")
        generated_nav_bytes = build_nav_xhtml(package_title(new_root), language(new_root), entries)
        ET.SubElement(manifest_elem, tag(OPF_URI, "item"), {
          "id": unique_item_id(new_root, "nav"),
          "href": generated_nav_href,
          "media-type": "application/xhtml+xml",
          "properties": "nav",
        })
  ```

- **同时**：更新 `plan_epub3()`（约第 231–269 行）里对应 action / warnings 文案，从「生成并添加 nav」改为告知「将替换已有 nav 为唯一项」。
- **顺手检查**：`scripts/epub3_oneclick_converter.py` 的 `ensure_nav`（约第 608 行）有没有同类「只加不减」问题；若有，按同样思路修。
- **验证**：补完 §3.2 的多 nav 回归用例后跑 `python3 scripts/test_epub_cleanup_harnesses.py` 和新的 `python3 scripts/test_epub3_migration_harness.py`。改 OPF/nav 后可 `xmllint --noout` 抽查产物（本机无 xmllint 则记跳过理由）。
- **合规**：AGENTS.md §已有 EPUB 固定流程第 5 步、§最小验证矩阵。
- **风险**：中。「先删后建」幂等；确认 `nav_items()` 能识别全部 `properties="nav"` 项（含 `properties="... nav ..."` 多值写法）。

### 1.3 弹注 validator 的 zip-slip 〔high〕

- **文件**：`scripts/validate_popup_notes.py`（约第 245–246 行 `zf.extractall(extracted)`）
- **现状**：直接 `extractall` 到磁盘，不校验成员名。这是全仓**唯一**真正落盘解压且无防护的点（其它脚本要么用 `validate_archive_path`，要么只 `zf.read()` 进内存，无风险）。恶意 ZIP 成员名含 `../` 或绝对路径可越界写文件（`TemporaryDirectory` 只是部分缓解）。
- **改法**（复用 `epub_structure_tool.py:129-135` 的思路，逐个成员校验后单独 extract）：

  ```python
  def safe_extractall(zf: zipfile.ZipFile, target: Path) -> None:
    """逐成员校验路径后再解压，拒绝 zip-slip（同 epub_structure_tool.validate_archive_path 思路）。"""
    for member in zf.namelist():
      if member.startswith("/") or ".." in member.split("/"):
        raise ValueError(f"zip-slip attempt: {member}")
      zf.extract(member, target)

  # 调用处改为：
        with zipfile.ZipFile(args.epub) as zf:
          safe_extractall(zf, extracted)
  ```

  > 注：用 `".." in member.split("/")` 比 `".." in member` 更精确（避免误伤文件名里合法含 `..` 的情况，如 `a..b.png`）。
- **验证**：补完 §3.6 的 zip-slip 回归用例后跑 `python3 scripts/test_validate_popup_notes.py`。
- **合规**：AGENTS.md §最小验证矩阵；安全问题应强制。
- **风险**：低。提取语义不变，只加成员名校验。

### 1.4 detector registry 静默吞异常 〔high〕

- **文件**：`scripts/epub_ai_harness.py`（主点第 74–83 行 `collect_actionable_findings`；另有第 224、256 行同样的裸 `except Exception`）
- **现状**：`for d in DETECTORS: try: found.extend(d.fn(model)) except Exception: pass` —— 任何 detector 崩溃被无声吞掉，报告「看起来完整」实则漏检。
- **改法（主点）**：

  ```python
  # 顶部若无 import sys 则加（先确认）：
  import sys

  def collect_actionable_findings(model: EpubModel | None) -> list[dict]:
    if model is None:
      return []
    found: list[dict] = []
    detector_errors: list[dict[str, str]] = []
    for d in DETECTORS:
      try:
        found.extend(d.fn(model))
      except Exception as exc:
        detector_errors.append({"detector": d.kind, "error": str(exc)})
    for err in detector_errors:
      print(f"WARNING: detector {err['detector']} failed: {err['error']}", file=sys.stderr)
    return found
  ```

- **次要点（第 224、256 行）**：先 `grep -nE 'except Exception' scripts/epub_ai_harness.py` 定位这两处的真实上下文，再按**同样模式**（保留「单点失败不致命」，但把异常打到 stderr，不要 `pass` 静默）。这两处我没有逐字读取，请按真实代码套用，别照抄行号。
- **验证**：补完 §3.6 的 detector 异常回归用例后跑 `python3 scripts/test_epub_ai_harness.py`。
- **合规**：AGENTS.md §最小验证矩阵；改动只加日志，返回值/主流程不变。
- **风险**：低。注意调用方不要把 stderr 输出当失败条件。

### 1.5 文件名反混淆漏掉 `srcset` 〔high / 工作量 L〕

- **文件**：`scripts/epub_structure_tool.py`（`URI_ATTRIBUTE_RE` 第 48–52 行 + `rewrite_markup_references`）
- **现状**：`URI_ATTRIBUTE_RE` 只匹配 `href|src|poster|data|xlink:href|textref`，不含 `srcset`。所以反混淆重写引用时 `<img srcset="a.jpg 1x, b.jpg 2x">` / `<source srcset>` 的引用不会被改 → 响应式图片断链。AGENTS.md §3 承诺反混淆「并同步更新引用」。
- **改法**：新增 `rewrite_srcset_urls()` 专门处理逗号分隔候选，并在 `rewrite_markup_references` 里先调用它：

  ```python
  def rewrite_srcset_urls(text, old_document, new_document, path_map, files, report):
    """重写 srcset 里逗号分隔的 'url 描述符' 候选。"""
    srcset_re = re.compile(
      r"(?P<prefix>\bsrcset\s*=\s*)(?P<quote>[\"'])(?P<uri>.*?)(?P=quote)",
      flags=re.IGNORECASE | re.DOTALL,
    )
    def replace_srcset(match):
      candidates = []
      for candidate in match.group("uri").split(","):
        parts = candidate.strip().split()
        if not parts:
          continue
        url = rewrite_uri(parts[0], old_document, new_document, path_map, files, report)
        descriptor = " ".join(parts[1:])
        candidates.append(f"{url} {descriptor}".strip())
      return f"{match.group('prefix')}{match.group('quote')}{', '.join(candidates)}{match.group('quote')}"
    return srcset_re.sub(replace_srcset, text)

  # 在 rewrite_markup_references 里，URI_ATTRIBUTE_RE.sub(...) 之前加：
    text = rewrite_srcset_urls(text, old_document, new_document, path_map, files, report)
  ```

  > 正则安全性：`URI_ATTRIBUTE_RE` 的 `\bsrc\s*=` **不会**误匹配 `srcset=`（`src` 后面是 `set`，不是 `=`），所以两者不冲突；先处理 srcset 只是为了清晰。务必核对 `rewrite_uri` 的真实签名与上面一致。
- **验证**：补完 §3.6 的 srcset 回归用例后跑 `python3 scripts/test_epub_structure_tool.py`。建议在 `templates/epub-style-demo/` 加一个带 `srcset` 的最小场景再 `build.sh` 实测。
- **合规**：AGENTS.md §最小验证矩阵；若动 demo 还要 `validate-epub-style-demo.sh --epub`。
- **风险**：中。`srcset` 变体多（无描述符、含 query/fragment 的 URL）；务必靠 §3.6 用例覆盖。

---

## 2. 防御性 / 健壮性补丁（medium–low）

### 2.1 `ncx_entries()` 中途改 `files`，parse 失败不可逆 〔medium〕

- **文件**：`scripts/epub3_oneclick_converter.py`（约第 473–487 行）
- **改法**：把 `files[ncx_zip] = sanitized.encode("utf-8")` 从第 483 行**挪到 `parse_xml` 成功、entries 算完之后**再赋值：

  ```python
    sanitized = sanitize_ncx_text(files[ncx_zip], report)
    ncx_root = parse_xml(sanitized, ncx_zip)
    base = posixpath.dirname(ncx_href)
    points = ncx_root.findall("ncx:navMap/ncx:navPoint", NCX_NS)
    entries = [e for e in (parse_nav_points(p, base) for p in points) if e is not None]
    files[ncx_zip] = sanitized.encode("utf-8")  # 成功后才提交修改
    return entries
  ```

- **验证**：`python3 -m py_compile scripts/epub3_oneclick_converter.py && python3 scripts/test_epub3_oneclick_converter.py`
- **风险**：低，纯逻辑重排。

### 2.2 `write_epub()` 非原子写 〔medium〕

- **文件**：`scripts/epub3_oneclick_converter.py`（约第 1028–1038 行）
- **改法**：写 `.tmp` 再 `replace()`（与 `_repack` 同模式），异常时清理 `.tmp`：

  ```python
    tmp = output_path.with_suffix(output_path.suffix + ".tmp")
    try:
      with zipfile.ZipFile(tmp, "w") as zf:
        for name in ordered:
          info = zipfile.ZipInfo(name, FIXED_ZIP_TIME)
          info.compress_type = zipfile.ZIP_STORED if name == "mimetype" else zipfile.ZIP_DEFLATED
          zf.writestr(info, files[name])
      tmp.replace(output_path)
    except Exception:
      if tmp.exists():
        tmp.unlink()
      raise
  ```

- **验证**：`python3 -m py_compile scripts/epub3_oneclick_converter.py && python3 scripts/test_epub3_oneclick_converter.py`
- **风险**：低，模式已在 `_repack` 验证。

### 2.3 pipeline gate 失败后不清理 output，重跑被卡 〔medium〕

- **文件**：`scripts/epub_cleanup_pipeline.py`（约第 249、279–280 行）
- **改法**：把 `ensure_empty_target` 之后的转换 + preflight-after 段用 `try/except PipelineError` 包住，失败时 `shutil.rmtree(output_path)` 再 raise（确认顶部已 `import shutil`）：

  ```python
      ensure_empty_target(output_path, "cleanup output")
      try:
        # ... convert-epub3 与 preflight-after 原有代码 ...
        if preflight_after and preflight_after.get("preflight_status") == "fail":
          raise PipelineError("preflight-after: converter produced blocking findings")
      except PipelineError:
        if output_path.exists():
          shutil.rmtree(output_path)
        raise
  ```

- **验证**：`python3 -m py_compile scripts/epub_cleanup_pipeline.py && python3 scripts/test_epub_cleanup_pipeline.py`
- **风险**：低。确认 `output_path` 是目录（用 `rmtree`）还是文件（用 `unlink`）——按真实类型选。

### 2.4 cleanup-loop 成功轮次后 `files` 不从磁盘重读 〔medium，原报 critical 已降级〕

- **文件**：`scripts/epub_cleanup_loop.py`（约第 540–554 行，gate 判断的 `else` 分支）
- **改法**：gate 通过且本轮有应用时，从磁盘重读，保持内存与磁盘一致：

  ```python
          if not (ok_text and ok_check):
              # ... 原有失败回滚逻辑不动 ...
              applied = []
          else:
              if applied:
                  files = _read_xhtml_members(current_base)
  ```

- **验证**：`python3 scripts/test_epub_cleanup_loop.py`；**务必确认不破坏收敛/振荡逻辑**（`dry_limit`、`seen` 指纹集合仍正常）。建议手造一个需多轮收敛的输入跑长循环。
- **风险**：中。这条是设计一致性改进，不是已确证的功能 bug；若担心影响收敛可暂缓，靠 §3.2 类长轮次测试兜底后再合。

### 2.5 `epubcheck_ok` 字段 bool/str 漂移 〔medium〕

- **文件**：`scripts/epub_cleanup_loop.py`（约第 554–560 行 `round_log`）
- **改法**：拆成恒 bool + 恒 str：

  ```python
          round_log = {
              "round": rnd,
              "applied": [a["action"] for a in applied if "action" in a],
              "needs_human": needs_human,
              "text_ok": ok_text,
              "epubcheck_ok": ok_check,
              "epubcheck_message": check_txt[:200] if not ok_check else "",
          }
  ```

- **验证**：先 `grep -rn "epubcheck_ok" scripts/ docs/` 看有没有下游/测试读这个键的旧类型，再 `python3 scripts/test_epub_cleanup_loop.py`。
- **风险**：中。属字段 schema 变更——若 README/docs 文档了这个键，同步更新。

### 2.6 HandshakePlanner 静默过滤白名单外 op 〔low〕

- **文件**：`scripts/epub_cleanup_loop.py`（约第 171–173 行）
- **改法**（用复数键 `suggestions`，与既有 RulesPlanner 一致；`allowed_ops` 沿用上文已定义的，不要重复定义）：

  ```python
          filtered_actions = []
          filtered_out = []
          for a in plan.get("actions", []):
              if a.get("op") in allowed_ops:
                  filtered_actions.append(a)
              else:
                  filtered_out.append({"op": a.get("op"), "file": a.get("file")})
          if filtered_out:
              plan.setdefault("suggestions", []).append({
                  "kind": "filtered-disallowed-op",
                  "note": f"Filtered out {len(filtered_out)} disallowed ops: {filtered_out}",
              })
          plan["actions"] = filtered_actions
  ```

- **验证**：`python3 scripts/test_epub_cleanup_loop.py`（含 `test_handshake_planner_strips_non_whitelisted_ops`）。
- **风险**：低。`setdefault` 保证缺 `suggestions` 键时也安全。

### 2.7 `--path-map` 不完整时静默按未改名处理 〔low〕

- **文件**：`scripts/validate_text_invariance.py`（`load_path_map`，约第 392–412 行）
- **改法**：循环结束、`return path_map` 之前加空映射警告（`sys` 已在顶部导入，**不要**再 `import sys`）：

  ```python
    if not path_map:
      print(f"warning: --path-map {path} is empty or contains no valid mappings; assuming no file renames", file=sys.stderr)
    return path_map
  ```

- **验证**：`python3 -m py_compile scripts/validate_text_invariance.py && python3 scripts/test_validate_text_invariance.py`
- **风险**：低，只加警告。

---

## 3. 补 / 强测试

> ⚠️ **下面的测试文件是脚手架，不是即粘即跑**。起草时部分 CLI flag / 输出键是猜的；已知错误我已纠正并标注，但**粘贴前仍要对照真实脚本的 `argparse` 与真实 JSON 输出键**。真正的交付物是「必测用例清单」——代码可以重写，用例必须覆盖。本仓测试风格：纯标准库、模块级 `ROOT/SCRIPT` 常量、`subprocess` 跑脚本、`TemporaryDirectory` 造 EPUB、断言退出码与 JSON 结构（不是 pytest）。

### 3.1 `scripts/test_epub_preflight_harness.py`（新建，high）

必测用例：① 合法 EPUB3 → `preflight_status="pass"`、rc=0；② 缺 `container.xml`/损坏包 → `fail`、rc≠0；③ 有 `encryption.xml`（DRM 标记）→ `fail`；④ 缺 manifest/spine → `fail`；⑤ `findings` 是 list、`findings_by_level` 是 dict 且值为 int。
真实 CLI：`python3 scripts/epub_preflight_harness.py <epub> --format json`（位置参数 + `--format`）。`preflight_status` 键已确认存在；其余键先实跑一个 EPUB 看真实输出再写断言。

### 3.2 `scripts/test_epub3_migration_harness.py`（新建，high）★锁住 §1.2

- **真实 CLI（已确认）**：位置参数 `input` + 可选 `--write-output PATH` + `--format`。**没有 `--mode plan/write`**。「plan 模式」= 不带 `--write-output`；「write 模式」= 带 `--write-output <out.epub>`。
- 必测用例：
  1. **多 nav 回归**（直接对应 §1.2 的 bug）：造一个 manifest 里有 2–3 个 `properties="nav"` item 的输入 → `--write-output` 迁移后，解开输出 OPF 断言 `properties="nav"` 的 item **恰好 1 个**。这条用例必须在 §1.2 改之前是红、改之后是绿。
  2. plan 模式（不带 `--write-output`）输出含 `actions`/`warnings`。
  3. EPUB3→EPUB3 幂等（version 已是 3.0、nav 已唯一）→ 无多余改动。
  4. 生成的 `nav.xhtml` 命名空间/编码正确。
- 真实输出键先实跑确认（plan 的 JSON 顶层键、action 结构）。

### 3.3 `scripts/test_epub_refinement_harness.py`（新建，high）

- **真实 CLI（已确认）**：位置参数 `path` + `--format`。**没有 `--mode`**。
- 必测用例：scan 计数准确性（字体链长度、risky 图像如 webp、ruby/legacy note 标记）、`build_recommendations` 各条件分支与推荐 id。
- ⚠️ 草稿假设的 `recommended_skills`/`findings_by_level` 键可能不对——参考 `test_epub_cleanup_harnesses.py:121-132` 的真实断言方式（它检查一组 recommendation `id`，如 `{preflight, epub3-migration, popup-notes, typography-fonts, images, redline-and-diff}`），按真实输出结构写。

### 3.4 `scripts/test_validate_epub_style_demo.py`（新建，medium）

- **真实 CLI（已确认）**：只有 `--epub <built.epub>`（可选）。**没有 `--oebps`**；不传 `--epub` 时它校验仓库内的 demo 源树。
- 因此「用自造 OEBPS 目录跑子进程」的草稿思路行不通。两条可行路线：
  - (A) 直接 `import` 该模块、单测各 `check_*()` 函数（推荐，能覆盖畸形 XHTML/缺 nav/缺命名空间分支）；
  - (B) 用 `build.sh` 产出的 `--epub` 跑端到端 smoke（覆盖面窄）。
- 优先级低于 §3.1–3.3，可后置。

### 3.5 `scripts/test_validate_skills_basic.py` / `test_validate_ai_entrypoints.py`（新建，medium）

- 正常路径：对当前仓库跑 → rc=0、有 ok 信息。
- 异常路径：临时造「缺 SKILL.md / 缺 openai.yaml / 坏 CONTRACTS token」的 skill 目录，断言被检出。
- ⚠️ 这两个 validator 用模块级 `SKILLS`/`ROOT` 常量指向真实仓库路径，子进程方式难注入假目录。要测异常路径，更现实的是 `import` 函数并传入临时路径（需脚本支持参数化，或用 `monkeypatch`/改常量）。如果改造成本高，先只做正常路径 smoke，异常路径标为 TODO。

### 3.6 回归用例增量（medium）★锁住 §1.1/1.3/1.4/1.5

分别加到现有测试文件，每条对应一个 §1 的 bug：
- **NFC**（`test_validate_text_invariance.py`）：before 用合成型、after 用分解型的同一段文字（如 `café`）→ 红线应 **PASS**（改 §1.1 前会 FAIL）。
- **zip-slip**（`test_validate_popup_notes.py`）：造一个成员名含 `../../etc/passwd` 的 EPUB → 应被 `safe_extractall` 拒绝（rc≠0）。
- **detector 异常**（`test_epub_ai_harness.py`）：造会让某 detector 抛异常的畸形输入 → 主流程不崩、`actionable_findings` 仍返回、stderr 有 WARNING。
- **srcset**（`test_epub_structure_tool.py`）：造带 `<img srcset="a.jpg 1x, b.jpg 2x">` 的 EPUB，跑反混淆 → 断言 srcset 里两个 URL 都被重写。

**§3 验证**：每个新测试先 `python3 -m py_compile`，再单独 `python3 scripts/test_xxx.py`；最后跑全量（见 §7）。新测试落地后记得加进 CI（§4）与可选的 pre-commit（§4.6）。

---

## 4. CI 与工具链

### 4.1 接入 markdownlint-cli2 〔medium〕

- **文件**：`.github/workflows/build-epub-demo.yml`（在 "Validate local fixtures and skills" step 前加一个 step）

  ```yaml
        - name: Lint Markdown
          run: |
            npm install -g markdownlint-cli2
            markdownlint-cli2 --config .markdownlint-cli2.jsonc \
              'README.md' 'CONTRIBUTING.md' 'CHANGELOG.md' 'AGENTS.md' \
              'docs/final/**/*.md' 'docs/getting-started/**/*.md' \
              'docs/guides/**/*.md' 'docs/pipeline/**/*.md' 'skills/**/*.md'
  ```

- ⚠️ **重要范围限制**（草稿用了 `**/*.md` 全仓，会出问题）：**不要**对 `docs/plans/`、`docs/experiments/`、`docs/source/` 跑——这些是历史快照，AGENTS.md「不要为统一措辞重写历史」，让它们过 lint 反而逼你改历史。只 lint 上面这些**活跃维护**的目录。
- **验证**：本地 `markdownlint-cli2 --config .markdownlint-cli2.jsonc <上面这些路径>`，应无 ERROR。
- **风险**：低。先本地跑一遍，把活跃文档里真实的违规修掉再开 CI。

### 4.2 Python lint（ruff，opt-in，建议先不强制）〔low〕

- 全部脚本都 `from __future__ import annotations`，无 3.13+ 专属运行期语法；引入 ruff 与本仓「只用标准库」的克制风格略有张力。
- **建议**：在 workflow 里以**注释形式**预留，需要时再开：

  ```yaml
        # - name: Lint Python (opt-in)
        #   run: |
        #     pip install ruff
        #     ruff check scripts/ --select E,W,F --ignore E501
  ```

- **合规**：AGENTS.md「不要引入新依赖」——保持 opt-in。

### 4.3 对 samples/demo-books 跑 EPUBCheck 〔high〕

- **文件**：`.github/workflows/build-epub-demo.yml`（现有 EPUBCheck step 之后加一个）
- 先 `bash samples/demo-books/build.sh` 产出 6 个 EPUB，再对其中 5 个合法样本跑 EPUBCheck；`redline-trap-after-text-changed` 是**故意失败反例**，跳过（或断言其文本红线失败而非 EPUBCheck）：

  ```yaml
        - name: Build & EPUBCheck demo-books samples
          run: |
            bash samples/demo-books/build.sh
            for epub in samples/demo-books/dist/*.epub; do
              case "$(basename "$epub")" in
                redline-trap-after-text-changed.epub)
                  echo "[expected-fail sample] skip EPUBCheck: $epub" ;;
                *)
                  java -jar "$EPUBCHECK_JAR" "$epub" --mode epub --profile default || { echo "FAIL: $epub" >&2; exit 1; } ;;
              esac
            done
  ```

- ⚠️ 先确认 `samples/demo-books/build.sh` 真的会产出这 6 个文件名（`redline-trap` 的 before/after 命名以脚本为准）。
- **合规**：AGENTS.md §规范来源优先级（demo-books 的 `.epub` 不入 git，CI 里临时构建符合约定）。

### 4.4 Python 版本策略 〔low〕

- 现状：CI 钉 3.14，README/`.python-version` 也是 3.14。脚本未用 3.13+ 专属语法。
- **建议（最省事）**：保持 CI 钉 3.14，并在 `requirements.txt` 顶部或新建轻量 `pyproject.toml` 声明 `requires-python = ">=3.14"`，让意图显式。
- ⚠️ 草稿那个「精确字符串比对 `!= '3.14'`」的版本检查 step 太脆（`3.14.1` 会误报），**不建议**加；`actions/setup-python` 已经保证版本。
- 若将来想支持更低版本，再加 `strategy.matrix`（3.10–3.14）并补 CI；现在不必。

### 4.5 链接检查 / 覆盖率（opt-in，低优先）〔low〕

- 链接检查（lychee / markdown-link-check）有用，但会报 `docs/plans/` 历史死链——若开，必须排除历史目录（同 §4.1）。先以注释预留。
- 覆盖率：本仓不是 pytest 架构（独立 `test_*.py`），`--cov` 不直接适用，优先级低，先不做。

### 4.6 pre-commit hook 与 CI 的差距 〔medium〕

- 现状：hook 只跑 3 个快校验，CI 跑 13+。这是「本地快反馈 vs push 前完整网关」的有意权衡。
- **建议（选 A，省事且不伤开发节奏）**：在 `CONTRIBUTING.md` 或 `docs/pipeline/README.md` 写一段说明 hook 与 CI 的覆盖差异，并提示「改 `scripts/` 后请本地手跑相关 `test_*.py` 再 push」。
- **可选（选 B）**：让 hook 在检测到 `scripts/` 改动时多跑几个关键测试——但会拖慢 commit，可能诱使 `--no-verify`，先别上。
- 别忘了：§3 新增的测试要同时加进 CI 的 "Validate local fixtures and skills" 列表。

---

## 5. Skills 契约校验增强

### 5.1 CONTRACTS 从 5/15 扩到覆盖全部 15 个 skill 〔high〕

- **文件**：`scripts/validate_skills_basic.py`（`CONTRACTS` 字典）
- 给当前未覆盖的 10 个 skill（`epub-layout-auditor`、`epub-source-intake`、`epub-structure-normalizer`、`epub-image-layout-optimizer`、`epub-vertical-ruby-optimizer`、`epub-kindle-compatibility-checker`、`epub-alite-converter`、`epub-popup-footnote-converter`、`epub-legacy-footnote-fallback`、`epub-style-demo-maintainer`）各加 ≥1 条「(文件路径, 该文件里真实存在的 token)」约束。
- ⚠️ **关键纪律**：每个新增 token 必须是目标文件里**真实存在的字符串**，否则跑 validator 会对现有 skill 误报。起草的扩展条目是个**候选起点**，但 token 未全部逐字核验。落地办法：先把候选 CONTRACTS 贴进去，跑 `python3 scripts/validate_skills_basic.py`，它会逐条告诉你哪个 token 不存在；按报错把 token 改成该文件里确实有的字符串，直到 15 个 skill 全绿。
- **验证**：`python3 scripts/validate_skills_basic.py` 通过（15 skills ok）。
- **合规**：只扩 CONTRACTS 数据，不动 SKILL.md frontmatter 字段名。

### 5.2 新增「两份 skill 表格一致」校验 〔medium〕

- **文件**：`scripts/validate_skills_basic.py`（加 `validate_skill_tables()` 并在 `main()` 调用）
- 校验所有 skill 目录都同时出现在 `skills/README.md` 与 `docs/getting-started/04-skills.md` 的表格、且计数一致。
- ⚠️ **草稿有语法 bug，已修正**：解析行要用 `line.split("|")[1].strip().strip("\`")`（草稿写成 `line.split("|"[1]...`，括号位置错且会 IndexError）。参考实现：

  ```python
  def validate_skill_tables(skill_folders: list[Path]) -> list[str]:
    errors: list[str] = []
    expected = {p.name for p in skill_folders}

    def parse_table(path: Path) -> set[str]:
      names: set[str] = set()
      for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("| `epub-"):
          cell = line.split("|")[1].strip().strip("`")
          if cell.startswith("epub-"):
            names.add(cell)
      return names

    readme = parse_table(ROOT / "skills" / "README.md")
    gs = parse_table(ROOT / "docs" / "getting-started" / "04-skills.md")
    if readme != expected:
      errors.append(f"skills/README.md 表格与目录不一致: 缺 {sorted(expected - readme)} 多 {sorted(readme - expected)}")
    if gs != expected:
      errors.append(f"04-skills.md 表格与目录不一致: 缺 {sorted(expected - gs)} 多 {sorted(gs - expected)}")
    return errors
  ```

  并在 `main()` 里 `errors.extend(validate_skill_tables(folders))`。
- ⚠️ 上面 `| \`epub-` 的前缀匹配依赖两个表格的真实格式——先 `grep -n '| \`epub-' skills/README.md docs/getting-started/04-skills.md` 确认行首格式一致，否则调匹配规则。
- **验证**：`python3 scripts/validate_skills_basic.py`。
- **建议**：5.1/5.2 落地后顺手做 §3.5 的 `test_validate_skills_basic.py`。

### 5.3 把 SKILL.md 三段式结构写进 AGENTS.md（文档化，可选）〔low〕

- 当前 15 个 SKILL.md 都遵循「使用场景 → 目标/工作流/禁止事项 → 验证」三段式，但 AGENTS.md 没记。可在 §关键约束 加一条把它记为已批准规范（含「已批准的替代结构」豁免），便于后人对齐。
- ⚠️ 这是改 AGENTS.md（唯一权威），属规范层文档补充——确认措辞后再加，别引入会和现有 skill 冲突的硬性结构校验。
- **验证**：`git diff --check`；`python3 scripts/validate_ai_entrypoints.py`。

---

## 6. 文档治理

### 6.1 README 措辞：warn ≠ 已验证 〔medium〕

- **文件**：`README.md`（第 5 行那段）
- reader-matrix 当前 23 warn / 7 pass，README「确认后的阅读器兼容性结论会回写」读起来像已验证。改成显式说明：

  > …通过阅读器实测确认的兼容性结论写入 `reader-matrix.yaml`，标 `pass` 或 `fail`；**尚未完成复测的条目保留为 `warn`，warn 不等于已验证**…

- **验证**：人工核对 README 第 1–25 行与 CONTRIBUTING.md 中关于 reader-matrix 的措辞一致；`grep -c 'status: warn' docs/final/reader-matrix.yaml` 确认现状数字。
- **合规**：只改 README 入门层措辞，不动 `docs/final/` 硬规则。

### 6.2 SPEC 加版本日期表头 〔low〕

- **文件**：`docs/final/SPEC-实现约束.md`（标题下加一行）

  ```
  > 版本：2026-06-03
  ```

  与《EPUB 3 终极实践手册》（`版本：2026-05-23`）格式对齐。属元数据补充，非规则变更。
- **验证**：`head -5 docs/final/SPEC-实现约束.md`。

### 6.3 速查表 .md → .html 派生流程写进 CONTRIBUTING 〔medium〕

- **文件**：`CONTRIBUTING.md`
- `docs/final/EPUB 3 HTML CSS 属性速查表.html` 由同名 `.md` 手工派生（HTML 顶部注释已写明以 .md 为权威），但无同步约束。在 CONTRIBUTING 加一条：改 `.md` 后必须重新生成/同步 `.html`，并把这条纳入「改 docs/final 的检查清单」。可选在 pre-commit 里提示「.md 改了但 .html 没改」。
- **验证**：人工——改 .md 后用任意 md→html 工具核对 .html 主体一致。

### 6.4 README 给 fixtures-tiny 补例外说明 〔low〕

- **文件**：`README.md`（第 20 行「每条规则都必须有 demo 复现」那条）
- 补一句：`samples/fixtures-tiny/` 是快速迭代的极简槽位（当前自动化测试在 `test_validate_text_invariance.py` 内即时构造等价 EPUB），最终产品仍按 `templates/` + `samples/demo-books/` 的完整 demo 标准验证。

### 6.5 清个人绝对路径 〔low〕

- **做法**：`grep -rn '/Users/<name>' docs/ scripts/ *.md` 找出**所有**出现处（审计在 `docs/plans/2026-05-28-readme-tools-followup-review.md:81` 和疑似 `docs/plans/2026-05-28-remove-epub-diff.md` 都见过），逐个泛化成相对路径或 `$PROJECT_ROOT`。
- ⚠️ 这些在历史计划里——只动「脚本片段里的路径」（卫生改进），不重写历史叙述。
- **验证**：改完 `grep -rn '/Users/<name>' docs/ scripts/ *.md` 应为空（本计划文件本身含示例路径可豁免，或一并泛化）。

### 6.6 历史计划的废弃链接（可选，最轻处理）〔low〕

- `docs/plans/handbook-expansion-plan.md` 等正文里还有指向已删除 `diff-tool.md` / `tools/epub-diff/` 的**非删除线**活链接。
- 按「不重写历史」原则，**最轻处理**：在文件已有的作废 banner 后补一行「§4 及其子章已于 2026-05-28 整体作废，仅作历史快照」，或给这些链接加删除线 `~~...~~`。不强求，纯可读性改进。

---

## 7. 收尾：发布 v0.2.0

前面都改完、确认稳定后再做：

1. **全量验证**（等价 CI）：

   ```sh
   git diff --check
   for t in test_epub_ai_harness test_validate_text_invariance test_epub_cleanup_harnesses \
            test_epub_cleanup_pipeline test_epub3_oneclick_converter test_epub_css_cleanup \
            test_epub_structure_tool test_epub_anthology_refinement test_validate_popup_notes \
            test_build_demo_epubs test_epub_xhtml_transforms test_epub_text_gate test_epub_cleanup_loop \
            test_epub_preflight_harness test_epub3_migration_harness test_epub_refinement_harness; do
     python3 "scripts/$t.py" || echo "FAIL $t"
   done
   python3 scripts/validate_skills_basic.py
   python3 scripts/validate_ai_entrypoints.py
   bash templates/epub-style-demo/build.sh
   EPUB="$(ls -t templates/epub-style-demo/dist/*.epub | head -1)"
   bash scripts/validate-epub-style-demo.sh --epub "$EPUB"
   bash scripts/validate-popup-notes.sh --epub "$EPUB"
   ```

2. **更新 CHANGELOG.md**：在 `v0.1.0` 之上新增 `v0.2.0`，列本轮真实修的项（NFC 红线、多 nav、zip-slip、detector 日志、srcset、防御性补丁、新测试、CI markdownlint/EPUBCheck、skills 校验、文档治理）。在全部验证通过前，标题写 `## v0.2.0 - [待发布]`；通过后再改成日期并打 tag。
3. **同步检查**：若动了 `docs/final/`（本计划里只有 SPEC 版本表头、速查表派生流程），按 AGENTS.md §关键约束核对手册/速查表/SPEC 三件套与相关 skills 不分叉。

---

## 附：本轮发现严重度一览（不含 Z-Library）

| # | 项 | 严重度 | 类型 | 复核状态 |
|---|---|---|---|---|
| 1.1 | 红线 gate 缺 NFC 归一化 | high | 代码 | 已复现 |
| 1.2 | EPUB3 迁移多 nav 只加不减 | high | 代码 | 已复核（草稿改法已纠正） |
| 1.3 | 弹注 validator zip-slip | high | 代码/安全 | 已复核 |
| 1.4 | detector 静默吞异常 | high | 代码 | 已复核 |
| 1.5 | 反混淆漏 srcset | high | 代码 | 已确认 |
| 2.1–2.7 | 防御性（ncx/原子写/清理/重读/类型/过滤/path-map） | medium–low | 代码 | 2.6 由 critical 降级；06/07 草稿已纠正 |
| 3.1–3.6 | 补测试（preflight/migration/refinement/validator/回归） | high–medium | 测试 | 脚手架需按真实 CLI 校准 |
| 4.1–4.6 | CI（markdownlint/EPUBCheck/版本/hook 等） | high–low | CI | markdownlint 需限定活跃目录 |
| 5.1–5.3 | skills 契约校验增强 | high–low | 代码/文档 | contract-2 语法 bug 已纠正 |
| 6.1–6.6 | 文档治理（README/SPEC/CONTRIBUTING/路径/历史链接） | medium–low | 文档 | — |

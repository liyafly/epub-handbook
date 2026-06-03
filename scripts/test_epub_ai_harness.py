#!/usr/bin/env python3
"""Smoke-test epub_ai_harness output against the local demo tree."""

from __future__ import annotations

import json
import subprocess
import sys
import zipfile
from tempfile import TemporaryDirectory
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "scripts" / "epub_ai_harness.py"
DEMO = ROOT / "templates" / "epub-style-demo"


def run_harness(path: Path, *extra: str) -> tuple[int, dict[str, object]]:
  result = subprocess.run(
    [sys.executable, str(HARNESS), *extra, str(path), "--format", "json"],
    cwd=ROOT,
    check=False,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
  )
  if not result.stdout:
    print(result.stderr, file=sys.stderr)
    return result.returncode, {}
  return result.returncode, json.loads(result.stdout)


def validate_demo_route() -> int:
  returncode, data = run_harness(DEMO)
  if returncode:
    print(json.dumps(data, ensure_ascii=False, indent=2), file=sys.stderr)
    return returncode
  expected_skills = {
    "$epub-popup-footnote-converter",
    "$epub-vertical-ruby-optimizer",
    "$epub-css-layering-optimizer",
    "$epub-image-layout-optimizer",
    "$epub-package-nav-auditor",
  }
  skills = set(data.get("recommended_skills", []))
  missing = sorted(expected_skills - skills)
  if missing:
    print(f"ERROR: harness missing expected skills: {', '.join(missing)}", file=sys.stderr)
    return 1

  commands = data.get("suggested_commands", [])
  required_commands = [
    "sh templates/epub-style-demo/build.sh",
    "scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub",
    "scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub",
    "scripts/validate_skills_basic.py",
  ]
  missing_commands = [command for command in required_commands if command not in commands]
  if missing_commands:
    print(f"ERROR: harness missing suggested commands: {missing_commands}", file=sys.stderr)
    return 1

  if data.get("input_kind") != "epub-source-tree":
    print(f"ERROR: expected epub-source-tree input_kind, got {data.get('input_kind')}", file=sys.stderr)
    return 1

  returncode, cleanup_data = run_harness(DEMO, "--mode", "cleanup")
  if returncode:
    print(json.dumps(cleanup_data, ensure_ascii=False, indent=2), file=sys.stderr)
    return returncode
  if cleanup_data.get("mode") != "cleanup":
    print(f"ERROR: expected cleanup mode, got {cleanup_data.get('mode')}", file=sys.stderr)
    return 1
  cleanup_skills = cleanup_data.get("recommended_skills", [])
  if not cleanup_skills or cleanup_skills[0] != "$epub-layout-auditor":
    print(f"ERROR: cleanup mode should start with layout auditor: {cleanup_skills}", file=sys.stderr)
    return 1

  print("epub_ai_harness smoke test ok")
  return 0


def write_css_url_fixture(root: Path, css: str) -> None:
  (root / "OEBPS" / "Text").mkdir(parents=True)
  (root / "OEBPS" / "Styles").mkdir(parents=True)
  (root / "OEBPS" / "Styles" / "main.css").write_text(css, encoding="utf-8")
  (root / "OEBPS" / "Text" / "chapter.xhtml").write_text(
      '''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">
<head><title>Test</title><link rel="stylesheet" type="text/css" href="../Styles/main.css"/></head>
<body><p>Test</p></body></html>
''',
    encoding="utf-8",
  )
  (root / "OEBPS" / "content.opf").write_text(
      '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:test</dc:identifier>
    <dc:title>Test</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="nav" href="Text/chapter.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
  </manifest>
  <spine>
    <itemref idref="chapter"/>
  </spine>
</package>
''',
    encoding="utf-8",
  )


def validate_missing_css_url_detection() -> int:
  with TemporaryDirectory() as tmp:
    root = Path(tmp)
    write_css_url_fixture(root, 'body { background-image: url("../Images/Missing.png"); }\n')
    returncode, data = run_harness(root)
    cleanup_returncode, cleanup_data = run_harness(root, "--mode", "cleanup")
  if returncode == 0:
    print("ERROR: missing CSS url should make harness fail", file=sys.stderr)
    return 1
  findings = data.get("findings", [])
  if not any(item.get("message") == "CSS url() target missing" for item in findings):
    print(f"ERROR: missing CSS url finding not present: {findings}", file=sys.stderr)
    return 1
  if cleanup_returncode == 0:
    print("ERROR: missing CSS url should make cleanup harness fail", file=sys.stderr)
    return 1
  levels = cleanup_data.get("findings_by_level", {})
  if levels.get("error", 0) < 1 or levels.get("warn", 0) < 1:
    print(f"ERROR: cleanup findings_by_level missing expected counts: {levels}", file=sys.stderr)
    return 1
  cleanup_skills = cleanup_data.get("recommended_skills", [])
  error_skills = [
    "$epub-package-nav-auditor",
    "$epub-css-layering-optimizer",
  ]
  warn_skills = [
    "$epub-kindle-compatibility-checker",
    "$epub-image-layout-optimizer",
  ]
  if not cleanup_skills or cleanup_skills[0] != "$epub-layout-auditor":
    print(f"ERROR: cleanup mode should keep layout auditor first: {cleanup_skills}", file=sys.stderr)
    return 1
  try:
    last_error = max(cleanup_skills.index(skill) for skill in error_skills)
    first_warn = min(cleanup_skills.index(skill) for skill in warn_skills)
  except ValueError:
    print(f"ERROR: cleanup ordering test missing expected skills: {cleanup_skills}", file=sys.stderr)
    return 1
  if last_error >= first_warn:
    print(f"ERROR: cleanup skills not sorted by finding level: {cleanup_skills}", file=sys.stderr)
    return 1
  print("epub_ai_harness CSS url smoke test ok")
  return 0


def validate_missing_css_font_url_warning() -> int:
  with TemporaryDirectory() as tmp:
    root = Path(tmp)
    write_css_url_fixture(
      root,
      '@font-face { font-family: "Missing"; src: url("../Fonts/Missing.otf"), local("Missing"); }\n',
    )
    returncode, data = run_harness(root, "--mode", "cleanup")
  if returncode:
    print(json.dumps(data, ensure_ascii=False, indent=2), file=sys.stderr)
    return returncode
  findings = data.get("findings", [])
  font_fallbacks = [item for item in findings if item.get("kind") == "missing-css-font-fallback"]
  if not font_fallbacks or any(item.get("level") != "warn" for item in font_fallbacks):
    print(f"ERROR: missing CSS font url should be a warning: {findings}", file=sys.stderr)
    return 1
  print("epub_ai_harness CSS font fallback smoke test ok")
  return 0


def validate_obfuscated_filename_detection() -> int:
  with TemporaryDirectory() as tmp:
    root = Path(tmp)
    (root / "OEBPS" / "Text").mkdir(parents=True)
    (root / "OEBPS" / "Text" / "?mix.xhtml").write_text(
      '''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">
<head><title>Test</title></head><body><p>Test</p></body></html>
''',
      encoding="utf-8",
    )
    (root / "OEBPS" / "content.opf").write_text(
      '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:test</dc:identifier>
    <dc:title>Test</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="chapter" href="Text/%3Fmix.xhtml" media-type="application/xhtml+xml" properties="nav"/>
  </manifest>
  <spine><itemref idref="chapter"/></spine>
</package>
''',
      encoding="utf-8",
    )
    returncode, data = run_harness(root, "--mode", "cleanup")
  if returncode:
    print(json.dumps(data, ensure_ascii=False, indent=2), file=sys.stderr)
    return returncode
  findings = data.get("findings", [])
  if not any(item.get("kind") == "filename-obfuscation" for item in findings):
    print(f"ERROR: filename obfuscation finding not present: {findings}", file=sys.stderr)
    return 1
  if "$epub-structure-normalizer" not in data.get("recommended_skills", []):
    print(f"ERROR: structure normalizer not recommended: {data.get('recommended_skills')}", file=sys.stderr)
    return 1
  print("epub_ai_harness filename obfuscation smoke test ok")
  return 0


def _make_min_epub(path: str, body: str, *, with_html_lang: bool = False,
                   with_math: bool = False, calibre_class: str | None = None) -> str:
  """Create a minimal EPUB 3 zip and return its path."""
  lang_attr = ' xml:lang="zh-CN"' if with_html_lang else ""
  body_class = f' class="{calibre_class}"' if calibre_class else ""
  maybe_math = (
      '<p><math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math></p>'
      if with_math else ""
  )
  container = (
      '<?xml version="1.0" encoding="UTF-8"?>'
      '<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">'
      '  <rootfiles>'
      '    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>'
      '  </rootfiles>'
      '</container>'
  )
  opf = (
      '<?xml version="1.0" encoding="UTF-8"?>'
      '<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">'
      '  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">'
      '    <dc:identifier id="book-id">urn:uuid:test-harness-detector-01</dc:identifier>'
      '    <dc:title>Detector Fixture</dc:title>'
      '    <dc:language>zh-CN</dc:language>'
      '  </metadata>'
      '  <manifest>'
      '    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>'
      '    <item id="ch1" href="chapter.xhtml" media-type="application/xhtml+xml"/>'
      '  </manifest>'
      '  <spine><itemref idref="ch1"/></spine>'
      '</package>'
  )
  nav = (
      '<?xml version="1.0" encoding="UTF-8"?>'
      '<html xmlns="http://www.w3.org/1999/xhtml" '
      'xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">'
      '<head><title>Nav</title></head>'
      '<body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Ch</a></li></ol></nav></body>'
      '</html>'
  )
  chapter = (
      '<?xml version="1.0" encoding="UTF-8"?>'
      '<html xmlns="http://www.w3.org/1999/xhtml"'
      + lang_attr +
      '><head><title>Chapter</title></head>'
      '<body><p' + body_class + '>正文内容测试</p>' + maybe_math + '</body></html>'
  )
  with zipfile.ZipFile(path, "w") as zf:
    zf.writestr("mimetype", "application/epub+zip", compress_type=zipfile.ZIP_STORED)
    zf.writestr("META-INF/container.xml", container)
    zf.writestr("OEBPS/content.opf", opf)
    zf.writestr("OEBPS/nav.xhtml", nav)
    zf.writestr("OEBPS/chapter.xhtml", chapter)
  return path


def validate_actionable_findings_present() -> int:
  with TemporaryDirectory() as tmp:
    root = Path(tmp)
    path = _make_min_epub(str(root / "missing-lang.epub"), "正文",
                          with_html_lang=False)
    returncode, data = run_harness(Path(path))
  if returncode:
    print(f"ERROR: harness should succeed on valid epub: {data}", file=sys.stderr)
    return 1
  # Backward compat keys
  assert "findings" in data, "missing 'findings' key"
  assert "findings_by_level" in data, "missing 'findings_by_level' key"
  assert "recommended_skills" in data, "missing 'recommended_skills' key"
  af = data.get("actionable_findings")
  if af is None:
    print("ERROR: actionable_findings key missing", file=sys.stderr)
    return 1
  if not any(f["kind"] == "missing-html-lang" and f.get("auto_fixable")
             for f in af):
    print(f"ERROR: missing-html-lang not found in actionable_findings: {af}", file=sys.stderr)
    return 1
  one = next(f for f in af if f["kind"] == "missing-html-lang")
  if one["lane"] != "tag" or one["confidence"] not in ("high", "medium", "low"):
    print(f"ERROR: bad lane/confidence in finding: {one}", file=sys.stderr)
    return 1
  if "file" not in one or "params" not in one:
    print(f"ERROR: missing file/params in finding: {one}", file=sys.stderr)
    return 1
  print("epub_ai_harness actionable_findings smoke test ok")
  return 0


def validate_detector_idempotent() -> int:
  with TemporaryDirectory() as tmp:
    root = Path(tmp)
    path = _make_min_epub(str(root / "has-lang.epub"), "正文",
                          with_html_lang=True)
    returncode, data = run_harness(Path(path))
  if returncode:
    print(f"ERROR: harness should succeed: {data}", file=sys.stderr)
    return 1
  af = data.get("actionable_findings", [])
  if any(f["kind"] == "missing-html-lang" for f in af):
    print(f"ERROR: should not report missing-html-lang when already set: {af}", file=sys.stderr)
    return 1
  print("epub_ai_harness detector idempotency ok")
  return 0


def validate_registry_lists_detectors() -> int:
  sys.path.insert(0, str(Path(__file__).resolve().parent))
  import epub_ai_harness as H  # noqa: E402
  names = {d.kind for d in H.DETECTORS}
  required = {"missing-html-lang", "obfuscated-class", "empty-paragraph",
              "missing-manifest-properties"}
  missing = required - names
  if missing:
    print(f"ERROR: DETECTORS missing kinds: {missing}", file=sys.stderr)
    return 1
  print("epub_ai_harness detector registry ok")
  return 0


def validate_missing_manifest_properties_detector() -> int:
  with TemporaryDirectory() as tmp:
    root = Path(tmp)
    path = _make_min_epub(str(root / "mathml-missing-prop.epub"), "正文",
                          with_html_lang=True, with_math=True)
    returncode, data = run_harness(Path(path))
  # harness returns non-zero when error findings exist (expected for missing mathml props)
  # actionable_findings should be present regardless of exit code
  af = data.get("actionable_findings", [])
  if not any(f["kind"] == "missing-manifest-properties"
             and f["params"].get("properties") == "mathml" for f in af):
    print(f"ERROR: missing mathml manifest properties not detected: {af}", file=sys.stderr)
    return 1
  print("epub_ai_harness missing-manifest-properties detector ok")
  return 0


def validate_obfuscated_class_detector() -> int:
  with TemporaryDirectory() as tmp:
    root = Path(tmp)
    path = _make_min_epub(str(root / "obfuscated.epub"), "正文",
                          with_html_lang=True, calibre_class="calibre12")
    returncode, data = run_harness(Path(path))
  if returncode:
    print(f"ERROR: harness should succeed: {data}", file=sys.stderr)
    return 1
  af = data.get("actionable_findings", [])
  if not any(f["kind"] == "obfuscated-class" for f in af):
    print(f"ERROR: obfuscated-class not detected: {af}", file=sys.stderr)
    return 1
  one = next(f for f in af if f["kind"] == "obfuscated-class")
  if one.get("auto_fixable"):
    print(f"ERROR: obfuscated-class should be auto_fixable=False: {one}", file=sys.stderr)
    return 1
  print("epub_ai_harness obfuscated-class detector ok")
  return 0


def validate_detector_exception_warning() -> int:
  sys.path.insert(0, str(Path(__file__).resolve().parent))
  import epub_ai_harness as H  # noqa: E402
  from io import StringIO
  from contextlib import redirect_stderr

  def broken_detector(_model):
    raise RuntimeError("boom")

  model = H.EpubModel(xhtml_docs={}, opf_root=None, opf_path="", css_docs={}, book_language=None)
  original = list(H.DETECTORS)
  H.DETECTORS[:] = [H.Detector("broken-test", "tag", broken_detector)]
  stderr = StringIO()
  try:
    with redirect_stderr(stderr):
      findings = H.collect_actionable_findings(model)
  finally:
    H.DETECTORS[:] = original
  warning = stderr.getvalue()
  if findings or "WARNING: detector broken-test failed: boom" not in warning:
    print(f"ERROR: detector exception was not reported correctly: findings={findings}, stderr={warning!r}", file=sys.stderr)
    return 1
  print("epub_ai_harness detector exception warning ok")
  return 0


def main() -> int:
  for check in (
    validate_demo_route,
    validate_missing_css_url_detection,
    validate_missing_css_font_url_warning,
    validate_obfuscated_filename_detection,
    validate_actionable_findings_present,
    validate_detector_idempotent,
    validate_registry_lists_detectors,
    validate_missing_manifest_properties_detector,
    validate_obfuscated_class_detector,
    validate_detector_exception_warning,
  ):
    result = check()
    if result:
      return result
  return 0


if __name__ == "__main__":
  raise SystemExit(main())

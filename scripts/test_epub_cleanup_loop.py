#!/usr/bin/env python3
"""Regression tests for deterministic package cleanup rounds."""
from __future__ import annotations
import json
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET
sys.path.insert(0, str(Path(__file__).resolve().parent))
import epub_cleanup_loop as loop

OPF = '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
 <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:test</dc:identifier><dc:title>Loop fixture</dc:title><dc:language>zh-CN</dc:language></metadata>
 <manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/></manifest>
 <spine><itemref idref="chapter"/></spine>
</package>'''
CONTAINER='''<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>'''
NAV='''<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>目录</title></head><body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml">正文</a></li></ol></nav></body></html>'''
CHAPTER='''<html xmlns="http://www.w3.org/1999/xhtml"><head><title>正文</title></head><body><p>正文保持不变。</p><svg xmlns="http://www.w3.org/2000/svg"><circle r="1"/></svg><math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math></body></html>'''

def make_fixture(path: Path, chapter: str = CHAPTER, opf: str = OPF) -> Path:
  with zipfile.ZipFile(path, 'w') as zf:
    zf.writestr('mimetype', 'application/epub+zip', compress_type=zipfile.ZIP_STORED)
    zf.writestr('META-INF/container.xml', CONTAINER)
    zf.writestr('OEBPS/package.opf', opf)
    zf.writestr('OEBPS/nav.xhtml', NAV)
    zf.writestr('OEBPS/Text/chapter.xhtml', chapter)
  return path

def opf_root(path: Path) -> ET.Element:
  with zipfile.ZipFile(path) as zf: return ET.fromstring(zf.read('OEBPS/package.opf'))

def test_loop_applies_package_properties_and_audit_meta() -> None:
  with TemporaryDirectory() as tmp:
    root=Path(tmp); source=make_fixture(root/'dirty.epub')
    report=loop.run_loop(source, root/'work')
    assert report['status']=='complete' and report['stopped_by']=='dry'
    assert report['rounds_run']==3, report
    applied=[a for r in report['round_log'] for a in r['applied']]
    assert len(applied)==1 and set(applied[0]['added'])=={'svg','mathml'}, applied
    package=opf_root(Path(report['output'])); ns={'o':'http://www.idpf.org/2007/opf'}
    chapter=package.find("o:manifest/o:item[@id='chapter']", ns); assert chapter is not None
    assert set(chapter.attrib['properties'].split())=={'svg','mathml'}
    meta=package.find("o:metadata/o:meta[@property='epub-handbook:cleanup-rounds']", ns); assert meta is not None and meta.text=='3'
    assert 'epub-handbook: https://github.com/liyafly/epub-handbook#' in package.attrib['prefix']
    assert all('epubcheck' in r for r in report['round_log'])

def test_preflight_exposes_actionable_finding() -> None:
  with TemporaryDirectory() as tmp:
    source=make_fixture(Path(tmp)/'dirty.epub'); audit=loop._audit(source)
    actionable=audit['actionable_findings']
    assert {item['kind'] for item in actionable}=={'missing-manifest-properties'}
    assert all(item['auto_fixable'] and item['confidence']=='high' for item in actionable)

def test_stage_migrates_epub2_before_rounds() -> None:
  with TemporaryDirectory() as tmp:
    root=Path(tmp); source=make_fixture(root/'legacy.epub', opf=OPF.replace('version="3.0"', 'version="2.0"'))
    report=loop.run_loop(source, root/'work')
    assert any(stage['name']=='epub3-migration' and stage['status']=='pass' for stage in report['staging'])

def test_epubcheck_failure_rolls_back_package_action() -> None:
  with TemporaryDirectory() as tmp:
    root=Path(tmp); source=make_fixture(root/'dirty.epub')
    original=loop._epubcheck
    loop._epubcheck=lambda path: {'status':'fail', 'returncode':1, 'output':'fixture failure'}
    try: report=loop.run_loop(source, root/'work')
    finally: loop._epubcheck=original
    package=opf_root(Path(report['output'])); ns={'o':'http://www.idpf.org/2007/opf'}
    chapter=package.find("o:manifest/o:item[@id='chapter']", ns); assert chapter is not None
    assert 'properties' not in chapter.attrib
    assert any(item.get('reason')=='round-gate-failed' for row in report['round_log'] for item in row['needs_human'])

def test_handshake_planner_rejects_unsupported_ops() -> None:
  with TemporaryDirectory() as tmp:
    plan_dir=Path(tmp); (plan_dir/'round-1.plan.json').write_text('{"actions":[{"op":"rewrite-prose"}]}', encoding='utf-8')
    try: loop.HandshakePlanner(plan_dir).plan(1, {'findings': []})
    except loop.CleanupError as exc: assert 'unsupported op' in str(exc)
    else: raise AssertionError('handshake planner accepted unsupported op')

def main() -> int:
  test_loop_applies_package_properties_and_audit_meta(); test_preflight_exposes_actionable_finding(); test_stage_migrates_epub2_before_rounds(); test_epubcheck_failure_rolls_back_package_action(); test_handshake_planner_rejects_unsupported_ops()
  print('epub_cleanup_loop tests ok'); return 0
if __name__=='__main__': raise SystemExit(main())

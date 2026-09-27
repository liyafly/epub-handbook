#!/usr/bin/env bash
set -euo pipefail

binary="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
expected_version="$2"
expected_goos="$3"
expected_goarch="$4"
python_cmd="${PYTHON:-python3}"
if ! command -v "$python_cmd" >/dev/null 2>&1; then
  python_cmd=python
fi

"$python_cmd" - "$binary" "$expected_version" "$expected_goos" "$expected_goarch" <<'PY'
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from zipfile import ZIP_DEFLATED, ZIP_STORED, ZipFile

binary, expected_version, expected_goos, expected_goarch = sys.argv[1:]
env = os.environ.copy()
env.pop("EPUB_HANDBOOK_ROOT", None)


def run_json(args, cwd):
    result = subprocess.run(
        [binary, *args],
        cwd=cwd,
        env=env,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )
    if result.returncode != 0:
        raise SystemExit(
            f"command failed ({result.returncode}): {args!r}\n"
            f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as error:
        raise SystemExit(f"command returned invalid JSON: {args!r}: {error}\n{result.stdout}")


def run_text(args, cwd):
    result = subprocess.run(
        [binary, *args],
        cwd=cwd,
        env=env,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )
    if result.returncode != 0:
        raise SystemExit(
            f"command failed ({result.returncode}): {args!r}\n"
            f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return result.stdout


with tempfile.TemporaryDirectory(prefix="epub-release-smoke-") as scratch:
    empty = Path(scratch) / "empty"
    empty.mkdir()

    help_text = run_text(["help"], empty)
    assert "epub help" in help_text, help_text

    info = run_json(["version", "--json"], empty)
    assert info["version"] == expected_version, info
    assert info["commit"] not in ("", "unknown"), info
    assert info["builtAt"] not in ("", "unknown"), info
    assert (info["goos"], info["goarch"]) == (expected_goos, expected_goarch), info

    capabilities = run_json(["capabilities", "--json"], empty)
    assert len(capabilities) == 23, len(capabilities)
    assert any(item["id"] == "epub.typography.optimize" for item in capabilities)
    assert any(item["id"] == "epub.font.subset" for item in capabilities)

    scene_catalog = run_json(
        ["run", "epub.style.demo.maintain", "--json", "catalog=true", "query=poetry"],
        empty,
    )
    scene_facts = scene_catalog["facts"]
    assert scene_facts["epub.style.demo.maintain.resourceSource"] == "embedded", scene_facts
    scenes = scene_facts["epub.style.demo.maintain.scenes"]
    assert any(scene["path"] == "OEBPS/Text/29-poetry.xhtml" for scene in scenes), scenes

    empty_catalog = run_json(
        ["run", "epub.style.demo.maintain", "--json", "catalog=true", "query=no-such-style-scene"],
        empty,
    )
    assert empty_catalog["facts"]["epub.style.demo.maintain.sceneCount"] == 0, empty_catalog

    expected_presets = {
        "academic-cn",
        "classical-annotated-cn",
        "fiction-en",
        "literary-cn",
        "plain-cn",
        "poetry-cn",
    }
    preset_catalog = run_json(
        [
            "run",
            "epub.style.demo.maintain",
            "--json",
            "catalog=true",
            "collection=presets",
        ],
        empty,
    )
    preset_facts = preset_catalog["facts"]
    assert preset_facts["epub.style.demo.maintain.resourceSource"] == "embedded", preset_facts
    presets = preset_facts["epub.style.demo.maintain.presets"]
    assert {preset["id"] for preset in presets} == expected_presets, presets
    assert all(preset["readerStatus"] == "not-verified" for preset in presets), presets

    epub_path = empty / "preset-smoke.epub"
    with ZipFile(epub_path, "w") as epub:
        epub.writestr("mimetype", "application/epub+zip", compress_type=ZIP_STORED)
        epub.writestr(
            "META-INF/container.xml",
            '''<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>''',
            compress_type=ZIP_DEFLATED,
        )
        epub.writestr(
            "OEBPS/content.opf",
            '''<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="id"><metadata><dc:identifier id="id">urn:uuid:release-smoke</dc:identifier><dc:title>Release smoke</dc:title><dc:language>zh-CN</dc:language></metadata><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="nav"/><itemref idref="chapter"/></spine></package>''',
            compress_type=ZIP_DEFLATED,
        )
        epub.writestr(
            "OEBPS/Text/chapter.xhtml",
            '''<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" lang="zh-CN"><head><title>Release smoke</title>\n</head><body><h1>发布冒烟</h1><p>内嵌预设应能从仓库目录外读取。</p></body></html>''',
            compress_type=ZIP_DEFLATED,
        )
        epub.writestr(
            "OEBPS/nav.xhtml",
            '''<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>导航</title>\n</head><body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml">发布冒烟</a></li></ol></nav></body></html>''',
            compress_type=ZIP_DEFLATED,
        )

    for preset_id in sorted(expected_presets):
        report = run_json(
            [
                "run",
                "epub.typography.optimize",
                "--input",
                str(epub_path),
                "--dry-run",
                "--json",
                f"preset={preset_id}",
            ],
            empty,
        )
        assert report["status"] == "planned", report
        assert report["capability"] == "epub.typography.optimize", report
        report_facts = report["facts"]
        prefix = "epub.typography.optimize."
        assert report_facts[prefix + "preset"] == preset_id, report
        assert report_facts[prefix + "coverageBasis"] == "class-token", report
        assert report_facts[prefix + "scopeFileCount"] > 0, report
        coverage = report_facts[prefix + "coverage"]
        assert "uncoveredClasses" in coverage, report
        assert coverage["insufficientToDetermineApplicability"] is True, report

print(
    f"release smoke passed: {expected_version} {expected_goos}/{expected_goarch} "
    "(outside checkout)"
)
PY

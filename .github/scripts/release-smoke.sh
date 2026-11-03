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
    assert len(capabilities) == 21, len(capabilities)
    assert all(item["id"] != "epub.typography.optimize" for item in capabilities)
    assert any(item["id"] == "epub.typography.english.optimize" for item in capabilities)
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

print(
    f"release smoke passed: {expected_version} {expected_goos}/{expected_goarch} "
    "(outside checkout)"
)
PY

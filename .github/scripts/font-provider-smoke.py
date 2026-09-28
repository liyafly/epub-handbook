#!/usr/bin/env python3
"""Exercise the real font provider through a complete book-starter build."""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import io
import json
import os
import shlex
import shutil
import signal
import subprocess
import sys
import time
import zipfile
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
FONT_PROJECT = REPO_ROOT / "tools-font" / "epub-font"
sys.path.insert(0, str(FONT_PROJECT))

from epub_font import check, epubtext  # noqa: E402


SYNTH_PATH = FONT_PROJECT / "tests" / "synth.py"
SYNTH_SPEC = importlib.util.spec_from_file_location("font_provider_synth", SYNTH_PATH)
if SYNTH_SPEC is None or SYNTH_SPEC.loader is None:
    raise SystemExit(f"cannot load offline font fixture helpers from {SYNTH_PATH}")
SYNTH = importlib.util.module_from_spec(SYNTH_SPEC)
SYNTH_SPEC.loader.exec_module(SYNTH)

NEW_CHARACTER = "新"
MISSING_CHARACTER = "龘"
LIST_STYLE_TYPES = tuple(check.LIST_MARKERS)


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def run_logged(
    command: list[str], env: dict[str, str], log_path: Path
) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(command, env=env, text=True, capture_output=True, check=False)
    log_path.parent.mkdir(parents=True, exist_ok=True)
    log_path.write_text(result.stdout + result.stderr, encoding="utf-8")
    return result


def archive_source(epub_dir: Path, output: Path) -> None:
    with zipfile.ZipFile(output, "w") as archive:
        archive.writestr(
            zipfile.ZipInfo("mimetype", (2026, 1, 1, 0, 0, 0)),
            "application/epub+zip",
            compress_type=zipfile.ZIP_STORED,
        )
        for parent in ("META-INF", "OEBPS"):
            for path in sorted((epub_dir / parent).rglob("*")):
                if path.is_file():
                    archive.write(path, path.relative_to(epub_dir).as_posix(), compress_type=zipfile.ZIP_DEFLATED)


def prepare_book(book_dir: Path, env: dict[str, str]) -> tuple[Path, Path, Path]:
    starter = REPO_ROOT / "templates" / "book-starter" / "new-book.sh"
    result = run_logged(["sh", str(starter), str(book_dir)], env, book_dir.parent / "starter.log")
    if result.returncode != 0:
        raise RuntimeError(f"book starter failed; see {book_dir.parent / 'starter.log'}")

    epub_dir = book_dir / "03 制作工作区" / "epub"
    chapter = epub_dir / "OEBPS" / "Text" / "01-chapter.xhtml"
    chapter_text = chapter.read_text(encoding="utf-8")
    body_end = "</body>"
    if body_end not in chapter_text:
        raise RuntimeError("starter chapter has no body closing tag")

    list_samples = "\n".join(
        f'<ol class="marker-{style}"><li>列表标记</li></ol>' for style in LIST_STYLE_TYPES
    )
    added_markup = (
        f'<p class="synthetic-subset">字体覆盖测试：正文和罕见字 {MISSING_CHARACTER}。</p>\n'
        '<p class="synthetic-math">数学字体保留测试：数学</p>\n'
        f"{list_samples}\n"
    )
    chapter.write_text(chapter_text.replace(body_end, f"{added_markup}{body_end}", 1), encoding="utf-8")

    css = epub_dir / "OEBPS" / "Styles" / "fonts.css"
    css_text = css.read_text(encoding="utf-8")
    list_rules = "\n".join(
        f"ol.marker-{style} {{ list-style-type: {style}; }}" for style in LIST_STYLE_TYPES
    )
    css_text += (
        '\n@font-face { font-family: "SyntheticSubset"; src: url("../Fonts/full.ttf"); }\n'
        '@font-face { font-family: "SyntheticMath"; src: url("../Fonts/math.ttf"); }\n'
        '.synthetic-subset { font-family: "SyntheticSubset", serif; }\n'
        '.synthetic-math { font-family: "SyntheticMath", serif; }\n'
        f"{list_rules}\n"
    )
    css.write_text(css_text, encoding="utf-8")

    opf = epub_dir / "OEBPS" / "package.opf"
    opf_text = opf.read_text(encoding="utf-8")
    manifest_anchor = '    <item id="css-base"'
    font_items = (
        '    <item id="font-full" href="Fonts/full.ttf" media-type="font/ttf"/>\n'
        '    <item id="font-math" href="Fonts/math.ttf" media-type="font/ttf"/>\n'
    )
    if manifest_anchor not in opf_text:
        raise RuntimeError("starter OPF no longer has the expected manifest insertion point")
    opf.write_text(opf_text.replace(manifest_anchor, font_items + manifest_anchor, 1), encoding="utf-8")

    # Use the real package text collector to create a complete offline master,
    # then deliberately omit exactly one rare character for warning coverage.
    source_epub = book_dir.parent / "fixture-before-font-files.epub"
    archive_source(epub_dir, source_epub)
    with zipfile.ZipFile(source_epub) as archive:
        required = epubtext.read_book_text(archive).all_chars()
    source_epub.unlink()
    if NEW_CHARACTER in required:
        raise RuntimeError(f"fixture already requires the later-added character {NEW_CHARACTER}")
    if MISSING_CHARACTER not in required:
        raise RuntimeError(f"fixture does not require the deliberate missing character {MISSING_CHARACTER}")

    marker_chars = set("".join(check.LIST_MARKERS.values()))
    master_chars = (required | marker_chars | {NEW_CHARACTER}) - {MISSING_CHARACTER}
    full_font = SYNTH.build_font_for("".join(sorted(master_chars, key=ord)))
    math_font = SYNTH.build_math_font()
    fonts_dir = epub_dir / "OEBPS" / "Fonts"
    fonts_dir.mkdir(parents=True, exist_ok=True)
    (fonts_dir / "full.ttf").write_bytes(full_font)
    (fonts_dir / "math.ttf").write_bytes(math_font)
    return epub_dir, fonts_dir / "full.ttf", fonts_dir / "math.ttf"


def dist_epub(book_dir: Path) -> Path:
    return book_dir / "03 制作工作区" / "dist" / "book.epub"


def assert_only_fixed_dist(book_dir: Path, expected: Path) -> None:
    matches = sorted((book_dir / "03 制作工作区").rglob("*.epub"))
    if matches != [expected]:
        raise RuntimeError(f"expected one EPUB at {expected}, found {matches}")
    if not expected.is_file() or expected.stat().st_size == 0:
        raise RuntimeError(f"fixed distribution artifact is missing or empty: {expected}")


def read_entry(epub_path: Path, entry: str) -> bytes:
    with zipfile.ZipFile(epub_path) as archive:
        return archive.read(entry)


def run_build(book_dir: Path, env: dict[str, str], log_path: Path, *, success: bool) -> None:
    build = book_dir / "03 制作工作区" / "epub" / "build.sh"
    result = run_logged(["sh", str(build)], env, log_path)
    if success and result.returncode != 0:
        raise RuntimeError(f"book build failed; see {log_path}")
    if not success and result.returncode == 0:
        raise RuntimeError(f"book build unexpectedly succeeded; see {log_path}")


def validate_provider_report(book_dir: Path) -> tuple[dict, dict]:
    report_path = book_dir / "03 制作工作区" / ".pipeline" / "font-subset.json"
    envelope = json.loads(report_path.read_text(encoding="utf-8"))
    if envelope.get("status") != "complete":
        raise RuntimeError(f"font capability did not complete: {envelope.get('status')}")

    findings = envelope.get("findings", [])
    warning = next(
        (
            item
            for item in findings
            if item.get("id") == "font-subset.not-in-master"
            and item.get("level") == "warn"
            and item.get("location") == "OEBPS/Fonts/full.ttf"
        ),
        None,
    )
    if warning is None or "1 required character" not in warning.get("detail", ""):
        raise RuntimeError(f"public capability report is missing the structured missing-master warning: {findings}")

    provider_report = envelope.get("facts", {}).get("epub.font.subset.providerReport")
    if not isinstance(provider_report, dict) or not provider_report.get("providerVersion"):
        raise RuntimeError("public capability facts did not retain the provider version/report")
    fonts = provider_report.get("fonts", [])
    regular = next((item for item in fonts if item.get("target") == "OEBPS/Fonts/full.ttf"), None)
    math = next((item for item in fonts if item.get("target") == "OEBPS/Fonts/math.ttf"), None)
    expected_missing = f"U+9F98 {MISSING_CHARACTER}"
    if (
        regular is None
        or regular.get("notInMasterCount") != 1
        or expected_missing not in regular.get("notInMaster", [])
    ):
        raise RuntimeError(f"provider report did not retain the expected missing glyph: {regular}")
    if math is None or math.get("action") != "preserve" or math.get("reason") != "math-table":
        raise RuntimeError(f"provider report did not identify the MATH font preservation: {math}")
    return envelope, {"provider": provider_report, "warning": warning}


def check_list_markers(epub_path: Path, report_path: Path, marker_file: Path) -> dict:
    marker_file.write_text("".join(sorted(set("".join(check.LIST_MARKERS.values())), key=ord)), encoding="utf-8")
    result = subprocess.run(
        [
            "epub-font",
            "check",
            str(epub_path),
            "--font",
            "OEBPS/Fonts/full.ttf",
            "--chars-file",
            str(marker_file),
            "--json",
            str(report_path),
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"independent list-marker check failed ({result.returncode}): {result.stdout}{result.stderr}"
        )
    report = json.loads(report_path.read_text(encoding="utf-8"))
    fonts = report.get("fonts", [])
    if (
        len(fonts) != 1
        or not report.get("ok")
        or not fonts[0].get("ok")
        or fonts[0].get("missing")
        or fonts[0].get("noInk")
    ):
        raise RuntimeError(f"independent list-marker check did not cover every generated glyph: {fonts}")
    return report


def verify_cmap(font_bytes: bytes, char: str, expected: bool) -> None:
    from fontTools.ttLib import TTFont

    with TTFont(io.BytesIO(font_bytes), lazy=True) as font:
        present = ord(char) in (font.getBestCmap() or {})
    if present != expected:
        raise RuntimeError(f"font cmap presence for {char} was {present}, expected {expected}")


def assert_failed_build_preserves(
    book_dir: Path,
    expected_dist: Path,
    old_sha: str,
    log_path: Path,
) -> None:
    assert_only_fixed_dist(book_dir, expected_dist)
    new_sha = sha256(expected_dist.read_bytes())
    if new_sha != old_sha:
        raise RuntimeError(f"failed build replaced the fixed dist artifact: before={old_sha} after={new_sha}")
    pipeline = book_dir / "03 制作工作区" / ".pipeline"
    if (pipeline / "build.lock").exists() or list(pipeline.glob("build.*")):
        raise RuntimeError(f"failed build left a build lock or temporary candidate; see {log_path}")


def cancel_build(book_dir: Path, env: dict[str, str], root: Path, log_path: Path) -> int:
    if os.name != "posix":
        raise RuntimeError("the real cancellation smoke requires POSIX process groups")
    marker = root / "provider-started"
    wrapper_dir = root / "cancel-provider-bin"
    wrapper_dir.mkdir(parents=True, exist_ok=True)
    wrapper = wrapper_dir / "epub-font"
    wrapper.write_text(
        "#!/bin/sh\n"
        f"printf started > {shlex.quote(str(marker))}\n"
        "exec sleep 60\n",
        encoding="utf-8",
    )
    wrapper.chmod(0o755)
    cancel_env = dict(env)
    cancel_env["PATH"] = str(wrapper_dir) + os.pathsep + env.get("PATH", "")
    build = book_dir / "03 制作工作区" / "epub" / "build.sh"
    with log_path.open("w", encoding="utf-8") as log:
        process = subprocess.Popen(
            ["sh", str(build)],
            env=cancel_env,
            stdout=log,
            stderr=subprocess.STDOUT,
            text=True,
            start_new_session=True,
        )
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline and not marker.exists() and process.poll() is None:
            time.sleep(0.1)
        if not marker.exists():
            try:
                os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            process.wait(timeout=15)
            raise RuntimeError(f"the cancellation provider wrapper was not reached; see {log_path}")
        os.killpg(process.pid, signal.SIGTERM)
        return process.wait(timeout=15)


def check_gate_failure(
    book_dir: Path,
    dist: Path,
    env: dict[str, str],
    root: Path,
    old_sha: str,
    log_path: Path,
) -> dict:
    provider = shutil.which("epub-font", path=env.get("PATH"))
    if provider is None:
        raise RuntimeError("epub-font is not available on PATH for the independent gate smoke")
    marker = root / "independent-check-arguments.txt"
    wrapper_dir = root / "check-failure-bin"
    wrapper_dir.mkdir(parents=True, exist_ok=True)
    wrapper = wrapper_dir / "epub-font"
    wrapper.write_text(
        "#!/bin/sh\n"
        f"REAL_PROVIDER={shlex.quote(provider)}\n"
        f"MARKER={shlex.quote(str(marker))}\n"
        'if [ "${1-}" = check ]; then\n'
        '  printf \'%s\\n\' "$*" > "$MARKER"\n'
        '  "$REAL_PROVIDER" "$@"\n'
        '  status=$?\n'
        '  if [ "$status" -ne 0 ]; then exit "$status"; fi\n'
        "  echo 'simulated independent coverage regression' >&2\n"
        "  exit 1\n"
        "fi\n"
        'exec "$REAL_PROVIDER" "$@"\n',
        encoding="utf-8",
    )
    wrapper.chmod(0o755)
    check_env = dict(env)
    check_env["PATH"] = str(wrapper_dir) + os.pathsep + env.get("PATH", "")
    run_build(book_dir, check_env, log_path, success=False)
    assert_failed_build_preserves(book_dir, dist, old_sha, log_path)

    arguments = marker.read_text(encoding="utf-8")
    if "check" not in arguments or "--against" not in arguments or "full-font.epub" not in arguments:
        raise RuntimeError(f"build did not call the independent checker against FULL: {arguments}")
    log = log_path.read_text(encoding="utf-8")
    if "FAIL epub-font check --against FULL" not in log or "simulated independent coverage regression" not in log:
        raise RuntimeError(f"build did not surface the independent check failure; see {log_path}")
    report_path = book_dir / "03 制作工作区" / ".pipeline" / "font-check.json"
    report = json.loads(report_path.read_text(encoding="utf-8"))
    if report.get("mode") != "against" or not report.get("ok"):
        raise RuntimeError(f"the real differential check did not pass before simulated failure: {report}")
    return {"invokedAgainstFull": True, "realCheckPassed": True, "distSHA256Preserved": old_sha}


def validate_synth_nav_audit(epub_bin: Path, env: dict[str, str], root: Path) -> dict:
    synthetic = root / "provider-synthetic.epub"
    synthetic.write_bytes(SYNTH.build_epub({
        "OEBPS/Fonts/st-all.ttf": SYNTH.build_font_for(" "),
        "OEBPS/Fonts/st-all-semibold.ttf": SYNTH.build_font_for(" "),
        "OEBPS/Fonts/kt.otf": SYNTH.build_font_for(" "),
    }))
    result = subprocess.run(
        [str(epub_bin.resolve()), "run", "epub.package.nav.audit", "--input", str(synthetic), "--json"],
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(f"synthetic provider EPUB nav audit failed: {result.stdout}{result.stderr}")
    report = json.loads(result.stdout)
    findings = report.get("findings", [])
    errors = [item for item in findings if item.get("level") == "error"]
    if errors:
        raise RuntimeError(f"synthetic provider EPUB has nav audit errors: {errors}")
    return {"errorFindings": 0, "findingCount": len(findings)}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--epub-bin", required=True, type=Path)
    parser.add_argument("--workspace", required=True, type=Path)
    args = parser.parse_args()

    root = args.workspace.resolve()
    if root.exists():
        if not root.is_dir() or any(root.iterdir()):
            raise RuntimeError(f"smoke workspace must be absent or empty: {root}")
    root.mkdir(parents=True, exist_ok=True)
    book_dir = root / "合成字体书"
    env = dict(os.environ)
    env["EPUB_BIN"] = str(args.epub_bin.resolve())
    synth_nav = validate_synth_nav_audit(args.epub_bin, env, root)

    epub_dir, full_font, math_font = prepare_book(book_dir, env)
    dist = dist_epub(book_dir)
    source_hashes = {"full.ttf": sha256(full_font.read_bytes()), "math.ttf": sha256(math_font.read_bytes())}

    run_build(book_dir, env, root / "build-first.log", success=True)
    assert_only_fixed_dist(book_dir, dist)
    first_epub = dist.read_bytes()
    first_sha = sha256(first_epub)
    first_full = read_entry(dist, "OEBPS/Fonts/full.ttf")
    first_math = read_entry(dist, "OEBPS/Fonts/math.ttf")
    verify_cmap(first_full, NEW_CHARACTER, expected=False)
    if first_math != math_font.read_bytes():
        raise RuntimeError("automatic build changed the bytes of the source MATH font")
    _, report_evidence = validate_provider_report(book_dir)
    first_marker_report = check_list_markers(
        dist, root / "list-marker-check-first.json", root / "list-markers.txt"
    )
    first_source_hashes = {
        "full.ttf": sha256(full_font.read_bytes()),
        "math.ttf": sha256(math_font.read_bytes()),
    }
    if first_source_hashes != source_hashes:
        raise RuntimeError("the first book build changed one of the full-font source masters")

    chapter = epub_dir / "OEBPS" / "Text" / "01-chapter.xhtml"
    chapter_text = chapter.read_text(encoding="utf-8")
    anchor = f"字体覆盖测试：正文和罕见字 {MISSING_CHARACTER}。"
    if anchor not in chapter_text:
        raise RuntimeError("cannot locate the synthetic font coverage paragraph")
    chapter.write_text(chapter_text.replace(anchor, anchor + NEW_CHARACTER, 1), encoding="utf-8")

    run_build(book_dir, env, root / "build-with-new-character.log", success=True)
    assert_only_fixed_dist(book_dir, dist)
    second_sha = sha256(dist.read_bytes())
    second_full = read_entry(dist, "OEBPS/Fonts/full.ttf")
    second_math = read_entry(dist, "OEBPS/Fonts/math.ttf")
    verify_cmap(second_full, NEW_CHARACTER, expected=True)
    if second_sha == first_sha or second_full == first_full:
        raise RuntimeError("rebuilding after a new source character did not regenerate the subset from the full master")
    if second_math != math_font.read_bytes():
        raise RuntimeError("the rebuild changed the bytes of the source MATH font")
    validate_provider_report(book_dir)
    shutil.copyfile(
        book_dir / "03 制作工作区" / ".pipeline" / "font-subset.json",
        root / "font-subset-success.json",
    )
    second_marker_report = check_list_markers(
        dist, root / "list-marker-check-second.json", root / "list-markers.txt"
    )
    second_source_hashes = {
        "full.ttf": sha256(full_font.read_bytes()),
        "math.ttf": sha256(math_font.read_bytes()),
    }
    if second_source_hashes != source_hashes:
        raise RuntimeError("the second book build changed one of the full-font source masters")

    stable_sha = second_sha
    independent_gate = check_gate_failure(
        book_dir,
        dist,
        env,
        root,
        stable_sha,
        root / "independent-check-failure.log",
    )
    missing_env = dict(env)
    missing_env["PATH"] = "/usr/bin:/bin"
    run_build(book_dir, missing_env, root / "provider-missing.log", success=False)
    assert_failed_build_preserves(book_dir, dist, stable_sha, root / "provider-missing.log")
    missing_report = json.loads(
        (book_dir / "03 制作工作区" / ".pipeline" / "font-subset.json").read_text(encoding="utf-8")
    )
    if not any(
        item.get("id") == "font-subset.provider-missing"
        for item in missing_report.get("findings", [])
    ):
        raise RuntimeError("missing-provider failure did not preserve its structured finding")

    original_font = full_font.read_bytes()
    try:
        full_font.write_bytes(b"deliberately invalid synthetic font")
        run_build(book_dir, env, root / "provider-failure.log", success=False)
        assert_failed_build_preserves(book_dir, dist, stable_sha, root / "provider-failure.log")
        failure_report = json.loads(
            (book_dir / "03 制作工作区" / ".pipeline" / "font-subset.json").read_text(encoding="utf-8")
        )
        if failure_report.get("status") == "complete":
            raise RuntimeError("corrupt-font provider failure was incorrectly reported as complete")
    finally:
        full_font.write_bytes(original_font)
    if sha256(full_font.read_bytes()) != source_hashes["full.ttf"]:
        raise RuntimeError("the test did not restore the complete source font master after the failure case")

    cancellation_code = cancel_build(book_dir, env, root, root / "provider-cancelled.log")
    if cancellation_code == 0:
        raise RuntimeError("cancelled provider build unexpectedly returned success")
    assert_failed_build_preserves(book_dir, dist, stable_sha, root / "provider-cancelled.log")
    cancelled_evidence = {
        "providerStarted": (root / "provider-started").is_file(),
        "signal": "SIGTERM",
        "buildExitCode": cancellation_code,
        "distSHA256": sha256(dist.read_bytes()),
    }
    (root / "provider-cancelled.json").write_text(
        json.dumps(cancelled_evidence, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    final_source_hashes = {
        "full.ttf": sha256(full_font.read_bytes()),
        "math.ttf": sha256(math_font.read_bytes()),
    }
    if final_source_hashes != source_hashes:
        raise RuntimeError(f"a failure scenario changed the full-font source masters: {final_source_hashes}")

    evidence = {
        "providerVersion": report_evidence["provider"]["providerVersion"],
        "fontToolsVersion": report_evidence["provider"]["fontToolsVersion"],
        "sourceFontSHA256": source_hashes,
        "firstBuildSHA256": first_sha,
        "secondBuildSHA256": second_sha,
        "newCharacter": {"character": NEW_CHARACTER, "absentBefore": True, "presentAfter": True},
        "missingMasterWarning": report_evidence["warning"],
        "mathFontPreservedByteForByte": True,
        "independentMarkerCheck": {
            "firstRequired": first_marker_report["requiredChars"],
            "secondRequired": second_marker_report["requiredChars"],
            "missing": 0,
            "noInk": 0,
        },
        "failurePreservedDistSHA256": stable_sha,
        "independentCheckGate": independent_gate,
        "syntheticNavAudit": synth_nav,
        "failedScenarios": ["independent-coverage-check", "provider-missing", "corrupt-font", "cancelled-provider"],
        "goRacePackages": ["internal/book", "internal/extern", "internal/pipeline", "internal/zipfs"],
    }
    (root / "result.json").write_text(
        json.dumps(evidence, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(json.dumps(evidence, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(f"font provider smoke failed: {exc}", file=sys.stderr)
        raise

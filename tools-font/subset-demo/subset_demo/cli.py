"""Command line: subset the fonts listed in a config for one EPUB.

    uv run python -m subset_demo BOOK.epub --config fonts.json --out-dir OUT [--out-epub CANDIDATE.epub]

Exit codes: 0 = every check passed; 1 = a check failed (report written, no
candidate EPUB); 2 = bad input or unsupported font (nothing trustworthy written).
The candidate EPUB only replaces the bytes of the configured font entries; the
OPF, CSS, XHTML and every other entry are copied unchanged, in the same order
and with the same compression method.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import zipfile
from pathlib import Path

import fontTools

from . import epubtext, fontops

REPORT_NAME = "subset-report.json"
CONFIG_KEYS = {"version", "fonts"}
FONT_KEYS = {"target", "master", "variation", "extraText"}


class UsageError(Exception):
    pass


def _load_config(path: Path) -> dict:
    try:
        config = json.loads(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise UsageError(f"cannot read config {path}: {exc}") from exc
    except json.JSONDecodeError as exc:
        raise UsageError(f"config {path} is not valid JSON: {exc}") from exc
    if not isinstance(config, dict):
        raise UsageError("config must be a JSON object")
    unknown = set(config) - CONFIG_KEYS
    if unknown:
        raise UsageError(f"config has unknown keys: {sorted(unknown)}")
    if config.get("version") != 1:
        raise UsageError("config.version must be 1")
    fonts = config.get("fonts")
    if not isinstance(fonts, list) or not fonts:
        raise UsageError("config.fonts must be a non-empty list")
    seen = set()
    for index, job in enumerate(fonts):
        if not isinstance(job, dict):
            raise UsageError(f"fonts[{index}] must be an object")
        unknown = set(job) - FONT_KEYS
        if unknown:
            raise UsageError(f"fonts[{index}] has unknown keys: {sorted(unknown)}")
        target = job.get("target")
        if not isinstance(target, str) or not target or target.startswith("/"):
            raise UsageError(f"fonts[{index}].target must be a ZIP path such as OEBPS/Fonts/st-all.ttf")
        if target in seen:
            raise UsageError(f"fonts[{index}].target {target} is listed twice")
        seen.add(target)
        if "master" in job and (not isinstance(job["master"], str) or not job["master"]):
            raise UsageError(f"fonts[{index}].master must be a file path")
        if "extraText" in job and not isinstance(job["extraText"], str):
            raise UsageError(f"fonts[{index}].extraText must be a string")
    return config


def _check_output_paths(epub: Path, out_dir: Path, out_epub: Path | None) -> None:
    if out_dir.exists() and (not out_dir.is_dir() or any(out_dir.iterdir())):
        raise UsageError(f"--out-dir {out_dir} must not exist or must be an empty directory")
    if out_epub is not None:
        if out_epub.exists():
            raise UsageError(f"--out-epub {out_epub} already exists; choose a new path")
        if out_epub.resolve() == epub.resolve():
            raise UsageError("--out-epub must differ from the input EPUB")


def _read_master(job: dict, config_dir: Path, zf: zipfile.ZipFile) -> tuple[bytes, str]:
    if "master" in job:
        path = Path(job["master"])
        if not path.is_absolute():
            path = config_dir / path
        if not path.is_file():
            raise UsageError(f"master font {path} is not a regular file")
        return path.read_bytes(), str(path)
    return zf.read(job["target"]), f"epub:{job['target']}"


def _process_job(job: dict, book: epubtext.BookText, zf: zipfile.ZipFile, config_dir: Path) -> dict:
    target = job["target"]
    if target not in zf.namelist():
        raise UsageError(f"{target} is not in the EPUB (only existing font entries can be replaced)")
    item = book.item_by_path(target)
    if item is None:
        raise UsageError(f"{target} has no OPF manifest item")
    if target in book.encrypted_paths:
        raise UsageError(f"{target} is listed in META-INF/encryption.xml; stop (encrypted/obfuscated fonts are not handled)")
    spec = fontops.parse_variation(job.get("variation"))
    master_bytes, master_label = _read_master(job, config_dir, zf)
    original_bytes = zf.read(target)

    master_font = fontops.load_font(master_bytes, master_label)
    if "MATH" in master_font:
        raise fontops.FontJobError(f"{master_label}: has an OpenType MATH table; keep the complete math font (handbook §4.6 step 4)")
    master = fontops.font_facts(master_font)
    fontops.check_target_format(target, master.outline)
    limits = fontops.axis_limits(master, spec, master_label)

    required = {ord(ch) for ch in book.all_chars() | set(job.get("extraText", ""))}
    location = fontops.target_location(master, spec, limits)
    master_shapes = fontops.glyph_shapes(master_font, master.cmap, sorted(required), location)
    flavor = fontops.FLAVOR_BY_EXT[fontops.target_extension(target)]
    out_bytes, warnings = fontops.make_subset(master_font, required, spec, limits, flavor, target)

    out_font = fontops.load_font(out_bytes, f"{target} (output)")
    out = fontops.font_facts(out_font)
    checks, not_in_master = fontops.verify(master, out, required, spec, limits, target, out_font, master_shapes)
    if not_in_master:
        warnings.append(f"{target}: {len(not_in_master)} required characters are not in the master font (fallback fonts must cover them)")
    return {
        "target": target,
        "manifestId": item.item_id,
        "mediaType": item.media_type,
        "master": {"source": master_label, "sha256": fontops.sha256(master_bytes), "bytes": len(master_bytes),
                   "glyphs": master.glyph_count, "outline": master.outline, "axes": master.axes},
        "variation": {"mode": spec.mode, "axes": spec.axes or {}},
        "original": {"sha256": fontops.sha256(original_bytes), "bytes": len(original_bytes)},
        "output": {"sha256": fontops.sha256(out_bytes), "bytes": len(out_bytes), "glyphs": out.glyph_count,
                   "outline": out.outline, "flavor": out.flavor, "axes": out.axes, "tables": out.tables},
        "requiredCodepoints": len(required),
        "notInMaster": [fontops.cp_label(cp) for cp in not_in_master[:200]],
        "notInMasterCount": len(not_in_master),
        "checks": checks,
        "ok": all(check["ok"] for check in checks.values()),
        "warnings": warnings,
        "_bytes": out_bytes,
    }


def write_candidate_epub(src: Path, dst: Path, replacements: dict) -> list:
    """Copy every entry in order; replace only the given entries' bytes. Returns warnings."""
    warnings = []
    partial = dst.with_name(dst.name + ".partial")
    try:
        with zipfile.ZipFile(src) as zin, zipfile.ZipFile(partial, "w") as zout:
            infos = zin.infolist()
            if not infos or infos[0].filename != "mimetype" or infos[0].compress_type != zipfile.ZIP_STORED:
                warnings.append("input EPUB does not start with a stored 'mimetype' entry; copied as-is")
            for info in infos:
                data = replacements.get(info.filename)
                if data is None:
                    data = zin.read(info)
                out_info = zipfile.ZipInfo(info.filename, date_time=info.date_time)
                out_info.compress_type = info.compress_type
                out_info.external_attr = info.external_attr
                out_info.create_system = info.create_system
                zout.writestr(out_info, data)
        os.replace(partial, dst)
    except BaseException:
        partial.unlink(missing_ok=True)
        raise
    return warnings


def run(args) -> int:
    epub = Path(args.epub)
    out_dir = Path(args.out_dir)
    out_epub = Path(args.out_epub) if args.out_epub else None
    config_path = Path(args.config)
    if not epub.is_file():
        raise UsageError(f"{epub} is not a regular file")
    config = _load_config(config_path)
    _check_output_paths(epub, out_dir, out_epub)
    try:
        zf = zipfile.ZipFile(epub)
    except zipfile.BadZipFile as exc:
        raise UsageError(f"{epub} is not a ZIP/EPUB file: {exc}") from exc
    with zf:
        try:
            book = epubtext.read_book_text(zf)
        except epubtext.EpubError as exc:
            raise UsageError(str(exc)) from exc
        try:
            results = [_process_job(job, book, zf, config_path.parent) for job in config["fonts"]]
        except fontops.FontJobError as exc:
            raise UsageError(str(exc)) from exc

    out_dir.mkdir(parents=True, exist_ok=True)
    for result in results:
        path = out_dir / result["target"]
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(result["_bytes"])

    all_ok = all(result["ok"] for result in results)
    report = {
        "tool": "subset_demo",
        "fontTools": fontTools.version,
        "input": {"path": str(epub), "sha256": fontops.sha256(epub.read_bytes())},
        "config": {"path": str(config_path), "sha256": fontops.sha256(config_path.read_bytes())},
        "charset": {
            "total": len(book.all_chars()),
            "bySource": {name: len(chars) for name, chars in sorted(book.chars_by_source.items())},
        },
        "fonts": [{k: v for k, v in result.items() if k != "_bytes"} for result in results],
        "ok": all_ok,
        "candidateEpub": None,
    }
    if all_ok and out_epub is not None:
        warnings = write_candidate_epub(epub, out_epub, {r["target"]: r["_bytes"] for r in results})
        report["candidateEpub"] = {"path": str(out_epub), "sha256": fontops.sha256(out_epub.read_bytes()),
                                   "warnings": warnings}
    (out_dir / REPORT_NAME).write_text(
        json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )

    for result in results:
        status = "OK" if result["ok"] else "FAIL"
        failed = [name for name, check in result["checks"].items() if not check["ok"]]
        print(f"[{status}] {result['target']} mode={result['variation']['mode']} "
              f"{result['original']['bytes']} -> {result['output']['bytes']} bytes, "
              f"glyphs {result['master']['glyphs']} -> {result['output']['glyphs']}"
              + (f", failed: {', '.join(failed)}" if failed else ""))
        for warning in result["warnings"]:
            print(f"  warning: {warning}")
    print(f"report: {out_dir / REPORT_NAME}")
    if out_epub is not None:
        print(f"candidate: {out_epub}" if report["candidateEpub"] else "candidate: not written (a check failed)")
    return 0 if all_ok else 1


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(prog="python -m subset_demo", description=__doc__.splitlines()[0])
    parser.add_argument("epub", help="input EPUB (read only)")
    parser.add_argument("--config", required=True, help="fonts.json (see examples/fonts.sample.json)")
    parser.add_argument("--out-dir", required=True, help="new or empty directory for subset fonts and the report")
    parser.add_argument("--out-epub", help="optional new EPUB path; written only when every check passes")
    args = parser.parse_args(argv)
    try:
        return run(args)
    except UsageError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

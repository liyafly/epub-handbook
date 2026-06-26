"""EPUB Font Coverage Detector — CLI entry point."""

import argparse
import json
import sys
import zipfile
from .reader import read_epub
from .harvester import harvest_runs
from .font_index import build_font_index, build_font_index_by_path
from .resolver import build_font_face_registry, resolve_chains
from .classifier import classify, _cp_string, _is_pua, _is_vs
from .profiles import translate, PROFILES, PROFILE_DESCRIPTIONS
from .reporter import generate_report, print_summary, save_report
from .chain_health import assess_chains
from .charset_tiers import build_standard_charsets, load_standard_charset, classify_tier


OCC_CAP = 1000      # max occurrences for rare / problem characters
OCC_CAP_COMMON = 0   # common (GB2312/GBK/non-CJK) chars: count only, no occurrence bloat


def _build_char_inventory(runs: list, charsets: dict | None = None, extra=None) -> list:
    """Build character inventory from text runs.

    Collects all unique characters, their counts, run indices, and occurrences.
    When *charsets* is provided, only out-of-gbk / PUA rare characters get full
    occurrence collection (up to OCC_CAP). Common characters (GB2312/GBK/non-CJK)
    get at most OCC_CAP_COMMON reference entries — enough to locate a sample,
    not to bloat the report with every "的".
    """
    char_map = {}  # char → {count, runs: set, occurrences: list}
    _rare_cps: set[int] | None = None

    if charsets is not None:
        _rare_cps = set()

        def _is_rare_cached(cp: int) -> bool:
            if cp in _rare_cps:
                return True
            t = classify_tier(cp, charsets, extra)
            if t["is_rare"]:
                _rare_cps.add(cp)
                return True
            return False

    for run_idx, run in enumerate(runs):
        text = run.get("text", "")
        for offset, ch in enumerate(text):
            cp = ord(ch)
            # Skip whitespace and low-range ASCII
            if ch.isspace():
                continue
            if cp < 0x2E80 and cp > 0x2000:
                if not (0x3000 <= cp < 0x303F):  # Keep CJK punctuation
                    continue
            if cp < 0x300:  # Skip basic ASCII
                continue

            if ch not in char_map:
                char_map[ch] = {
                    "char": ch,
                    "count": 0,
                    "runs": set(),
                    "occurrences": [],
                }

            entry = char_map[ch]
            entry["count"] += 1
            entry["runs"].add(run_idx)

            # Rare chars → full occurrence tracking; common chars → capped.
            cap = OCC_CAP
            if _rare_cps is not None and not _is_rare_cached(cp):
                cap = OCC_CAP_COMMON

            if len(entry["occurrences"]) < cap:
                entry["occurrences"].append({
                    "file": run.get("file", ""),
                    "node_path": run.get("node_path", ""),
                    "offset": run.get("offset", 0) + offset,
                    "context": _get_context(text, offset),
                })

    # Convert to list, convert run sets to lists
    result = []
    for ch, data in sorted(char_map.items(), key=lambda x: ord(x[0])):
        result.append({
            "char": data["char"],
            "count": data["count"],
            "runs": sorted(data["runs"]),
            "occurrences": data["occurrences"],
        })

    return result


def _get_context(text: str, offset: int, radius: int = 20) -> str:
    """Get context around a position in text."""
    start = max(0, offset - radius)
    end = min(len(text), offset + radius + 1)
    ctx = text[start:end]
    return ("…" + ctx if start > 0 else ctx) + ("…" if end < len(text) else "")


def _flatten_cause_coverage(ci: dict) -> None:
    """Flatten per-chain coverage into top-level _causes / _covered_by
    fields that the HTML viewer consumes."""
    causes: list = []
    covered_by = None
    for cdata in ci.get("coverage", {}).values():
        if not isinstance(cdata, dict):
            continue
        c = cdata.get("cause")
        if c and c not in causes:
            causes.append(c)
        cb = cdata.get("covered_by")
        if cb and covered_by is None:
            covered_by = cb
    ci["_causes"] = causes
    ci["_covered_by"] = covered_by


def _attach_position_aggregates(ci: dict, worst_order: dict) -> None:
    """Compute worst (_pos, for Kindle risk) AND best (_pos_best) positions
    plus _has_embedded_cover across all of a char's chains."""
    positions = ci.get("coverage", {})
    worst = "first-embedded"
    best = None
    has_cover = False
    for cov in positions.values():
        pos = cov.get("position", "first-embedded") if isinstance(cov, dict) else getattr(cov, "position", "first-embedded")
        if worst_order.get(pos, 0) > worst_order.get(worst, 0):
            worst = pos
        if best is None or worst_order.get(pos, 9) < worst_order.get(best, 9):
            best = pos
        if pos in ("first-embedded", "later-embedded"):
            has_cover = True
    ci["_pos"] = worst
    ci["_pos_best"] = best or "none"
    ci["_has_embedded_cover"] = has_cover


def _attach_tier(ci: dict, charsets: dict, extra) -> None:
    """Attach standard-zone tier fields the viewer reads."""
    t = classify_tier(ord(ci["char"]), charsets, extra)
    ci["_block_tier"] = t["block_tier"]
    ci["_std_zone"] = t["std_zone"]
    ci["_rare"] = t["is_rare"]


def _annotate_book_meta(book: dict, epub_path: str) -> None:
    """Populate top-level title/path the reporter summary reads."""
    book["title"] = (book.get("opf_meta") or {}).get("title", "")
    book["path"] = epub_path


def _build_candidate_missing(results: list, cmap: set) -> list:
    """Chars not covered by the candidate font, tagged with rare flag —
    the residual set that would need 造字/合成字库."""
    missing = []
    for ci in results:
        if ord(ci["char"]) not in cmap:
            missing.append({
                "char": ci["char"],
                "cp": ci.get("cp"),
                "count": ci.get("count", 0),
                "rare": bool(ci.get("_rare", False)),
            })
    return missing


def _generate_html(report: dict, output_path: str) -> None:
    """Generate a self-contained HTML report with embedded data and fonts."""
    import os.path
    # Find viewer template relative to this source file
    src_dir = os.path.dirname(os.path.abspath(__file__))
    # Walk up to find font-coverage-viewer.html
    viewer_path = os.path.join(src_dir, "..", "..", "font-coverage-viewer.html")
    viewer_path = os.path.normpath(viewer_path)
    if not os.path.exists(viewer_path):
        return  # viewer not found, skip HTML generation
    with open(viewer_path, "r", encoding="utf-8") as f:
        template = f.read()
    # Embed report as compact JSON (no indentation, no extra spaces)
    report_json = json.dumps(report, ensure_ascii=False, separators=(",", ":"))
    # Inject the data script BEFORE the main <script> so window.__REPORT_DATA__
    # is defined when the viewer's auto-load check runs (otherwise nothing
    # renders until the user manually drops the JSON).
    injection = (
        '\n<script>window.__REPORT_DATA__=JSON.parse('
        + json.dumps(report_json)
        + ');</script>\n'
    )
    if "<body>" in template:
        html = template.replace("<body>", "<body>" + injection, 1)
    else:
        html = template.replace("</body>", injection + "</body>")
    html_path = output_path.replace(".json", ".html")
    if html_path == output_path:
        html_path = output_path + ".html"
    with open(html_path, "w", encoding="utf-8") as f:
        f.write(html)


def main():
    parser = argparse.ArgumentParser(
        description="EPUB Font Coverage Detector — classify characters by font coverage"
    )
    parser.add_argument("epub", help="Path to EPUB file")
    parser.add_argument("--output", "-o", help="Write JSON report to file")
    parser.add_argument(
        "--validate-with", help="Candidate font file: verify coverage"
    )
    parser.add_argument(
        "--profile", default="kindle-pessimistic",
        choices=list(PROFILES.keys()),
        help="Reader profile for summary",
    )
    parser.add_argument(
        "--standard-table",
        help="Optional path to a custom standard charset list to overlay "
             "(e.g. 通用规范汉字表). Default zones come from GB2312/GBK codecs.",
    )
    parser.add_argument("--json", action="store_true", help="Output full JSON to stdout")
    parser.add_argument("--quiet", "-q", action="store_true", help="Suppress output")
    args = parser.parse_args()

    try:
        # 1. Read EPUB structure
        book = read_epub(args.epub)

        # 2. Harvest text runs
        zf = zipfile.ZipFile(args.epub, "r")
        try:
            runs = harvest_runs(zf, book["xhtml_docs"])
        finally:
            zf.close()

        if not runs:
            print("No text runs found in EPUB.", file=sys.stderr)
            sys.exit(0)

        # 3. Build font cmap index
        zf = zipfile.ZipFile(args.epub, "r")
        try:
            font_paths = [f["resolved_path"] for f in book["font_files"]]
            font_index = build_font_index(zf, font_paths)
        finally:
            zf.close()

        # 4. Read CSS files
        zf = zipfile.ZipFile(args.epub, "r")
        try:
            css_files_with_content = []
            for cf in book.get("css_files", []):
                try:
                    content = zf.read(cf["resolved_path"])
                    css_files_with_content.append({**cf, "content_bytes": content})
                except KeyError:
                    continue
        finally:
            zf.close()

        # 5. Build font-face registry
        font_face_registry = build_font_face_registry(css_files_with_content)

        # 6. Resolve CSS chains
        chains, unresolved, run_chains = resolve_chains(
            runs, css_files_with_content, font_face_registry,
            [f["resolved_path"] for f in book["font_files"]]
        )

        # 7. Build character inventory.
        # Build standard-zone charsets early so we can skip full occurrence
        # tracking for common (GB2312/GBK) characters.
        standard_charsets = build_standard_charsets()
        extra_table = load_standard_charset(args.standard_table) if args.standard_table else None
        char_inventory = _build_char_inventory(runs, standard_charsets, extra_table)

        # 8. Build font index keyed by EXACT file path (not name-table family).
        # The CSS @font-face family and the font's internal family name often
        # disagree (e.g. "kxs" → rarefont.ttf whose name table says "Untitled");
        # keying by path is exact and avoids that misattribution.
        import os.path
        font_paths = [f["resolved_path"] for f in book["font_files"]]
        zf_idx = zipfile.ZipFile(args.epub, "r")
        try:
            font_by_path = build_font_index_by_path(zf_idx, font_paths)
        finally:
            zf_idx.close()

        font_index_dict: dict = {}
        for fpath, fi in font_by_path.items():
            entry = {
                "cmap": fi.codepoints,
                "subset": fi.is_subset,
                "family": fi.family,
                "file_path": fpath,
            }
            font_index_dict[fpath] = entry
            font_index_dict[os.path.basename(fpath)] = entry
            font_index_dict[os.path.basename(fpath).lower()] = entry

        # Normalize each chain segment's file hint to a key the index has.
        for chain in chains.values():
            for seg in chain:
                if seg.embedded and seg.file and seg.file not in font_index_dict:
                    bn = os.path.basename(seg.file)
                    if bn in font_index_dict:
                        seg.file = bn
                    elif bn.lower() in font_index_dict:
                        seg.file = bn.lower()

        # family(lower) → (file_path, real name-table family) for enrichment.
        family_to_font: dict = {}
        for chain in chains.values():
            for seg in chain:
                if seg.embedded and seg.file in font_index_dict:
                    fe = font_index_dict[seg.file]
                    family_to_font[seg.family.lower()] = (fe["file_path"], fe["family"])

        results = classify(char_inventory, font_index_dict, chains, run_chains)

        # Enrich + convert CoverageResult to plain dicts for JSON serialization
        enriched_results = []
        for ci in results:
            d = ci.__dict__ if hasattr(ci, "__dict__") else ci
            rfamily = None
            ffile = None
            for cdata in d.get("coverage", {}).values():
                cby = cdata.get("covered_by") if isinstance(cdata, dict) else None
                if cby and cby.lower() in family_to_font:
                    ffile, rfamily = family_to_font[cby.lower()]
                    break
            d["_real_family"] = rfamily
            d["_font_file"] = ffile
            _flatten_cause_coverage(d)
            _attach_tier(d, standard_charsets, extra_table)
            enriched_results.append(d)
        results = enriched_results

        # 9. Apply profiles.
        # _pos = worst position across chains (Kindle risk is pessimistic);
        # _pos_best / _has_embedded_cover capture "covered somewhere" so the
        # uncovered/big-font export isn't masked by one non-embedded chain.
        WORST_ORDER = {"first-embedded": 0, "later-embedded": 1, "only-non-embedded": 2, "none": 3}
        for ci in results:
            _attach_position_aggregates(ci, WORST_ORDER)
            profiles_dict = {pname: translate(ci["_pos"], pname) for pname in PROFILES}
            ci["profiles"] = profiles_dict
            ci["_kindle"] = profiles_dict.get("kindle-pessimistic", "fail")

        # 10. Candidate validation
        candidate = None
        if args.validate_with:
            try:
                from fontTools.ttLib import TTFont
                cfont = TTFont(args.validate_with)
                cmap = set(cfont.getBestCmap().keys())
                missing = _build_candidate_missing(results, cmap)
                candidate = {
                    "candidate_font": args.validate_with,
                    "total": len(results),
                    "covered": len(results) - len(missing),
                    "missing": len(missing),
                    "missing_chars": missing[:500],
                }
            except Exception as e:
                candidate = {"error": str(e)}

        # 11. Build fonts section
        embedded_info = []
        for file_path, fi in font_index.items():
            family = fi.family if hasattr(fi, "family") else file_path.rsplit("/", 1)[-1]
            is_subset = fi.is_subset if hasattr(fi, "is_subset") else False
            glyph_count = len(fi.codepoints) if hasattr(fi, "codepoints") else 0
            ivs_count = len(fi.ivs_records) if hasattr(fi, "ivs_records") else 0
            embedded_info.append({
                "family": family,
                "file": file_path,
                "is_subset": is_subset,
                "glyph_count": glyph_count,
                "ivs_records": ivs_count,
            })

        # 11.5 Embed font files as base64 for self-contained HTML reports
        import base64
        MAX_EMBED = 1024 * 1024  # 1 MB max per font for embedding
        embedded_fonts = {}
        zf_embed = zipfile.ZipFile(args.epub, "r")
        try:
            for fi in font_index.values():
                try:
                    data = zf_embed.read(fi.file_path)
                    if len(data) <= MAX_EMBED:
                        b64 = base64.b64encode(data).decode("ascii")
                        embedded_fonts[fi.file_path] = {
                            "data": b64,
                            "family": fi.family,
                            "size": len(data),
                        }
                except Exception:
                    continue
        finally:
            zf_embed.close()

        # 11.6 Font-chain health assessment
        chain_health = assess_chains(chains, font_index_dict, run_chains)
        _annotate_book_meta(book, args.epub)

        # 12. Generate report
        report = generate_report(
            book=book,
            fonts={
                "embedded": embedded_info,
                "referenced_non_embedded": [],
            },
            chains=chains,
            char_inventory=results,
            unresolved=unresolved,
            candidate_validation=candidate,
            chain_health=chain_health,
            standard_extra_name=(extra_table.name if extra_table else None),
        )

        report["_embedded_fonts"] = embedded_fonts

        # 13. Output
        if args.output:
            save_report(report, args.output)
            if not args.quiet:
                print(f"Report saved to {args.output}", file=sys.stderr)
            # Also generate self-contained HTML
            _generate_html(report, args.output)

        if args.json:
            print(json.dumps(report, ensure_ascii=False, indent=2))
        elif not args.quiet:
            print_summary(report)

        # Exit code
        profile = args.profile
        fails = 0
        for ci in results:
            p = ci.get("profiles", {})
            if p.get(profile) == "fail":
                fails += 1
        if fails > 0:
            sys.exit(1 if fails >= 5 else 0)

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc(file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()

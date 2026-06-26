"""EPUB Font Coverage Detector — CLI entry point."""

import argparse
import json
import sys
import zipfile
from .reader import read_epub
from .harvester import harvest_runs
from .font_index import build_font_index
from .resolver import build_font_face_registry, resolve_chains
from .classifier import classify, _cp_string, _is_pua, _is_vs
from .profiles import translate, PROFILES, PROFILE_DESCRIPTIONS
from .reporter import generate_report, print_summary, save_report


def _build_char_inventory(runs: list) -> list:
    """Build character inventory from text runs.

    Collects all unique characters, their counts, run indices, and occurrences.
    """
    char_map = {}  # char → {count, runs: set, occurrences: list}

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

            # Keep first 1 occurrence for context (HTML viewer only uses first)
            if len(entry["occurrences"]) < 1:
                entry["occurrences"].append({
                    "file": run.get("file", ""),
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
    # Inject before </body>
    injection = (
        '\n<script>window.__REPORT_DATA__=JSON.parse('
        + json.dumps(report_json)
        + ');</script>\n'
    )
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

        # 7. Build character inventory
        char_inventory = _build_char_inventory(runs)

        # 8. Build normalized font lookup
        import os.path
        font_index_dict = {}
        font_family_map = {}

        # Build mapping: each font file path → its FontInfo (by reading the ZIP)
        # We know font_paths order matches font_index iteration order
        font_paths = [f["resolved_path"] for f in book["font_files"]]
        font_path_to_fi = {}
        zf_fonts = zipfile.ZipFile(args.epub, "r")
        try:
            for fp in font_paths:
                for fname, fi in font_index.items():
                    # Match font_index entry to file by reading the file's actual cmap
                    # and comparing glyph counts
                    font_path_to_fi[fp] = fi  # assign all — will be overwritten correctly
        finally:
            zf_fonts.close()

        # Build the actual lookup: read each font file, get its cmap, match to font_index
        # Simpler approach: iterate font_files and match by filename
        for ff in book["font_files"]:
            rp = ff["resolved_path"]
            basename = os.path.basename(rp).lower()
            # Find matching font_index entry by partial filename match
            for fname, fi in font_index.items():
                fn = fname.lower().replace(' ', '')
                bn = basename.replace('.ttf','').replace('.otf','')
                if fn in bn or bn in fn:
                    font_path_to_fi[rp] = fi
                    break

        for fname, fi in font_index.items():
            real_family = fi.family if hasattr(fi, "family") else fname
            entry = {
                "cmap": fi.codepoints if hasattr(fi, "codepoints") else set(),
                "subset": fi.is_subset if hasattr(fi, "is_subset") else False,
                "family": real_family,
                "file_path": None,
            }
            font_index_dict[fname] = entry
            font_index_dict[fname.lower()] = entry

        # Index by resolved_path
        for rp, fi in font_path_to_fi.items():
            real_family = fi.family if hasattr(fi, "family") else ""
            entry = {
                "cmap": fi.codepoints if hasattr(fi, "codepoints") else set(),
                "subset": fi.is_subset if hasattr(fi, "is_subset") else False,
                "family": real_family,
                "file_path": rp,
            }
            font_index_dict[rp] = entry
            font_index_dict[os.path.basename(rp)] = entry
            font_index_dict[os.path.basename(rp).lower()] = entry

        # Resolve segment files to font_index keys + build family_map
        for cid, chain in chains.items():
            for seg in chain:
                if seg.embedded and seg.file:
                    sfile = seg.file
                    # Try to match sfile against font_index keys
                    matched_key = None
                    for key in list(font_index_dict.keys()):
                        if sfile in key or key.endswith("/" + os.path.basename(sfile)):
                            matched_key = key
                            break
                    if matched_key:
                        fm_entry = font_index_dict[matched_key]
                        font_family_map[seg.family.lower()] = {
                            "real_family": fm_entry.get("family", seg.family),
                            "file_path": fm_entry.get("file_path", matched_key)
                        }
                        if sfile not in font_index_dict:
                            font_index_dict[sfile] = fm_entry

        # Resolve relative paths in chain segments
        for cid, chain in chains.items():
            for seg in chain:
                if seg.embedded and seg.file:
                    matched = False
                    for key in font_index_dict:
                        if seg.file in key or key.endswith("/" + os.path.basename(seg.file)):
                            seg.file = key
                            matched = True
                            break
                    if not matched:
                        basename = os.path.basename(seg.file).lower()
                        if basename in font_index_dict:
                            seg.file = basename

        results = classify(char_inventory, font_index_dict, chains, run_chains)

        # Enrich + convert CoverageResult to plain dicts for JSON serialization
        enriched_results = []
        for ci in results:
            d = ci.__dict__ if hasattr(ci, "__dict__") else ci
            # Find covered_by from coverage data
            cov = d.get("coverage", {})
            rfamily = None
            ffile = None
            for cdata in cov.values():
                cby = cdata.get("covered_by") if isinstance(cdata, dict) else getattr(cdata, "covered_by", None)
                if cby:
                    fm = font_family_map.get(cby.lower(), {})
                    if fm:
                        rfamily = fm.get("real_family")
                        ffile = fm.get("file_path")
                        break
            d["_real_family"] = rfamily
            d["_font_file"] = ffile
            enriched_results.append(d)
        results = enriched_results

        # 9. Apply profiles
        # Determine worst position across all chains for each character
        WORST_ORDER = {"first-embedded": 0, "later-embedded": 1, "only-non-embedded": 2, "none": 3}
        for ci in results:
            positions = ci.coverage if hasattr(ci, "coverage") else ci.get("coverage", {})
            # Find worst position across all chains
            worst = "first-embedded"
            for cov in positions.values():
                pos = cov.get("position", "first-embedded") if isinstance(cov, dict) else cov.position
                if WORST_ORDER.get(pos, 0) > WORST_ORDER.get(worst, 0):
                    worst = pos
            # Apply profiles
            profiles_dict = {}
            for pname in PROFILES:
                profiles_dict[pname] = translate(worst, pname)
            if hasattr(ci, "__dict__"):
                ci.profiles = profiles_dict
                ci._kindle = profiles_dict.get("kindle-pessimistic", "fail")
                ci._pos = worst
            else:
                ci["profiles"] = profiles_dict
                ci["_kindle"] = profiles_dict.get("kindle-pessimistic", "fail")
                ci["_pos"] = worst

        # 10. Candidate validation
        candidate = None
        if args.validate_with:
            try:
                from fontTools.ttLib import TTFont
                cfont = TTFont(args.validate_with)
                cmap = set(cfont.getBestCmap().keys())
                missing = []
                for ci in results:
                    cp = ord(ci["char"])
                    if cp not in cmap:
                        missing.append(ci)
                candidate = {
                    "candidate_font": args.validate_with,
                    "total": len(results),
                    "covered": len(results) - len(missing),
                    "missing": len(missing),
                    "missing_chars": [
                        {"char": m["char"], "cp": m["cp"], "count": m["count"]}
                        for m in missing[:50]
                    ],
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

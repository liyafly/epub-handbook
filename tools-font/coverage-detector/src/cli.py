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

            # Keep first 20 occurrences for context
            if len(entry["occurrences"]) < 20:
                entry["occurrences"].append({
                    "file": run.get("file", ""),
                    "node_path": run.get("node_path", ""),
                    "offset": offset,
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

        # 8. Build normalized font lookup with real family names + file paths
        import os.path
        font_index_dict = {}
        font_family_map = {}  # family_name → {real_family, file_path}
        for fname, fi in font_index.items():
            real_family = fi.family if hasattr(fi, "family") else fname
            entry = {
                "cmap": fi.codepoints if hasattr(fi, "codepoints") else set(),
                "subset": fi.is_subset if hasattr(fi, "is_subset") else False,
                "family": real_family,
            }
            font_index_dict[fname] = entry
            for ff in book["font_files"]:
                rp = ff["resolved_path"]
                if fname in rp.lower() or rp.endswith("/" + fname):
                    font_index_dict[rp] = entry
                    font_index_dict[os.path.basename(rp)] = entry
                    font_index_dict[os.path.basename(rp).lower()] = entry
                    font_family_map[fname] = {"real_family": real_family, "file_path": rp}
                    # Also map by @font-face alias
                    for cid, chain in chains.items():
                        for seg in chain:
                            if seg.embedded and seg.file and fname in seg.file.lower():
                                font_family_map[seg.family.lower()] = {"real_family": real_family, "file_path": rp}
                    break

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
            else:
                ci["profiles"] = profiles_dict

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

        # 13. Output
        if args.output:
            save_report(report, args.output)
            if not args.quiet:
                print(f"Report saved to {args.output}", file=sys.stderr)

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

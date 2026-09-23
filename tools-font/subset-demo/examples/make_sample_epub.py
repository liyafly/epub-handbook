"""Build a sample EPUB that embeds a complete CJK variable font twice (400 and 600 roles).

    uv run python examples/make_sample_epub.py --master masters/NotoSerifSC-VF.ttf --out /tmp/x/sample-ttf.epub

The font entries are OEBPS/Fonts/st-all.<ext> and OEBPS/Fonts/st-all-semibold.<ext>,
where <ext> is .ttf for TrueType masters and .otf for CFF/CFF2 masters. Use an
uncompressed .ttf/.otf master here (not .woff2).
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from tests import synth  # noqa: E402

CHAPTER = """<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
<head><title>兰亭集序</title><link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/></head>
<body>
<h1>兰亭集序</h1>
<p>永和九年，岁在癸丑，暮春之初，会于会稽山阴之兰亭，修禊事也。群贤毕至，少长咸集。此地有崇山峻岭，茂林修竹；又有清流激湍，映带左右，引以为流觞曲水，列坐其次。</p>
<p>虽无丝竹管弦之盛，一觞一咏，亦足以畅叙幽情。是日也，天朗气清，惠风和畅。仰观宇宙之大，俯察品类之盛，所以游目骋怀，足以极视听之娱，信可乐也。</p>
<p class="em">夫人之相与，俯仰一世。或取诸怀抱，悟言一室之内；或因寄所托，放浪形骸之外。</p>
<div class="v"><p>“每览昔人兴感之由，若合一契，未尝不临文嗟悼，不能喻之于怀。”——《兰亭集序》</p></div>
<p class="note">后之视今，亦犹今之视昔。Wang Xizhi, 353 AD.</p>
</body>
</html>
"""

CSS = """@charset "utf-8";
@font-face { font-family: "st-all"; font-weight: 400; src: url("../Fonts/st-all.EXT"); }
@font-face { font-family: "st-all"; font-weight: 600; src: url("../Fonts/st-all-semibold.EXT"); }
body { font-family: "st-all", "Songti SC", "SimSun", "Noto Serif CJK SC", serif; }
h1 { font-weight: 600; }
.em { text-emphasis: filled sesame; }
.v { writing-mode: vertical-rl; height: 12em; }
.note::before { content: "\\3014\\6CE8\\3015"; }
"""


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--master", required=True, help="complete variable font (.ttf or .otf)")
    parser.add_argument("--out", required=True, help="new EPUB path")
    args = parser.parse_args()
    master = Path(args.master).read_bytes()
    if master[:4] == b"OTTO":
        ext = "otf"
    elif master[:4] in (b"\x00\x01\x00\x00", b"true"):
        ext = "ttf"
    else:
        print("error: --master must be an uncompressed .ttf or .otf font", file=sys.stderr)
        return 2
    out = Path(args.out)
    if out.exists():
        print(f"error: {out} already exists", file=sys.stderr)
        return 2
    out.parent.mkdir(parents=True, exist_ok=True)
    fonts = {f"OEBPS/Fonts/st-all.{ext}": master, f"OEBPS/Fonts/st-all-semibold.{ext}": master}
    out.write_bytes(synth.build_epub(fonts, chapter=CHAPTER, css=CSS.replace("EXT", ext)))
    print(f"wrote {out} ({ext} targets)")
    return 0


if __name__ == "__main__":
    sys.exit(main())

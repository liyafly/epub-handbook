"""epub-font: subset the fonts of an EPUB, or check that its fonts cover every character.

    epub-font subset BOOK.epub --out NEW.epub [--config fonts.json]
    epub-font check  BOOK.epub [--font PATH_IN_EPUB ...] [--font-file FILE --chars-file FILE] [--json REPORT]
"""

from __future__ import annotations

import sys

from . import check, subset

COMMANDS = {"subset": subset.main, "check": check.main}


def main(argv=None) -> int:
    argv = list(sys.argv[1:] if argv is None else argv)
    if argv[:1] in (["-h"], ["--help"]):
        print(__doc__)
        return 0
    if not argv or argv[0] not in COMMANDS:
        print(__doc__, file=sys.stderr)
        return 2
    return COMMANDS[argv[0]](argv[1:])

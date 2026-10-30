"""Package the retained synthetic scenes without regenerating assets or evidence."""

import os
from pathlib import Path
import sys
import tempfile
import zipfile


def main():
    if len(sys.argv) != 1:
        raise SystemExit("Usage: sh templates/shape-wrap-demo/build.sh")

    root = Path(__file__).resolve().parent
    source = root / "src-v3"
    output = root / "dist" / "shape-wrap-scenes-v3.epub"
    if (source / "mimetype").read_bytes() != b"application/epub+zip":
        raise SystemExit("Expected EPUB mimetype in src-v3/mimetype")
    for required in ("META-INF/container.xml", "OEBPS/package.opf"):
        if not (source / required).is_file():
            raise SystemExit(f"Missing source file: {required}")

    # Keep the historical ZIP order, timestamp and compression so a rebuild
    # can be compared byte for byte with the retained reader-tested artifact.
    files = []
    for path in sorted(source.rglob("*")):
        relative = path.relative_to(source)
        if any(part.startswith(".") for part in relative.parts):
            continue
        if path.is_symlink():
            raise SystemExit(f"Source contains a symlink: {relative}")
        if path.is_file() and relative.as_posix() != "mimetype":
            files.append(path)

    output.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=".shape-wrap-", suffix=".epub", dir=output.parent)
    os.close(fd)
    try:
        with zipfile.ZipFile(temporary, "w") as archive:
            for path in [source / "mimetype", *files]:
                name = path.relative_to(source).as_posix()
                entry = zipfile.ZipInfo(name, (2026, 10, 8, 9, 20, 0))
                entry.compress_type = zipfile.ZIP_STORED if name == "mimetype" else zipfile.ZIP_DEFLATED
                archive.writestr(entry, path.read_bytes())
        os.replace(temporary, output)
    finally:
        Path(temporary).unlink(missing_ok=True)
    print(f"Built {output}")


if __name__ == "__main__":
    main()

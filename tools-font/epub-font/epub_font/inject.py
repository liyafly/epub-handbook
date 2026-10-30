"""Inject SHA-locked font masters into an unpacked EPUB tree."""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import sys
import tempfile

from .subset import UsageError, _load_config


def find_library(config: Path, explicit=None) -> Path:
    selected = explicit or os.environ.get("EPUB_FONT_LIBRARY")
    if selected:
        library = Path(selected).expanduser().resolve()
        if not library.is_dir():
            raise UsageError(f"font library does not exist: {library}")
        return library
    for ancestor in config.resolve().parents:
        library = ancestor.parent / "fonts"
        if library.is_dir():
            return library.resolve()
    raise UsageError("font library not found; set EPUB_FONT_LIBRARY or create ../fonts/ above the book")


def atomic_write(path: Path, data: bytes):
    fd, temporary = tempfile.mkstemp(prefix=".font-", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
        os.replace(temporary, path)
    finally:
        Path(temporary).unlink(missing_ok=True)


def run(config_path: Path, tree: Path | None, library_path=None, update_lock=False):
    config = _load_config(config_path)
    jobs = [job for job in config["fonts"] if "from" in job]
    if not jobs:
        return {"injected": [], "library": None, "updatedLock": False}
    library = find_library(config_path, library_path)
    copies = []
    for job in jobs:
        name = job["from"]
        if not isinstance(name, str) or not name or name in (".", "..") or any(c in name for c in "/\\:\0"):
            raise UsageError(f"{job['target']}: from must be a font library filename")
        source = library / name
        if source.is_symlink() or not source.is_file():
            raise UsageError(f"font library missing regular file: {name} ({library})")
        data = source.read_bytes()
        actual = hashlib.sha256(data).hexdigest()
        expected = job.get("sha256")
        if not update_lock and (not isinstance(expected, str) or not re.fullmatch(r"[0-9a-f]{64}", expected)):
            raise UsageError(f"{name}: expected a full lowercase sha256 lock; run inject --update-lock after review")
        if not update_lock and actual != expected:
            raise UsageError(f"{name}: SHA-256 mismatch; expected {expected}; actual {actual}")
        target = PurePosixPath(job["target"])
        if target.is_absolute() or ".." in target.parts or "\\" in str(target) or target.suffix.lower() not in {".ttf", ".otf", ".woff", ".woff2"}:
            raise UsageError(f"unsafe font target: {target}")
        if tree is not None:
            destination = tree / str(target)
            # Reject every symlink component before any writes, including an
            # existing preview font symlink and the tree directory itself.
            for parent in (tree, destination, *destination.parents):
                if parent.is_symlink():
                    raise UsageError(f"font target contains a symlink: {destination}")
            if not destination.resolve().is_relative_to(tree.resolve()):
                raise UsageError(f"font target escapes source tree: {destination}")
            copies.append((destination, data))
        job["sha256"] = actual
    # Validate the entire plan before touching the source tree or the lock.
    for destination, data in copies:
        destination.parent.mkdir(parents=True, exist_ok=True)
        atomic_write(destination, data)
    if update_lock:
        atomic_write(config_path, (json.dumps(config, ensure_ascii=False, indent=2) + "\n").encode())
    return {"library": str(library), "injected": [job["target"] for job in jobs], "updatedLock": update_lock}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--tree")
    parser.add_argument("--library")
    parser.add_argument("--update-lock", action="store_true")
    args = parser.parse_args(argv)
    if not args.tree and not args.update_lock:
        parser.error("--tree is required unless updating only the lock")
    try:
        result = run(Path(args.config), Path(args.tree) if args.tree else None, args.library, args.update_lock)
    except (UsageError, OSError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(result, ensure_ascii=False))
    return 0

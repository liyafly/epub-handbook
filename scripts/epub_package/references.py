"""Stable local-reference rewrite boundary."""

from __future__ import annotations

from . import core


def transform_resource(
  data: bytes,
  old_path: str,
  new_path: str,
  path_map: dict[str, str],
  known_files: set[str],
) -> bytes:
  return core.transform_resource(data, old_path, new_path, path_map, known_files)

"""Stable navigation parse/build boundary."""

from __future__ import annotations

from . import core
from .models import Package, TocEntry


def parse_toc(files: dict[str, bytes], package: Package) -> list[TocEntry]:
  return core.parse_toc(files, package)


def build_nav(
  title: str,
  entries_by_group: list[tuple[str, list[TocEntry]]],
  nav_path: str,
  path_map: dict[str, str],
) -> bytes:
  return core.build_nav(title, entries_by_group, nav_path, path_map)


def build_ncx(
  title: str,
  entries_by_group: list[tuple[str, list[TocEntry]]],
  ncx_path: str,
  path_map: dict[str, str],
) -> bytes:
  return core.build_ncx(title, entries_by_group, ncx_path, path_map)

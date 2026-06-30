"""Stable package I/O boundary used by focused operations."""

from __future__ import annotations

from pathlib import Path

from . import core
from .models import Package


def read_package(files: dict[str, bytes]) -> Package:
  return core.read_package(files)


def write_epub(output_path: Path, files: dict[str, bytes], order: list[str] | None = None) -> None:
  core.write_epub(output_path, files, order)


def read_archive(path: Path) -> tuple[dict[str, bytes], list[str]]:
  return core.read_archive(path)


def ensure_no_encryption(files: dict[str, bytes], action: str) -> None:
  core.ensure_no_encryption(files, action)


def build_container(opf_path: str) -> bytes:
  return core.build_container(opf_path)

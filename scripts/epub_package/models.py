"""Data contracts shared by EPUB package operations."""

from __future__ import annotations

from dataclasses import asdict, dataclass, field
from xml.etree import ElementTree as ET


class PackageToolError(Exception):
  """The EPUB cannot be changed conservatively."""


@dataclass
class ManifestItem:
  item_id: str
  href: str
  media_type: str
  properties: str
  archive_path: str


@dataclass
class SpineItem:
  idref: str
  linear: str = ""
  properties: str = ""


@dataclass
class Package:
  opf_path: str
  root: ET.Element
  manifest_items: list[ManifestItem]
  spine_items: list[SpineItem]
  toc_id: str = ""


@dataclass
class MetadataInfo:
  title: str = ""
  subtitle: str = ""
  author: str = ""
  language: str = "zh-CN"
  publisher: str = ""
  description: str = ""
  identifier: str = ""
  rights: str = ""
  cover_href: str = ""

  def as_dict(self) -> dict[str, str]:
    return asdict(self)


@dataclass
class OperationReport:
  operation: str
  input: str | None = None
  inputs: list[str] = field(default_factory=list)
  output: str | None = None
  outputs: list[str] = field(default_factory=list)
  opf: str = ""
  merged_items: int = 0
  renamed_resources: int = 0
  segments_created: int = 0
  fields_updated: int = 0
  cover_path: str = ""
  warnings: list[str] = field(default_factory=list)

  def as_dict(self) -> dict[str, object]:
    return asdict(self)


@dataclass
class TocEntry:
  title: str
  href: str
  level: int = 1

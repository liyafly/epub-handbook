"""EPUB3 migration report model."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class ConversionReport:
  input_sha256: str
  output: str
  opf: str = ""
  package_version_before: str | None = None
  nav_entries: int = 0
  xhtml_files_updated: int = 0
  stylesheet_links_added: int = 0
  plain_notes_converted: int = 0
  duokan_notes_normalized: int = 0
  manifest_items_added: list[str] = field(default_factory=list)
  manifest_items_updated: int = 0
  metadata_updates: list[str] = field(default_factory=list)
  typography_roles: list[str] = field(default_factory=list)
  warnings: list[str] = field(default_factory=list)

  def as_dict(self) -> dict[str, object]:
    return {
      "harness": "epub3_oneclick_converter",
      "input_sha256": self.input_sha256,
      "output": self.output,
      "opf": self.opf,
      "package_version_before": self.package_version_before,
      "nav_entries": self.nav_entries,
      "xhtml_files_updated": self.xhtml_files_updated,
      "stylesheet_links_added": self.stylesheet_links_added,
      "plain_notes_converted": self.plain_notes_converted,
      "duokan_notes_normalized": self.duokan_notes_normalized,
      "manifest_items_added": self.manifest_items_added,
      "manifest_items_updated": self.manifest_items_updated,
      "metadata_updates": self.metadata_updates,
      "typography_roles": self.typography_roles,
      "warnings": self.warnings,
    }

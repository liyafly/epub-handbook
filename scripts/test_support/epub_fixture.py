"""Small, explicit EPUB ZIP builder for tests."""

from __future__ import annotations

import zipfile
from dataclasses import dataclass, field
from pathlib import Path


@dataclass
class EpubFixture:
  """Accumulate archive members without hiding package-specific test data."""

  members: dict[str, bytes] = field(default_factory=dict)

  def add_text(self, name: str, value: str, encoding: str = "utf-8") -> "EpubFixture":
    return self.add_bytes(name, value.encode(encoding))

  def add_bytes(self, name: str, value: bytes) -> "EpubFixture":
    if name == "mimetype":
      raise ValueError("mimetype is controlled by EpubFixture.write()")
    if not name or name in self.members:
      raise ValueError(f"duplicate or empty fixture member: {name!r}")
    self.members[name] = value
    return self

  def write(
    self,
    output: Path,
    *,
    mimetype: bytes = b"application/epub+zip",
    mimetype_compress_type: int = zipfile.ZIP_STORED,
  ) -> Path:
    output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(output, "w") as zf:
      zf.writestr("mimetype", mimetype, compress_type=mimetype_compress_type)
      for name, value in self.members.items():
        zf.writestr(name, value, compress_type=zipfile.ZIP_DEFLATED)
    return output


def write_epub(
  output: Path,
  files: dict[str, str | bytes],
  *,
  mimetype: bytes = b"application/epub+zip",
  mimetype_compress_type: int = zipfile.ZIP_STORED,
) -> Path:
  fixture = EpubFixture()
  for name, value in files.items():
    if name == "mimetype":
      mimetype = value.encode("utf-8") if isinstance(value, str) else value
      continue
    if isinstance(value, str):
      fixture.add_text(name, value)
    else:
      fixture.add_bytes(name, value)
  return fixture.write(
    output,
    mimetype=mimetype,
    mimetype_compress_type=mimetype_compress_type,
  )

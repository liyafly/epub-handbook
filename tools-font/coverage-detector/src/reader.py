"""
EPUB reader — extract structure (manifest, spine, metadata) from an EPUB file.

Parses ``META-INF/container.xml`` to locate the OPF, then extracts manifest
items classified by media type (XHTML, CSS, fonts), spine reading order, and
Dublin Core metadata.

Fails loudly on DRM protection, ZIP corruption, or missing OPF.
"""

import zipfile
import os
from lxml import etree


__all__ = [
    "read_epub",
    "EpubError",
    "DrmError",
    "CorruptZipError",
    "MissingOpfError",
]

# ---------------------------------------------------------------------------
# Namespace URIs
# ---------------------------------------------------------------------------
NS_CONTAINER = "urn:oasis:names:tc:opendocument:xmlns:container"
NS_OPF = "http://www.idpf.org/2007/opf"
NS_DC = "http://purl.org/dc/elements/1.1/"

# ---------------------------------------------------------------------------
# Media-type classification sets
# ---------------------------------------------------------------------------

XHTML_TYPES = frozenset((
    "application/xhtml+xml",
    "text/html",                          # rare in EPUB 3, common in EPUB 2
))

CSS_TYPES = frozenset((
    "text/css",
))

FONT_TYPES = frozenset((
    "application/vnd.ms-opentype",        # OTF
    "application/x-font-ttf",             # TTF
    "application/font-woff",              # WOFF
    "application/font-woff2",             # WOFF2
    "font/otf",
    "font/ttf",
    "font/woff",
    "font/woff2",
    "application/vnd.ms-fontobject",      # EOT (legacy)
    "application/x-font-opentype",         # alternative OTF
    "application/x-font-truetype",         # alternative TTF
    "application/x-font-woff",             # alternative WOFF
))


# ---------------------------------------------------------------------------
# Custom exceptions
# ---------------------------------------------------------------------------

class EpubError(Exception):
    """Base exception for EPUB reading failures."""
    pass


class DrmError(EpubError):
    """EPUB appears to be DRM-protected (encryption.xml found)."""
    pass


class CorruptZipError(EpubError):
    """ZIP archive is invalid or one or more entries are corrupted."""
    pass


class MissingOpfError(EpubError):
    """Container.xml or OPF file is missing or structurally invalid."""
    pass


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _read_zip_entry(zf, path):
    """Read *path* from the open ZipFile *zf*; raise MissingOpfError on miss."""
    try:
        return zf.read(path)
    except KeyError as exc:
        raise MissingOpfError(
            f"Required file not found in EPUB: {path}"
        ) from exc


def _find_opf_path(zf):
    """
    Parse ``META-INF/container.xml`` and return the *full-path* attribute
    of the first ``<rootfile>`` element.
    """
    container_bytes = _read_zip_entry(zf, "META-INF/container.xml")
    nsmap = {"c": NS_CONTAINER}
    root = etree.fromstring(container_bytes)
    rootfile = root.find(".//c:rootfile", nsmap)
    if rootfile is None:
        raise MissingOpfError(
            "container.xml has no <rootfile> element — cannot locate OPF"
        )
    opf_path = rootfile.get("full-path")
    if not opf_path:
        raise MissingOpfError(
            "<rootfile> element is missing a full-path attribute"
        )
    return opf_path


def _check_drm(zf):
    """
    Raise DrmError if ``META-INF/encryption.xml`` exists (DRM indicator).

    This catches Adobe Adept, Apple FairPlay, and most commercial DRM schemes
    that place an encryption.xml in the ZIP.  Font obfuscation keys are also
    stored here; downstream code that needs to handle standard font obfuscation
    should check the content of *encryption.xml* more carefully.
    """
    try:
        zf.getinfo("META-INF/encryption.xml")
    except KeyError:
        return
    raise DrmError(
        "EPUB appears to be DRM-protected "
        "(META-INF/encryption.xml found)"
    )


def _classify_item(media_type, href):
    """
    Classify a manifest item by its declared media-type, falling back to file
    extension when the type is unrecognised.

    Returns one of: ``"xhtml"``, ``"css"``, ``"font"``, ``"other"``.
    """
    mt = media_type.lower().strip()
    if mt in XHTML_TYPES:
        return "xhtml"
    if mt in CSS_TYPES:
        return "css"
    if mt in FONT_TYPES:
        return "font"
    # Last-resort guess by extension (some EPUB 2 sources omit media-type).
    ext = os.path.splitext(href)[1].lower()
    if ext in (".xhtml", ".html", ".htm"):
        return "xhtml"
    if ext == ".css":
        return "css"
    if ext in (".otf", ".ttf", ".woff", ".woff2", ".eot"):
        return "font"
    return "other"


def _parse_opf(opf_bytes, opf_dir):
    """
    Parse an OPF document and return a structured result.

    Parameters
    ----------
    opf_bytes : bytes
        Raw byte content of the OPF file.
    opf_dir : str
        ZIP directory containing the OPF (used to resolve relative hrefs).

    Returns
    -------
    dict
        ``xhtml_docs``, ``css_files``, ``font_files`` : list[dict]
        ``opf_meta`` : dict
        ``spine_order`` : list[str]
        ``other_items`` : list[dict]
    """
    nsmap = {
        "opf": NS_OPF,
        "dc": NS_DC,
    }
    root = etree.fromstring(opf_bytes)

    # -- Metadata --
    title = root.findtext("opf:metadata/dc:title", "", nsmap).strip()
    identifier = root.findtext("opf:metadata/dc:identifier", "", nsmap).strip()
    package_version = root.get("version", "unknown")

    opf_meta = {
        "title": title,
        "identifier": identifier,
        "package_dir": opf_dir,
        "package_version": package_version,
    }

    # -- Manifest --
    manifest = root.find("opf:manifest", nsmap)
    if manifest is None:
        raise MissingOpfError("OPF has no <manifest> element")

    xhtml_docs = []
    css_files = []
    font_files = []
    other_items = []

    for item in manifest.findall("opf:item", nsmap):
        item_id = item.get("id", "")
        href = item.get("href", "")
        media_type = item.get("media-type", "")
        properties = item.get("properties", "")

        # Resolve relative href against the OPF directory
        resolved_path = (
            os.path.normpath(os.path.join(opf_dir, href))
            if opf_dir and href
            else href
        )

        entry = {
            "id": item_id,
            "href": href,
            "media_type": media_type,
            "properties": properties,
            "resolved_path": resolved_path,
        }

        kind = _classify_item(media_type, href)
        if kind == "xhtml":
            xhtml_docs.append(entry)
        elif kind == "css":
            css_files.append(entry)
        elif kind == "font":
            font_files.append(entry)
        else:
            other_items.append(entry)

    # -- Spine (reading order) --
    spine = root.find("opf:spine", nsmap)
    spine_order = []
    if spine is not None:
        for itemref in spine.findall("opf:itemref", nsmap):
            spine_order.append(itemref.get("idref", ""))

    return {
        "xhtml_docs": xhtml_docs,
        "css_files": css_files,
        "font_files": font_files,
        "opf_meta": opf_meta,
        "spine_order": spine_order,
        "other_items": other_items,
    }


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def read_epub(epub_path):
    """
    Open an EPUB file and return its parsed structure.

    Parameters
    ----------
    epub_path : str
        Path to the ``.epub`` file on disk.

    Returns
    -------
    dict
        ``xhtml_docs``
            ``[{id, href, media_type, properties, resolved_path}, ...]``
        ``css_files``
            ``[{id, href, media_type, properties, resolved_path}, ...]``
        ``font_files``
            ``[{id, href, media_type, properties, resolved_path}, ...]``
        ``opf_meta``
            ``{title, identifier, package_dir, package_version}``
        ``spine_order``
            List of ``idref`` values in spine reading order.
        ``other_items``
            Unclassified manifest entries (images, audio, etc.).

    Raises
    ------
    FileNotFoundError
        *epub_path* does not exist on disk.
    CorruptZipError
        File is not a valid ZIP or has corrupted entries.
    DrmError
        ``META-INF/encryption.xml`` exists (DRM indicator).
    MissingOpfError
        Container.xml is missing, the OPF cannot be located, or the OPF
        lacks a ``<manifest>`` element.
    """
    # -- File existence --
    if not os.path.isfile(epub_path):
        raise FileNotFoundError(f"EPUB file not found: {epub_path}")

    # -- Open ZIP --
    try:
        zf = zipfile.ZipFile(epub_path, "r")
    except zipfile.BadZipFile as exc:
        raise CorruptZipError(
            f"Not a valid ZIP archive: {epub_path}"
        ) from exc

    with zf:
        # Full integrity check — decompress every entry and verify CRC.
        bad_entry = zf.testzip()
        if bad_entry is not None:
            raise CorruptZipError(
                f"ZIP corruption detected in entry: {bad_entry}"
            )

        # DRM check before any further processing.
        _check_drm(zf)

        # Locate and read the OPF.
        opf_path = _find_opf_path(zf)
        opf_bytes = zf.read(opf_path)
        opf_dir = os.path.dirname(opf_path) or ""

        result = _parse_opf(opf_bytes, opf_dir)

    return result

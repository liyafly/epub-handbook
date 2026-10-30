#!/usr/bin/env python3
"""Package authored synthetic recipe fixtures; no user-book mutation."""

import hashlib
import html
import json
from pathlib import Path
import uuid
import xml.etree.ElementTree as ET
import zipfile


ROOT = Path(__file__).resolve().parent
RECIPES = ("shape-wrap", "drop-cap", "text-emphasis", "theme-adaptive-art", "parallel-pair")


def package(name: str, baseline: bool) -> dict:
    source = ROOT / name
    page = (source / "sample.xhtml").read_bytes()
    tree = ET.fromstring(page)
    title = tree.findtext("{http://www.w3.org/1999/xhtml}head/{http://www.w3.org/1999/xhtml}title")
    language = tree.get("{http://www.w3.org/XML/1998/namespace}lang", "zh-CN")
    label = html.escape(title)
    identity = uuid.uuid5(uuid.NAMESPACE_URL, "https://example.invalid/epub-recipes/" + name)
    css = b"/* Baseline: recipe enhancement absent. */\n" if baseline else (source / "recipe.css").read_bytes()
    files = {
        "META-INF/container.xml": b'<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>',
        "OEBPS/sample.xhtml": page,
        "OEBPS/recipe.css": css,
        "OEBPS/nav.xhtml": f'<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="{language}"><head><title>Contents</title></head><body><nav epub:type="toc" id="toc"><h1>Contents</h1><ol><li><a href="sample.xhtml">{label}</a></li></ol></nav></body></html>'.encode(),
        "OEBPS/toc.ncx": f'<?xml version="1.0"?><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head><meta name="dtb:uid" content="urn:uuid:{identity}"/></head><docTitle><text>{label}</text></docTitle><navMap><navPoint id="sample" playOrder="1"><navLabel><text>{label}</text></navLabel><content src="sample.xhtml"/></navPoint></navMap></ncx>'.encode(),
    }
    image_item = ""
    for image_name, media_type in (("art.png", "image/png"), ("art.svg", "image/svg+xml"), ("art-backed.svg", "image/svg+xml")):
        if (source / image_name).exists():
            files["OEBPS/" + image_name] = (source / image_name).read_bytes()
            item_id = "art-backed" if image_name == "art-backed.svg" else "art"
            image_item += f'<item id="{item_id}" href="{image_name}" media-type="{media_type}"/>'
    files["OEBPS/package.opf"] = f'''<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="bookid">urn:uuid:{identity}</dc:identifier><dc:title>{label}</dc:title><dc:language>{language}</dc:language><meta property="dcterms:modified">2026-10-30T00:00:00Z</meta></metadata>
<manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="sample" href="sample.xhtml" media-type="application/xhtml+xml"/><item id="css" href="recipe.css" media-type="text/css"/>{image_item}</manifest><spine toc="ncx"><itemref idref="sample"/></spine></package>'''.encode()
    output = ROOT / "dist" / (name + ("-baseline" if baseline else "") + ".epub")
    with zipfile.ZipFile(output, "w") as archive:
        for path, data in [("mimetype", b"application/epub+zip"), *sorted(files.items())]:
            info = zipfile.ZipInfo(path, (2026, 10, 30, 0, 0, 0))
            info.compress_type = zipfile.ZIP_STORED if path == "mimetype" else zipfile.ZIP_DEFLATED
            archive.writestr(info, data)
    return {"artifact": output.name, "sha256": hashlib.sha256(output.read_bytes()).hexdigest(), "readerStatus": "not-verified"}


if __name__ == "__main__":
    (ROOT / "dist").mkdir(exist_ok=True)
    inventory = [package(name, baseline) for name in RECIPES for baseline in (False, True)]
    (ROOT / "dist" / "inventory.json").write_text(json.dumps(inventory, indent=2) + "\n", encoding="utf-8")
    print(f"Built {len(RECIPES)} recipe fixtures and their CSS baselines in {ROOT / 'dist'}")

"""Rebuild a separate tool-generated polygon candidate; keep historical v3 intact."""

import hashlib
import json
from pathlib import Path
import re
import shutil
import uuid
import zipfile

from epub_art import shapes

root = Path(__file__).resolve().parent
output = root / '.pipeline' / 'generated'
source = output / 'src'
shutil.copytree(root / 'src-v3', source, dirs_exist_ok=True)
stylesheet = source / 'OEBPS/Styles/shape.css'
css = stylesheet.read_text().split('.s1 {', 1)[0]
scenes = []
for index, path in enumerate(sorted((source / 'OEBPS/Images').glob('*.png')), 1):
    result = shapes.polygon(path, side='right' if 'right' in path.stem else 'left')
    css += shapes.css(result, '.s' + str(index))
    (output / (path.stem + '.json')).write_text(json.dumps(result, indent=2) + '\n')
    scenes.append({'scene': path.stem, 'points': len(result['polygon'])})
stylesheet.write_text(css)
identifier = 'urn:uuid:' + str(uuid.uuid5(uuid.NAMESPACE_URL, 'generated-shape-css:' + hashlib.sha256(css.encode()).hexdigest()))
old = re.search(r'<dc:identifier[^>]*>(.*?)</dc:identifier>', (source / 'OEBPS/package.opf').read_text())[1]
for path in (source / 'OEBPS/package.opf', source / 'OEBPS/toc.ncx'):
    path.write_text(path.read_text().replace(old, identifier))
artifact = root / 'dist/shape-wrap-generated.epub'
artifact.parent.mkdir(exist_ok=True)
with zipfile.ZipFile(artifact, 'w') as archive:
    for path in [source / 'mimetype', *sorted(p for p in source.rglob('*') if p.is_file() and p.name != 'mimetype')]:
        info = zipfile.ZipInfo(path.relative_to(source).as_posix(), (2026, 10, 30, 0, 0, 0))
        info.compress_type = zipfile.ZIP_STORED if path.name == 'mimetype' else zipfile.ZIP_DEFLATED
        archive.writestr(info, path.read_bytes())
(output / 'summary.json').write_text(json.dumps({'artifact': artifact.name, 'sha256': hashlib.sha256(artifact.read_bytes()).hexdigest(), 'scenes': scenes}, indent=2) + '\n')
print(artifact)

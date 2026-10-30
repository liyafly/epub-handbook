"""Freeze and audit an existing OEBPS EPUB before creating a maintainable book."""

import csv
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import subprocess
import sys
import tempfile
import xml.etree.ElementTree as ET
import zipfile


def sha(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def register_font(source, library, origin):
    """Reuse an exact master; never overwrite a different library version."""
    library.mkdir(parents=True, exist_ok=True)
    digest = sha(source)
    name = source.name
    for existing in sorted(library.iterdir()):
        if existing.is_file() and existing.suffix.lower() in {'.otf', '.ttf', '.woff', '.woff2'} and sha(existing) == digest:
            return existing.name, digest
    if (library / name).exists():
        name = f'{source.stem}-{digest[:12]}{source.suffix}'
    if (library / name).exists() and sha(library / name) != digest:
        raise RuntimeError(f'Font library filename collision: {name}')
    shutil.copy2(source, library / name)
    inventory = library / '字体清单.tsv'
    is_new = not inventory.exists()
    with inventory.open('a', encoding='utf-8', newline='') as stream:
        writer = csv.writer(stream, delimiter='\t')
        if is_new:
            writer.writerow(['文件名', 'sha256', '来源', '许可', '备注'])
        writer.writerow([name, digest, origin, '待核', '按 SHA 去重；仅本地制作使用，发布前核验许可'])
    (library / 'README.md').write_text('# 共享字体库\n\n全量字体按 SHA-256 锁定；来源与许可见 `字体清单.tsv`。子集字体留在各书源树。\n字体版本变化须审阅后运行 `epub-font inject --config <fonts.json> --update-lock`。\n')
    return name, digest


def command(argv, report):
    result = subprocess.run(argv, capture_output=True, text=True)
    report.write_text(result.stdout)
    if result.stderr:
        report.with_suffix('.stderr.log').write_text(result.stderr)
    return result


def main(argv=None):
    argv = sys.argv[1:] if argv is None else argv
    if len(argv) != 2:
        raise SystemExit('Usage: import-book.sh ORIGINAL.epub NEW_BOOK_DIRECTORY')
    original, destination = (Path(value).expanduser().resolve() for value in argv)
    if not original.is_file() or destination.exists():
        raise SystemExit('Input must be a regular EPUB and destination must not exist')
    destination.parent.mkdir(parents=True, exist_ok=True)
    retained = Path(tempfile.mkdtemp(prefix='.book-import-', dir=destination.parent))
    book = retained / 'book'
    starter = Path(__file__).resolve().parent
    try:
        subprocess.run(['sh', str(starter / 'new-book.sh'), str(book)], check=True, capture_output=True)
        frozen = book / '01 源文件' / original.name
        shutil.copy2(original, frozen)
        work = book / '03 制作工作区'
        reports = work / '.pipeline' / 'import'
        reports.mkdir(parents=True)
        cli = os.environ.get('EPUB_BIN', 'epub')
        audit = command([cli, 'run', 'epub.package.nav.audit', '--input', str(frozen), '--json'], reports / 'nav-audit.json')
        if audit.returncode:
            raise RuntimeError('Navigation audit failed; inspect report and use cleanup-flow.md for repairs. DRM is not decrypted.')
        with zipfile.ZipFile(frozen) as archive:
            names = archive.namelist()
            if len(names) != len(set(names)):
                raise RuntimeError('Duplicate ZIP entries are not supported')
            container = ET.fromstring(archive.read('META-INF/container.xml'))
            rootfiles = [node.get('full-path', '') for node in container.iter() if node.tag.endswith('}rootfile')]
            if not rootfiles or not rootfiles[0].startswith('OEBPS/'):
                raise RuntimeError('Non-OEBPS layout: preserve the existing build path; no automatic renaming')
            if any(name != 'mimetype' and not name.startswith(('META-INF/', 'OEBPS/')) for name in names if not name.endswith('/')):
                raise RuntimeError('Resources outside OEBPS/META-INF: keep the existing build path')
            if 'META-INF/encryption.xml' in names or 'META-INF/rights.xml' in names:
                raise RuntimeError('Encryption metadata present; stop for explicit audit, no decryption')
            shutil.rmtree(work / 'epub')
            source = work / 'epub'
            source.mkdir()
            for info in archive.infolist():
                relative = PurePosixPath(info.filename)
                if relative.is_absolute() or '..' in relative.parts or '\\' in info.filename or (info.external_attr >> 16) & 0o170000 == 0o120000:
                    raise RuntimeError(f'Unsafe ZIP entry: {info.filename}')
                target = source / str(relative)
                if info.is_dir():
                    target.mkdir(parents=True, exist_ok=True)
                else:
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(archive.read(info))
        provider = os.environ.get('EPUB_FONT_BIN', 'epub-font')
        font_files = [p for p in source.rglob('*') if p.suffix.lower() in {'.otf', '.ttf', '.woff', '.woff2'}]
        jobs, decisions = [], []
        if font_files:
            coverage = command([provider, 'coverage', str(frozen), '--json'], reports / 'coverage.json')
            if coverage.returncode not in (0, 1):
                raise RuntimeError('Font inventory failed; source and report retained')
            inventory = json.loads(coverage.stdout)['fonts']['embedded']
            indexed = {item['file']: item for item in inventory}
            library = Path(os.environ.get('EPUB_FONT_LIBRARY', str(destination.parent / 'fonts'))).expanduser().resolve()
            # Check that every font was classified before moving any master.
            for font in font_files:
                target = font.relative_to(source).as_posix()
                if target not in indexed:
                    raise RuntimeError(f'Unclassified font: {target}')
            for font in font_files:
                target = font.relative_to(source).as_posix()
                info = indexed[target]
                job = {'target': target}
                if info.get('variation_axes'):
                    job['variation'] = {'mode': 'instance', 'axes': info['variation_axes']}
                suspected_subset = info['is_subset'] or info['glyph_count'] < 10000
                if not suspected_subset:
                    name, digest = register_font(font, library, destination.name)
                    job.update({'from': name, 'sha256': digest})
                    font.unlink()
                jobs.append(job)
                decisions.append({'target': target, 'glyphs': info['glyph_count'], 'subsetHeuristic': suspected_subset, 'action': 'retain-source-subset' if suspected_subset else 'library'})
        ignore = book / '.gitignore'
        text = ignore.read_text().replace('/03 制作工作区/epub/OEBPS/Fonts/\n', '')
        text += ''.join(f"/03 制作工作区/epub/{job['target']}\n" for job in jobs if 'from' in job)
        ignore.write_text(text)
        if jobs:
            (work / 'fonts.json').write_text(json.dumps({'version': 1, 'fonts': jobs}, ensure_ascii=False, indent=2) + '\n')
        digest = sha(frozen)
        description = book / '制作说明.md'
        description.write_text(description.read_text() + f'\n- 冻结底本：`01 源文件/{original.name}`；SHA-256 `{digest}`\n- 导入：导航审计通过；保留原布局与内容，未执行规范化、迁移或 CSS 清洗。字体判断见 `03 制作工作区/.pipeline/import/fonts.json`。\n')
        (reports / 'relocation.json').write_text(json.dumps({'auditedPath': str(frozen), 'retainedPath': str(destination / '01 源文件' / original.name), 'sha256': digest}, ensure_ascii=False, indent=2) + '\n')
        (reports / 'fonts.json').write_text(json.dumps(decisions, ensure_ascii=False, indent=2) + '\n')
        subprocess.run(['git', '-C', str(book), 'add', '-A'], check=True)
        subprocess.run(['git', '-C', str(book), 'commit', '-m', f'import: freeze EPUB baseline sha256 {digest}'], check=True, capture_output=True)
        os.rename(book, destination)
        retained.rmdir()
        print('Imported:', destination)
    except Exception as exc:
        raise SystemExit(f'{exc}\nImport stopped; frozen input and diagnostics retained: {retained}') from exc


if __name__ == '__main__':
    main()

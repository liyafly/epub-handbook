"""Render one laid-out XHTML/HTML page to an opaque image with headless Chrome or macOS WebKit."""

import hashlib
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
from urllib.parse import unquote, urlsplit

from PIL import Image

CHROME_CANDIDATES = (
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
    '/Applications/Chromium.app/Contents/MacOS/Chromium',
    'google-chrome', 'google-chrome-stable', 'chromium', 'chromium-browser', 'microsoft-edge',
)
# Stretch the root boxes to the viewport so full-page backgrounds reach the bottom edge.
FILL_CSS = 'html, body { height: 100%; min-height: 100%; }'

LINK_RE = re.compile(r'<link\b[^>]*>', re.IGNORECASE)
STYLE_RE = re.compile(r'<style\b[^>]*>(.*?)</style\s*>', re.IGNORECASE | re.DOTALL)
ATTR_RE = re.compile(r"""\b([\w:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')""")
FONT_FACE_RE = re.compile(r'@font-face\s*\{([^}]*)\}', re.IGNORECASE)
URL_RE = re.compile(r"""url\(\s*(?:"([^"]*)"|'([^']*)'|([^)'"\s]+))\s*\)""", re.IGNORECASE)


def find_chrome(explicit=None):
    for candidate in ([explicit] if explicit else [os.environ.get('EPUB_ART_CHROME'), *CHROME_CANDIDATES]):
        if not candidate:
            continue
        found = candidate if Path(candidate).is_file() else shutil.which(candidate)
        if found:
            return found
    raise ValueError('Chrome/Chromium/Edge not found; pass --chrome or set EPUB_ART_CHROME')


def parse_size(text):
    match = re.fullmatch(r'(\d+)x(\d+)', text)
    if not match:
        raise ValueError('size must be WIDTHxHEIGHT, e.g. 1264x1680')
    width, height = int(match[1]), int(match[2])
    if not (100 <= width <= 8000 and 100 <= height <= 8000):
        raise ValueError('size must be 100..8000 pixels per side')
    return width, height


def prepare(source, fill):
    """Return page text with a <base> pointing at the source folder and optional fill CSS."""
    text = source.read_text(encoding='utf-8')
    head = re.search(r'<head\b[^>]*>', text, re.IGNORECASE)
    if not head:
        raise ValueError('page has no <head> element')
    base = f'<base href="{source.resolve().parent.as_uri()}/"/>'
    text = text[:head.end()] + base + text[head.end():]
    if fill:
        close = re.search(r'</head\s*>', text, re.IGNORECASE)
        text = text[:close.start()] + f'<style type="text/css">{FILL_CSS}</style>' + text[close.start():]
    return text


def _local_target(base_dir, href):
    parts = urlsplit(href)
    if parts.scheme or parts.netloc or not parts.path:
        return None
    return (base_dir / unquote(parts.path)).resolve()


def missing_fonts(source):
    """Return @font-face files the page asks for but that are not on disk.

    A missing master makes the browser fall back to a system font, so the image
    would not show the book's typography. Run `epub book fonts` first.
    """
    text = source.read_text(encoding='utf-8')
    sheets = []  # (css text, directory its url() values resolve against)
    for tag in LINK_RE.findall(text):
        attrs = {m[0].lower(): m[1] or m[2] for m in ATTR_RE.findall(tag)}
        if 'stylesheet' not in attrs.get('rel', '').lower().split():
            continue
        target = _local_target(source.parent, attrs.get('href', ''))
        if target and target.is_file():
            sheets.append((target.read_text(encoding='utf-8', errors='replace'), target.parent))
    sheets += [(css, source.parent) for css in STYLE_RE.findall(text)]
    missing = []
    for css, base in sheets:
        for block in FONT_FACE_RE.findall(css):
            for m in URL_RE.finditer(block):
                target = _local_target(base, m[1] or m[2] or m[3] or '')
                if target and not target.exists() and str(target) not in missing:
                    missing.append(str(target))
    return missing


def webkit_helper():
    """Compile the WebKit snapshot helper once and cache it by source hash."""
    if sys.platform != 'darwin':
        raise ValueError('--engine webkit needs macOS')
    swift = Path(__file__).with_name('webkit_shot.swift')
    digest = hashlib.sha256(swift.read_bytes()).hexdigest()[:16]
    cache = Path(os.environ.get('XDG_CACHE_HOME') or Path.home() / 'Library' / 'Caches') / 'epub-art'
    binary = cache / f'webkit_shot-{digest}'
    if binary.is_file():
        return binary
    compiler = shutil.which('swiftc')
    if not compiler:
        raise ValueError('--engine webkit needs swiftc (install Xcode Command Line Tools)')
    cache.mkdir(parents=True, exist_ok=True)
    partial = binary.with_suffix('.tmp')
    run = subprocess.run([compiler, '-O', '-o', str(partial), str(swift)], capture_output=True, text=True, timeout=300)
    if run.returncode:
        raise ValueError('compiling the WebKit helper failed: ' + (run.stderr.strip().splitlines() or ['no output'])[-1])
    partial.replace(binary)
    return binary


def _shoot_chrome(browser, page, shot, width, height, scale, wait_ms, tmp):
    command = [browser, '--headless=new', '--disable-gpu', '--hide-scrollbars', '--no-first-run',
               '--no-default-browser-check', '--allow-file-access-from-files',
               f'--user-data-dir={Path(tmp) / "profile"}',
               f'--force-device-scale-factor={scale:g}',
               f'--window-size={int(width / scale)},{int(height / scale)}',
               f'--virtual-time-budget={wait_ms}',
               f'--screenshot={shot}', page.as_uri()]
    try:
        run = subprocess.run(command, capture_output=True, text=True, timeout=120)
    except subprocess.TimeoutExpired:
        raise ValueError('Chrome screenshot timed out after 120 s') from None
    if run.returncode or not shot.exists():
        raise ValueError('Chrome screenshot failed: ' + (run.stderr.strip().splitlines() or ['no output'])[-1])


def _shoot_webkit(helper, page, shot, width, height, scale, wait_ms):
    command = [str(helper), page.as_uri(), f'{width / scale:g}', f'{height / scale:g}',
               str(width), str(height), str(shot), f'{wait_ms / 1000:g}']
    try:
        run = subprocess.run(command, capture_output=True, text=True, timeout=wait_ms / 1000 + 60)
    except subprocess.TimeoutExpired:
        raise ValueError('WebKit screenshot timed out') from None
    if run.returncode or not shot.exists():
        raise ValueError('WebKit screenshot failed: ' + (run.stderr.strip().splitlines() or ['no output'])[-1])


def render(source, output, size, scale=2.0, fill=True, chrome=None, quality=90, wait_ms=3000, engine='chrome'):
    width, height = size
    if not 1 <= scale <= 4:
        raise ValueError('scale must be 1..4')
    if (width / scale) % 1 or (height / scale) % 1:
        raise ValueError('size must be divisible by scale')
    if output.suffix.lower() not in ('.jpg', '.jpeg', '.png'):
        raise ValueError('output must be .jpg, .jpeg or .png')
    if engine not in ('chrome', 'webkit'):
        raise ValueError('engine must be chrome or webkit')
    browser = webkit_helper() if engine == 'webkit' else find_chrome(chrome)
    warnings = ['Text in the image is not searchable or resizable; keep the title in alt and the nav.']
    missing = missing_fonts(source)
    if missing:
        warnings.insert(0, f'{len(missing)} font file(s) named in @font-face are missing, so a fallback font '
                           'was used; run `epub book fonts` first: ' + ', '.join(missing[:3]))
    with tempfile.TemporaryDirectory(prefix='epub-art-page-') as tmp:
        page = Path(tmp) / ('page' + (source.suffix.lower() if source.suffix.lower() in ('.xhtml', '.html', '.htm') else '.xhtml'))
        page.write_text(prepare(source, fill), encoding='utf-8')
        shot = Path(tmp) / 'shot.png'
        if engine == 'webkit':
            _shoot_webkit(browser, page, shot, width, height, scale, wait_ms)
        else:
            _shoot_chrome(browser, page, shot, width, height, scale, wait_ms, tmp)
        with Image.open(shot) as im:
            if im.size != (width, height):
                raise ValueError(f'{engine} returned {im.size[0]}x{im.size[1]}, expected {width}x{height}')
            image = im.convert('RGB')  # opaque on purpose: no alpha for Kindle conversion to drop
        if output.suffix.lower() == '.png':
            image.save(output, 'PNG', optimize=True)
        else:
            image.save(output, 'JPEG', quality=quality, optimize=True, progressive=False)
    return {'input': str(source), 'inputSHA256': hashlib.sha256(source.read_bytes()).hexdigest(),
            'output': str(output), 'outputSHA256': hashlib.sha256(output.read_bytes()).hexdigest(),
            'width': width, 'height': height, 'scale': scale, 'fill': fill, 'engine': engine, 'browser': str(browser),
            'missingFonts': missing, 'warnings': warnings}

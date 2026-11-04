"""Render one laid-out XHTML/HTML page to an opaque image with headless Chrome."""

import hashlib
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

from PIL import Image

CHROME_CANDIDATES = (
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
    '/Applications/Chromium.app/Contents/MacOS/Chromium',
    'google-chrome', 'google-chrome-stable', 'chromium', 'chromium-browser', 'microsoft-edge',
)
# Stretch the root boxes to the viewport so full-page backgrounds reach the bottom edge.
FILL_CSS = 'html, body { height: 100%; min-height: 100%; }'


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


def render(source, output, size, scale=2.0, fill=True, chrome=None, quality=90, wait_ms=3000):
    width, height = size
    if not 1 <= scale <= 4:
        raise ValueError('scale must be 1..4')
    if (width / scale) % 1 or (height / scale) % 1:
        raise ValueError('size must be divisible by scale')
    if output.suffix.lower() not in ('.jpg', '.jpeg', '.png'):
        raise ValueError('output must be .jpg, .jpeg or .png')
    browser = find_chrome(chrome)
    with tempfile.TemporaryDirectory(prefix='epub-art-page-') as tmp:
        page = Path(tmp) / ('page' + (source.suffix.lower() if source.suffix.lower() in ('.xhtml', '.html', '.htm') else '.xhtml'))
        page.write_text(prepare(source, fill), encoding='utf-8')
        shot = Path(tmp) / 'shot.png'
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
        with Image.open(shot) as im:
            if im.size != (width, height):
                raise ValueError(f'Chrome returned {im.size[0]}x{im.size[1]}, expected {width}x{height}')
            image = im.convert('RGB')  # opaque on purpose: no alpha for Kindle conversion to drop
        if output.suffix.lower() == '.png':
            image.save(output, 'PNG', optimize=True)
        else:
            image.save(output, 'JPEG', quality=quality, optimize=True, progressive=False)
    return {'input': str(source), 'inputSHA256': hashlib.sha256(source.read_bytes()).hexdigest(),
            'output': str(output), 'outputSHA256': hashlib.sha256(output.read_bytes()).hexdigest(),
            'width': width, 'height': height, 'scale': scale, 'fill': fill, 'browser': browser,
            'warnings': ['Text in the image is not searchable or resizable; keep the title in alt and the nav.']}

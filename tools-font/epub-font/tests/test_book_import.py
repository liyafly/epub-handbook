import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys

from tests import synth


def test_import_preserves_subset_fonts_and_freezes_input(tmp_path):
    root = Path(__file__).resolve().parents[3]
    provider = Path(__file__).resolve().parents[1]
    original = tmp_path / 'original.epub'
    original.write_bytes(synth.build_epub({
        'OEBPS/Fonts/st-all.ttf': synth.build_glyf_font(variable=False),
        'OEBPS/Fonts/st-all-semibold.ttf': synth.build_glyf_font(variable=False),
        'OEBPS/Fonts/kt.otf': synth.build_cff2_font(),
    }))
    fake = tmp_path / 'nav'
    fake.write_text('#!/bin/sh\nprintf \'{"findings":[],"status":"complete"}\\n\'\n')
    fake.chmod(0o755)
    font = tmp_path / 'font'
    font.write_text(f'#!/bin/sh\nexec {shlex.quote(sys.executable)} -m epub_font "$@"\n')
    font.chmod(0o755)
    env = os.environ | {'EPUB_BIN': str(fake), 'EPUB_FONT_BIN': str(font), 'PYTHONPATH': str(provider),
                        'GIT_CONFIG_COUNT': '2', 'GIT_CONFIG_KEY_0': 'user.name', 'GIT_CONFIG_VALUE_0': 'Fixture',
                        'GIT_CONFIG_KEY_1': 'user.email', 'GIT_CONFIG_VALUE_1': 'fixture@example.invalid'}
    destination = tmp_path / 'book'
    command = ['sh', str(root / 'templates/book-starter/import-book.sh'), str(original), str(destination)]
    result = subprocess.run(command, env=env, capture_output=True, text=True)
    assert result.returncode == 0, result.stdout + result.stderr
    frozen = destination / '01 源文件/original.epub'
    assert hashlib.sha256(frozen.read_bytes()).digest() == hashlib.sha256(original.read_bytes()).digest()
    config = json.loads((destination / '03 制作工作区/fonts.json').read_text())
    assert len(config['fonts']) == 3
    assert all('from' not in job for job in config['fonts'])
    assert (destination / '03 制作工作区/epub/OEBPS/Fonts/st-all.ttf').is_file()
    assert not (tmp_path / 'fonts').exists()
    assert subprocess.run(['git', '-C', str(destination), 'rev-parse', 'HEAD'], capture_output=True).returncode == 0
    assert subprocess.run(command, env=env, capture_output=True).returncode != 0

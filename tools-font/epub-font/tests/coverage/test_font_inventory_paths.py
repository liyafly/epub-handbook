import json

from epub_font import cli
from tests import synth


def test_inventory_keeps_exact_paths_and_same_family_variants(tmp_path, capsys):
    font = synth.build_glyf_font(variable=True)
    book = tmp_path / 'book.epub'
    paths = ['OEBPS/Fonts/one.ttf', 'OEBPS/Fonts/two.ttf']
    book.write_bytes(synth.build_epub({path: font for path in paths}))
    try:
        cli.main(['coverage', str(book), '--json'])
    except SystemExit as exc:
        assert exc.code in (0, 1)
    report = json.loads(capsys.readouterr().out)
    fonts = report['fonts']['embedded']
    assert sorted(row['file'] for row in fonts) == paths
    assert all(row['glyph_count'] == len(synth.GLYPH_ORDER) for row in fonts)
    assert all(row['variation_axes'] == {'wght': 400.0} for row in fonts)

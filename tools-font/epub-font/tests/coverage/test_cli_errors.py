from __future__ import annotations

from epub_font.coverage import cli


def test_input_errors_exit_two_without_traceback(tmp_path, capsys, monkeypatch):
    invalid_epub = tmp_path / "notzip.epub"
    invalid_epub.write_bytes(b"x")
    monkeypatch.delenv("EPUB_FONT_DEBUG", raising=False)

    try:
        cli.main([str(invalid_epub), "--json", "--quiet"])
    except SystemExit as exc:
        assert exc.code == 2
    else:
        raise AssertionError("invalid EPUB input should exit with code 2")

    captured = capsys.readouterr()
    assert captured.out == ""
    assert captured.err.startswith("error: ")
    assert len(captured.err.splitlines()) == 1
    assert "Traceback" not in captured.err


def test_input_errors_show_traceback_with_debug(tmp_path, capsys, monkeypatch):
    invalid_epub = tmp_path / "notzip.epub"
    invalid_epub.write_bytes(b"x")
    monkeypatch.setenv("EPUB_FONT_DEBUG", "1")

    try:
        cli.main([str(invalid_epub), "--json", "--quiet"])
    except SystemExit as exc:
        assert exc.code == 2
    else:
        raise AssertionError("invalid EPUB input should exit with code 2")

    assert "Traceback" in capsys.readouterr().err

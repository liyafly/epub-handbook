from epub_font import cli


def test_coverage_subcommand_dispatches_arguments(monkeypatch):
    calls = []

    def fake_coverage(argv):
        calls.extend(argv)
        return 0

    monkeypatch.setitem(cli.COMMANDS, "coverage", fake_coverage)

    assert cli.main(["coverage", "book.epub", "--json"]) == 0
    assert calls == ["book.epub", "--json"]

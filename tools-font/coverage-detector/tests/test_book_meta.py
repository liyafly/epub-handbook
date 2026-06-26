from src.cli import _annotate_book_meta


def test_book_meta_filled():
    book = {"opf_meta": {"title": "测试书"}}
    _annotate_book_meta(book, "/path/to/book.epub")
    assert book["title"] == "测试书"
    assert book["path"] == "/path/to/book.epub"


def test_book_meta_missing_title():
    book = {"opf_meta": {}}
    _annotate_book_meta(book, "x.epub")
    assert book["title"] == ""
    assert book["path"] == "x.epub"

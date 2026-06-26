from src.cli import _build_candidate_missing


def test_candidate_missing_marks_rare():
    results = [
        {"char": "的", "cp": "U+7684", "count": 5, "_rare": False},
        {"char": "\U00023AA0", "cp": "U+23AA0", "count": 1, "_rare": True},
    ]
    cmap = {0x7684}
    missing = _build_candidate_missing(results, cmap)
    assert len(missing) == 1
    assert missing[0]["char"] == "\U00023AA0"
    assert missing[0]["rare"] is True

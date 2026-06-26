from src.reporter import generate_report


def _mk(char, tier, zone, rare):
    return {"char": char, "cp": "U+0000", "count": 1, "coverage": {},
            "flags": [], "profiles": {},
            "_block_tier": tier, "_std_zone": zone, "_rare": rare}


def test_summary_block_tier_and_rare():
    inv = [
        _mk("的", "cjk-basic", "gb2312", False),
        _mk("龘", "cjk-basic", "gbk", False),
        _mk("𠀀", "cjk-ext-b-plus", "out-of-gbk", True),
    ]
    rep = generate_report(book={}, fonts={"embedded": []}, chains={},
                          char_inventory=inv, unresolved=[], standard_extra_name=None)
    s = rep["summary"]
    assert s["by_block_tier"]["cjk-basic"] == 2
    assert s["by_std_zone"]["gb2312"] == 1
    assert s["by_std_zone"]["out-of-gbk"] == 1
    assert s["rare_count"] == 1
    assert s["out_of_standard"] == 1
    assert rep["standard_zone"] == {"source": "gb2312+gbk", "extra": None}

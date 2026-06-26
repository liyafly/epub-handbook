from src.cli import _attach_tier
from src.charset_tiers import build_standard_charsets


def test_attach_tier_common():
    cs = build_standard_charsets()
    ci = {"char": "的"}
    _attach_tier(ci, cs, None)
    assert ci["_block_tier"] == "cjk-basic"
    assert ci["_std_zone"] == "gb2312"
    assert ci["_rare"] is False


def test_attach_tier_rare():
    cs = build_standard_charsets()
    ci = {"char": "\U00023AA0"}   # 𣪠, Ext B, not in GBK
    _attach_tier(ci, cs, None)
    assert ci["_std_zone"] == "out-of-gbk"
    assert ci["_rare"] is True

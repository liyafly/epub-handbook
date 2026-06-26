from src.charset_tiers import block_tier, is_cjk_tier


def test_block_tier_ranges():
    assert block_tier(ord("的")) == "cjk-basic"       # U+7684
    assert block_tier(0x3400) == "cjk-ext-a"          # Ext A start
    assert block_tier(0x20000) == "cjk-ext-b-plus"    # Ext B start
    assert block_tier(0x2A700) == "cjk-ext-b-plus"    # Ext C
    assert block_tier(0xF900) == "cjk-compat"         # Compat Ideographs
    assert block_tier(0x2F800) == "cjk-compat"        # Compat Supplement
    assert block_tier(0xE000) == "pua"
    assert block_tier(0xFE0F) == "vs"
    assert block_tier(ord("A")) == "non-cjk"
    assert block_tier(ord("，")) == "non-cjk"          # CJK punctuation is not a hanzi tier


def test_is_cjk_tier():
    assert is_cjk_tier("cjk-basic") is True
    assert is_cjk_tier("pua") is False
    assert is_cjk_tier("non-cjk") is False

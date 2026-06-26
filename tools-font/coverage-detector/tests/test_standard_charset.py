from src.charset_tiers import build_standard_charsets, classify_tier, standard_zone, StandardCharset


def test_codec_charsets_counts():
    cs = build_standard_charsets()
    gb_hanzi = [c for c in cs["gb2312"] if 0x4E00 <= c <= 0x9FFF]
    assert len(gb_hanzi) == 6763
    assert cs["gb2312"] <= cs["gbk"]                 # GB2312 ⊂ GBK
    assert ord("的") in cs["gb2312"]
    assert ord("龘") in cs["gbk"] and ord("龘") not in cs["gb2312"]
    assert 0x23AA0 not in cs["gbk"]                  # 𣪠 (Ext B) not in GBK


def test_standard_zone_levels():
    cs = build_standard_charsets()
    assert standard_zone(ord("的"), cs) == "gb2312"
    assert standard_zone(ord("龘"), cs) == "gbk"
    assert standard_zone(0x23AA0, cs) == "out-of-gbk"   # 𣪠
    assert standard_zone(ord("，"), cs) is None          # punctuation, not hanzi


def test_classify_tier_rare():
    cs = build_standard_charsets()
    assert classify_tier(ord("的"), cs)["is_rare"] is False
    assert classify_tier(ord("龘"), cs)["is_rare"] is False   # in GBK → not rare
    t = classify_tier(0x23AA0, cs)                            # 𣪠 → rare
    assert t["std_zone"] == "out-of-gbk" and t["is_rare"] is True
    assert classify_tier(0xE000, cs)["is_rare"] is True       # PUA


def test_optional_extra_table():
    cs = build_standard_charsets()
    extra = StandardCharset(name="x", codepoints=frozenset({0x23AA0}))
    assert standard_zone(0x23AA0, cs, extra) == "gbk"
    assert classify_tier(0x23AA0, cs, extra)["is_rare"] is False

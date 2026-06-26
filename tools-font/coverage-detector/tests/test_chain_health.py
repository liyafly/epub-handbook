from dataclasses import dataclass
from src.chain_health import assess_chains


@dataclass
class Seg:
    family: str
    embedded: bool = False
    file: str | None = None
    generic: bool = False
    defaulted: bool = False
    system_ref: bool = False


def test_defaulted_chain_is_fail():
    chains = {"c1": [Seg("serif", generic=True, defaulted=True)]}
    res = assess_chains(chains, font_index={}, run_chains={0: "c1"})
    assert "no-font-family-declared" in res["c1"]["issues"]
    assert res["c1"]["grade"] == "fail"
    assert res["c1"]["run_count"] == 1


def test_full_font_not_at_head():
    chains = {"c1": [
        Seg("Sub", embedded=True, file="sub.ttf"),
        Seg("Big", embedded=True, file="big.otf"),
    ]}
    fi = {"sub.ttf": {"subset": True}, "big.otf": {"subset": False}}
    res = assess_chains(chains, fi, run_chains={})
    assert "full-font-not-at-head" in res["c1"]["issues"]
    assert res["c1"]["grade"] == "fail"


def test_clean_chain_ok():
    chains = {"c1": [Seg("Big", embedded=True, file="big.otf"), Seg("serif", generic=True)]}
    fi = {"big.otf": {"subset": False}}
    res = assess_chains(chains, fi, run_chains={})
    assert res["c1"]["issues"] == []
    assert res["c1"]["grade"] == "ok"
    assert res["c1"]["head_is_full_font"] is True


def test_broken_font_face():
    chains = {"c1": [Seg("Ghost", embedded=False, file="missing.ttf")]}
    res = assess_chains(chains, font_index={}, run_chains={})
    assert "broken-font-face" in res["c1"]["issues"]


def test_system_ref_not_broken():
    # 坟 pattern: @font-face → reader system font; system_ref=True, file=None.
    chains = {"c1": [
        Seg("仿宋", embedded=False, file=None, system_ref=True),
        Seg("serif", generic=True),
    ]}
    res = assess_chains(chains, font_index={}, run_chains={})
    assert "broken-font-face" not in res["c1"]["issues"]
    assert res["c1"]["grade"] == "ok"


def test_generic_only():
    chains = {"c1": [Seg("serif", generic=True)]}
    res = assess_chains(chains, font_index={}, run_chains={})
    assert "generic-only" in res["c1"]["issues"]
    assert res["c1"]["grade"] == "warn"


def test_no_generic_fallback():
    chains = {"c1": [Seg("ls", embedded=True, file="xls.ttf")]}
    fi = {"xls.ttf": {"subset": True}}
    res = assess_chains(chains, fi, run_chains={})
    assert "no-generic-fallback" in res["c1"]["issues"]
    assert res["c1"]["grade"] == "warn"


def test_subset_with_generic_tail_ok():
    # 坟 pattern: rare-char subset → generic serif. Legit, not flagged.
    chains = {"c1": [Seg("kxs", embedded=True, file="rare.ttf"), Seg("serif", generic=True)]}
    fi = {"rare.ttf": {"subset": True}}
    res = assess_chains(chains, fi, run_chains={})
    assert "subset-only" not in res["c1"]["issues"]
    assert res["c1"]["grade"] == "ok"

from src.resolver import resolve_chains


def test_defaulted_chain_flagged():
    # A run with no font-family anywhere → defaulted serif segment.
    runs = [{"ancestor_chain": [{"tag": "p"}], "inline_style": ""}]
    chains, _unresolved, run_chains = resolve_chains(runs, css_files=[], font_face_registry={}, font_files=[])
    cid = run_chains[0]
    seg = chains[cid][0]
    assert seg.family == "serif"
    assert seg.defaulted is True


def test_explicit_serif_not_flagged():
    runs = [{"ancestor_chain": [{"tag": "p"}], "inline_style": "font-family: serif"}]
    chains, _unresolved, run_chains = resolve_chains(runs, css_files=[], font_face_registry={}, font_files=[])
    seg = chains[run_chains[0]][0]
    assert seg.family == "serif"
    assert seg.defaulted is False

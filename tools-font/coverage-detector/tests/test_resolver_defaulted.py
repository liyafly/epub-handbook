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


def test_universal_selector_is_resolved_without_false_warning():
    runs = [{"ancestor_chain": [{"tag": "html"}, {"tag": "body"}, {"tag": "p"}], "inline_style": ""}]
    css_files = [{"id": "base", "content_bytes": b'* { font-family: "Base Serif", serif; }'}]
    chains, unresolved, run_chains = resolve_chains(
        runs,
        css_files=css_files,
        font_face_registry={},
        font_files=[],
    )
    assert unresolved == []
    assert [segment.family for segment in chains[run_chains[0]]] == ["Base Serif", "serif"]

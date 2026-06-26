from src.resolver import resolve_chains

CSS = [{
    "id": "main", "href": "main.css",
    "content_bytes": (
        '.kxs { font-family: "kxs", Serif; }\n'
        'li.duokan-footnote-item p { font-family: "eng", "仿宋", sans-serif; }\n'
    ).encode("utf-8"),
}]


def test_closer_span_class_wins_over_farther_descendant_rule():
    # ruby text inside: li.duokan-footnote-item > p.footnote > span.kxs > ruby
    runs = [{
        "ancestor_chain": [
            {"tag": "li", "class": "duokan-footnote-item"},
            {"tag": "p", "class": "footnote"},
            {"tag": "span", "class": "kxs"},
            {"tag": "ruby"},
        ],
        "inline_style": "",
    }]
    chains, _u, run_chains = resolve_chains(runs, CSS, font_face_registry={}, font_files=[])
    fams = [s.family for s in chains[run_chains[0]]]
    assert fams[0] == "kxs"          # closer .kxs wins, not the footnote-p rule
    assert "仿宋" not in fams

from src.resolver import resolve_chains, build_font_face_registry

# @font-face whose src points at reader/device system fonts (res:/// + missing pkg path)
CSS = [{
    "id": "fonts", "href": "fonts.css",
    "content_bytes": (
        '@font-face { font-family: "仿宋"; '
        'src: url("../Fonts/FangSong.ttf"), url(res:///system/fonts/fs.ttf); }\n'
        'p { font-family: "仿宋", serif; }\n'
    ).encode("utf-8"),
}]


def test_system_ref_not_broken():
    reg = build_font_face_registry(CSS)
    assert reg["仿宋"]["system_ref"] is True
    runs = [{"ancestor_chain": [{"tag": "p"}], "inline_style": ""}]
    chains, _u, rc = resolve_chains(runs, CSS, reg, font_files=[])   # FangSong.ttf not in pkg
    seg = [s for s in chains[rc[0]] if s.family == "仿宋"][0]
    assert seg.embedded is False
    assert seg.system_ref is True
    assert seg.file is None        # no misleading hint → chain_health won't call it broken

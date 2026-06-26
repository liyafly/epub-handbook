from src.cli import _build_char_inventory


def test_inventory_keeps_all_occurrences_with_paths():
    runs = [
        {"text": "龘字在此龘", "file": "ch01.xhtml", "node_path": "/html/body/p[1]/text()[1]", "offset": 100},
        {"text": "又见龘", "file": "ch02.xhtml", "node_path": "/html/body/p[3]/text()[1]", "offset": 5},
    ]
    inv = _build_char_inventory(runs)
    rare = [c for c in inv if c["char"] == "龘"][0]
    assert rare["count"] == 3
    assert len(rare["occurrences"]) == 3
    occ = rare["occurrences"][0]
    assert set(occ) == {"file", "node_path", "offset", "context"}
    assert occ["node_path"] == "/html/body/p[1]/text()[1]"
    assert occ["offset"] == 100


def test_inventory_caps_occurrences():
    runs = [{"text": "的" * 5000, "file": "f", "node_path": "p", "offset": 0}]
    inv = _build_char_inventory(runs)
    assert inv[0]["count"] == 5000
    assert len(inv[0]["occurrences"]) == 1000

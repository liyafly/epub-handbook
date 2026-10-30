import hashlib
import json

from epub_font import inject


def setup_config(tmp_path, jobs=None):
    library = tmp_path / "fonts"
    library.mkdir()
    raw = b"font master bytes"
    (library / "master.ttf").write_bytes(raw)
    config = tmp_path / "book" / "03 制作工作区" / "fonts.json"
    config.parent.mkdir(parents=True)
    entries = jobs or [{"target": "OEBPS/Fonts/st.ttf", "from": "master.ttf",
                        "sha256": hashlib.sha256(raw).hexdigest()}]
    config.write_text(json.dumps({"version": 1, "fonts": entries}))
    tree = config.parent / "epub"
    tree.mkdir()
    return library, config, tree


def test_inject_finds_library_and_preserves_legacy_jobs(tmp_path):
    library, config, tree = setup_config(tmp_path)
    assert inject.main(["--config", str(config), "--tree", str(tree)]) == 0
    assert (tree / "OEBPS/Fonts/st.ttf").read_bytes() == (library / "master.ttf").read_bytes()


def test_inject_validates_all_fonts_before_copying(tmp_path, capsys):
    library, config, tree = setup_config(tmp_path)
    doc = json.loads(config.read_text())
    doc["fonts"].append({"target": "OEBPS/Fonts/missing.ttf", "from": "missing.ttf", "sha256": "a" * 64})
    config.write_text(json.dumps(doc))
    assert inject.main(["--config", str(config), "--tree", str(tree)]) == 2
    assert not (tree / "OEBPS").exists()
    assert "missing.ttf" in capsys.readouterr().err


def test_inject_sha_mismatch_and_explicit_update(tmp_path, capsys):
    library, config, tree = setup_config(tmp_path)
    (library / "master.ttf").write_bytes(b"new master")
    args = ["--config", str(config), "--tree", str(tree)]
    assert inject.main(args) == 2
    assert "expected" in capsys.readouterr().err
    assert inject.main(args + ["--update-lock"]) == 0
    assert json.loads(config.read_text())["fonts"][0]["sha256"] == hashlib.sha256(b"new master").hexdigest()


def test_inject_rejects_traversal_and_symlink(tmp_path):
    library, config, tree = setup_config(tmp_path)
    (tree / "OEBPS").symlink_to(tmp_path, target_is_directory=True)
    assert inject.main(["--config", str(config), "--tree", str(tree)]) == 2
    (tree / "OEBPS").unlink()
    doc = json.loads(config.read_text())
    doc["fonts"][0]["target"] = "../escape.ttf"
    config.write_text(json.dumps(doc))
    assert inject.main(["--config", str(config), "--tree", str(tree)]) == 2


def test_environment_library_is_authoritative(tmp_path, monkeypatch):
    _, config, tree = setup_config(tmp_path)
    monkeypatch.setenv("EPUB_FONT_LIBRARY", str(tmp_path / "absent"))
    assert inject.main(["--config", str(config), "--tree", str(tree)]) == 2

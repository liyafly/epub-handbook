import os.path, subprocess, sys, json

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DEMO = os.path.normpath(os.path.join(
    ROOT, "..", "..", "templates", "epub-style-demo", "dist",
    "epub-style-demo-20260602-162129.epub"))


def test_end_to_end_report_shape(tmp_path):
    if not os.path.exists(DEMO):
        import pytest; pytest.skip("demo epub not present")
    out = tmp_path / "r.json"
    subprocess.run([sys.executable, "-m", "src.cli", DEMO, "-o", str(out), "-q"],
                   cwd=ROOT, check=True)
    rep = json.loads(out.read_text(encoding="utf-8"))
    assert {"book", "summary", "char_inventory", "chain_health", "standard_zone"} <= set(rep)
    assert rep["standard_zone"]["source"] == "gb2312+gbk"
    assert rep["book"]["title"]
    s = rep["summary"]
    assert {"by_block_tier", "by_std_zone", "rare_count", "out_of_standard"} <= set(s)
    ci = rep["char_inventory"][0]
    assert {"_causes", "_covered_by", "_block_tier", "_std_zone", "_rare",
            "_pos_best", "_has_embedded_cover"} <= set(ci)
    assert all(set(o) >= {"file", "node_path", "offset", "context"} for o in ci["occurrences"])
    html = (tmp_path / "r.html").read_text(encoding="utf-8")
    assert "async function loadFontFile(blob,fname){" in html
    # data injected before main script (auto-load)
    assert html.find("window.__REPORT_DATA__=JSON.parse") < html.find("let report=null")

from src.classifier import classify


class Seg:
    def __init__(self, family, embedded, file):
        self.family = family
        self.embedded = embedded
        self.file = file
        self.generic = False


def test_embedded_attribution_by_path():
    # The font's internal family ("untitled") is unrelated to the CSS family
    # ("kxs"). With a PATH-keyed index the rare char must resolve as embedded.
    font_index = {"OEBPS/Fonts/rarefont.ttf": {
        "cmap": {0x23AA0}, "subset": True, "family": "untitled",
    }}
    chains = {"c1": [Seg("kxs", True, "OEBPS/Fonts/rarefont.ttf")]}
    chars = [{"char": "\U00023AA0", "count": 1, "runs": [0], "occurrences": []}]
    res = classify(chars, font_index, chains, {0: "c1"})
    cov = res[0].coverage["c1"]
    assert cov["position"] == "first-embedded"
    assert cov["covered_by"] == "kxs"
    assert cov["cause"] is None

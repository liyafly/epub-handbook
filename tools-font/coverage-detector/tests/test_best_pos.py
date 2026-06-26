from src.cli import _attach_position_aggregates

WORST_ORDER = {"first-embedded": 0, "later-embedded": 1, "only-non-embedded": 2, "none": 3}


def test_best_and_has_cover():
    ci = {"coverage": {
        "c1": {"position": "first-embedded"},
        "c2": {"position": "only-non-embedded"},
    }}
    _attach_position_aggregates(ci, WORST_ORDER)
    assert ci["_pos"] == "only-non-embedded"      # worst, kept for risk
    assert ci["_pos_best"] == "first-embedded"    # best
    assert ci["_has_embedded_cover"] is True      # covered somewhere


def test_truly_uncovered():
    ci = {"coverage": {"c1": {"position": "only-non-embedded"}}}
    _attach_position_aggregates(ci, WORST_ORDER)
    assert ci["_has_embedded_cover"] is False

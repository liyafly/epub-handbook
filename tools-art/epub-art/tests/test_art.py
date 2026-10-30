from pathlib import Path

import pytest
from PIL import Image, ImageDraw
from epub_art import images, shapes


def area(points):
    return abs(sum(a[0] * b[1] - a[1] * b[0] for a, b in zip(points, points[1:] + points[:1]))) / 2


@pytest.mark.parametrize('scene', ['01-l-left-bottom', '02-l-left-top', '05-cat-left', '07-wave-left', '06-cat-right', '08-wave-right'])
def test_polygon_area_matches_handwritten_v3(scene):
    root = Path(__file__).resolve().parents[3] / 'templates/shape-wrap-demo'
    # The committed CSS is the authoritative handwritten synthetic outline.
    import re
    index = int(scene[:2])
    text = (root / 'src-v3/OEBPS/Styles/shape.css').read_text()
    handwritten = re.search(r'\.s' + str(index) + r' \{ shape-outside: polygon\((.*?)\)', text)[1]
    points = [[float(value.strip()[:-1]) for value in pair.split()] for pair in handwritten.split(',')]
    result = shapes.polygon(root / f'src-v3/OEBPS/Images/{scene}.png', side='right' if 'right' in scene else 'left')
    assert abs(area(result['polygon']) - area(points)) / area(points) <= .05
    assert result['referenceBox'] == 'content-box'
    assert len(result['polygon']) <= 40


def test_transparency_protects_enclosed_white_and_mask():
    image = Image.new('RGB', (20, 20), 'white')
    ImageDraw.Draw(image).rectangle((5, 5, 14, 14), outline='black')
    result = images.transparent(image)
    assert result.getpixel((0, 0))[3] == 0
    assert result.getpixel((10, 10)) == (255, 255, 255, 255)
    mask = Image.new('L', image.size)
    mask.putpixel((0, 0), 255)
    assert images.transparent(image, protect=mask).getpixel((0, 0))[3] == 255


def test_white_matte_recomposes_on_white():
    image = Image.new('RGB', (3, 1))
    image.putdata([(255, 255, 255), (128, 128, 128), (140, 170, 200)])
    result = images.transparent(image, method='white-matte')
    white = Image.new('RGBA', image.size, 'white')
    assert Image.alpha_composite(white, result).convert('RGB').tobytes() == image.tobytes()

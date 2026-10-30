"""Explicit, reversible image operations; never rewrite an input file."""

from collections import deque
from PIL import Image


def audit(path):
    with Image.open(path) as im:
        alpha = im.convert('RGBA').getchannel('A')
        histogram = alpha.histogram()
        width, height = im.size
        warnings = []
        if im.mode not in ('RGB', 'RGBA', 'L', 'LA', 'P'):
            warnings.append('Convert the color space deliberately before EPUB use: ' + im.mode)
        if width / height > 8 or height / width > 8:
            warnings.append('Extreme aspect ratio; inspect narrow-screen pagination')
        if width * height > 40_000_000:
            warnings.append('Large pixel count; inspect memory and delivery size')
        return {'width': width, 'height': height, 'aspectRatio': width / height, 'mode': im.mode,
                'format': im.format, 'embeddedICC': bool(im.info.get('icc_profile')),
                'transparentPixels': histogram[0], 'partiallyTransparentPixels': sum(histogram[1:255]),
                'opaquePixels': histogram[255], 'warnings': warnings}


def transparent(image, threshold=248, method='exterior', protect=None):
    if image.mode not in ('RGB', 'RGBA', 'L', 'LA', 'P'):
        raise ValueError('Convert non-RGB color space deliberately before removing the background')
    rgba = image.convert('RGBA')
    width, height = rgba.size
    pixels = list(rgba.get_flattened_data() if hasattr(rgba, 'get_flattened_data') else rgba.getdata())
    if protect is not None and protect.size != rgba.size:
        raise ValueError('protection mask dimensions differ from the source')
    if protect:
        mask = protect.convert('L')
        protection = list(mask.get_flattened_data() if hasattr(mask, 'get_flattened_data') else mask.getdata())
    else:
        protection = [0] * len(pixels)
    exterior = bytearray(len(pixels))
    queue = deque()
    def add(index):
        r, g, b, _ = pixels[index]
        if not exterior[index] and not protection[index] and min(r, g, b) >= threshold and max(r, g, b) - min(r, g, b) <= 20:
            exterior[index] = 1
            queue.append(index)
    for x in range(width):
        add(x); add((height - 1) * width + x)
    for y in range(height):
        add(y * width); add(y * width + width - 1)
    while queue:
        index = queue.popleft(); x, y = index % width, index // width
        if x: add(index - 1)
        if x + 1 < width: add(index + 1)
        if y: add(index - width)
        if y + 1 < height: add(index + width)
    output = []
    for index, (r, g, b, alpha) in enumerate(pixels):
        if protection[index]:
            output.append((r, g, b, alpha))
        elif method == 'white-matte':
            matte = min(r, g, b); a = 255 - matte
            rgb = tuple(min(255, (255 * (c - matte) + a // 2) // a) if a else 0 for c in (r, g, b))
            output.append((*rgb, (alpha * a + 127) // 255))
        else:
            output.append((r, g, b, 0 if exterior[index] else alpha))
    result = Image.new('RGBA', rgba.size)
    result.putdata(output)
    return result

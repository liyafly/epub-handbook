"""Conservative one-sided alpha envelopes in image-content coordinates."""

import hashlib
import math

from PIL import Image, ImageFilter


def simplify(points, tolerance):
    if len(points) <= 2:
        return points
    a, b = points[0], points[-1]
    dx, dy = b[0] - a[0], b[1] - a[1]
    length = math.hypot(dx, dy)
    distances = [abs(dy * (p[0] - a[0]) - dx * (p[1] - a[1])) / length if length else math.dist(p, a) for p in points[1:-1]]
    greatest = max(distances, default=0)
    if greatest <= tolerance:
        return [a, b]
    index = distances.index(greatest) + 1
    return simplify(points[:index + 1], tolerance)[:-1] + simplify(points[index:], tolerance)


def polygon(path, side='left', threshold=16, padding=1, tolerance=1.5):
    with Image.open(path) as image:
        if image.format != 'PNG' or 'A' not in image.getbands() and 'transparency' not in image.info:
            raise ValueError('shape requires a PNG with an alpha channel')
        alpha = image.convert('RGBA').getchannel('A')
        width, height = image.size
        if width * height > 40_000_000:
            raise ValueError('shape input exceeds 40 million pixels; prepare a smaller planning image')
    mask = alpha.point(lambda a: 255 if a >= threshold else 0)
    if padding:
        mask = mask.filter(ImageFilter.MaxFilter(2 * padding + 1))
    pixels = mask.load()
    points = []
    for y in range(height):
        occupied = [x for x in range(width) if pixels[x, y]]
        if occupied:
            x = max(occupied) + 1 if side == 'left' else min(occupied)
        else:
            x = 0 if side == 'left' else width
        points.extend([(x, y), (x, y + 1)])
    if not mask.getbbox():
        raise ValueError('image has no pixels above the alpha threshold')
    edge = simplify(points, tolerance)
    # RDP can cut into an ink boundary. Expand the simplified edge by its
    # greatest inward interpolation error; every original row stays enclosed.
    loss = 0
    indices = []
    cursor = 0
    for point in edge:
        while points[cursor] != point:
            cursor += 1
        indices.append(cursor)
        cursor += 1
    for a, b, start, end in zip(edge, edge[1:], indices, indices[1:]):
        if a[1] == b[1]:
            continue
        for x, y in points[start:end + 1]:
            if a[1] <= y <= b[1]:
                predicted = a[0] + (b[0] - a[0]) * (y - a[1]) / (b[1] - a[1])
                loss = max(loss, x - predicted if side == 'left' else predicted - x)
    shift = loss + .01
    edge = [(min(width, x + shift) if side == 'left' else max(0, x - shift), y) for x, y in edge]
    border = 0 if side == 'left' else width
    outline = [(border, 0), *edge, (border, height)]
    percent = []
    for x, y in outline:
        point = [round(x / width * 100, 5), round(y / height * 100, 5)]
        if not percent or percent[-1] != point:
            percent.append(point)
    return {'inputSHA256': hashlib.sha256(path.read_bytes()).hexdigest(), 'width': width, 'height': height,
            'float': side, 'referenceBox': 'content-box', 'polygon': percent,
            'parameters': {'alphaThreshold': threshold, 'paddingPixels': padding, 'tolerancePixels': tolerance},
            'warnings': ['More than 40 points; inspect before increasing simplification tolerance'] if len(percent) > 40 else []}


def css(result, selector):
    coordinates = ', '.join(f'{x:g}% {y:g}%' for x, y in result['polygon'])
    return f'{selector} {{ shape-outside: polygon({coordinates}) content-box; shape-margin: .45em; }}\n'

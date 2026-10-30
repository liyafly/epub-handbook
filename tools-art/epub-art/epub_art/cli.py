import argparse
import hashlib
import json
from pathlib import Path
import re

from PIL import Image
from . import images, shapes


def destination(path, source, force):
    if path.resolve() == source.resolve() or path.is_symlink():
        raise ValueError('output must be a new regular path, distinct from the input')
    if path.exists() and not force:
        raise ValueError(f'output already exists: {path}')
    path.parent.mkdir(parents=True, exist_ok=True)


def main(argv=None):
    parser = argparse.ArgumentParser(prog='epub-art')
    sub = parser.add_subparsers(dest='command', required=True)
    for name in ('shape', 'transparent', 'audit'):
        p = sub.add_parser(name)
        p.add_argument('input', type=Path)
        p.add_argument('--output', type=Path, required=name == 'transparent')
        p.add_argument('--force', action='store_true')
        if name == 'shape':
            p.add_argument('--float', dest='side', choices=['left', 'right'], default='left')
            p.add_argument('--threshold', type=int, default=16)
            p.add_argument('--padding', type=int, default=1)
            p.add_argument('--tolerance', type=float, default=1.5)
            p.add_argument('--selector', default='figure.outline')
            p.add_argument('--css', type=Path)
        if name == 'transparent':
            p.add_argument('--threshold', type=int, default=248)
            p.add_argument('--method', choices=['exterior', 'white-matte'], default='exterior')
            p.add_argument('--protect-mask', type=Path)
    args = parser.parse_args(argv)
    try:
        if args.output:
            destination(args.output, args.input, args.force)
        if args.command == 'shape':
            if not 1 <= args.threshold <= 255 or not 0 <= args.padding <= 32 or not 0 <= args.tolerance <= 50:
                raise ValueError('threshold 1..255, padding 0..32 and tolerance 0..50 are required')
            if not re.fullmatch(r'(?:[A-Za-z][\w-]*)?(?:[.#][\w-]+)+', args.selector):
                raise ValueError('selector must be a simple type/class/id selector')
            if args.css:
                destination(args.css, args.input, args.force)
                if args.output and args.css.resolve() == args.output.resolve():
                    raise ValueError('CSS and JSON outputs must differ')
            result = shapes.polygon(args.input, args.side, args.threshold, args.padding, args.tolerance)
            result['css'] = shapes.css(result, args.selector)
            if args.css:
                args.css.write_text(result['css'])
        elif args.command == 'transparent':
            if args.output.suffix.lower() != '.png' or not 0 <= args.threshold <= 255:
                raise ValueError('transparent output must be PNG; threshold must be 0..255')
            with Image.open(args.input) as im:
                mask = Image.open(args.protect_mask) if args.protect_mask else None
                try:
                    out = images.transparent(im, args.threshold, args.method, mask)
                finally:
                    if mask: mask.close()
                out.save(args.output, 'PNG')
            result = images.audit(args.output)
            result.update(inputSHA256=hashlib.sha256(args.input.read_bytes()).hexdigest(), outputSHA256=hashlib.sha256(args.output.read_bytes()).hexdigest(), method=args.method)
        else:
            result = images.audit(args.input)
        text = json.dumps(result, ensure_ascii=False, indent=2) + '\n'
        if args.output and args.command != 'transparent':
            args.output.write_text(text)
        print(text, end='')
        return 0
    except (OSError, ValueError) as exc:
        parser.exit(2, f'error: {exc}\n')

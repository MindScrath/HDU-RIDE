#!/usr/bin/env python3
"""Wrap unaligned LaTeX display-math blocks in \begin{aligned}...\end{aligned}."""

import re, sys

ALIGNED_ENVS = {
    'aligned', 'align', 'alignat', 'gathered', 'split',
    'cases', 'array', 'matrix', 'pmatrix', 'bmatrix', 'Bmatrix',
    'vmatrix', 'Vmatrix', 'smallmatrix',
}

def has_env(body: str) -> bool:
    for env in ALIGNED_ENVS:
        if f'\\begin{{{env}}}' in body:
            return True
    return False

def fix_block(m: re.Match) -> str:
    body = m.group(1)
    if has_env(body):
        return m.group(0)
    if '&' not in body and '\\\\' not in body:
        return m.group(0)
    return f'$$\n\\begin{{aligned}}\n{body.strip()}\n\\end{{aligned}}\n$$'

def fix_file(path: str) -> int:
    with open(path, 'r', encoding='utf-8') as f:
        text = f.read()

    # Match $$ ... $$ blocks (non-greedy across lines)
    fixed = re.sub(r'\$\$\s*(.+?)\s*\$\$', fix_block, text, flags=re.DOTALL)

    if fixed == text:
        return 0

    with open(path, 'w', encoding='utf-8') as f:
        f.write(fixed)
    return 1

if __name__ == '__main__':
    for p in sys.argv[1:]:
        changed = fix_file(p)
        print(f'{"FIXED" if changed else "OK"}: {p}')

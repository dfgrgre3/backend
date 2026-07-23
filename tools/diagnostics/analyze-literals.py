#!/usr/bin/env python3
"""Report frequently repeated SQL string literals in a migration file."""

import argparse
import re
from collections import Counter
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("migration", type=Path, help="SQL migration file to inspect")
    parser.add_argument("--minimum", type=int, default=5, help="minimum count to report (default: 5)")
    args = parser.parse_args()

    literals = re.findall(r"'([a-zA-Z_]+)'", args.migration.read_text(encoding="utf-8"))
    for literal, count in Counter(literals).most_common():
        if count >= args.minimum:
            print(f"'{literal}': {count} occurrences")


if __name__ == "__main__":
    main()

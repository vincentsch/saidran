#!/usr/bin/env python3
"""Print the changelog section matching a release tag."""
import pathlib
import re
import sys

version = sys.argv[1].removeprefix("v")
text = pathlib.Path("CHANGELOG.md").read_text()
for section in re.split(r"(?m)^## ", text)[1:]:
    heading, _, body = section.partition("\n")
    if re.match(r"\[?" + re.escape(version) + r"(?:\]|\s|$)", heading):
        if "unreleased" in heading.lower():
            sys.exit("The changelog section is unreleased. Add a release date before tagging.")
        if not body.strip():
            sys.exit("The release changelog section is empty.")
        print(body.strip())
        break
else:
    sys.exit(f"No changelog section found for {version}.")

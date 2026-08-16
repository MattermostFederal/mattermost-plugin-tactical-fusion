#!/usr/bin/env python3
"""Rewrite a GeoJSON file keeping geometry and dropping every attribute.

The style draws land, lakes and boundary lines from geometry alone, so nothing
downstream reads their attributes. Two reasons to remove them rather than let
tippecanoe carry them into every tile:

Size. Natural Earth's lakes carry 37 fields, most of them name translations, and
its boundary lines carry 38.

Viewpoint. Those boundary-line fields include 34 FCLASS_* columns, which are
Natural Earth's per-country classifications of disputed boundaries. This plugin
takes no position on those, and the way to take no position is to not ship the
data rather than to ship it unread.
"""
import json
import sys


def main(src: str, dst: str) -> int:
    with open(src, encoding="utf-8") as handle:
        collection = json.load(handle)

    dropped = set()
    for feature in collection["features"]:
        dropped.update(feature.get("properties") or {})
        feature["properties"] = {}

    with open(dst, "w", encoding="utf-8") as handle:
        json.dump(collection, handle, separators=(",", ":"))

    print(f"{dst}: {len(collection['features'])} features, dropped {len(dropped)} fields")
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("usage: strip.py <in.geojson> <out.geojson>", file=sys.stderr)
        raise SystemExit(2)
    raise SystemExit(main(sys.argv[1], sys.argv[2]))

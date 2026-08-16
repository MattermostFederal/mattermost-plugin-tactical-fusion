#!/usr/bin/env python3
"""Rewrite a GeoJSON file keeping geometry and only the named attributes.

The rule is: if the style does not read it, it does not ship. Two reasons to
apply that rather than letting tippecanoe carry every field into every tile:

Size. Natural Earth carries around 40 fields on most layers, the bulk of them
name translations. Stripping them cut the first archive by 45%.

Viewpoint. Among the fields on the boundary layers are 34 FCLASS_* columns,
which are Natural Earth's per-country classifications of disputed boundaries.
This plugin takes no position on those, and the way to take no position is to
not ship the data rather than to ship it unread. The same test applies to
anything kept here: name the field because a layer draws with it, not because
it might be useful later.

Fields are matched case-insensitively, because Natural Earth is inconsistent
about it between layers and between scales: populated places carries SCALERANK
while roads carries scalerank.
"""
import json
import sys


def main(src: str, dst: str, keep: list[str]) -> int:
    wanted = {k.lower() for k in keep}

    with open(src, encoding="utf-8") as handle:
        collection = json.load(handle)

    seen: set[str] = set()
    found: set[str] = set()

    for feature in collection["features"]:
        props = feature.get("properties") or {}
        seen.update(props)

        kept = {}
        for name, value in props.items():
            if name.lower() in wanted and value is not None and value != "":
                kept[name.lower()] = value
                found.add(name.lower())

        feature["properties"] = kept

    with open(dst, "w", encoding="utf-8") as handle:
        json.dump(collection, handle, ensure_ascii=False, separators=(",", ":"))

    missing = wanted - found
    if missing:
        print(f"error: {src} has no {sorted(missing)}; "
              f"available fields are {sorted(seen)}", file=sys.stderr)
        return 1

    print(f"{dst}: {len(collection['features'])} features, "
          f"kept {sorted(wanted) or 'nothing'} of {len(seen)} fields")
    return 0


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("usage: strip.py <in.geojson> <out.geojson> [field ...]", file=sys.stderr)
        raise SystemExit(2)
    raise SystemExit(main(sys.argv[1], sys.argv[2], sys.argv[3:]))

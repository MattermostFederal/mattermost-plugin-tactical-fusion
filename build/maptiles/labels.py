#!/usr/bin/env python3
"""Derive country label points from a Natural Earth admin-0 polygon set.

LABEL_X/LABEL_Y are Natural Earth's own cartographic anchors, which is why they
are read rather than a centroid being computed: Norway's centroid falls in
Sweden, and several countries' fall in the sea.

Every admin-0 feature gets a label. There is deliberately no LABELRANK cutoff:
the only feature above rank 6 is Western Sahara, so any such cutoff omits one
country and that country is a disputed territory, which is a determination this
plugin does not make. It would also put the label set out of step with the
Region row, which names a country from ADMIN over this same feature set. Rank is
carried through as a draw priority instead, and density is left to the renderer's
collision index, which is what that index is for.
"""
import json
import sys


def main(src: str, dst: str) -> int:
    with open(src, encoding="utf-8") as handle:
        collection = json.load(handle)

    features = []
    for feature in collection["features"]:
        props = feature.get("properties") or {}
        name = props.get("ADMIN")
        lon, lat = props.get("LABEL_X"), props.get("LABEL_Y")
        rank = props.get("LABELRANK")

        if not name or lon is None or lat is None:
            continue
        if not (-180 <= lon <= 180) or not (-85.0511 <= lat <= 85.0511):
            continue

        features.append({
            "type": "Feature",
            "geometry": {"type": "Point", "coordinates": [lon, lat]},
            "properties": {"name": name, "rank": int(rank) if rank is not None else 99},
        })

    features.sort(key=lambda f: (f["properties"]["rank"], f["properties"]["name"]))

    if len(features) != len(collection["features"]):
        missing = {f["properties"].get("ADMIN") for f in collection["features"]}
        missing -= {f["properties"]["name"] for f in features}
        print(f"error: dropped {sorted(missing)}", file=sys.stderr)
        return 1

    with open(dst, "w", encoding="utf-8") as handle:
        json.dump({"type": "FeatureCollection", "features": features}, handle,
                  ensure_ascii=False, separators=(",", ":"), sort_keys=True)

    print(f"country_labels: {len(features)} of {len(collection['features'])} features")
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("usage: labels.py <admin0.geojson> <out.geojson>", file=sys.stderr)
        raise SystemExit(2)
    raise SystemExit(main(sys.argv[1], sys.argv[2]))

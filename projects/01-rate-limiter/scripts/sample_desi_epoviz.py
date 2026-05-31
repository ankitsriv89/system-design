#!/usr/bin/env python3
"""Sample the public DESI EDR visualization catalog for the galaxy demo.

Input:
  EDR-Viz-Outreach-VAC.csv.gz from
  https://data.desi.lbl.gov/public/edr/vac/edr/epoviz/v1.0/

Output:
  web/data/desi-sample.json
"""

from __future__ import annotations

import csv
import gzip
import json
import sys
from pathlib import Path


TRACERS = {
    "0": "QSO",
    "1": "ELG",
    "2": "LRG",
    "3": "BGS",
}


def main() -> int:
    if len(sys.argv) not in {2, 3}:
        print("usage: sample_desi_epoviz.py <EDR-Viz-Outreach-VAC.csv.gz> [limit]", file=sys.stderr)
        return 2

    source = Path(sys.argv[1])
    limit = int(sys.argv[2]) if len(sys.argv) == 3 else 2400
    root = Path(__file__).resolve().parents[1]
    output = root / "web" / "data" / "desi-sample.json"
    output.parent.mkdir(parents=True, exist_ok=True)

    objects = []
    with gzip.open(source, "rt", newline="") as f:
        reader = csv.DictReader(f)
        # Deterministic stride sampling keeps the file tiny while preserving
        # objects from the full catalog ordering and all 20 rosettes.
        stride = 293
        for row_number, row in enumerate(reader):
            if row_number % stride != 0:
                continue
            objects.append(
                {
                    "targetId": row["TARGETID"],
                    "ra": round(float(row["RA"]), 5),
                    "dec": round(float(row["DEC"]), 5),
                    "redshift": round(float(row["REDSHIFT"]), 5),
                    "rosette": int(row["ROSETTE"]),
                    "tracer": TRACERS.get(row["TRACER"], row["TRACER"]),
                    "x": float(row["X"]),
                    "y": float(row["Y"]),
                    "z": float(row["Z"]),
                }
            )
            if len(objects) >= limit:
                break

    payload = {
        "source": "DESI EDR Visualization and Outreach Catalog v1.0",
        "sourceUrl": "https://data.desi.lbl.gov/public/edr/vac/edr/epoviz/v1.0/EDR-Viz-Outreach-VAC.csv.gz",
        "license": "DESI EDR public data; see DESI data license and acknowledgments",
        "sampleSize": len(objects),
        "objects": objects,
    }
    output.write_text(json.dumps(payload, indent=2), encoding="utf-8")
    print(f"wrote {len(objects)} objects to {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

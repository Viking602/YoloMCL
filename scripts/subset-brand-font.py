"""Rebuild the bundled wordmark face from the upstream Fusion Pixel OTF.

Requires fontTools (4.67.0 was used). The launcher uses this face only for
the literal YoloMCL wordmark, never for user text or CJK fallback.
Usage: python scripts/subset-brand-font.py /path/to/upstream.otf
"""
import argparse
from pathlib import Path

from fontTools import subset
from fontTools.ttLib import TTFont


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("--output", type=Path,
                        default=Path(__file__).resolve().parents[1] /
                        "internal/nativeui/assets/pixel.otf")
    args = parser.parse_args()
    font = TTFont(args.source, recalcTimestamp=False)
    options = subset.Options()
    options.name_IDs = ["*"]  # preserve copyright, attribution and OFL metadata
    options.name_legacy = True
    options.name_languages = ["*"]
    sub = subset.Subsetter(options=options)
    sub.populate(text=" YoloMCL")
    sub.subset(font)
    # Give the modified face its own name; outlines and advances stay intact.
    names = {1: "MCL Pixel Brand", 2: "Regular", 3: "MCL Pixel Brand Regular",
             4: "MCL Pixel Brand", 6: "MCLPixelBrand-Regular",
             16: "MCL Pixel Brand", 17: "Regular"}
    for record in font["name"].names:
        if record.nameID in names:
            record.string = names[record.nameID].encode(record.getEncoding())
    if "CFF " in font:
        cff = font["CFF "].cff
        cff.fontNames = ["MCLPixelBrand-Regular"]
        top = cff.topDictIndex[0]
        top.FamilyName = "MCL Pixel Brand"
        top.FullName = "MCL Pixel Brand Regular"
    font.save(args.output)
    print(f"Wordmark font: {args.output.stat().st_size:,} bytes")


if __name__ == "__main__":
    main()

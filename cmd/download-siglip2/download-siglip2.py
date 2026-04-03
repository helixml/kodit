#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = [
#     "huggingface-hub>=0.20",
# ]
# ///
"""Download pre-converted SigLIP2 ONNX model from onnx-community."""
import os
import sys

from huggingface_hub import snapshot_download

REPO_ID = "onnx-community/siglip2-base-patch16-512-ONNX"

# Download separate vision and text INT8 models (~380 MB total) plus config
# and tokenizer files. Separate models let the runtime create independent
# pipelines for image embedding and text query embedding.
ALLOW_PATTERNS = [
    "config.json",
    "preprocessor_config.json",
    "tokenizer.json",
    "tokenizer.model",
    "tokenizer_config.json",
    "special_tokens_map.json",
    "onnx/vision_model_int8.onnx",
    "onnx/text_model_int8.onnx",
]


def main():
    if len(sys.argv) < 2:
        print("usage: download-siglip2 <dest>", file=sys.stderr)
        sys.exit(1)

    output_dir = sys.argv[1]

    # Skip if already downloaded.
    vision_dest = os.path.join(output_dir, "onnx", "vision_model.onnx")
    text_dest = os.path.join(output_dir, "onnx", "text_model.onnx")
    if os.path.exists(vision_dest) and os.path.exists(text_dest):
        print(f"Model already present at {output_dir}")
        return

    snapshot_download(
        REPO_ID,
        local_dir=output_dir,
        allow_patterns=ALLOW_PATTERNS,
    )

    # Rename INT8 models to canonical names.
    renames = {
        "vision_model_int8.onnx": "vision_model.onnx",
        "text_model_int8.onnx": "text_model.onnx",
    }
    onnx_dir = os.path.join(output_dir, "onnx")
    for src_name, dst_name in renames.items():
        src = os.path.join(onnx_dir, src_name)
        dst = os.path.join(onnx_dir, dst_name)
        if os.path.exists(src) and not os.path.exists(dst):
            os.rename(src, dst)

    print(f"Model downloaded to {output_dir}")


if __name__ == "__main__":
    main()

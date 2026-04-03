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

# Only download the INT8 quantized model (381 MB) and config/tokenizer files.
# The full FP32 model is 1.5 GB; INT8 keeps quality while matching the size
# of the existing bundled text embedding model (~311 MB).
ALLOW_PATTERNS = [
    "config.json",
    "preprocessor_config.json",
    "tokenizer.json",
    "tokenizer.model",
    "tokenizer_config.json",
    "special_tokens_map.json",
    "onnx/model_int8.onnx",
]


def main():
    if len(sys.argv) < 2:
        print("usage: download-siglip2 <dest>", file=sys.stderr)
        sys.exit(1)

    output_dir = sys.argv[1]

    # Skip if already downloaded.
    onnx_dest = os.path.join(output_dir, "onnx", "model.onnx")
    if os.path.exists(onnx_dest):
        print(f"Model already present at {output_dir}")
        return

    snapshot_download(
        REPO_ID,
        local_dir=output_dir,
        allow_patterns=ALLOW_PATTERNS,
    )

    # Rename int8 model to model.onnx so the runtime finds it at the
    # conventional path (onnx/model.onnx).
    int8_path = os.path.join(output_dir, "onnx", "model_int8.onnx")
    if os.path.exists(int8_path) and not os.path.exists(onnx_dest):
        os.rename(int8_path, onnx_dest)

    print(f"Model downloaded to {output_dir}")


if __name__ == "__main__":
    main()

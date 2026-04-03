#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = [
#     "huggingface-hub>=0.20",
#     "onnx>=1.14",
# ]
# ///
"""Download pre-converted SigLIP2 ONNX model from onnx-community."""
import os
import sys

from huggingface_hub import snapshot_download
from onnx import TensorProto, load, save

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

    # Fix vision model input shape: the onnx-community export leaves all four
    # dimensions dynamic (batch_size, num_channels, height, width) but the
    # hugot runtime only allows at most 2 dynamic dims. Pin the known fixed
    # dimensions so only batch_size remains dynamic.
    _fix_vision_input_shape(vision_dest)

    print(f"Model downloaded to {output_dir}")


def _fix_vision_input_shape(path: str) -> None:
    """Fix the vision model's dynamic dimensions.

    The onnx-community export leaves all input dimensions dynamic and uses
    complex expressions in the output shapes. Pin the known fixed dimensions
    so the hugot runtime can validate and allocate tensors correctly.

    Input:  pixel_values [batch, 3, 512, 512]
    Output: last_hidden_state [batch, 1024, 768]
            pooler_output [batch, 768]
    """
    model = load(path)

    # Fix input: pin channels, height, width.
    for inp in model.graph.input:
        if inp.name != "pixel_values":
            continue
        dims = inp.type.tensor_type.shape.dim
        if len(dims) != 4:
            continue
        for dim, value in zip(dims[1:], [3, 512, 512]):
            dim.Clear()
            dim.dim_value = value
        break

    # Fix outputs: replace formula-based dim params with simple values.
    # The batch dimension uses a complex formula; replace it with a simple
    # dynamic param name so hugot counts at most 1 dynamic dimension.
    for out in model.graph.output:
        dims = out.type.tensor_type.shape.dim
        if out.name == "last_hidden_state" and len(dims) == 3:
            # [batch, 1024, 768]
            dims[0].Clear()
            dims[0].dim_param = "batch_size"
            dims[1].Clear()
            dims[1].dim_value = 1024
            dims[2].Clear()
            dims[2].dim_value = 768
        elif out.name == "pooler_output" and len(dims) == 2:
            # [batch, 768]
            dims[0].Clear()
            dims[0].dim_param = "batch_size"
            dims[1].Clear()
            dims[1].dim_value = 768

    save(model, path)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3

"""
Simple script to run inference on a dataset using an OpenAI-compatible API endpoint.
Maintains the same output format as run_api.py but allows custom API endpoints.
"""

import json
import os
import logging
from pathlib import Path
from tqdm.auto import tqdm
import openai
from datasets import load_dataset, load_from_disk
from swebench.inference.make_datasets.utils import extract_diff
from argparse import ArgumentParser

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

def call_api(model_name: str, inputs: str, api_base: str = None, api_key: str = None, temperature: float = 0.2, top_p: float = 0.95):
    """
    Call the API with the given inputs.
    
    Args:
        model_name: Name of the model to use
        inputs: Input text to send to the model
        api_base: Base URL for the API (optional, defaults to OpenAI)
        api_key: API key to use (optional, defaults to OPENAI_API_KEY env var)
        temperature: Sampling temperature
        top_p: Top-p sampling parameter
    
    Returns:
        tuple: (response, input_tokens, output_tokens)
    """
    # Split system and user messages
    system_message = inputs.split("\n", 1)[0]
    user_message = inputs.split("\n", 1)[1]
    
    # Configure OpenAI client
    client_kwargs = {}
    if api_base:
        client_kwargs["base_url"] = api_base
    if api_key:
        client_kwargs["api_key"] = api_key
    
    client = openai.OpenAI(**client_kwargs)
    
    # Make the API call
    response = client.chat.completions.create(
        model=model_name,
        messages=[
            {"role": "system", "content": system_message},
            {"role": "user", "content": user_message}
        ],
        temperature=temperature,
        top_p=top_p
    )
    
    # Extract token counts if available
    input_tokens = getattr(response.usage, 'prompt_tokens', 0)
    output_tokens = getattr(response.usage, 'completion_tokens', 0)
    
    return response, input_tokens, output_tokens

def run_inference(
    dataset_name_or_path: str,
    split: str,
    model_name: str,
    output_dir: str,
    api_base: str = None,
    api_key: str = None,
    temperature: float = 0.2,
    top_p: float = 0.95
):
    """
    Run inference on a dataset using a custom API endpoint.
    
    Args:
        dataset_name_or_path: Path to or name of the dataset
        split: Dataset split to use
        model_name: Name of the model to use
        output_dir: Directory to save outputs
        api_base: Base URL for the API (optional, defaults to OpenAI)
        api_key: API key to use (optional, defaults to OPENAI_API_KEY env var)
        temperature: Sampling temperature
        top_p: Top-p sampling parameter
    """
    # Setup output file
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    output_file = output_dir / f"{model_name}__{Path(dataset_name_or_path).name}__{split}.jsonl"
    
    # Load dataset
    dataset = (
        load_from_disk(dataset_name_or_path)
        if Path(dataset_name_or_path).exists()
        else load_dataset(dataset_name_or_path)
    )
    
    if split not in dataset:
        raise ValueError(f"Invalid split {split} for dataset {dataset_name_or_path}")
    
    dataset = dataset[split]
    
    # Track existing instances
    existing_ids = set()
    if output_file.exists():
        with open(output_file) as f:
            for line in f:
                data = json.loads(line)
                existing_ids.add(data["instance_id"])
    
    logger.info(f"Found {len(existing_ids)} existing instances")
    
    # Filter out existing instances
    dataset = dataset.filter(
        lambda x: x["instance_id"] not in existing_ids,
        desc="Filtering out existing instances"
    )
    
    # Run inference
    with open(output_file, "a+") as f:
        for datum in tqdm(dataset, desc=f"Running inference with {model_name}"):
            instance_id = datum["instance_id"]
            
            # Prepare output dictionary
            output_dict = {
                "instance_id": instance_id,
                "model_name_or_path": model_name,
                "text": f"{datum['text']}\n\n"
            }
            
            try:
                # Call API
                response, input_tokens, output_tokens = call_api(
                    model_name=model_name,
                    inputs=output_dict["text"],
                    api_base=api_base,
                    api_key=api_key,
                    temperature=temperature,
                    top_p=top_p
                )
                
                # Extract completion
                completion = response.choices[0].message.content
                
                # Add to output
                output_dict["full_output"] = completion
                output_dict["model_patch"] = extract_diff(completion)
                
                # Add token counts if available
                if input_tokens > 0:
                    output_dict["input_tokens"] = input_tokens
                if output_tokens > 0:
                    output_dict["output_tokens"] = output_tokens
                
                # Write to file
                print(json.dumps(output_dict), file=f, flush=True)
                
            except Exception as e:
                logger.error(f"Error processing instance {instance_id}: {str(e)}")
                continue
    
    logger.info(f"Finished! Output saved to {output_file}")

def main():
    parser = ArgumentParser(description=__doc__)
    parser.add_argument(
        "--dataset_name_or_path",
        type=str,
        required=True,
        help="Path to or name of the dataset"
    )
    parser.add_argument(
        "--split",
        type=str,
        default="test",
        help="Dataset split to use"
    )
    parser.add_argument(
        "--model_name",
        type=str,
        required=True,
        help="Name of the model to use"
    )
    parser.add_argument(
        "--output_dir",
        type=str,
        required=True,
        help="Directory to save outputs"
    )
    parser.add_argument(
        "--api_base",
        type=str,
        default=None,
        help="Base URL for the API (e.g., http://localhost:8000/v1). If not provided, uses OpenAI's API."
    )
    parser.add_argument(
        "--api_key",
        type=str,
        default=None,
        help="API key to use. If not provided, uses OPENAI_API_KEY environment variable."
    )
    parser.add_argument(
        "--temperature",
        type=float,
        default=0.2,
        help="Sampling temperature"
    )
    parser.add_argument(
        "--top_p",
        type=float,
        default=0.95,
        help="Top-p sampling parameter"
    )
    
    args = parser.parse_args()
    
    # If no API key provided, try to get it from environment
    if not args.api_key:
        args.api_key = os.environ.get("OPENAI_API_KEY")
        if not args.api_key and not args.api_base:
            raise ValueError("No API key provided and OPENAI_API_KEY environment variable not set")
    
    run_inference(**vars(args))

if __name__ == "__main__":
    main() 
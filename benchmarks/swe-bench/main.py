#!/usr/bin/env python3

"""
Main script for creating datasets with custom context injection.
This script combines functionality from create_text_dataset.py and create_instance.py
while allowing users to inject their own context into the final text field.
"""

import json
import logging
import os
from argparse import ArgumentParser
from pathlib import Path
from typing import Dict, List, Optional, Union, Callable
from datasets import Dataset, DatasetDict, load_dataset, load_from_disk
from tqdm.auto import tqdm

from swebench.inference.make_datasets.create_instance import (
    add_text_inputs,
    PROMPT_FUNCTIONS,
    make_code_text,
    make_code_text_edits_only,
)
from swebench.inference.make_datasets.tokenize_dataset import TOKENIZER_FUNCS

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

def load_jsonl_file(filename: Union[str, Path]) -> List[Dict]:
    """Load data from a JSONL or JSON file."""
    if isinstance(filename, str):
        filename = Path(filename)
    if filename.name.endswith(".jsonl") or filename.name.endswith(".jsonl.all"):
        with open(filename) as f:
            return [json.loads(line) for line in f]
    elif filename.name.endswith(".json"):
        with open(filename) as f:
            return json.load(f)
    else:
        raise ValueError(f"Unknown file type {filename}")

def instances_generator(files: List[Union[str, Path]]) -> List[Dict]:
    """Generate instances from a list of files."""
    all_data = list()
    for file in tqdm(files, desc="Loading instance files"):
        all_data.extend(load_jsonl_file(file))
    return all_data

def extract_fields(instance: Dict) -> Optional[Dict]:
    """Extract fields from an instance and format them for the dataset."""
    instance_id = instance["instance_id"]
    if instance["text_inputs"] is None or instance["patch"] is None:
        logger.warning(f"No text for {instance_id}")
        return None
    text_inputs = instance["text_inputs"].strip() + "\n\n"
    if text_inputs is None or instance["patch"] is None:
        logger.warning(f"No inputs for {instance_id}")
        return None
    patch = "\n".join(["<patch>", instance["patch"], "</patch>"])
    return {**instance, "text": text_inputs, "patch": patch}

def inject_custom_context(
    instance: Dict,
    code_generator: Callable[[Dict], str],
    prompt_style: str = "2"
) -> Dict:
    """
    Replace the code section in an instance's text field with custom generated content.
    
    Args:
        instance: The instance dictionary
        code_generator: A function that takes an instance and returns the new code content
        prompt_style: The prompt style to use (must be one of PROMPT_FUNCTIONS)
    
    Returns:
        Modified instance with custom code content
    """
    if prompt_style not in PROMPT_FUNCTIONS:
        raise ValueError(f"Unknown prompt style {prompt_style}")
    
    # Get the base text using the specified prompt style
    base_text = PROMPT_FUNCTIONS[prompt_style](instance)
    
    # Generate new code content based on the instance
    new_code_content = code_generator(instance)
    
    # If code generator returns None, don't modify the text
    if new_code_content is not None:
        # Split the text into sections using both opening and closing tags
        sections = base_text.split("<code>")
        if len(sections) != 2:
            raise ValueError("Could not find opening <code> tag in text")
        
        before_code, after_code = sections
        after_sections = after_code.split("</code>")
        if len(after_sections) != 2:
            raise ValueError("Could not find closing </code> tag in text")
        
        after_code = after_sections[1]
        
        # Replace the code section with new content
        modified_text = f"{before_code}<code>{new_code_content}</code>{after_code}"
        instance["text"] = modified_text
    else:
        instance["text"] = base_text
    
    return instance

def process_dataset(
    dataset_name_or_path: Union[str, Path],
    output_dir: Union[str, Path],
    code_generator: Callable[[Dict], str],
    prompt_style: str = "style-3",
    file_source: str = "oracle",
    k: Optional[int] = None,
    max_context_len: Optional[int] = None,
    tokenizer_name: Optional[str] = None,
    retrieval_file: Optional[Union[str, Path]] = None,
    splits: List[str] = ["train", "test"],
    validation_ratio: float = 0.01,
) -> None:
    """
    Process a dataset and replace code sections with custom generated content.
    
    Args:
        dataset_name_or_path: Path to or name of the dataset
        output_dir: Directory to save the processed dataset
        code_generator: Function that generates new code content based on instance
        prompt_style: The prompt style to use
        file_source: Source of files to include
        k: Number of files to retrieve
        max_context_len: Maximum context length
        tokenizer_name: Name of tokenizer to use
        retrieval_file: Path to retrieval results file
        splits: Dataset splits to process
        validation_ratio: Ratio of validation split
    """
    # Setup output directory
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    # Load dataset
    dataset = (
        load_from_disk(dataset_name_or_path)
        if Path(dataset_name_or_path).exists()
        else load_dataset(dataset_name_or_path)
    )
    
    # Define columns for final dataset
    columns = [
        "instance_id",
        "text",
        "repo",
        "base_commit",
        "problem_statement",
        "hints_text",
        "created_at",
        "patch",
        "test_patch",
        "version",
        "FAIL_TO_PASS",
        "PASS_TO_PASS",
        "environment_setup_commit",
    ]
    
    # Process each split
    final_dataset = DatasetDict()
    progress_files = {}
    
    for split in splits:
        if split not in dataset:
            logger.warning(f"Split {split} not found in dataset, skipping")
            continue
            
        logger.info(f"Processing {split} split")
        split_instances = {x["instance_id"]: x for x in dataset[split]}
        
        # Setup progress file
        progress_file = output_dir / f"progress_{split}.jsonl"
        progress_files[split] = progress_file
        
        # Add text inputs with progress tracking
        add_text_inputs(
            split_instances,
            retrieval_file=retrieval_file,
            k=k,
            prompt_style=prompt_style,
            file_source=file_source,
            max_context_len=max_context_len,
            tokenizer_name=tokenizer_name,
            progress_file=progress_file,
        )
        
        # Process instances from progress file
        split_data = {key: [] for key in columns}
        valid_instance_ids = set(dataset[split]["instance_id"])
        invalid_instances = []
        
        with open(progress_file) as f:
            for line in f:
                datum = extract_fields(json.loads(line))
                if not datum:
                    continue
                if datum["instance_id"] not in valid_instance_ids:
                    invalid_instances.append(datum["instance_id"])
                    continue
                    
                # Inject custom code content
                datum = inject_custom_context(
                    datum,
                    code_generator,
                    prompt_style
                )
                
                for key in columns:
                    split_data[key].append(datum.get(key, ""))
        
        if invalid_instances:
            logger.warning(
                f"Found {len(invalid_instances)} instances in progress file that are not in the {split} dataset: {invalid_instances}. These will be removed from the final dataset."
            )
        
        final_dataset[split] = Dataset.from_dict(split_data)
    
    # Handle validation split
    if validation_ratio > 0 and "train" in final_dataset:
        train_val = final_dataset["train"].train_test_split(
            test_size=validation_ratio,
            seed=42
        )
        final_dataset["train"] = train_val["train"]
        final_dataset["validation"] = train_val["test"]
    
    # Save dataset
    output_file = output_dir / f"dataset_with_{code_generator.__name__}_code"
    final_dataset.save_to_disk(output_file)
    logger.info(f"Saved dataset to {output_file}")
    
    # Cleanup progress files
    for progress_file in progress_files.values():
        if os.path.exists(progress_file):
            os.remove(progress_file)

def main():
    parser = ArgumentParser(description="Create dataset with custom code generation")
    parser.add_argument(
        "--dataset_name_or_path",
        type=str,
        default="SWE-bench/SWE-bench",
        help="Dataset to use for test set from HuggingFace Datasets or path to a save_to_disk directory.",
    )
    parser.add_argument(
        "--splits",
        nargs="+",
        default=["train", "test"],
        help="Splits to use from the dataset.",
    )
    parser.add_argument(
        "--validation_ratio",
        type=float,
        default=0.01,
        help="Ratio of the training set to use for validation.",
    )
    parser.add_argument("--output_dir", type=str, help="Path to the output directory.")
    parser.add_argument(
        "--retrieval_file",
        type=str,
        help="Path to the file where the retrieval results are stored.",
    )
    parser.add_argument(
        "--prompt_style",
        type=str,
        default="style-3",
        choices=PROMPT_FUNCTIONS.keys(),
        help="Prompt style to use. See create_instance.PROMPT_FUNCTIONS for details.",
    )
    parser.add_argument(
        "--file_source",
        type=str,
        default="oracle",
        choices=["oracle", "bm25", "all"],
        help="How to select the files to use in context.",
    )
    parser.add_argument(
        "--k",
        type=int,
        default=None,
        help="Maximum number of files to use for retrieval.",
    )
    parser.add_argument(
        "--max_context_len",
        type=int,
        default=None,
        help="Maximum number of tokens to use for context.",
    )
    parser.add_argument(
        "--tokenizer_name",
        type=str,
        default=None,
        choices=TOKENIZER_FUNCS.keys(),
        help="Tokenizer to use for max_context_len. Only needed if max_context_len is specified.",
    )
    parser.add_argument(
        "--code_generator",
        type=str,
        default="null",
        choices=["null", "dummy"],
        help="Which code generator to use. 'null' preserves original text, 'dummy' injects dummy text.",
    )
    
    args = parser.parse_args()
    
    def null_code_generator(instance: Dict) -> str:
        return None

    def dummy_code_generator(instance: Dict) -> str:
        return f"# Dummy code for {instance.get('repo', '')}\n# Problem: {instance.get('problem_statement', '')}\n\ndef dummy_function():\n    pass"

    # Dictionary of available code generators
    CODE_GENERATORS = {
        "null": null_code_generator,  # Preserves original text
        "dummy": dummy_code_generator
    }
    
    process_dataset(
        args.dataset_name_or_path,
        args.output_dir,
        CODE_GENERATORS[args.code_generator],
        args.prompt_style,
        args.file_source,
        args.k,
        args.max_context_len,
        args.tokenizer_name,
        args.retrieval_file,
        args.splits,
        args.validation_ratio,
    )

if __name__ == "__main__":
    main() 
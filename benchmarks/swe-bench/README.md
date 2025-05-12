uv run python main.py \
  --dataset "princeton-nlp/SWE-bench_Lite" \
  --output_dir ./base_datasets \
  --code_generator null
  


## Openai example


uv run python prediction.py \
    --dataset_name_or_path base_datasets/dataset_with_custom_code \
    --split test \
    --model_name gpt-4.1-nano-2025-04-14 \
    --output_dir results \
    --temperature 0.0 \
    --top_p 0.95




## VLLM example:


docker run -d --runtime nvidia --gpus all -v
~/.cache/huggingface:/root/.cache/huggingface --env "HUGGING_FACE_HUB_TOKEN"     -p
8000:8000     --ipc=host     vllm/vllm-openai:latest     --model Qwen/Qwen3-8B
--max-model-len 32768

uv run python prediction.py \
    --dataset_name_or_path base_datasets/dataset_with_custom_code \
    --split test \
    --model_name Qwen/Qwen3-8B \
    --output_dir results \
    --api_base http://localhost:8000/v1 \
    --temperature 0.0 \
    --top_p 0.95


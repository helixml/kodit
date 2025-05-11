
---

`evalplus.evaluate --model "ise-uiuc/Magicoder-S-DS-6.7B" --dataset humaneval --backend vllm --greedy`

humaneval (base tests)
pass@1: 0.756
humaneval+ (base + extra tests)
pass@1: 0.695

---

`evalplus.evaluate --model "ise-uiuc/Magicoder-S-DS-6.7B" --dataset mbpp --backend vllm --greedy`
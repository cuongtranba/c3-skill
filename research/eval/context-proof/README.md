# Context-proof gold set

`codebase-qa.json` is the gold question set for **`scripts/c3_memory_bench.py`** —
questions about this repository whose answers are already known, used to measure
whether C3 retrieves the right documentation and answers correctly.

It is the C3 analogue of LoCoMo / LongMemEval in
[mem0ai/memory-benchmarks](https://github.com/mem0ai/memory-benchmarks). Their
pipeline shape (ingest → search → answer → judge) transfers; their datasets do
not, because they measure *conversational* memory and C3's memory is a codebase.

## Entry contract

Every entry is validated at load time by `load_dataset()`; a malformed entry
aborts the run rather than quietly scoring zero.

| Field | Required | Meaning |
|---|---|---|
| `id` | yes | stable case id (`qa-NN`), used by `--case` |
| `category` | yes | `navigation` · `model` · `change-impact` · `trust` |
| `question` | yes | the question, sent verbatim to `c3x search` |
| `answer` | yes | reference answer the judge grades against |
| `doc_trace` | yes | **gold entity ids** that should be retrieved — the retrieval ground truth |
| `code_trace` | no | source paths backing the answer |
| `via` | no | the commands a human used to answer it |
| `coverage` | yes | `clean` · `partial` · `gap` (below) |

## Coverage — and why it matters

`coverage` records how well the docs actually cover the question. This is the one
thing C3's set has that mem0's cannot: every LoCoMo question is answerable from
memory, so their harness cannot measure **confabulation under absence**.

| Coverage | `doc_trace` | Passing means |
|---|---|---|
| `clean` | populated | the judge grades the answer CORRECT |
| `partial` | populated | CORRECT **and** the caveat in the reference answer is surfaced |
| `gap` | **empty** | the answer admits the docs don't cover it, instead of inventing one |

A `gap` entry with a non-empty `doc_trace` is a contradiction and is rejected at
load, as is a `clean`/`partial` entry with an empty one.

`gap` questions are **skipped in `--retrieval-only` mode** — there is no gold
entity to retrieve, so there is nothing to score. The run prints which ids were
skipped; it never silently counts them as passes.

## Adding a question

1. Answer it yourself first, recording the commands in `via`.
2. Put the entity ids you genuinely needed in `doc_trace` — this is the retrieval
   ground truth, so an aspirational list makes the benchmark lie.
3. Set `coverage` honestly. A question the docs *should* cover but don't is a
   `gap`, and recording it as such is what makes the gap visible.
4. Re-run the free retrieval pass to see the effect:

```bash
python scripts/c3_memory_bench.py --run --retrieval-only --skip-build \
  --cutoff 1,3,5,10 --output /tmp/retrieval.jsonl
```

## Keeping it honest

The gold answers describe the repository at a point in time. When a component is
renamed, split, or retired, entries pointing at it go stale and the benchmark
silently measures the wrong thing. Re-answer affected entries as part of the
change that moves the code — `c3x lookup <path>` maps a changed file back to the
entities a `doc_trace` may reference.

#!/usr/bin/env python3
"""C3 Memory Bench — retrieval + answer benchmark over the context-proof gold set.

Ports the pipeline shape of mem0ai/memory-benchmarks (ingest -> search -> answer
-> judge) onto C3's own domain. Their datasets are conversational (LoCoMo,
LongMemEval, BEAM: "what did Caroline say about her dog?"); C3's memory is a
codebase, so the datasets do not transfer but the pipeline does.

The gold set is ``research/eval/context-proof/codebase-qa.json``: hand-authored
questions carrying ``doc_trace`` (the entities that should be retrieved) and
``answer`` (the reference answer). Until this harness existed nothing consumed
it.

How this differs from the sibling harnesses, and why it is separate:

* ``agent_efficiency_eval.py`` (frozen) grades *agent behaviour on tasks* — did
  the agent write a good ADR, did it stay under a token budget. It never asks
  whether C3 retrieved the right documents for a known-answer question.
* ``paired_skill_eval.py`` grades *with-C3 vs without-C3* on an external repo.
* This harness grades *retrieval and answer quality* on questions whose answers
  are already known.

One genuine improvement over the mem0 design: the gold set labels each question
``clean`` / ``partial`` / ``gap``. Every LoCoMo question is answerable from
memory, so their harness cannot measure confabulation under absence. Here the
``gap`` questions have empty ``doc_trace`` — the docs genuinely do not cover
them — and the correct behaviour is to say so rather than invent an answer.

Isolation note: unlike ``agent_efficiency_eval.py`` this runs against the
checkout in place rather than a temp copy. That harness isolates because its
agents mutate the workspace; here stage 1 is a read-only search and stage 2 is a
closed-book answerer with no tools, so nothing can mutate the tree — and the
point is to measure *this* checkout's documentation.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shlex
import statistics
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parent))

import agent_efficiency_eval as ev

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_DATASET = REPO_ROOT / "research" / "eval" / "context-proof" / "codebase-qa.json"
DEFAULT_OUTPUT = REPO_ROOT / "research" / "eval" / "memory-runs" / "latest.jsonl"
C3X = "skills/c3/bin/c3x.sh"

# Retrieval depths to score. mem0 sweeps @10/@20/@50/@200 over a corpus of
# hundreds of thousands of memories; a C3 project is tens of entities, so the
# interesting resolution is much finer.
DEFAULT_CUTOFFS = (5,)
ALL_CUTOFFS = (1, 3, 5, 10)

# Per-entity context budget, in characters, when assembling the answerer prompt.
# Bounds prompt size so a @10 cutoff cannot blow past the model's window.
ENTITY_CONTEXT_CHARS = 4000

ANSWER_PROMPT = """\
You are answering a question about a codebase using ONLY the architecture \
documentation excerpts provided below. Do not use outside knowledge and do not \
guess.

If the excerpts do not contain enough information to answer, reply with exactly \
DOCS_INSUFFICIENT on the first line, then one sentence naming what is missing.

## Question
{question}

## Retrieved architecture documentation (top-{k})
{context}

Answer in under 150 words.
"""

JUDGE_PROMPT = """\
You are grading a generated answer about a codebase against a reference answer.

Grade CORRECT if the generated answer conveys the substantive facts of the \
reference answer. Wording, ordering, and extra detail do not matter. Grade \
WRONG if it misses or contradicts a key fact, or if it is too vague to be \
useful.

## Question
{question}

## Reference answer
{reference}

## Generated answer
{generated}

Respond with ONLY a JSON object and nothing else:
{{"label": "CORRECT" or "WRONG", "reasoning": "<one sentence>"{extra_schema}}}
"""

GAP_SCHEMA = ', "gap_acknowledged": true or false'
GAP_INSTRUCTION = """
This question is known to be NOT covered by the architecture documentation. Set \
gap_acknowledged=true only if the generated answer admits the docs are \
insufficient (or points at source code instead) rather than confidently \
inventing a documented answer. An answer that confabulates scores \
gap_acknowledged=false even if it happens to be factually close.
"""

PARTIAL_SCHEMA = ', "caveat_surfaced": true or false'
PARTIAL_INSTRUCTION = """
This question is only PARTIALLY covered by the documentation; the reference \
answer contains a caveat or exception. Set caveat_surfaced=true only if the \
generated answer surfaces that caveat.
"""


# ---------------------------------------------------------------------------
# Dataset
# ---------------------------------------------------------------------------

REQUIRED_FIELDS = ("id", "category", "question", "answer", "doc_trace", "coverage")
VALID_COVERAGE = {"clean", "partial", "gap"}


def load_dataset(path: Path) -> list[dict[str, Any]]:
    """Load and validate the gold set, failing loudly on a malformed entry."""
    try:
        items = json.loads(path.read_text())
    except FileNotFoundError:
        raise SystemExit(f"error: dataset not found at {path}")
    except json.JSONDecodeError as exc:
        raise SystemExit(f"error: dataset at {path} is not valid JSON: {exc}") from exc

    if not isinstance(items, list) or not items:
        raise SystemExit(f"error: dataset at {path} must be a non-empty JSON array")

    for idx, item in enumerate(items):
        missing = [f for f in REQUIRED_FIELDS if f not in item]
        if missing:
            raise SystemExit(
                f"error: dataset entry {idx} ({item.get('id', '?')}) missing fields: {', '.join(missing)}"
            )
        coverage = item["coverage"]
        if coverage not in VALID_COVERAGE:
            raise SystemExit(
                f"error: {item['id']} has coverage={coverage!r}, expected one of {sorted(VALID_COVERAGE)}"
            )
        if coverage == "gap" and item["doc_trace"]:
            raise SystemExit(
                f"error: {item['id']} is coverage=gap but carries a non-empty doc_trace; "
                "a gap means the docs do not cover the question"
            )
        if coverage != "gap" and not item["doc_trace"]:
            raise SystemExit(
                f"error: {item['id']} is coverage={coverage} but has an empty doc_trace; "
                "label it coverage=gap if the docs genuinely do not cover it"
            )
    return items


# ---------------------------------------------------------------------------
# Stage 0 — ingest (C3's analogue of mem0's conversation upload)
# ---------------------------------------------------------------------------


def ensure_ingested(workspace: Path, *, build: bool = True) -> dict[str, Any]:
    """Make sure a c3x binary exists and the local cache is built from .c3/.

    mem0 uploads conversations to a memory service; C3's equivalent is building
    the queryable cache from the canonical ``.c3/`` markdown. Fails loudly — a
    silent failure here would score every question zero and read as "C3 is bad
    at retrieval" rather than "the harness never ingested anything".
    """
    started = time.perf_counter()
    wrapper = workspace / C3X
    if not wrapper.exists():
        raise SystemExit(f"error: c3x wrapper missing at {wrapper}")

    if build:
        proc = subprocess.run(
            ["bash", "scripts/build.sh"],
            cwd=workspace,
            text=True,
            capture_output=True,
            check=False,
        )
        if proc.returncode != 0:
            raise SystemExit(
                "error: stage 0 build failed — refusing to benchmark an unbuilt CLI\n"
                + (proc.stderr or proc.stdout)[-2000:]
            )

    proc = subprocess.run(
        ["bash", C3X, "check"],
        cwd=workspace,
        text=True,
        capture_output=True,
        check=False,
        env={**os.environ, "C3X_MODE": "agent"},
    )
    if proc.returncode != 0:
        raise SystemExit(
            "error: stage 0 'c3x check' failed — the local cache is not queryable\n"
            + (proc.stderr or proc.stdout)[-2000:]
        )

    entity_count = None
    match = re.search(r"total:\s*(\d+)", proc.stdout)
    if match:
        entity_count = int(match.group(1))

    return {
        "ingest_ms": int((time.perf_counter() - started) * 1000),
        "entity_count": entity_count,
        "built": build,
    }


# ---------------------------------------------------------------------------
# Stage 1 — search (deterministic, zero tokens)
# ---------------------------------------------------------------------------


def run_search(workspace: Path, question: str) -> dict[str, Any]:
    """Run `c3x search <question> --json` and return the ranked hit list.

    Deliberately NOT in agent mode: agent/default output is TOON, and this needs
    machine-parseable JSON.
    """
    started = time.perf_counter()
    proc = subprocess.run(
        ["bash", C3X, "search", question, "--json"],
        cwd=workspace,
        text=True,
        capture_output=True,
        check=False,
        env={k: v for k, v in os.environ.items() if k != "C3X_MODE"},
    )
    latency_ms = int((time.perf_counter() - started) * 1000)

    if proc.returncode != 0:
        return {
            "ranked_ids": [],
            "match_sources": {},
            "search_latency_ms": latency_ms,
            "total_results": 0,
            "error": (proc.stderr or proc.stdout)[-500:],
        }
    try:
        payload = json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        return {
            "ranked_ids": [],
            "match_sources": {},
            "search_latency_ms": latency_ms,
            "total_results": 0,
            "error": f"unparseable search JSON: {exc}",
        }

    results = payload.get("results") or []
    ranked_ids = [r.get("id", "") for r in results if r.get("id")]
    match_sources = {
        r.get("id", ""): list(r.get("match_sources") or []) for r in results if r.get("id")
    }
    return {
        "ranked_ids": ranked_ids,
        "match_sources": match_sources,
        "search_latency_ms": latency_ms,
        "total_results": len(ranked_ids),
        "error": None,
    }


def score_retrieval(ranked_ids: list[str], gold: list[str], cutoffs: tuple[int, ...]) -> dict[str, Any]:
    """Recall/precision/hit at each cutoff, plus MRR over the full ranked list.

    Returns None-valued metrics when gold is empty (a `gap` question): there is
    nothing to recall, and averaging a zero in would understate real recall.
    """
    gold_set = {g for g in gold if g}
    if not gold_set:
        return {
            "mrr": None,
            "first_hit_rank": None,
            **{f"@{k}": {"recall": None, "precision": None, "hit": None} for k in cutoffs},
        }

    first_hit_rank = None
    for idx, entity_id in enumerate(ranked_ids):
        if entity_id in gold_set:
            first_hit_rank = idx + 1
            break

    out: dict[str, Any] = {
        "mrr": round(1.0 / first_hit_rank, 4) if first_hit_rank else 0.0,
        "first_hit_rank": first_hit_rank,
    }
    for k in cutoffs:
        window = ranked_ids[:k]
        found = gold_set & set(window)
        out[f"@{k}"] = {
            "recall": round(len(found) / len(gold_set), 4),
            "precision": round(len(found) / k, 4),
            "hit": bool(found),
        }
    return out


# ---------------------------------------------------------------------------
# Stage 2 / 3 — answer and judge (spend tokens)
# ---------------------------------------------------------------------------


def read_entity(workspace: Path, entity_id: str, *, limit: int = ENTITY_CONTEXT_CHARS) -> str:
    proc = subprocess.run(
        ["bash", C3X, "read", entity_id],
        cwd=workspace,
        text=True,
        capture_output=True,
        check=False,
        env={k: v for k, v in os.environ.items() if k != "C3X_MODE"},
    )
    if proc.returncode != 0:
        return ""
    return proc.stdout[:limit]


def build_context(workspace: Path, ranked_ids: list[str], k: int) -> str:
    chunks = []
    for entity_id in ranked_ids[:k]:
        body = read_entity(workspace, entity_id)
        if body.strip():
            chunks.append(f"### {entity_id}\n{body}")
    return "\n\n".join(chunks) if chunks else "(no documentation retrieved)"


def _invoke(command: list[str], prompt: str, workspace: Path, timeout: int) -> dict[str, Any]:
    """Run a CLI agent with the prompt substituted, reusing sibling conventions."""
    cmd = [part.replace("{prompt}", prompt) for part in command]
    started = time.perf_counter()
    try:
        proc = subprocess.run(
            cmd, cwd=workspace, text=True, capture_output=True, timeout=timeout, check=False
        )
    except subprocess.TimeoutExpired:
        return {"stdout": "", "stderr": "timeout", "exit_code": 124, "elapsed_ms": timeout * 1000}
    except FileNotFoundError:
        return {
            "stdout": "",
            "stderr": f"agent CLI not found: {cmd[0]}",
            "exit_code": 127,
            "elapsed_ms": 0,
            "agent_unavailable": True,
        }
    return {
        "stdout": proc.stdout,
        "stderr": proc.stderr,
        "exit_code": proc.returncode,
        "elapsed_ms": int((time.perf_counter() - started) * 1000),
    }


# `claude -p` prints bare text, but `claude --output-format stream-json` and
# `codex --json` both stream JSONL events — taking raw stdout as the answer
# would capture event noise instead of the model's prose.
def _text_from_event(obj: Any) -> str:
    """Pull assistant prose out of one streamed event, across CLI dialects."""
    if isinstance(obj, str):
        return obj
    if not isinstance(obj, dict):
        return ""
    if obj.get("type") == "error" or "error" in obj:
        return ""

    for key in ("text", "result", "content"):
        value = obj.get(key)
        if isinstance(value, str) and value.strip():
            return value
        if isinstance(value, list):
            parts = [
                part.get("text", "")
                for part in value
                if isinstance(part, dict) and part.get("type") in (None, "text")
            ]
            joined = "".join(parts).strip()
            if joined:
                return joined

    for key in ("item", "message", "delta"):
        nested = obj.get(key)
        if isinstance(nested, dict):
            found = _text_from_event(nested)
            if found:
                return found
    return ""


def extract_final_message(stdout: str) -> str:
    """Return the agent's final prose, whether stdout is plain text or JSONL.

    Falls back to the raw text when no line parses as JSON, so a plain
    `claude -p` invocation still works.
    """
    text = (stdout or "").strip()
    if not text:
        return ""

    candidates: list[str] = []
    saw_envelope = False
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue
        # A `type` key marks a transport envelope (codex/claude stream events).
        # Without one the object is the payload itself — e.g. the judge replying
        # with its bare verdict object.
        if isinstance(obj, dict) and "type" in obj:
            saw_envelope = True
        found = _text_from_event(obj)
        if found:
            candidates.append(found)

    if candidates:
        return candidates[-1].strip()
    # An event stream that yielded no prose produced no answer — a failed turn
    # must read as empty, never as its own error envelopes.
    return "" if saw_envelope else text


def generate_answer(
    workspace: Path, command: list[str], question: str, context: str, k: int, timeout: int
) -> dict[str, Any]:
    prompt = ANSWER_PROMPT.format(question=question, context=context, k=k)
    result = _invoke(command, prompt, workspace, timeout)
    text = extract_final_message(result["stdout"])
    combined = f"{result['stdout']}\n{result['stderr']}"
    return {
        "generated_answer": text,
        "exit_code": result["exit_code"],
        "elapsed_ms": result["elapsed_ms"],
        "agent_unavailable": result.get("agent_unavailable", False),
        "declared_insufficient": "DOCS_INSUFFICIENT" in text.upper(),
        "tokens": ev.extract_token_usage(combined),
        "cost_usd": ev.extract_reported_cost_usd(combined),
    }


def _parse_judge_json(text: str) -> dict[str, Any] | None:
    """Pull the verdict object out of judge stdout.

    `claude -p` returns prose, so the object may be fenced or prefixed. Scan for
    balanced brace spans and take the first one carrying a `label`.
    """
    for match in re.finditer(r"\{", text):
        depth = 0
        for idx in range(match.start(), len(text)):
            if text[idx] == "{":
                depth += 1
            elif text[idx] == "}":
                depth -= 1
                if depth == 0:
                    try:
                        obj = json.loads(text[match.start() : idx + 1])
                    except json.JSONDecodeError:
                        break
                    if isinstance(obj, dict) and "label" in obj:
                        return obj
                    break
    return None


def judge_answer(
    workspace: Path,
    command: list[str],
    question: str,
    reference: str,
    generated: str,
    coverage: str,
    timeout: int,
) -> dict[str, Any]:
    extra_schema = ""
    instruction = ""
    if coverage == "gap":
        extra_schema, instruction = GAP_SCHEMA, GAP_INSTRUCTION
    elif coverage == "partial":
        extra_schema, instruction = PARTIAL_SCHEMA, PARTIAL_INSTRUCTION

    prompt = JUDGE_PROMPT.format(
        question=question,
        reference=reference,
        generated=generated or "(no answer produced)",
        extra_schema=extra_schema,
    ) + instruction

    result = _invoke(command, prompt, workspace, timeout)
    # Unwrap streamed events first: the verdict object lives inside the final
    # message, not alongside the transport envelopes.
    verdict = _parse_judge_json(extract_final_message(result["stdout"]))
    if verdict is None:
        # Never silently score an unparseable verdict as WRONG — that would read
        # as a model failure when it is a harness failure.
        return {
            "judgment": None,
            "score": 0.0,
            "reason": "judge output was not parseable JSON",
            "judge_error": True,
            "judge_exit_code": result["exit_code"],
            "agent_unavailable": result.get("agent_unavailable", False),
        }

    label = str(verdict.get("label", "")).upper()
    out = {
        "judgment": label if label in {"CORRECT", "WRONG"} else None,
        "score": 1.0 if label == "CORRECT" else 0.0,
        "reason": verdict.get("reasoning", ""),
        "judge_error": label not in {"CORRECT", "WRONG"},
        "judge_exit_code": result["exit_code"],
        "agent_unavailable": result.get("agent_unavailable", False),
    }
    if coverage == "gap":
        out["gap_acknowledged"] = bool(verdict.get("gap_acknowledged"))
    if coverage == "partial":
        out["caveat_surfaced"] = bool(verdict.get("caveat_surfaced"))
    return out


# ---------------------------------------------------------------------------
# Pass rule
# ---------------------------------------------------------------------------


def record_passed(record: dict[str, Any], headline_k: int) -> bool:
    """Whether one question counts as a pass, stratified by doc coverage.

    * ``clean``   — the judge called the answer CORRECT.
    * ``partial`` — CORRECT *and* the documented caveat was surfaced.
    * ``gap``     — the docs cannot answer it, so passing means admitting that
      instead of confabulating. This is the hallucination-resistance metric.
    """
    if record.get("exit_code") != 0:
        return False
    cutoff = (record.get("cutoff_results") or {}).get(f"@{headline_k}")
    if not cutoff or cutoff.get("judge_error"):
        return False

    coverage = record.get("coverage")
    if coverage == "gap":
        return bool(cutoff.get("gap_acknowledged"))
    if coverage == "partial":
        return cutoff.get("judgment") == "CORRECT" and bool(cutoff.get("caveat_surfaced"))
    return cutoff.get("judgment") == "CORRECT"


def retrieval_only_passed(record: dict[str, Any], headline_k: int) -> bool:
    """Retrieval-only pass rule: did the gold entity make the top-k window."""
    metrics = (record.get("retrieval") or {}).get("metrics") or {}
    return bool((metrics.get(f"@{headline_k}") or {}).get("hit"))


# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------


def _mean(values: list[float]) -> float | None:
    clean = [v for v in values if v is not None]
    return round(statistics.mean(clean), 4) if clean else None


def summarize(records: list[dict[str, Any]], cutoffs: tuple[int, ...], headline_k: int) -> dict[str, Any]:
    scored = [r for r in records if not r.get("dry_run")]
    if not scored:
        return {"record_count": 0}

    def retrieval_block(subset: list[dict[str, Any]]) -> dict[str, Any]:
        block: dict[str, Any] = {}
        for k in cutoffs:
            block[f"@{k}"] = {
                "recall": _mean([
                    ((r.get("retrieval") or {}).get("metrics") or {}).get(f"@{k}", {}).get("recall")
                    for r in subset
                ]),
                "precision": _mean([
                    ((r.get("retrieval") or {}).get("metrics") or {}).get(f"@{k}", {}).get("precision")
                    for r in subset
                ]),
                "hit_rate": _mean([
                    1.0 if ((r.get("retrieval") or {}).get("metrics") or {}).get(f"@{k}", {}).get("hit")
                    else (None if ((r.get("retrieval") or {}).get("metrics") or {}).get(f"@{k}", {}).get("hit") is None else 0.0)
                    for r in subset
                ]),
            }
        block["mrr"] = _mean([(r.get("retrieval") or {}).get("metrics", {}).get("mrr") for r in subset])
        return block

    passes = [r for r in scored if r.get("passed")]
    summary: dict[str, Any] = {
        "record_count": len(scored),
        "pass_count": len(passes),
        "pass_rate": round(len(passes) / len(scored), 4),
        "headline_cutoff": headline_k,
        "cutoffs": list(cutoffs),
        "retrieval": retrieval_block(scored),
        "search_latency_ms_mean": _mean([
            (r.get("retrieval") or {}).get("search_latency_ms") for r in scored
        ]),
    }

    for field, key in (("category", "by_category"), ("coverage", "by_coverage")):
        grouped: dict[str, Any] = {}
        for value in sorted({str(r.get(field)) for r in scored if r.get(field)}):
            subset = [r for r in scored if r.get(field) == value]
            subset_passes = [r for r in subset if r.get("passed")]
            grouped[value] = {
                "records": len(subset),
                "passes": len(subset_passes),
                "pass_rate": round(len(subset_passes) / len(subset), 4),
                "retrieval": retrieval_block(subset),
            }
        summary[key] = grouped

    # Which retrieval channel surfaced the gold entity — C3's analogue of mem0
    # reporting semantic vs BM25 vs entity-boost contribution.
    channels: dict[str, int] = {}
    for record in scored:
        for source in (record.get("retrieval") or {}).get("gold_match_sources") or []:
            channel = source.split(":")[0]
            channels[channel] = channels.get(channel, 0) + 1
    summary["gold_match_channels"] = dict(sorted(channels.items()))

    answered = [r for r in scored if r.get("cutoff_results")]
    if answered:
        summary["accuracy_by_cutoff"] = {
            f"@{k}": _mean([
                (r.get("cutoff_results") or {}).get(f"@{k}", {}).get("score") for r in answered
            ])
            for k in cutoffs
        }
        summary["judge_errors"] = sum(
            1
            for r in answered
            for k in cutoffs
            if (r.get("cutoff_results") or {}).get(f"@{k}", {}).get("judge_error")
        )
        summary["tokens_total"] = sum(int(r.get("tokens_total") or 0) for r in answered)
        summary["cost_usd_total"] = round(sum(float(r.get("cost_usd") or 0.0) for r in answered), 4)
    return summary


# ---------------------------------------------------------------------------
# Runner
# ---------------------------------------------------------------------------


def run_item(
    workspace: Path,
    item: dict[str, Any],
    cutoffs: tuple[int, ...],
    headline_k: int,
    *,
    retrieval_only: bool,
    answer_cmd: list[str],
    judge_cmd: list[str],
    timeout: int,
    trial: int,
) -> dict[str, Any]:
    search = run_search(workspace, item["question"])
    gold = list(item.get("doc_trace") or [])
    metrics = score_retrieval(search["ranked_ids"], gold, cutoffs)

    gold_sources: list[str] = []
    for entity_id in gold:
        gold_sources.extend(search["match_sources"].get(entity_id, []))

    record: dict[str, Any] = {
        "bench": "c3_memory",
        "case": item["id"],
        "trial": trial,
        "category": item["category"],
        "coverage": item["coverage"],
        "question": item["question"],
        "exit_code": 0 if search["error"] is None else 1,
        "retrieval": {
            "ranked_ids": search["ranked_ids"],
            "gold_ids": gold,
            "gold_match_sources": gold_sources,
            "search_latency_ms": search["search_latency_ms"],
            "total_results": search["total_results"],
            "metrics": metrics,
            "error": search["error"],
        },
    }

    if retrieval_only:
        record["mode"] = "retrieval_only"
        record["passed"] = record["exit_code"] == 0 and retrieval_only_passed(record, headline_k)
        return record

    record["mode"] = "full"
    cutoff_results: dict[str, Any] = {}
    tokens_total = 0
    cost_usd = 0.0
    unavailable = False

    for k in cutoffs:
        context = build_context(workspace, search["ranked_ids"], k)
        answer = generate_answer(
            workspace, answer_cmd, item["question"], context, k, timeout
        )
        unavailable = unavailable or answer["agent_unavailable"]
        if answer["tokens"]:
            tokens_total += int(answer["tokens"].get("total_tokens") or 0)
        if answer["cost_usd"]:
            cost_usd += float(answer["cost_usd"])

        verdict = judge_answer(
            workspace,
            judge_cmd,
            item["question"],
            item["answer"],
            answer["generated_answer"],
            item["coverage"],
            timeout,
        )
        unavailable = unavailable or verdict.get("agent_unavailable", False)
        cutoff_results[f"@{k}"] = {
            "generated_answer": answer["generated_answer"],
            "memories_evaluated": min(k, len(search["ranked_ids"])),
            "declared_insufficient": answer["declared_insufficient"],
            "answer_exit_code": answer["exit_code"],
            "answer_elapsed_ms": answer["elapsed_ms"],
            **verdict,
        }
        if answer["exit_code"] != 0:
            record["exit_code"] = answer["exit_code"]

    record["cutoff_results"] = cutoff_results
    record["tokens_total"] = tokens_total or None
    record["cost_usd"] = round(cost_usd, 6) or None
    record["agent_unavailable"] = unavailable
    record["passed"] = record_passed(record, headline_k)
    return record


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="C3 Memory Bench — mem0-shaped retrieval+answer benchmark over the C3 gold set."
    )
    parser.add_argument("--run", action="store_true", help="actually execute (default is a dry plan)")
    parser.add_argument("--dry-run", action="store_true", help="plan only, spend nothing")
    parser.add_argument("--dataset", default=str(DEFAULT_DATASET))
    parser.add_argument("--workspace", default=str(REPO_ROOT))
    parser.add_argument("--case", action="append", dest="cases", help="restrict to case id(s)")
    parser.add_argument("--category", action="append", dest="categories")
    parser.add_argument(
        "--cutoff",
        default=",".join(str(k) for k in DEFAULT_CUTOFFS),
        help=f"comma-separated retrieval depths (full sweep: {','.join(str(k) for k in ALL_CUTOFFS)})",
    )
    parser.add_argument(
        "--retrieval-only",
        action="store_true",
        help="score retrieval only — deterministic and zero tokens (CI-safe)",
    )
    parser.add_argument("--repeat", type=int, default=1)
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT))
    parser.add_argument("--summary", default=None, help="optional path for the JSON summary")
    parser.add_argument("--skip-build", action="store_true", help="skip scripts/build.sh in stage 0")
    parser.add_argument(
        "--timeout",
        type=int,
        default=int(os.environ.get("C3_EVAL_TIMEOUT_SECONDS", "900")),
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    dry_run = args.dry_run or not args.run

    cutoffs = tuple(sorted({int(c) for c in args.cutoff.split(",") if c.strip()}))
    if not cutoffs:
        raise SystemExit("error: --cutoff must name at least one depth")
    headline_k = max(cutoffs) if len(cutoffs) == 1 else sorted(cutoffs)[len(cutoffs) // 2]

    items = load_dataset(Path(args.dataset))
    if args.cases:
        wanted = set(args.cases)
        items = [i for i in items if i["id"] in wanted]
    if args.categories:
        wanted = set(args.categories)
        items = [i for i in items if i["category"] in wanted]
    if not items:
        raise SystemExit("error: no dataset items matched the filters")

    skipped: list[str] = []
    if args.retrieval_only:
        # A `gap` question has no gold entity to retrieve, so retrieval-only has
        # nothing to score. Drop them loudly rather than pass or fail them by
        # accident — a silent drop would read as full coverage.
        gaps = [i["id"] for i in items if i["coverage"] == "gap"]
        if gaps:
            skipped = gaps
            items = [i for i in items if i["coverage"] != "gap"]

    answer_cmd = ev._env_command("C3_MEMBENCH_ANSWER_CMD", "claude -p {prompt}")
    judge_cmd = ev._env_command("C3_MEMBENCH_JUDGE_CMD", "claude -p {prompt}")

    workspace = Path(args.workspace).resolve()
    plan = [(item, trial) for trial in range(1, args.repeat + 1) for item in items]

    if dry_run:
        calls = 0 if args.retrieval_only else len(plan) * len(cutoffs) * 2
        print(f"C3 Memory Bench — dry run ({len(plan)} question-trials)")
        print(f"  dataset       : {args.dataset}")
        print(f"  mode          : {'retrieval-only (zero tokens)' if args.retrieval_only else 'full pipeline'}")
        print(f"  cutoffs       : {', '.join('@' + str(k) for k in cutoffs)} (headline @{headline_k})")
        print(f"  agent CLI calls: {calls}")
        if skipped:
            print(f"  skipped (gap, unscoreable without judge): {', '.join(skipped)}")
        for item, trial in plan:
            print(f"    {item['id']:<8} [{item['coverage']:<7}] {item['category']:<14} t{trial}  {item['question'][:60]}")
        return 0

    ingest = ensure_ingested(workspace, build=not args.skip_build)
    print(
        f"stage 0: ingested {ingest['entity_count']} entities in {ingest['ingest_ms']}ms"
        + ("" if ingest["built"] else " (build skipped)")
    )
    if skipped:
        print(f"note: skipped {len(skipped)} gap question(s) in retrieval-only mode: {', '.join(skipped)}")

    records = []
    for item, trial in plan:
        record = run_item(
            workspace,
            item,
            cutoffs,
            headline_k,
            retrieval_only=args.retrieval_only,
            answer_cmd=answer_cmd,
            judge_cmd=judge_cmd,
            timeout=args.timeout,
            trial=trial,
        )
        record["ingest"] = ingest
        records.append(record)
        mark = "PASS" if record["passed"] else "FAIL"
        rank = (record["retrieval"]["metrics"] or {}).get("first_hit_rank")
        print(f"  {mark}  {item['id']:<8} [{item['coverage']:<7}] gold_rank={rank}")

    out_path = Path(args.output)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with out_path.open("w") as handle:
        for record in records:
            handle.write(json.dumps(record) + "\n")

    summary = summarize(records, cutoffs, headline_k)
    if skipped:
        summary["skipped_gap_cases"] = skipped
    if args.summary:
        Path(args.summary).write_text(json.dumps(summary, indent=2))

    print("\n" + json.dumps(summary, indent=2))
    print(f"\nwrote {len(records)} records to {out_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

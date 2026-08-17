import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import c3_memory_bench as bench
import eval_gate as gate


def item(qa_id="qa-01", *, coverage="clean", doc_trace=None, category="navigation"):
    """One dataset entry shaped like codebase-qa.json emits."""
    if doc_trace is None:
        doc_trace = [] if coverage == "gap" else ["c3-104"]
    return {
        "id": qa_id,
        "category": category,
        "question": "Where is the change-apply saga?",
        "answer": "It lives in the changeset package.",
        "doc_trace": doc_trace,
        "code_trace": ["cli/internal/changeset/**"],
        "via": ["c3x read c3-104"],
        "coverage": coverage,
    }


def write_dataset(entries):
    handle = tempfile.NamedTemporaryFile("w", suffix=".json", delete=False)
    json.dump(entries, handle)
    handle.close()
    return Path(handle.name)


class DatasetValidationTests(unittest.TestCase):
    """The gold set is hand-authored, so malformed entries must fail loudly."""

    def test_loads_valid_dataset(self):
        path = write_dataset([item(), item("qa-02", coverage="gap")])
        loaded = bench.load_dataset(path)
        self.assertEqual([i["id"] for i in loaded], ["qa-01", "qa-02"])

    def test_rejects_missing_field(self):
        broken = item()
        del broken["coverage"]
        with self.assertRaises(SystemExit) as ctx:
            bench.load_dataset(write_dataset([broken]))
        self.assertIn("missing fields", str(ctx.exception))

    def test_rejects_unknown_coverage(self):
        with self.assertRaises(SystemExit) as ctx:
            bench.load_dataset(write_dataset([item(coverage="mostly")]))
        self.assertIn("coverage", str(ctx.exception))

    def test_rejects_gap_carrying_doc_trace(self):
        """A gap means the docs do not cover it — a doc_trace contradicts that."""
        with self.assertRaises(SystemExit) as ctx:
            bench.load_dataset(write_dataset([item(coverage="gap", doc_trace=["c3-104"])]))
        self.assertIn("non-empty doc_trace", str(ctx.exception))

    def test_rejects_clean_without_doc_trace(self):
        with self.assertRaises(SystemExit) as ctx:
            bench.load_dataset(write_dataset([item(coverage="clean", doc_trace=[])]))
        self.assertIn("empty doc_trace", str(ctx.exception))

    def test_rejects_empty_dataset(self):
        with self.assertRaises(SystemExit):
            bench.load_dataset(write_dataset([]))


class RetrievalScoringTests(unittest.TestCase):
    def test_perfect_rank_one(self):
        m = bench.score_retrieval(["c3-104", "c3-102"], ["c3-104"], (1, 3))
        self.assertEqual(m["@1"], {"recall": 1.0, "precision": 1.0, "hit": True})
        self.assertEqual(m["mrr"], 1.0)
        self.assertEqual(m["first_hit_rank"], 1)

    def test_miss_at_shallow_cutoff_hit_at_deep(self):
        ranked = ["a", "b", "c", "d", "c3-104"]
        m = bench.score_retrieval(ranked, ["c3-104"], (1, 5))
        self.assertFalse(m["@1"]["hit"])
        self.assertEqual(m["@1"]["recall"], 0.0)
        self.assertTrue(m["@5"]["hit"])
        self.assertEqual(m["@5"]["recall"], 1.0)
        self.assertEqual(m["mrr"], 0.2)

    def test_partial_recall_across_multiple_gold(self):
        m = bench.score_retrieval(["c3-104", "x", "y"], ["c3-104", "c3-112"], (3,))
        self.assertEqual(m["@3"]["recall"], 0.5)
        self.assertAlmostEqual(m["@3"]["precision"], 0.3333, places=3)

    def test_gap_yields_none_not_zero(self):
        """Averaging a zero for an unanswerable question would understate recall."""
        m = bench.score_retrieval(["a", "b"], [], (1, 5))
        self.assertIsNone(m["mrr"])
        self.assertIsNone(m["@5"]["recall"])
        self.assertIsNone(m["@5"]["hit"])

    def test_no_hit_anywhere(self):
        m = bench.score_retrieval(["a", "b"], ["c3-104"], (5,))
        self.assertEqual(m["mrr"], 0.0)
        self.assertIsNone(m["first_hit_rank"])


class JudgeParsingTests(unittest.TestCase):
    """`claude -p` returns prose, so the verdict object arrives wrapped."""

    def test_bare_object(self):
        parsed = bench._parse_judge_json('{"label": "CORRECT", "reasoning": "ok"}')
        self.assertEqual(parsed["label"], "CORRECT")

    def test_fenced_with_prose_around_it(self):
        text = 'Here is my verdict:\n```json\n{"label": "WRONG", "reasoning": "missed the saga"}\n```\nDone.'
        self.assertEqual(bench._parse_judge_json(text)["label"], "WRONG")

    def test_skips_leading_object_without_label(self):
        text = '{"note": "thinking"} then {"label": "CORRECT", "reasoning": "y"}'
        self.assertEqual(bench._parse_judge_json(text)["label"], "CORRECT")

    def test_nested_braces(self):
        text = '{"label": "CORRECT", "reasoning": "a", "meta": {"k": "v"}}'
        self.assertEqual(bench._parse_judge_json(text)["meta"]["k"], "v")

    def test_unparseable_returns_none(self):
        self.assertIsNone(bench._parse_judge_json("no json here at all"))
        self.assertIsNone(bench._parse_judge_json("{not: valid}"))


class FinalMessageExtractionTests(unittest.TestCase):
    """Agent CLIs disagree on output shape; the answer must survive all of them."""

    def test_plain_prose_passes_through(self):
        self.assertEqual(bench.extract_final_message("The saga lives in changeset."),
                         "The saga lives in changeset.")

    def test_codex_json_events(self):
        stream = "\n".join([
            '{"type":"thread.started","thread_id":"t1"}',
            '{"type":"item.completed","item":{"type":"agent_message","text":"It lives in changeset."}}',
            '{"type":"turn.completed","usage":{"input_tokens":10}}',
        ])
        self.assertEqual(bench.extract_final_message(stream), "It lives in changeset.")

    def test_claude_stream_json(self):
        stream = "\n".join([
            '{"type":"assistant","message":{"content":[{"type":"text","text":"Answer A"}]}}',
            '{"type":"result","result":"Answer B"}',
        ])
        self.assertEqual(bench.extract_final_message(stream), "Answer B")

    def test_error_events_are_not_answers(self):
        stream = '{"type":"error","message":"401 Unauthorized"}\n{"type":"turn.failed"}'
        self.assertEqual(bench.extract_final_message(stream), "")

    def test_bare_verdict_object_survives(self):
        """The judge replies with its verdict object directly — do not swallow it."""
        raw = '{"label": "CORRECT", "reasoning": "matches"}'
        self.assertEqual(bench.extract_final_message(raw), raw)
        self.assertEqual(bench._parse_judge_json(bench.extract_final_message(raw))["label"], "CORRECT")

    def test_empty(self):
        self.assertEqual(bench.extract_final_message(""), "")


def stub_cli(payload: str):
    """A fake agent CLI that ignores the prompt and prints `payload`."""
    script = tempfile.NamedTemporaryFile("w", suffix=".sh", delete=False)
    script.write("#!/bin/sh\ncat <<'STUB_EOF'\n" + payload + "\nSTUB_EOF\n")
    script.close()
    Path(script.name).chmod(0o755)
    return ["sh", script.name, "{prompt}"]


class StubbedPipelineTests(unittest.TestCase):
    """Stages 2-3 end-to-end against a stub CLI — no model, no tokens."""

    def test_answer_extraction_and_insufficiency_flag(self):
        cmd = stub_cli('{"type":"item.completed","item":{"type":"agent_message","text":"DOCS_INSUFFICIENT\nno component covers it"}}')
        out = bench.generate_answer(Path.cwd(), cmd, "q?", "ctx", 5, 30)
        self.assertEqual(out["exit_code"], 0)
        self.assertTrue(out["declared_insufficient"])
        self.assertIn("no component covers it", out["generated_answer"])

    def test_judge_returns_structured_verdict(self):
        cmd = stub_cli('{"label": "CORRECT", "reasoning": "conveys the saga"}')
        out = bench.judge_answer(Path.cwd(), cmd, "q?", "ref", "gen", "clean", 30)
        self.assertEqual(out["judgment"], "CORRECT")
        self.assertEqual(out["score"], 1.0)
        self.assertFalse(out["judge_error"])

    def test_judge_gap_field_captured(self):
        cmd = stub_cli('{"label": "WRONG", "reasoning": "declined", "gap_acknowledged": true}')
        out = bench.judge_answer(Path.cwd(), cmd, "q?", "ref", "gen", "gap", 30)
        self.assertTrue(out["gap_acknowledged"])

    def test_unparseable_judge_is_an_error_not_a_wrong(self):
        """A harness failure must never masquerade as a model getting it wrong."""
        cmd = stub_cli("I think it is correct, honestly.")
        out = bench.judge_answer(Path.cwd(), cmd, "q?", "ref", "gen", "clean", 30)
        self.assertTrue(out["judge_error"])
        self.assertIsNone(out["judgment"])

    def test_missing_cli_is_flagged_unavailable(self):
        out = bench.generate_answer(Path.cwd(), ["definitely-not-a-real-binary"], "q?", "c", 5, 30)
        self.assertTrue(out["agent_unavailable"])
        self.assertEqual(out["exit_code"], 127)


class PassRuleTests(unittest.TestCase):
    def record(self, coverage, cutoff, exit_code=0):
        return {
            "coverage": coverage,
            "exit_code": exit_code,
            "cutoff_results": {"@5": cutoff},
        }

    def test_clean_passes_on_correct(self):
        self.assertTrue(bench.record_passed(self.record("clean", {"judgment": "CORRECT"}), 5))
        self.assertFalse(bench.record_passed(self.record("clean", {"judgment": "WRONG"}), 5))

    def test_partial_requires_caveat(self):
        correct_no_caveat = {"judgment": "CORRECT", "caveat_surfaced": False}
        correct_caveat = {"judgment": "CORRECT", "caveat_surfaced": True}
        self.assertFalse(bench.record_passed(self.record("partial", correct_no_caveat), 5))
        self.assertTrue(bench.record_passed(self.record("partial", correct_caveat), 5))

    def test_gap_passes_only_by_admitting_the_gap(self):
        """Confabulating a documented answer must fail even if it reads well."""
        confabulated = {"judgment": "CORRECT", "gap_acknowledged": False}
        admitted = {"judgment": "WRONG", "gap_acknowledged": True}
        self.assertFalse(bench.record_passed(self.record("gap", confabulated), 5))
        self.assertTrue(bench.record_passed(self.record("gap", admitted), 5))

    def test_judge_error_never_passes(self):
        broken = {"judgment": None, "judge_error": True}
        self.assertFalse(bench.record_passed(self.record("clean", broken), 5))

    def test_nonzero_exit_never_passes(self):
        ok = {"judgment": "CORRECT"}
        self.assertFalse(bench.record_passed(self.record("clean", ok, exit_code=1), 5))

    def test_missing_headline_cutoff_fails(self):
        rec = {"coverage": "clean", "exit_code": 0, "cutoff_results": {"@1": {"judgment": "CORRECT"}}}
        self.assertFalse(bench.record_passed(rec, 5))

    def test_retrieval_only_rule_follows_hit(self):
        hit = {"retrieval": {"metrics": {"@5": {"hit": True}}}}
        miss = {"retrieval": {"metrics": {"@5": {"hit": False}}}}
        self.assertTrue(bench.retrieval_only_passed(hit, 5))
        self.assertFalse(bench.retrieval_only_passed(miss, 5))


class SummaryTests(unittest.TestCase):
    def rec(self, qa_id, *, passed, coverage="clean", category="navigation", recall=1.0, hit=True):
        return {
            "case": qa_id,
            "coverage": coverage,
            "category": category,
            "passed": passed,
            "retrieval": {
                "search_latency_ms": 400,
                "gold_match_sources": ["semantic", "graph:uses:ref-x"],
                "metrics": {"mrr": 1.0, "@5": {"recall": recall, "precision": 0.2, "hit": hit}},
            },
        }

    def test_pass_rate_and_breakdowns(self):
        records = [
            self.rec("qa-01", passed=True),
            self.rec("qa-02", passed=False, recall=0.0, hit=False),
            self.rec("qa-03", passed=True, category="trust"),
        ]
        s = bench.summarize(records, (5,), 5)
        self.assertEqual(s["record_count"], 3)
        self.assertAlmostEqual(s["pass_rate"], 0.6667, places=3)
        self.assertEqual(s["by_category"]["trust"]["pass_rate"], 1.0)
        self.assertAlmostEqual(s["retrieval"]["@5"]["recall"], 0.6667, places=3)

    def test_gold_match_channels_strip_edge_detail(self):
        """graph:uses:ref-x and graph:cites:ref-y are both the graph channel."""
        s = bench.summarize([self.rec("qa-01", passed=True)], (5,), 5)
        self.assertEqual(s["gold_match_channels"], {"graph": 1, "semantic": 1})

    def test_gap_records_do_not_drag_recall_down(self):
        records = [
            self.rec("qa-01", passed=True),
            {
                "case": "qa-11",
                "coverage": "gap",
                "category": "change-impact",
                "passed": True,
                "retrieval": {
                    "search_latency_ms": 400,
                    "gold_match_sources": [],
                    "metrics": {"mrr": None, "@5": {"recall": None, "precision": None, "hit": None}},
                },
            },
        ]
        s = bench.summarize(records, (5,), 5)
        self.assertEqual(s["retrieval"]["@5"]["recall"], 1.0)

    def test_empty_is_safe(self):
        self.assertEqual(bench.summarize([], (5,), 5), {"record_count": 0})


class GateIntegrationTests(unittest.TestCase):
    """The `passed` short-circuit must serve this bench without disturbing the frozen one."""

    def test_gate_honours_bench_verdict(self):
        self.assertTrue(gate.record_passes_quality({"exit_code": 0, "passed": True}))
        self.assertFalse(gate.record_passes_quality({"exit_code": 0, "passed": False}))

    def test_bench_verdict_beats_missing_accuracy_score(self):
        """Memory-bench records carry no accuracy_score; the old rule would fail them."""
        self.assertTrue(
            gate.record_passes_quality({"exit_code": 0, "passed": True, "case": "qa-01"})
        )

    def test_nonzero_exit_still_wins(self):
        self.assertFalse(gate.record_passes_quality({"exit_code": 1, "passed": True}))

    def test_legacy_records_unchanged(self):
        """No `passed` key -> the frozen harness's path, bit-identical."""
        legacy = {"exit_code": 0, "accuracy_score": 1.0, "case": "task_session"}
        self.assertTrue(gate.record_passes_quality(legacy))
        legacy_fail = {"exit_code": 0, "accuracy_score": 0.5, "case": "task_session"}
        self.assertFalse(gate.record_passes_quality(legacy_fail))


class CliTests(unittest.TestCase):
    def test_dry_run_is_the_default(self):
        args = bench.parse_args([])
        self.assertFalse(args.run)

    def test_cutoff_parsing_dedups_and_sorts(self):
        args = bench.parse_args(["--cutoff", "10,1,5,1"])
        self.assertEqual(
            tuple(sorted({int(c) for c in args.cutoff.split(",")})), (1, 5, 10)
        )

    def test_dry_run_reports_call_count(self):
        dataset = write_dataset([item("qa-01"), item("qa-02")])
        rc = bench.main(["--dry-run", "--dataset", str(dataset)])
        self.assertEqual(rc, 0)

    def test_retrieval_only_dry_run_costs_nothing(self):
        dataset = write_dataset([item("qa-01")])
        rc = bench.main(["--dry-run", "--retrieval-only", "--dataset", str(dataset)])
        self.assertEqual(rc, 0)

    def test_case_filter_rejects_empty_match(self):
        dataset = write_dataset([item("qa-01")])
        with self.assertRaises(SystemExit):
            bench.main(["--dry-run", "--dataset", str(dataset), "--case", "qa-99"])


if __name__ == "__main__":
    unittest.main()

import json
import os
import re
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]
# Set by the release workflow to the version its tag names. Every surface is
# compared to the manifest, so pinning the manifest pins the whole set.
EXPECTED_VERSION = os.environ.get("C3_EXPECTED_VERSION")
MANIFEST = REPO_ROOT / ".release-please-manifest.json"
CONFIG = REPO_ROOT / "release-please-config.json"
VERSION_FILE = REPO_ROOT / "skills" / "c3" / "bin" / "VERSION"
AST_GREP_VERSION_FILE = REPO_ROOT / "skills" / "c3" / "bin" / "AST_GREP_VERSION"
VERSION_TS = REPO_ROOT / "packages" / "cli" / "src" / "version.ts"

MARKER = "x-release-please-version"
SEMVER = re.compile(r"^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$")

# Every surface release-please must move, and how the version is spelled there.
# `generic` surfaces carry an inline marker; `json` surfaces are addressed by jsonpath.
GENERIC_SURFACES = ("skills/c3/bin/VERSION", "packages/cli/src/version.ts")
JSON_SURFACES = {
    ".claude-plugin/plugin.json": ("$.version",),
    ".claude-plugin/marketplace.json": ("$.plugins[0].version",),
    "packages/cli/package.json": ("$.version",),
    "packages/cli/package-lock.json": ("$.version", "$.packages[''].version"),
}

# Everything that reads skills/c3/bin/VERSION must take the first whitespace-separated
# token, never the whole file, now that the file carries a marker. README.md is in here
# because its build snippet is meant to be pasted into a shell.
VERSION_READERS = (
    "skills/c3/bin/c3x.sh",
    "scripts/build.sh",
    "scripts/paired_skill_eval.py",
    "scripts/migrate_c3_project.py",
    "research/eval/skill-eval/harness/bin/run-blindbox.sh",
    ".github/workflows/release.yml",
    ".c3/eval/c3-301.yaml",
    ".c3/eval/ref-fat-thin-distribution.yaml",
    "README.md",
)
WHOLE_FILE_READS = (
    re.compile(r"tr -d '\[:space:\]'\s*<\s*\"?[^\"\n]*VERSION"),
    re.compile(r"VERSION[^\n]*\.read_text\([^)]*\)\s*\.strip\(\)"),
    re.compile(r"cat\s+\S*skills/c3/bin/VERSION"),
    re.compile(r"file:\s*\S*skills/c3/bin/VERSION"),  # an eval binding's whole-file gather
)

# jsonpath token grammar release-please's GenericJson needs here. Anything outside
# it is a parse failure, not a silent skip — an unparseable path must never pass.
_TOKEN = re.compile(r"\.([A-Za-z_][\w-]*)|\['((?:[^'\\]|\\.)*)'\]|\[(\d+)\]")


def resolve_jsonpath(path, document):
    """Resolve a release-please jsonpath, returning every match.

    Raises ValueError on any syntax this resolver does not model, so a jsonpath
    the guard cannot verify fails the test instead of passing vacuously.
    """
    if not path.startswith("$"):
        raise ValueError(f"jsonpath must start with '$': {path!r}")
    cursor = 1
    current = [document]
    while cursor < len(path):
        match = _TOKEN.match(path, cursor)
        if match is None:
            raise ValueError(f"unsupported jsonpath syntax at offset {cursor}: {path!r}")
        dotted, quoted, index = match.groups()
        key = dotted if dotted is not None else quoted
        nxt = []
        for node in current:
            if key is not None:
                if isinstance(node, dict) and key in node:
                    nxt.append(node[key])
            else:
                position = int(index)
                if isinstance(node, list) and position < len(node):
                    nxt.append(node[position])
        current = nxt
        cursor = match.end()
    return current


def read_json(relative):
    return json.loads((REPO_ROOT / relative).read_text(encoding="utf-8"))


class ReleaseVersionSurfacesTest(unittest.TestCase):
    def setUp(self):
        self.assertTrue(MANIFEST.exists(), f"{MANIFEST.name} is missing")
        self.assertTrue(CONFIG.exists(), f"{CONFIG.name} is missing")
        self.manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))
        self.config = json.loads(CONFIG.read_text(encoding="utf-8"))
        self.package = self.config.get("packages", {}).get(".", {})
        self.extra_files = self.package.get("extra-files", [])
        self.version = self.manifest.get(".")

    def test_manifest_declares_exactly_the_root_package(self):
        self.assertEqual(set(self.manifest), {"."})
        self.assertRegex(self.version, SEMVER)

    def test_manifest_matches_the_version_the_release_tag_names(self):
        if EXPECTED_VERSION is None:
            self.skipTest("C3_EXPECTED_VERSION is unset; nothing pins the manifest here")
        self.assertEqual(self.version, EXPECTED_VERSION)

    def test_release_please_tags_without_a_component_prefix(self):
        self.assertIs(
            self.package.get("include-component-in-tag"),
            False,
            "include-component-in-tag defaults to true, which would tag c3-skill-vX.Y.Z "
            "and break the v* convention every release asset resolves against",
        )

    def test_every_version_surface_equals_the_manifest_version(self):
        for relative, jsonpaths in JSON_SURFACES.items():
            document = read_json(relative)
            for jsonpath in jsonpaths:
                with self.subTest(surface=relative, jsonpath=jsonpath):
                    matches = resolve_jsonpath(jsonpath, document)
                    self.assertEqual(len(matches), 1, f"{relative} {jsonpath} matched {len(matches)}")
                    self.assertEqual(matches[0], self.version)

        self.assertEqual(VERSION_FILE.read_text(encoding="utf-8").split()[0], self.version)
        declared = re.search(r"C3X_VERSION\s*=\s*'([^']+)'", VERSION_TS.read_text(encoding="utf-8"))
        self.assertIsNotNone(declared, "version.ts does not declare C3X_VERSION")
        self.assertEqual(declared.group(1), self.version)

    def test_config_declares_an_updater_for_every_version_surface(self):
        declared_generic = {e["path"] for e in self.extra_files if e.get("type") == "generic"}
        self.assertEqual(declared_generic, set(GENERIC_SURFACES))

        declared_json = {}
        for entry in self.extra_files:
            if entry.get("type") == "json":
                declared_json.setdefault(entry["path"], set()).add(entry["jsonpath"])
        self.assertEqual(declared_json, {p: set(v) for p, v in JSON_SURFACES.items()})

    def test_declared_jsonpaths_resolve_to_a_single_semver_string(self):
        for entry in self.extra_files:
            if entry.get("type") != "json":
                continue
            with self.subTest(surface=entry["path"], jsonpath=entry["jsonpath"]):
                matches = resolve_jsonpath(entry["jsonpath"], read_json(entry["path"]))
                self.assertEqual(
                    len(matches),
                    1,
                    "GenericJson silently no-ops when a jsonpath matches nothing",
                )
                self.assertIsInstance(matches[0], str)
                self.assertRegex(matches[0], SEMVER)

    def test_config_declares_no_updater_outside_the_known_surfaces(self):
        known = set(GENERIC_SURFACES) | set(JSON_SURFACES)
        for entry in self.extra_files:
            self.assertIn(entry["path"], known)

    def test_generic_surfaces_carry_one_marked_version_line(self):
        for relative in GENERIC_SURFACES:
            with self.subTest(surface=relative):
                lines = (REPO_ROOT / relative).read_text(encoding="utf-8").splitlines()
                marked = [line for line in lines if MARKER in line]
                self.assertEqual(len(marked), 1, "the Generic updater rewrites every marked line")
                self.assertIn(self.version, marked[0])

    def test_ast_grep_version_is_not_release_please_managed(self):
        self.assertNotIn("x-release-please", AST_GREP_VERSION_FILE.read_text(encoding="utf-8"))
        for entry in self.extra_files:
            self.assertNotIn("AST_GREP_VERSION", entry["path"])

        pinned = re.search(r"AST_GREP_VERSION\s*=\s*'([^']+)'", VERSION_TS.read_text(encoding="utf-8"))
        self.assertIsNotNone(pinned)
        self.assertEqual(pinned.group(1), AST_GREP_VERSION_FILE.read_text(encoding="utf-8").split()[0])

    def test_no_reader_parses_the_whole_version_file(self):
        for relative in VERSION_READERS:
            content = (REPO_ROOT / relative).read_text(encoding="utf-8")
            for pattern in WHOLE_FILE_READS:
                with self.subTest(reader=relative, pattern=pattern.pattern):
                    self.assertIsNone(pattern.search(content))

    def test_release_workflow_no_longer_creates_the_github_release(self):
        release = (REPO_ROOT / ".github" / "workflows" / "release.yml").read_text(encoding="utf-8")
        self.assertNotIn("gh release create", release)
        self.assertIn("workflow_call:", release)
        self.assertNotIn("branches: [main]", release)

        distribute = (REPO_ROOT / ".github" / "workflows" / "distribute.yml").read_text(encoding="utf-8")
        self.assertNotIn("tags:", distribute)

    def test_only_the_workflow_call_chain_builds_a_release(self):
        """The PAT makes release-please's events propagate, so a `release:` trigger
        on release.yml would duplicate the chain release-please.yml already invokes."""
        release = (REPO_ROOT / ".github" / "workflows" / "release.yml").read_text(encoding="utf-8")
        triggers = release.split("permissions:", 1)[0]
        self.assertNotIn("release:", triggers)
        self.assertNotIn("github.event.release", release)

    def test_release_please_runs_under_a_token_whose_events_propagate(self):
        """GITHUB_TOKEN raises no pull_request event, so ci.yml would never gate the
        Release PR — the one PR that carries release-please's own output."""
        rp = (REPO_ROOT / ".github" / "workflows" / "release-please.yml").read_text(encoding="utf-8")
        self.assertIn("secrets.RELEASE_PLEASE_TOKEN", rp)
        self.assertNotIn("secrets.GITHUB_TOKEN", rp)

    def test_pull_requests_run_the_version_surface_gate(self):
        ci = (REPO_ROOT / ".github" / "workflows" / "ci.yml").read_text(encoding="utf-8")
        self.assertIn("pull_request:", ci)
        self.assertIn("scripts/test_release_version_surfaces.py", ci)

    def test_skill_frontmatter_stays_installable_by_the_skills_cli(self):
        """vercel-labs/skills reads only name+description, and refuses a ---js fence."""
        text = (REPO_ROOT / "skills" / "c3" / "SKILL.md").read_text(encoding="utf-8")
        self.assertTrue(text.startswith("---\n"), "frontmatter must open with a plain --- fence")
        body = text[4:]
        end = body.find("\n---")
        self.assertNotEqual(end, -1, "frontmatter is not terminated")
        frontmatter = body[:end]
        for field in ("name", "description"):
            self.assertRegex(frontmatter, rf"(?m)^{field}:\s*\S", f"SKILL.md is skipped without {field}")


if __name__ == "__main__":
    unittest.main()

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func reportFixture(t *testing.T) string {
	t.Helper()
	c3Dir := filepath.Join(t.TempDir(), ".c3")
	if err := os.MkdirAll(c3Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return c3Dir
}

func buildReport(t *testing.T, opts ReportOptions) reportEnvelope {
	t.Helper()
	if opts.Version == "" {
		opts.Version = "11.12.0"
	}
	env, err := buildReportEnvelope(opts)
	if err != nil {
		t.Fatalf("buildReportEnvelope: %v", err)
	}
	return env
}

func TestReportFingerprintIgnoresVersion(t *testing.T) {
	c3Dir := reportFixture(t)
	base := ReportOptions{C3Dir: c3Dir, Args: []string{"gap"}, Subject: "diff", Summary: "no way to diff two facts"}

	old := buildReport(t, ReportOptions{C3Dir: base.C3Dir, Args: base.Args, Subject: base.Subject, Summary: base.Summary, Version: "11.1.0"})
	new := buildReport(t, ReportOptions{C3Dir: base.C3Dir, Args: base.Args, Subject: base.Subject, Summary: base.Summary, Version: "12.4.2"})

	if old.Fingerprint != new.Fingerprint {
		t.Fatalf("fingerprint must survive a version bump so a recurrence dedupes onto one issue: %q vs %q", old.Fingerprint, new.Fingerprint)
	}
	if old.Fingerprint == "" {
		t.Fatal("fingerprint must not be empty")
	}
}

func TestReportFingerprintSeparatesDistinctReports(t *testing.T) {
	c3Dir := reportFixture(t)
	seen := map[string]string{}
	cases := []ReportOptions{
		{Args: []string{"gap"}, Subject: "diff", Summary: "no way to diff two facts"},
		{Args: []string{"gap"}, Subject: "diff", Summary: "cannot list staged patches"},
		{Args: []string{"gap"}, Subject: "search", Summary: "no way to diff two facts"},
		{Args: []string{"guidance"}, Subject: "diff", Summary: "no way to diff two facts"},
	}
	for _, tc := range cases {
		tc.C3Dir = c3Dir
		env := buildReport(t, tc)
		label := strings.Join(append(tc.Args, tc.Subject, tc.Summary), "/")
		if prev, dup := seen[env.Fingerprint]; dup {
			t.Fatalf("fingerprint collision between %q and %q", prev, label)
		}
		seen[env.Fingerprint] = label
	}
}

func TestReportFingerprintSurvivesShiftingErrorDetail(t *testing.T) {
	c3Dir := reportFixture(t)
	appendFault(t, c3Dir, "check", "error: index out of range [3] with length 2 in /Users/ada/proj/.c3/refs/a.md")
	first := buildReport(t, ReportOptions{C3Dir: c3Dir, Args: []string{"fault"}, Summary: "seal validation panics"})

	c3Dir2 := reportFixture(t)
	appendFault(t, c3Dir2, "check", "error: index out of range [7] with length 5 in /home/bob/other/.c3/refs/z.md")
	second := buildReport(t, ReportOptions{C3Dir: c3Dir2, Args: []string{"fault"}, Summary: "seal validation panics"})

	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("the same fault with different indices and paths must share a fingerprint: %q vs %q", first.Fingerprint, second.Fingerprint)
	}
}

func TestReportSubjectRequiredForProseKinds(t *testing.T) {
	c3Dir := reportFixture(t)
	for _, kind := range []string{"guidance", "gap"} {
		_, err := buildReportEnvelope(ReportOptions{C3Dir: c3Dir, Version: "11.12.0", Args: []string{kind}, Summary: "something is off"})
		if err == nil {
			t.Fatalf("%s without --subject must be refused: a stable fingerprint needs a subject", kind)
		}
		if !strings.Contains(err.Error(), "hint:") {
			t.Errorf("%s refusal must carry a hint, got: %v", kind, err)
		}
	}
}

func TestReportSummaryRequired(t *testing.T) {
	c3Dir := reportFixture(t)
	_, err := buildReportEnvelope(ReportOptions{C3Dir: c3Dir, Version: "11.12.0", Args: []string{"fault"}})
	if err == nil {
		t.Fatal("a report without --summary must be refused")
	}
	if !strings.Contains(err.Error(), "hint:") {
		t.Errorf("refusal must carry a hint, got: %v", err)
	}
}

func TestReportUnknownKindRefused(t *testing.T) {
	c3Dir := reportFixture(t)
	_, err := buildReportEnvelope(ReportOptions{C3Dir: c3Dir, Version: "11.12.0", Args: []string{"typo"}, Summary: "x"})
	if err == nil {
		t.Fatal("an unknown kind must be refused")
	}
	if !strings.Contains(err.Error(), "fault") || !strings.Contains(err.Error(), "hint:") {
		t.Errorf("refusal must name the legal kinds in a hint, got: %v", err)
	}
}

func TestReportRedactsFilesystemPaths(t *testing.T) {
	c3Dir := reportFixture(t)
	appendFault(t, c3Dir, "check", "error: cannot read /Users/ada/secret-client/.c3/refs/pricing.md")

	env := buildReport(t, ReportOptions{
		C3Dir:   c3Dir,
		Args:    []string{"fault"},
		Summary: "check cannot read a ref under /Users/ada/secret-client",
		Detail:  "reproduced from /home/ada/work/private-repo",
	})

	for _, leak := range []string{"/Users/ada", "secret-client", "/home/ada", "private-repo"} {
		if strings.Contains(env.Body, leak) {
			t.Errorf("body leaks %q into a public tracker:\n%s", leak, env.Body)
		}
	}
	if !strings.Contains(env.Body, "pricing.md") {
		t.Errorf("redaction must keep the basename so the report stays reproducible:\n%s", env.Body)
	}
	if len(env.Redacted) == 0 {
		t.Error("envelope must declare what it redacted so the consent gate is informed")
	}
}

func TestReportKeepsEntityIdsIntact(t *testing.T) {
	c3Dir := reportFixture(t)
	env := buildReport(t, ReportOptions{
		C3Dir:   c3Dir,
		Args:    []string{"guidance"},
		Subject: "change.md",
		Summary: "change.md says c3-202 accepts a frontmatter uses: patch, apply rejects it",
	})
	if !strings.Contains(env.Body, "c3-202") {
		t.Errorf("entity ids are structural and carry the reproduction value — they must survive redaction:\n%s", env.Body)
	}
}

func TestReportFaultAttachesLastFailureAndSkipsItsOwnRows(t *testing.T) {
	c3Dir := reportFixture(t)
	appendActivityRow(t, c3Dir, "add", nil, true, "")
	appendFault(t, c3Dir, "change", "error: change apply: 2 gate failure(s)")
	appendActivityRow(t, c3Dir, "report", nil, false, "error: report went wrong")

	env := buildReport(t, ReportOptions{C3Dir: c3Dir, Args: []string{"fault"}, Summary: "apply gate misfires"})

	if !strings.Contains(env.Body, "gate failure(s)") {
		t.Errorf("fault must attach the last real failure:\n%s", env.Body)
	}
	if strings.Contains(env.Body, "report went wrong") {
		t.Errorf("report must not attach its own activity rows:\n%s", env.Body)
	}
	if !strings.Contains(env.Title, "change") {
		t.Errorf("a fault title must name the failing command, got %q", env.Title)
	}
}

// A gap or guidance report is grounded in what the docs and CLI say, not in
// what just ran. Attaching an unrelated failure and the command trail is noise
// on a public tracker and exposure of a workflow the report is not about.
func TestReportProseKindsCarryNoActivityTrail(t *testing.T) {
	c3Dir := reportFixture(t)
	appendFault(t, c3Dir, "add", "error: content validation failed for component-widget")
	appendActivityRow(t, c3Dir, "check", nil, true, "")

	for _, kind := range []string{"guidance", "gap"} {
		env := buildReport(t, ReportOptions{C3Dir: c3Dir, Args: []string{kind}, Subject: "diff", Summary: "no way to diff two facts"})
		for _, leak := range []string{"Failing command", "Recent activity", "component-widget"} {
			if strings.Contains(env.Body, leak) {
				t.Errorf("%s report must not carry %q:\n%s", kind, leak, env.Body)
			}
		}
	}
}

func TestReportWorksWithoutAnyProject(t *testing.T) {
	env := buildReport(t, ReportOptions{
		C3Dir:   "",
		Args:    []string{"fault"},
		Subject: "init",
		Summary: "init panics outside a git repo",
	})
	if env.Fingerprint == "" || env.Body == "" {
		t.Fatal("a report must survive a missing .c3/ — that is exactly when c3x is most broken")
	}
	if env.Consent != consentAsk {
		t.Errorf("consent with no project must fall back to %q, got %q", consentAsk, env.Consent)
	}
}

func TestReportLabelsAndRepoPerKind(t *testing.T) {
	c3Dir := reportFixture(t)
	want := map[string]string{"fault": "bug", "guidance": "documentation", "gap": "enhancement"}
	for kind, label := range want {
		env := buildReport(t, ReportOptions{C3Dir: c3Dir, Args: []string{kind}, Subject: "x", Summary: "y"})
		if env.Repo != ReportRepo {
			t.Errorf("%s must target %q, got %q", kind, ReportRepo, env.Repo)
		}
		if len(env.Labels) != 2 || env.Labels[0] != label || env.Labels[1] != reportSelfLabel {
			t.Errorf("%s labels = %v, want [%s %s]", kind, env.Labels, label, reportSelfLabel)
		}
	}
}

func TestReportBodyCarriesFingerprintForDedupeSearch(t *testing.T) {
	c3Dir := reportFixture(t)
	env := buildReport(t, ReportOptions{C3Dir: c3Dir, Args: []string{"gap"}, Subject: "diff", Summary: "no fact diff"})
	if !strings.Contains(env.Body, env.Fingerprint) {
		t.Errorf("the fingerprint must appear in the body — the dedupe probe is a full-text search:\n%s", env.Body)
	}
}

func TestReportConsentDefaultsToAsk(t *testing.T) {
	c3Dir := reportFixture(t)
	var buf bytes.Buffer
	if err := RunReport(ReportOptions{C3Dir: c3Dir, Version: "11.12.0", Args: []string{"consent"}}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), consentAsk) {
		t.Errorf("consent must default to %q, got:\n%s", consentAsk, buf.String())
	}
}

func TestReportConsentRoundTrips(t *testing.T) {
	c3Dir := reportFixture(t)
	var buf bytes.Buffer
	if err := RunReport(ReportOptions{C3Dir: c3Dir, Version: "11.12.0", Args: []string{"consent", consentAuto}}, &buf); err != nil {
		t.Fatal(err)
	}

	env := buildReport(t, ReportOptions{C3Dir: c3Dir, Args: []string{"gap"}, Subject: "diff", Summary: "no fact diff"})
	if env.Consent != consentAuto {
		t.Errorf("consent = %q, want %q after being switched on", env.Consent, consentAuto)
	}
}

func TestReportConsentOffRefusesToBuild(t *testing.T) {
	c3Dir := reportFixture(t)
	if err := RunReport(ReportOptions{C3Dir: c3Dir, Version: "11.12.0", Args: []string{"consent", consentOff}}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	err := RunReport(ReportOptions{C3Dir: c3Dir, Version: "11.12.0", Args: []string{"gap"}, Subject: "diff", Summary: "no fact diff"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("consent off must disable reporting entirely")
	}
	if !strings.Contains(err.Error(), "hint:") {
		t.Errorf("refusal must say how to turn it back on, got: %v", err)
	}
}

func TestReportConsentRejectsUnknownMode(t *testing.T) {
	c3Dir := reportFixture(t)
	err := RunReport(ReportOptions{C3Dir: c3Dir, Version: "11.12.0", Args: []string{"consent", "sometimes"}}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("an unknown consent mode must be refused")
	}
	if !strings.Contains(err.Error(), "hint:") {
		t.Errorf("refusal must carry a hint, got: %v", err)
	}
}

func TestReportConsentRejectsUnknownFieldInPolicyFile(t *testing.T) {
	c3Dir := reportFixture(t)
	if err := os.WriteFile(filepath.Join(c3Dir, reportPolicyFile), []byte(`{"consent":"auto","token":"ghp_secret"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := RunReport(ReportOptions{C3Dir: c3Dir, Version: "11.12.0", Args: []string{"consent"}}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("an unknown field must be rejected — this file must never grow into a credential store")
	}
	if !strings.Contains(err.Error(), "hint:") {
		t.Errorf("refusal must carry a hint, got: %v", err)
	}
}

func TestReportConsentNeedsAProject(t *testing.T) {
	err := RunReport(ReportOptions{C3Dir: "", Version: "11.12.0", Args: []string{"consent", consentAuto}}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("setting consent with no .c3/ must be refused — there is nowhere to persist it")
	}
	if !strings.Contains(err.Error(), "hint:") {
		t.Errorf("refusal must carry a hint, got: %v", err)
	}
}

func TestReportAgentOutputShape(t *testing.T) {
	t.Setenv("C3X_MODE", "agent")
	c3Dir := reportFixture(t)

	var buf bytes.Buffer
	err := RunReport(ReportOptions{
		C3Dir: c3Dir, Version: "11.12.0", JSON: true,
		Args: []string{"guidance"}, Subject: "audit.md", Summary: "audit.md omits the repair loop",
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	requireAll(t, out, "kind:", "fingerprint:", "consent:", "repo:", "title:", "body:", "help[")
	if !strings.Contains(out, "gh issue") {
		t.Errorf("agent output must hand back the gh commands that file it:\n%s", out)
	}
}

func TestReportJSONRoundTrips(t *testing.T) {
	c3Dir := reportFixture(t)
	var buf bytes.Buffer
	err := RunReport(ReportOptions{
		C3Dir: c3Dir, Version: "11.12.0", JSON: true,
		Args: []string{"gap"}, Subject: "diff", Summary: "no fact diff",
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}

	var env reportEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("--json must emit valid JSON: %v\n%s", err, buf.String())
	}
	if env.Kind != "gap" || env.Fingerprint == "" || env.Title == "" {
		t.Errorf("decoded envelope is incomplete: %+v", env)
	}
}

func appendActivityRow(t *testing.T, c3Dir, command string, args []string, success bool, errText string) {
	t.Helper()
	AppendActivity(c3Dir, command, args, false, success, errText)
}

func appendFault(t *testing.T, c3Dir, command, errText string) {
	t.Helper()
	AppendActivity(c3Dir, command, nil, false, false, errText)
}

package cmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// ReportRepo is where C3's own friction goes. It is fixed rather than
// configurable: a c3x defect routed to a downstream tracker never reaches
// anyone who can fix it.
const ReportRepo = "cuongtranba/c3-skill"

const (
	reportSelfLabel  = "self-reported"
	reportPolicyFile = "report.json"

	consentAsk  = "ask"
	consentAuto = "auto"
	consentOff  = "off"

	reportActivityTail = 5
)

// reportKinds is the closed set of reportable friction. User-input errors are
// deliberately absent — a schema violation is c3x working, and filing those
// would bury real defects.
var reportKinds = map[string]struct {
	label       string
	titlePrefix string
	needSubject bool
}{
	"fault":    {label: "bug", titlePrefix: "c3x", needSubject: false},
	"guidance": {label: "documentation", titlePrefix: "docs", needSubject: true},
	"gap":      {label: "enhancement", titlePrefix: "gap", needSubject: true},
}

// ReportOptions carries everything a report needs. C3Dir may be empty: the
// moment c3x is most worth reporting is the moment its project is unreadable.
type ReportOptions struct {
	C3Dir   string
	Version string
	Args    []string
	Summary string
	Detail  string
	Subject string
	JSON    bool
}

// reportEnvelope is a filed-ready GitHub issue. The CLI builds it and stops:
// the network call belongs to the agent, which owns the consent gate. The gh
// commands ride the help[] line rather than a field here, so they are stated
// once.
type reportEnvelope struct {
	Kind        string   `json:"kind"`
	Fingerprint string   `json:"fingerprint"`
	Consent     string   `json:"consent"`
	Repo        string   `json:"repo"`
	Title       string   `json:"title"`
	Labels      []string `json:"labels"`
	Redacted    []string `json:"redacted,omitempty"`
	Body        string   `json:"body"`
}

type reportPolicy struct {
	Consent string `json:"consent"`
}

// RunReport builds a bug report, or reads and writes the consent policy that
// decides whether the agent may file one without asking.
func RunReport(opts ReportOptions, w io.Writer) error {
	if len(opts.Args) == 0 {
		return fmt.Errorf("error: report requires a kind\nhint: c3x report <fault|guidance|gap> --summary \"<one line>\", or c3x report consent <ask|auto|off>")
	}

	if opts.Args[0] == "consent" {
		return runReportConsent(opts, w)
	}

	env, err := buildReportEnvelope(opts)
	if err != nil {
		return err
	}
	return WriteObjectOutput(w, env, ResolveFormat(opts.JSON, isAgentMode()), reportHelpHints(env))
}

func runReportConsent(opts ReportOptions, w io.Writer) error {
	if len(opts.Args) == 1 {
		policy, err := readReportPolicy(opts.C3Dir)
		if err != nil {
			return err
		}
		return WriteObjectOutput(w, policy, ResolveFormat(opts.JSON, isAgentMode()), nil)
	}

	mode := opts.Args[1]
	if mode != consentAsk && mode != consentAuto && mode != consentOff {
		return fmt.Errorf("error: unknown consent mode %q\nhint: valid modes are %s (show the issue and wait), %s (file it silently), %s (disable reporting)",
			mode, consentAsk, consentAuto, consentOff)
	}
	if opts.C3Dir == "" {
		return fmt.Errorf("error: no .c3/ directory to store the consent policy in\nhint: run this inside a C3 project, or pass --c3-dir <path>")
	}

	path := filepath.Join(opts.C3Dir, reportPolicyFile)
	data, err := json.MarshalIndent(reportPolicy{Consent: mode}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode consent policy: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return WriteObjectOutput(w, reportPolicy{Consent: mode}, ResolveFormat(opts.JSON, isAgentMode()), nil)
}

// readReportPolicy rejects unknown fields so this file cannot quietly grow into
// a credential store: c3x never authenticates to GitHub.
func readReportPolicy(c3Dir string) (reportPolicy, error) {
	policy := reportPolicy{Consent: consentAsk}
	if c3Dir == "" {
		return policy, nil
	}
	path := filepath.Join(c3Dir, reportPolicyFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return policy, nil
		}
		return policy, fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return policy, fmt.Errorf("error: invalid consent policy at %s: %v\nhint: %s holds only a \"consent\" field — rewrite it with c3x report consent <ask|auto|off>",
			path, err, reportPolicyFile)
	}
	if policy.Consent == "" {
		policy.Consent = consentAsk
	}
	return policy, nil
}

func buildReportEnvelope(opts ReportOptions) (reportEnvelope, error) {
	kind := opts.Args[0]
	spec, known := reportKinds[kind]
	if !known {
		return reportEnvelope{}, fmt.Errorf("error: unknown report kind %q\nhint: use fault (c3x misbehaved), guidance (a doc contradicts the CLI), or gap (a needed operation does not exist)", kind)
	}
	if strings.TrimSpace(opts.Summary) == "" {
		return reportEnvelope{}, fmt.Errorf("error: report %s requires --summary\nhint: c3x report %s --summary \"<one line describing the friction>\"", kind, kind)
	}

	policy, err := readReportPolicy(opts.C3Dir)
	if err != nil {
		return reportEnvelope{}, err
	}
	if policy.Consent == consentOff {
		return reportEnvelope{}, fmt.Errorf("error: self-reporting is disabled for this project\nhint: run 'c3x report consent %s' to re-enable it, or '%s' to file without confirmation", consentAsk, consentAuto)
	}

	// Only a fault is grounded in what just ran. Attaching the trail to a
	// guidance or gap report would put an unrelated failure and the user's
	// command history on a public tracker.
	var fault activityEntry
	var trail []activityEntry
	if kind == "fault" {
		fault = lastFault(opts.C3Dir)
		trail = recentActivity(opts.C3Dir, reportActivityTail)
	}

	subject := strings.TrimSpace(opts.Subject)
	if subject == "" && kind == "fault" {
		subject = fault.Cmd
	}
	if subject == "" {
		if spec.needSubject {
			return reportEnvelope{}, fmt.Errorf("error: report %s requires --subject\nhint: name the reference or operation the report is about, e.g. --subject change.md — it is what keeps the fingerprint stable across rewordings", kind)
		}
		subject = "c3x"
	}

	summary := redactPaths(opts.Summary)
	detail := redactPaths(opts.Detail)
	cause := redactPaths(fault.Error)

	// The signature hashes the raw text, not the redacted text: redaction keeps
	// a basename, and two sightings of one defect in different files would
	// otherwise fingerprint apart.
	signature := opts.Summary
	if kind == "fault" && fault.Error != "" {
		signature = fault.Error
	}

	env := reportEnvelope{
		Kind:        kind,
		Fingerprint: reportFingerprint(kind, subject, signature),
		Consent:     policy.Consent,
		Repo:        ReportRepo,
		Title:       fmt.Sprintf("%s(%s): %s", spec.titlePrefix, subject, summary),
		Labels:      []string{spec.label, reportSelfLabel},
		Redacted:    []string{"filesystem paths (filenames kept, directories elided)"},
	}
	env.Body = renderReportBody(opts, env, subject, summary, detail, fault.Cmd, cause, trail)
	return env, nil
}

// reportFingerprint excludes the c3x version on purpose: the same defect
// resurfacing on a later release must land on the one existing issue rather
// than opening a fresh one every upgrade.
func reportFingerprint(kind, subject, signature string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + strings.ToLower(subject) + "\x00" + normalizeSignature(signature)))
	return fmt.Sprintf("%x", sum)[:12]
}

var (
	signatureDigits = regexp.MustCompile(`\d+`)
	signatureQuoted = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	signatureNoise  = regexp.MustCompile(`[^a-z ]+`)
	signatureSpace  = regexp.MustCompile(` +`)
	pathToken       = regexp.MustCompile(`(~|\.{0,2})?/[^\s"'` + "`" + `,;:)\]]*`)
)

// normalizeSignature strips the parts of a failure that vary between two
// sightings of the same defect — indices, quoted values, paths — so both
// sightings hash alike.
func normalizeSignature(s string) string {
	s = strings.ToLower(s)
	s = pathToken.ReplaceAllString(s, " ")
	s = signatureQuoted.ReplaceAllString(s, " ")
	s = signatureDigits.ReplaceAllString(s, " ")
	s = signatureNoise.ReplaceAllString(s, " ")
	return strings.TrimSpace(signatureSpace.ReplaceAllString(s, " "))
}

// redactPaths strips filesystem paths before a report reaches a public tracker.
// A filename survives — it locates the defect; a bare directory does not — its
// last segment is the user's home or their private project. Entity ids are
// untouched: they are structural and carry the reproduction value.
func redactPaths(s string) string {
	return pathToken.ReplaceAllStringFunc(s, func(match string) string {
		base := filepath.Base(strings.TrimRight(match, "/"))
		if ext := filepath.Ext(base); ext != "" && ext != base {
			return base
		}
		return "<path>"
	})
}

// lastFault returns the most recent failed command, ignoring report's own rows
// so a second report never describes the first.
func lastFault(c3Dir string) activityEntry {
	for _, entry := range recentActivity(c3Dir, 0) {
		if !entry.Success && entry.Cmd != "report" {
			return entry
		}
	}
	return activityEntry{}
}

// recentActivity returns the trail newest-first, capped at limit (0 = all).
func recentActivity(c3Dir string, limit int) []activityEntry {
	if c3Dir == "" {
		return nil
	}
	entries, _ := readNewActivity(filepath.Join(c3Dir, activityFileName), 0)

	var newestFirst []activityEntry
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Cmd == "report" {
			continue
		}
		newestFirst = append(newestFirst, entries[i])
		if limit > 0 && len(newestFirst) == limit {
			break
		}
	}
	return newestFirst
}

func renderReportBody(opts ReportOptions, env reportEnvelope, subject, summary, detail, failedCmd, cause string, trail []activityEntry) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## What happened\n\n%s\n", summary)
	if detail != "" {
		fmt.Fprintf(&b, "\n## Detail\n\n%s\n", detail)
	}

	fmt.Fprintf(&b, "\n## Environment\n\n| Field | Value |\n| --- | --- |\n")
	fmt.Fprintf(&b, "| c3x | %s |\n", opts.Version)
	fmt.Fprintf(&b, "| platform | %s/%s |\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "| kind | %s |\n", env.Kind)
	fmt.Fprintf(&b, "| subject | %s |\n", subject)
	fmt.Fprintf(&b, "| fingerprint | %s |\n", env.Fingerprint)

	if cause != "" {
		fmt.Fprintf(&b, "\n## Failing command\n\n```\n$ c3x %s\n%s\n```\n", failedCmd, cause)
	}

	if len(trail) > 0 {
		fmt.Fprintf(&b, "\n## Recent activity\n\n```\n")
		for i := len(trail) - 1; i >= 0; i-- {
			status := "ok"
			if !trail[i].Success {
				status = "FAILED"
			}
			line := fmt.Sprintf("%-8s c3x %s %s", status, trail[i].Cmd, redactPaths(strings.Join(trail[i].Args, " ")))
			fmt.Fprintf(&b, "%s\n", strings.TrimRight(line, " "))
		}
		fmt.Fprintf(&b, "```\n")
	}

	fmt.Fprintf(&b, "\n---\n\nFiled by `c3x report`. Fingerprint `%s` — search it before opening a duplicate.\nFilesystem paths were stripped before filing: filenames kept, directories elided.\n", env.Fingerprint)
	return b.String()
}

func reportHelpHints(env reportEnvelope) []HelpHint {
	return []HelpHint{
		{
			Command:     fmt.Sprintf("gh issue list -R %s --state all --search %q", env.Repo, env.Fingerprint),
			Description: "dedupe first — comment on the hit instead of opening a duplicate",
		},
		{
			Command:     fmt.Sprintf("gh issue create -R %s --title <title> --body <body> --label %s", env.Repo, strings.Join(env.Labels, " --label ")),
			Description: "file it, once the consent gate above is satisfied",
		},
	}
}

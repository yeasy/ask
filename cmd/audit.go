package cmd

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/skill"
)

// AuditReport represents the full audit report
type AuditReport struct {
	GeneratedAt   time.Time         `json:"generated_at"`
	Version       string            `json:"version"`
	TotalSkills   int               `json:"total_skills"`
	TotalFindings int               `json:"total_findings"`
	CriticalCount int               `json:"critical_count"`
	WarningCount  int               `json:"warning_count"`
	InfoCount     int               `json:"info_count"`
	Skills        []AuditSkillEntry `json:"skills"`
}

// AuditSkillEntry represents one skill in the audit
type AuditSkillEntry struct {
	Name        string          `json:"name"`
	URL         string          `json:"url,omitempty"`
	Version     string          `json:"version,omitempty"`
	Commit      string          `json:"commit,omitempty"`
	InstalledAt string          `json:"installed_at,omitempty"`
	Source      string          `json:"source,omitempty"`
	Findings    []skill.Finding `json:"findings,omitempty"`
	Status      string          `json:"status"` // "clean", "warning", "critical"
}

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Generate a security audit report of installed skills",
	Long: `Scan all installed skills and generate a comprehensive security audit report.

The report includes:
  - Complete inventory of installed skills with versions
  - Security scan results for each skill
  - Summary statistics of findings by severity
  - Source and provenance information

Output formats: console (default), json, html, markdown`,
	Example: `  # Console audit
  ask audit

  # Generate JSON report
  ask audit --format json --output audit-report.json

  # Generate HTML report
  ask audit --format html --output audit-report.html

  # Audit global skills
  ask audit --global`,
	Run: runAudit,
}

func runAudit(cmd *cobra.Command, _ []string) {
	global, _ := cmd.Flags().GetBool("global")
	format, _ := cmd.Flags().GetString("format")
	output, _ := cmd.Flags().GetString("output")

	// Load lock file for version info
	lockFile, _ := config.LoadLockFileByScope(global)

	// Find skills directory
	skillsDir, err := config.GetSkillsDirByScope(global)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("No skills directory found at %s\n", skillsDir)
			return
		}
		fmt.Fprintf(os.Stderr, "Error reading skills directory: %v\n", err)
		os.Exit(1)
	}

	report := AuditReport{
		GeneratedAt: time.Now(),
		Version:     Version,
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(skillsDir, entry.Name())
		auditEntry := AuditSkillEntry{
			Name:   entry.Name(),
			Status: "clean",
		}

		// Get lock info if available
		if lockFile != nil {
			lockEntry := lockFile.GetEntry(entry.Name())
			if lockEntry != nil {
				auditEntry.URL = lockEntry.URL
				auditEntry.Version = lockEntry.Version
				auditEntry.Commit = lockEntry.Commit
				auditEntry.Source = lockEntry.Source
				if !lockEntry.InstalledAt.IsZero() {
					auditEntry.InstalledAt = lockEntry.InstalledAt.Format(time.RFC3339)
				}
			}
		}

		// Run security check
		if skill.FindSkillMD(skillPath) {
			result, err := skill.CheckSafety(skillPath)
			if err == nil {
				auditEntry.Findings = result.Findings
				for _, f := range result.Findings {
					switch f.Severity {
					case skill.SeverityCritical:
						report.CriticalCount++
						auditEntry.Status = "critical"
					case skill.SeverityWarning:
						report.WarningCount++
						if auditEntry.Status != "critical" {
							auditEntry.Status = "warning"
						}
					case skill.SeverityInfo:
						report.InfoCount++
					}
				}
				report.TotalFindings += len(result.Findings)
			}
		}

		report.Skills = append(report.Skills, auditEntry)
		report.TotalSkills++
	}

	// Output
	writeReport := func(content string) {
		if output != "" {
			if err := os.WriteFile(output, []byte(content), 0600); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Audit report saved to %s\n", output)
		} else {
			fmt.Println(content)
		}
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling report: %v\n", err)
			os.Exit(1)
		}
		writeReport(string(data))
	case "markdown", "md":
		writeReport(generateAuditMarkdown(report))
	case "html":
		writeReport(generateAuditHTML(report))
	default:
		// Console output
		fmt.Println("Security Audit Report")
		fmt.Println("=====================")
		fmt.Printf("Skills scanned: %d\n", report.TotalSkills)
		fmt.Printf("Findings: %d Critical, %d Warning, %d Info\n\n",
			report.CriticalCount, report.WarningCount, report.InfoCount)

		for _, s := range report.Skills {
			switch s.Status {
			case "critical":
				fmt.Printf("  %s %s", color.RedString("✗"), s.Name)
			case "warning":
				fmt.Printf("  %s %s", color.YellowString("!"), s.Name)
			default:
				fmt.Printf("  %s %s", color.GreenString("✓"), s.Name)
			}
			if s.Version != "" {
				fmt.Printf(" (%s)", s.Version)
			}
			fmt.Println()

			for _, f := range s.Findings {
				switch f.Severity {
				case skill.SeverityCritical:
					fmt.Printf("    %s %s\n", color.RedString("[CRITICAL]"), f.Description)
				case skill.SeverityWarning:
					fmt.Printf("    %s %s\n", color.YellowString("[WARNING]"), f.Description)
				}
			}
		}

		if report.CriticalCount > 0 {
			fmt.Printf("\n%s Critical issues found. Review and fix before deployment.\n", color.RedString("!"))
		} else {
			fmt.Printf("\n%s All skills passed security audit.\n", color.GreenString("✓"))
		}
	}

	if report.CriticalCount > 0 {
		os.Exit(1)
	}
}

func generateAuditMarkdown(report AuditReport) string {
	var sb strings.Builder
	sb.WriteString("# Security Audit Report\n\n")
	fmt.Fprintf(&sb, "Generated: %s | ASK v%s\n\n", report.GeneratedAt.Format(time.RFC3339), report.Version)
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Metric | Count |\n|--------|-------|\n")
	fmt.Fprintf(&sb, "| Total skills | %d |\n", report.TotalSkills)
	fmt.Fprintf(&sb, "| Critical | %d |\n", report.CriticalCount)
	fmt.Fprintf(&sb, "| Warnings | %d |\n", report.WarningCount)
	fmt.Fprintf(&sb, "| Info | %d |\n\n", report.InfoCount)
	sb.WriteString("## Skills\n\n")
	for _, s := range report.Skills {
		status := "✓"
		switch s.Status {
		case "critical":
			status = "✗"
		case "warning":
			status = "!"
		}
		fmt.Fprintf(&sb, "### %s %s\n\n", status, s.Name)
		if s.URL != "" {
			fmt.Fprintf(&sb, "- URL: %s\n", s.URL)
		}
		if s.Version != "" {
			fmt.Fprintf(&sb, "- Version: %s\n", s.Version)
		}
		if len(s.Findings) > 0 {
			fmt.Fprintf(&sb, "- Findings: %d\n\n", len(s.Findings))
			for _, f := range s.Findings {
				fmt.Fprintf(&sb, "  - [%s] %s (%s:%d)\n", f.Severity, f.Description, f.File, f.Line)
			}
		} else {
			sb.WriteString("- No issues found\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func generateAuditHTML(report AuditReport) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>ASK Security Audit Report</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 900px; margin: 0 auto; padding: 2rem; background: #f8f9fa; color: #333; }
  h1 { border-bottom: 2px solid #333; padding-bottom: 0.5rem; }
  .meta { color: #666; font-size: 0.9rem; margin-bottom: 2rem; }
  .summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1rem; margin-bottom: 2rem; }
  .card { background: #fff; border-radius: 8px; padding: 1.2rem; text-align: center; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  .card .number { font-size: 2rem; font-weight: bold; }
  .card .label { color: #666; font-size: 0.85rem; text-transform: uppercase; }
  .card.critical .number { color: #dc3545; }
  .card.warning .number { color: #ffc107; }
  .card.info .number { color: #17a2b8; }
  .card.total .number { color: #333; }
  .skill { background: #fff; border-radius: 8px; padding: 1.2rem; margin-bottom: 1rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  .skill-header { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.5rem; }
  .skill-name { font-weight: 600; font-size: 1.1rem; }
  .badge { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: 600; }
  .badge-clean { background: #d4edda; color: #155724; }
  .badge-warning { background: #fff3cd; color: #856404; }
  .badge-critical { background: #f8d7da; color: #721c24; }
  .skill-meta { color: #666; font-size: 0.85rem; margin-bottom: 0.5rem; }
  .finding { padding: 0.4rem 0.6rem; margin: 0.3rem 0; border-radius: 4px; font-size: 0.85rem; }
  .finding-critical { background: #f8d7da; }
  .finding-warning { background: #fff3cd; }
  .finding-info { background: #d1ecf1; }
  a { color: #007bff; text-decoration: none; }
</style>
</head>
<body>
`)
	sb.WriteString("<h1>Security Audit Report</h1>\n")
	fmt.Fprintf(&sb, `<p class="meta">Generated: %s &middot; ASK v%s</p>`+"\n",
		report.GeneratedAt.Format("2006-01-02 15:04:05"), htmlEscape(report.Version))

	// Summary cards
	sb.WriteString(`<div class="summary">` + "\n")
	fmt.Fprintf(&sb, `<div class="card total"><div class="number">%d</div><div class="label">Total Skills</div></div>`+"\n", report.TotalSkills)
	fmt.Fprintf(&sb, `<div class="card critical"><div class="number">%d</div><div class="label">Critical</div></div>`+"\n", report.CriticalCount)
	fmt.Fprintf(&sb, `<div class="card warning"><div class="number">%d</div><div class="label">Warnings</div></div>`+"\n", report.WarningCount)
	fmt.Fprintf(&sb, `<div class="card info"><div class="number">%d</div><div class="label">Info</div></div>`+"\n", report.InfoCount)
	sb.WriteString("</div>\n")

	// Skills
	sb.WriteString("<h2>Skills</h2>\n")
	for _, s := range report.Skills {
		sb.WriteString(`<div class="skill">` + "\n")
		badgeClass := "badge-clean"
		badgeText := "Clean"
		switch s.Status {
		case "critical":
			badgeClass = "badge-critical"
			badgeText = "Critical"
		case "warning":
			badgeClass = "badge-warning"
			badgeText = "Warning"
		}
		fmt.Fprintf(&sb, `<div class="skill-header"><span class="skill-name">%s</span><span class="badge %s">%s</span></div>`+"\n",
			htmlEscape(s.Name), badgeClass, badgeText)

		var meta []string
		if s.Version != "" {
			meta = append(meta, "v"+htmlEscape(s.Version))
		}
		if s.Source != "" {
			meta = append(meta, "source: "+htmlEscape(s.Source))
		}
		if s.URL != "" {
			// Only allow http/https URLs in href to prevent javascript: XSS
			if strings.HasPrefix(s.URL, "https://") || strings.HasPrefix(s.URL, "http://") {
				meta = append(meta, fmt.Sprintf(`<a href="%s">%s</a>`, htmlEscape(s.URL), htmlEscape(s.URL)))
			} else {
				meta = append(meta, htmlEscape(s.URL))
			}
		}
		if len(meta) > 0 {
			fmt.Fprintf(&sb, `<div class="skill-meta">%s</div>`+"\n", strings.Join(meta, " &middot; "))
		}

		for _, f := range s.Findings {
			findingClass := "finding-info"
			switch f.Severity {
			case skill.SeverityCritical:
				findingClass = "finding-critical"
			case skill.SeverityWarning:
				findingClass = "finding-warning"
			}
			fmt.Fprintf(&sb, `<div class="finding %s">[%s] %s <code>%s:%d</code></div>`+"\n",
				findingClass, htmlEscape(string(f.Severity)), htmlEscape(f.Description), htmlEscape(f.File), f.Line)
		}
		sb.WriteString("</div>\n")
	}

	sb.WriteString("</body>\n</html>\n")
	return sb.String()
}

func htmlEscape(s string) string {
	return html.EscapeString(s)
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().String("format", "console", "Output format: console, json, html, markdown")
	auditCmd.Flags().StringP("output", "o", "", "Write report to file")
}

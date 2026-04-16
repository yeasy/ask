package skill

import (
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"sync"
	"time"
)

// htmlReportOnce ensures the HTML report template is parsed only once.
var htmlReportOnce struct {
	once sync.Once
	tmpl *template.Template
	err  error
}

// SARIFReport represents a SARIF v2.1.0 report
type SARIFReport struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []SARIFRun `json:"runs"`
}

// SARIFRun represents a single run in a SARIF report
type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

// SARIFTool represents the tool that produced the SARIF report
type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

// SARIFDriver represents the driver (scanner) information
type SARIFDriver struct {
	Name    string      `json:"name"`
	Version string      `json:"version"`
	Rules   []SARIFRule `json:"rules,omitempty"`
}

// SARIFRule represents a rule definition in the SARIF report
type SARIFRule struct {
	ID               string          `json:"id"`
	ShortDescription SARIFMessage    `json:"shortDescription"`
	DefaultConfig    SARIFRuleConfig `json:"defaultConfiguration"`
}

// SARIFRuleConfig represents the default configuration for a rule
type SARIFRuleConfig struct {
	Level string `json:"level"`
}

// SARIFMessage represents a text message in the SARIF report
type SARIFMessage struct {
	Text string `json:"text"`
}

// SARIFResult represents a single finding result
type SARIFResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations,omitempty"`
}

// SARIFLocation represents the location of a finding
type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

// SARIFPhysicalLocation represents the physical file location
type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Region           *SARIFRegion          `json:"region,omitempty"`
}

// SARIFArtifactLocation represents the artifact (file) URI
type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

// SARIFRegion represents a region within a file
type SARIFRegion struct {
	StartLine int `json:"startLine"`
}

// GenerateSARIFReport generates a SARIF v2.1.0 formatted security report
func GenerateSARIFReport(result *CheckResult, version string) (string, error) {
	report := SARIFReport{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:    "ask-check",
						Version: version,
					},
				},
			},
		},
	}

	// Build rules set
	ruleSet := make(map[string]bool)
	for _, f := range result.Findings {
		if !ruleSet[f.RuleID] {
			ruleSet[f.RuleID] = true
			level := "note"
			switch f.Severity {
			case SeverityCritical:
				level = "error"
			case SeverityWarning:
				level = "warning"
			}
			report.Runs[0].Tool.Driver.Rules = append(report.Runs[0].Tool.Driver.Rules, SARIFRule{
				ID:               f.RuleID,
				ShortDescription: SARIFMessage{Text: f.Description},
				DefaultConfig:    SARIFRuleConfig{Level: level},
			})
		}
	}

	// Build results
	for _, f := range result.Findings {
		level := "note"
		switch f.Severity {
		case SeverityCritical:
			level = "error"
		case SeverityWarning:
			level = "warning"
		}

		sarifResult := SARIFResult{
			RuleID:  f.RuleID,
			Level:   level,
			Message: SARIFMessage{Text: f.Description + ": " + f.Match},
		}

		if f.File != "" {
			loc := SARIFLocation{
				PhysicalLocation: SARIFPhysicalLocation{
					ArtifactLocation: SARIFArtifactLocation{URI: f.File},
				},
			}
			if f.Line > 0 {
				loc.PhysicalLocation.Region = &SARIFRegion{StartLine: f.Line}
			}
			sarifResult.Locations = append(sarifResult.Locations, loc)
		}

		report.Runs[0].Results = append(report.Runs[0].Results, sarifResult)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GenerateReport generates a report in the specified format ("md", "html", or "json")
func GenerateReport(result *CheckResult, format string) (string, error) {
	switch strings.ToLower(format) {
	case "md", "markdown":
		return generateMarkdown(result), nil
	case "html", "htm":
		return generateHTML(result), nil
	case "json":
		return generateJSON(result), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func generateMarkdown(result *CheckResult) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Security Report: %s\n\n", result.SkillName)
	fmt.Fprintf(&sb, "**Date:** %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	criticals, warnings, infos := countSeverities(result.Findings)
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Severity | Count |\n")
	sb.WriteString("| :--- | :--- |\n")
	fmt.Fprintf(&sb, "| 🔴 Critical | %d |\n", criticals)
	fmt.Fprintf(&sb, "| 🟡 Warning | %d |\n", warnings)
	fmt.Fprintf(&sb, "| 🔵 Info | %d |\n", infos)
	sb.WriteString("\n")

	if len(result.Findings) == 0 {
		sb.WriteString("✅ **No issues found.** Skill appears safe.\n")
		return sb.String()
	}

	sb.WriteString("## Detailed Findings\n\n")
	for _, f := range result.Findings {
		var icon string
		switch f.Severity {
		case SeverityCritical:
			icon = "🔴"
		case SeverityWarning:
			icon = "🟡"
		default:
			icon = "🔵"
		}

		fmt.Fprintf(&sb, "### %s %s\n", icon, f.Description)
		fmt.Fprintf(&sb, "- **Rule ID:** `%s`\n", f.RuleID)
		fmt.Fprintf(&sb, "- **Location:** `%s:%d`\n", f.File, f.Line)
		sb.WriteString("```\n")
		sb.WriteString(f.Match)
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}

// htmlTemplate is the HTML report template string, defined at package level
// so it can be parsed once and reused across calls.
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Security Report: {{.SkillName}}</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.5.1/dist/chart.umd.min.js"></script>
    <style>
        :root {
            --critical: #d73a49;
            --warning: #dbab09;
            --info: #0366d6;
            --bg-color: #f6f8fa;
            --card-bg: #ffffff;
            --text-color: #24292e;
            --border-color: #e1e4e8;
        }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; color: var(--text-color); max-width: 1200px; margin: 0 auto; padding: 20px; background-color: var(--bg-color); }
        .header { background: var(--card-bg); padding: 20px; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); margin-bottom: 20px; display: flex; justify-content: space-between; align-items: center; }
        .header h1 { margin: 0; font-size: 24px; }
        .timestamp { color: #586069; font-size: 14px; }
        
        .dashboard { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-bottom: 20px; }
        .card { background: var(--card-bg); padding: 20px; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .card h2 { margin-top: 0; font-size: 18px; border-bottom: 1px solid var(--border-color); padding-bottom: 10px; }
        
        .summary-stats { display: flex; justify-content: space-around; text-align: center; margin-top: 20px; }
        .stat-item { flex: 1; }
        .stat-value { font-size: 36px; font-weight: bold; display: block; }
        .stat-label { font-size: 14px; color: #586069; text-transform: uppercase; letter-spacing: 0.5px; }
        
        .text-critical { color: var(--critical); }
        .text-warning { color: var(--warning); }
        .text-info { color: var(--info); }
        
        .tabs { display: flex; margin-bottom: 20px; border-bottom: 1px solid var(--border-color); }
        .tab { padding: 10px 20px; cursor: pointer; border-bottom: 2px solid transparent; font-weight: 500; }
        .tab.active { border-bottom-color: #0366d6; color: #0366d6; }
        
        .tab-content { display: none; }
        .tab-content.active { display: block; }
        
        .module-group { margin-bottom: 30px; background: var(--card-bg); border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .module-header { background: #f1f8ff; padding: 15px 20px; font-weight: bold; border-bottom: 1px solid var(--border-color); display: flex; justify-content: space-between; align-items: center; cursor: pointer; user-select: none; }
        .module-header:hover { background-color: #e6f3ff; }
        .module-header::after { content: '▼'; font-size: 12px; transition: transform 0.3s ease; }
        .module-group.collapsed .module-header::after { transform: rotate(-90deg); }
        .module-group.collapsed .finding-list { display: none; }
        .module-bg-critical { background-color: #ffeef0; }
        .module-bg-critical:hover { background-color: #ffdce0; }
        .module-bg-clean { background-color: #e6ffed; }
        .module-bg-clean:hover { background-color: #d1f7db; }
        
        .finding-list { padding: 0; margin: 0; list-style: none; }
        .finding-item { padding: 15px 20px; border-bottom: 1px solid var(--border-color); }
        .finding-item:last-child { border-bottom: none; }
        
        .finding-header { display: flex; justify-content: space-between; margin-bottom: 8px; }
        .finding-title { font-weight: 600; display: flex; align-items: center; gap: 10px; }
        .badge { padding: 2px 8px; border-radius: 12px; font-size: 12px; font-weight: 600; color: white; text-transform: uppercase; }
        .bg-critical { background-color: var(--critical); }
        .bg-warning { background-color: var(--warning); }
        .bg-info { background-color: var(--info); }
        
        .location { font-family: monospace; color: #586069; font-size: 13px; margin-bottom: 8px; }
        .code-snippet { background: #f6f8fa; padding: 10px; border-radius: 4px; overflow-x: auto; font-family: monospace; font-size: 12px; border: 1px solid var(--border-color); }
        
        .empty-state { text-align: center; padding: 50px; color: #586069; }
        .chart-container { position: relative; height: 250px; width: 100%; display: flex; justify-content: center; }
        
        .footer { text-align: center; margin-top: 40px; border-top: 1px solid var(--border-color); padding-top: 20px; color: #586069; font-size: 14px; }
        .footer a { color: #0366d6; text-decoration: none; }
        .footer a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="header">
        <div>
            <h1>🛡️ Agent Security Audit Report</h1>
            <div class="timestamp">Target: {{.SkillName}} | Generated: {{.Date}} by <a href="https://github.com/yeasy/ask" target="_blank" style="color: #0366d6; text-decoration: none;">ASK</a></div>
        </div>
        <div>
            <span class="badge {{if eq .Stats.Critical 0}}bg-info{{else}}bg-critical{{end}}" style="font-size: 14px; padding: 8px 16px;">
                {{if eq .Stats.Critical 0}}PASSED{{else}}FAILED{{end}}
            </span>
        </div>
    </div>

    <div class="dashboard">
        <!-- Overview Chart -->
        <div class="card">
            <h2>Severity Distribution</h2>
            <div class="chart-container">
                <canvas id="severityChart"></canvas>
            </div>
        </div>
        
        <!-- Summary Stats -->
        <div class="card">
            <h2>Overview</h2>
            <div class="summary-stats">
                <div class="stat-item">
                    <span class="stat-value text-critical">{{.Stats.Critical}}</span>
                    <span class="stat-label">Critical</span>
                </div>
                <div class="stat-item">
                    <span class="stat-value text-warning">{{.Stats.Warning}}</span>
                    <span class="stat-label">Warning</span>
                </div>
                <div class="stat-item">
                    <span class="stat-value text-info">{{.Stats.Info}}</span>
                    <span class="stat-label">Info</span>
                </div>
                <div class="stat-item">
                    <span class="stat-value">{{.Stats.Total}}</span>
                    <span class="stat-label">Total Findings</span>
                </div>
            </div>
            <div style="margin-top: 30px; padding: 15px; background: #f1f8ff; border-radius: 4px; font-size: 14px;">
                <strong>Scan Scope:</strong> {{.SkillName}}<br>
                <strong>Modules Scanned:</strong> {{.Stats.ModulesAffected}}<br>
                <strong>Status:</strong> 
                {{if gt .Stats.Critical 0}}
                <span class="text-critical">Immediate Action Required</span>
                {{else if gt .Stats.Warning 0}}
                <span class="text-warning">Review Recommended</span>
                {{else}}
                <span class="text-info" style="color: green;">Clean</span>
                {{end}}
            </div>
        </div>
    </div>

    <div class="tabs">
        <div class="tab active" onclick="switchTab('by-module')">By Module</div>
        <div class="tab" onclick="switchTab('by-severity')">By Severity</div>
    </div>

    <!-- Findings by Module -->
    <div id="by-module" class="tab-content active">
        {{if not .ModuleGroups}}
            <div class="card empty-state">
                <h3>✅ No security issues found</h3>
                <p>Your codebase appears clean based on the active ruleset.</p>
            </div>
        {{end}}

        {{range .ModuleGroups}}
        <div class="module-group collapsed">
            <div class="module-header {{if .HasCritical}}module-bg-critical{{else if eq .Total 0}}module-bg-clean{{end}}" onclick="toggleGroup(this)">
                <span>📦 {{.Name}}</span>
                <div>
                   {{if gt .Critical 0}}<span class="badge bg-critical" style="margin-right: 5px">{{.Critical}} Crit</span>{{end}}
                   {{if gt .Warning 0}}<span class="badge bg-warning" style="margin-right: 5px">{{.Warning}} Warn</span>{{end}}
                   {{if eq .Total 0}}<span class="badge bg-info" style="background-color: #28a745; margin-right: 5px">Safe</span>{{end}}
                   <span style="font-size: 12px; color: #586069;">{{.Total}} issues</span>
                </div>
            </div>
            <ul class="finding-list">
                {{range .Findings}}
                <li class="finding-item">
                    <div class="finding-header">
                        <div class="finding-title">
                            <span class="badge bg-{{.SeverityClass}}">{{.Severity}}</span>
                            {{.Description}}
                        </div>
                        <span style="font-size: 12px; color: #999;">{{.RuleID}}</span>
                    </div>
                    <div class="location">File: {{.File}}:{{.Line}}</div>
                    <div class="code-snippet">{{.Match}}</div>
                </li>
                {{end}}
            </ul>
        </div>
        {{end}}
    </div>

    <!-- Findings by Severity -->
    <div id="by-severity" class="tab-content">
        {{range .SeverityGroups}}
        <div class="module-group collapsed">
            <div class="module-header" onclick="toggleGroup(this)">
                <span class="badge bg-{{.Class}}" style="font-size: 14px;">{{.Name}}</span>
                <span style="font-size: 12px; color: #586069;">{{.Count}} findings</span>
            </div>
            <ul class="finding-list">
                {{range .Findings}}
                <li class="finding-item">
                    <div class="finding-header">
                        <div class="finding-title">
                            <span style="color: #666;">[{{.Module}}]</span> {{.Description}}
                        </div>
                        <span style="font-size: 12px; color: #999;">{{.RuleID}}</span>
                    </div>
                    <div class="location">File: {{.File}}:{{.Line}}</div>
                    <div class="code-snippet">{{.Match}}</div>
                </li>
                {{end}}
            </ul>
        </div>
        {{end}}
    </div>

    <script>
        // Charts
        const ctx = document.getElementById('severityChart').getContext('2d');
        
        const labels = [];
        const data = [];
        const colors = [];

        {{if gt .Stats.Critical 0}}
        labels.push('Critical');
        data.push({{.Stats.Critical}});
        colors.push('#d73a49');
        {{end}}

        {{if gt .Stats.Warning 0}}
        labels.push('Warning');
        data.push({{.Stats.Warning}});
        colors.push('#dbab09');
        {{end}}

        {{if gt .Stats.Info 0}}
        labels.push('Info');
        data.push({{.Stats.Info}});
        colors.push('#0366d6');
        {{end}}

        new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: labels,
                datasets: [{
                    data: data,
                    backgroundColor: colors,
                    borderWidth: 0
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { position: 'right' }
                }
            }
        });

        // Tabs
        function switchTab(id) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            
            event.target.classList.add('active');
            document.getElementById(id).classList.add('active');
        }

        function toggleGroup(header) {
            header.parentElement.classList.toggle('collapsed');
        }
    </script>
    <div class="footer">
        generated by <a href="https://github.com/yeasy/ask/" target="_blank">ASK</a>, Agent Skills Manager for Enterprise AI
    </div>
</body>
</html>`

func generateHTML(result *CheckResult) string {
	type FindingView struct {
		Finding
		SeverityClass string
	}

	type ModuleGroup struct {
		Name        string
		Findings    []FindingView
		Critical    int
		Warning     int
		Total       int
		HasCritical bool
	}

	type SeverityGroup struct {
		Name     string
		Class    string
		Findings []FindingView
		Count    int
	}

	// Process data for grouping
	moduleMap := make(map[string]*ModuleGroup)
	var moduleGroups []*ModuleGroup

	// Group by Module
	for _, moduleName := range result.ScannedModules {
		if _, exists := moduleMap[moduleName]; !exists {
			group := &ModuleGroup{Name: moduleName}
			moduleMap[moduleName] = group
			moduleGroups = append(moduleGroups, group)
		}
	}

	for _, f := range result.Findings {
		moduleName := f.Module
		if moduleName == "" {
			moduleName = "Unknown"
		}

		group, exists := moduleMap[moduleName]
		if !exists {
			group = &ModuleGroup{Name: moduleName}
			moduleMap[moduleName] = group
			moduleGroups = append(moduleGroups, group)
		}

		class := "info"
		switch f.Severity {
		case SeverityCritical:
			class = "critical"
			group.Critical++
			group.HasCritical = true
		case SeverityWarning:
			class = "warning"
			group.Warning++
		}

		group.Total++
		group.Findings = append(group.Findings, FindingView{Finding: f, SeverityClass: class})
	}

	// Sort module groups:
	// 1. Critical issues first
	// 2. Warnings next
	// 3. Info next
	// 4. Clean modules last
	// 5. Alphabetical within groups
	sort.Slice(moduleGroups, func(i, j int) bool {
		// If one has critical and other doesn't
		if moduleGroups[i].HasCritical != moduleGroups[j].HasCritical {
			return moduleGroups[i].HasCritical
		}

		// If both have critical or neither, check total issues count (descending)
		if moduleGroups[i].Total != moduleGroups[j].Total {
			return moduleGroups[i].Total > moduleGroups[j].Total
		}

		// Finally sort by name
		return moduleGroups[i].Name < moduleGroups[j].Name
	})

	// Group by Severity
	var critGroup, warnGroup, infoGroup SeverityGroup
	critGroup = SeverityGroup{Name: "CRITICAL", Class: "critical"}
	warnGroup = SeverityGroup{Name: "WARNING", Class: "warning"}
	infoGroup = SeverityGroup{Name: "INFO", Class: "info"}

	for _, f := range result.Findings {
		fv := FindingView{Finding: f}
		switch f.Severity {
		case SeverityCritical:
			fv.SeverityClass = "critical"
			critGroup.Findings = append(critGroup.Findings, fv)
		case SeverityWarning:
			fv.SeverityClass = "warning"
			warnGroup.Findings = append(warnGroup.Findings, fv)
		default:
			fv.SeverityClass = "info"
			infoGroup.Findings = append(infoGroup.Findings, fv)
		}
	}
	critGroup.Count = len(critGroup.Findings)
	warnGroup.Count = len(warnGroup.Findings)
	infoGroup.Count = len(infoGroup.Findings)

	var sevGroups []SeverityGroup
	if critGroup.Count > 0 {
		sevGroups = append(sevGroups, critGroup)
	}
	if warnGroup.Count > 0 {
		sevGroups = append(sevGroups, warnGroup)
	}
	if infoGroup.Count > 0 {
		sevGroups = append(sevGroups, infoGroup)
	}

	criticals, warnings, infos := countSeverities(result.Findings)

	data := struct {
		SkillName      string
		Date           string
		Stats          struct{ Critical, Warning, Info, Total, ModulesAffected int }
		Findings       []Finding
		ModuleGroups   []*ModuleGroup
		SeverityGroups []SeverityGroup
	}{
		SkillName:      result.SkillName,
		Date:           time.Now().Format("2006-01-02 15:04:05"),
		Stats:          struct{ Critical, Warning, Info, Total, ModulesAffected int }{criticals, warnings, infos, criticals + warnings + infos, len(moduleGroups)},
		Findings:       result.Findings,
		ModuleGroups:   moduleGroups,
		SeverityGroups: sevGroups,
	}

	tmpl, err := htmlReportTemplate()
	if err != nil {
		return fmt.Sprintf("Error generating HTML: %v", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return fmt.Sprintf("Error executing template: %v", err)
	}

	return sb.String()
}

// htmlReportTemplate returns the parsed HTML report template, parsing it only once.
func htmlReportTemplate() (*template.Template, error) {
	htmlReportOnce.once.Do(func() {
		htmlReportOnce.tmpl, htmlReportOnce.err = template.New("report").Parse(htmlTemplate)
	})
	return htmlReportOnce.tmpl, htmlReportOnce.err
}

func countSeverities(findings []Finding) (int, int, int) {
	c, w, i := 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			c++
		case SeverityWarning:
			w++
		case SeverityInfo:
			i++
		}
	}
	return c, w, i
}

func generateJSON(result *CheckResult) string {
	// Add timestamp to the result for JSON output since CheckResult doesn't have it by default
	type JSONResult struct {
		*CheckResult
		Timestamp string `json:"timestamp"`
	}

	output := JSONResult{
		CheckResult: result,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"error\": \"failed to marshal json: %v\"}", err)
	}
	return string(jsonData)
}

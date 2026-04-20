package service

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

const launchdTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.ServiceName}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.BinaryPath}}</string>
		<string>start</string>
	</array>
	<key>KeepAlive</key>
	<true/>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.LogDir}}/modeltap.log</string>
	<key>StandardErrorPath</key>
	<string>{{.LogDir}}/modeltap.err.log</string>
	<key>EnvironmentVariables</key>
	<dict>
	</dict>
	<key>WorkingDirectory</key>
	<string>{{.HomeDir}}</string>
</dict>
</plist>
`

type launchdData struct {
	ServiceName string
	BinaryPath  string
	LogDir      string
	HomeDir     string
}

// generateLaunchdPlist returns a macOS launchd plist XML string for the given config.
func generateLaunchdPlist(cfg Config) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}

	data := launchdData{
		ServiceName: cfg.ServiceName,
		BinaryPath:  cfg.BinaryPath,
		LogDir:      dataDir(home),
		HomeDir:     home,
	}

	tmpl, err := template.New("launchd").Parse(launchdTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing launchd template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing launchd template: %w", err)
	}

	return buf.String(), nil
}

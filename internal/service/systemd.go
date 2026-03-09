package service

import (
	"bytes"
	"fmt"
	"text/template"
)

const systemdTemplate = `[Unit]
Description=Modeltap API Proxy
After=network.target

[Service]
ExecStart={{.BinaryPath}} start
Restart=on-failure
RestartSec=5
Environment=MODELTAP_CONFIG={{.ConfigPath}}

[Install]
WantedBy=default.target
`

type systemdData struct {
	BinaryPath string
	ConfigPath string
}

// generateSystemdUnit returns a systemd unit file string for the given config.
func generateSystemdUnit(cfg Config) (string, error) {
	data := systemdData{
		BinaryPath: cfg.BinaryPath,
		ConfigPath: cfg.ConfigPath,
	}

	tmpl, err := template.New("systemd").Parse(systemdTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing systemd template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing systemd template: %w", err)
	}

	return buf.String(), nil
}

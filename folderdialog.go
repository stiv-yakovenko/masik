package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type folderPickResult struct {
	dir string
	err error
}

func chooseFolder(initialDir string) (string, error) {
	script := fmt.Sprintf(
		"Add-Type -AssemblyName System.Windows.Forms; "+
			"$dlg = New-Object System.Windows.Forms.OpenFileDialog; "+
			"$dlg.Title = 'Open Folder'; "+
			"$dlg.InitialDirectory = '%s'; "+
			"$dlg.Filter = 'Folders|*.folder'; "+
			"$dlg.CheckFileExists = $false; "+
			"$dlg.CheckPathExists = $true; "+
			"$dlg.ValidateNames = $false; "+
			"$dlg.FileName = 'Select this folder'; "+
			"if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { "+
			"[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; "+
			"$path = $dlg.FileName; "+
			"if (Test-Path -LiteralPath $path -PathType Container) { Write-Output $path } "+
			"else { Write-Output (Split-Path -Path $path -Parent) } }",
		escapePowerShellString(initialDir),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("folder dialog failed: %s", msg)
	}

	return strings.TrimSpace(string(out)), nil
}

func escapePowerShellString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func folderDisplayName(path string) string {
	clean := filepath.Clean(path)
	base := filepath.Base(clean)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return clean
	}
	return base
}

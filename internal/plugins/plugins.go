package plugins

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed embedded
var pluginFS embed.FS

func PluginManifestList() ([]string, error) {
	pluginsDir, err := pluginDir()
	if err != nil {
		return nil, err
	}

	entries, err := pluginFS.ReadDir("embedded")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded files: %w", err)
	}

	var plugins []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		plugins = append(plugins, filepath.Join(pluginsDir, entry.Name()))
	}
	return plugins, nil
}

func pluginDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	pluginsDir := filepath.Join(homeDir, ".k0da", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create plugins directory: %w", err)
	}
	return pluginsDir, nil
}

// ExtractPlugins extracts all embedded plugin YAML files to .k0da/plugins directory
func ExtractPlugins() ([]string, error) {
	pluginsDir, err := pluginDir()
	if err != nil {
		return nil, err
	}

	// Read all files from the embedded filesystem
	entries, err := pluginFS.ReadDir("embedded")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded files: %w", err)
	}

	var manifestPaths []string

	// Copy each YAML file to the plugins directory
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		fi, _ := entry.Info()

		// Read the embedded file
		filePath := filepath.Join("embedded", entry.Name())
		data, err := pluginFS.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded file %s: %w", filePath, err)
		}

		// Write to plugins directory
		destPath := filepath.Join(pluginsDir, entry.Name())
		dfi, err := os.Stat(destPath)
		if os.IsNotExist(err) || (err == nil && dfi.Size() != fi.Size()) {
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				return nil, fmt.Errorf("failed to write plugin file %s: %w", destPath, err)
			}
		}

		manifestPaths = append(manifestPaths, destPath)
	}

	return manifestPaths, nil
}

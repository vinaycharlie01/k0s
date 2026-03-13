// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// LoadRegistryConfig loads registry configuration from a YAML file
// Returns an empty config if the file doesn't exist
func LoadRegistryConfig(path string) (*RegistryConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RegistryConfig{
				Mirrors: make(map[string]MirrorConfig),
				Configs: make(map[string]HostConfig),
			}, nil
		}
		return nil, fmt.Errorf("failed to read registry config: %w", err)
	}

	var config RegistryConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse registry config: %w", err)
	}

	// Initialize maps if nil
	if config.Mirrors == nil {
		config.Mirrors = make(map[string]MirrorConfig)
	}
	if config.Configs == nil {
		config.Configs = make(map[string]HostConfig)
	}

	return &config, nil
}

// ApplyRegistryConfig applies registry configuration by generating hosts.toml files
// Accepts a RegistryConfig directly from k0s spec
func ApplyRegistryConfig(config *RegistryConfig, certsDir string) error {
	// If config is nil or no registries configured, nothing to do
	if config == nil || (len(config.Mirrors) == 0 && len(config.Configs) == 0) {
		logrus.Debug("No registry configuration found, skipping")
		return nil
	}

	// Initialize maps if nil
	if config.Mirrors == nil {
		config.Mirrors = make(map[string]MirrorConfig)
	}
	if config.Configs == nil {
		config.Configs = make(map[string]HostConfig)
	}

	// Ensure certs.d directory exists
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		return fmt.Errorf("failed to create certs.d directory: %w", err)
	}

	// Collect all registries that need configuration
	allRegistries := make(map[string]bool)
	for registry := range config.Mirrors {
		allRegistries[registry] = true
	}
	for registry := range config.Configs {
		allRegistries[registry] = true
	}

	// Generate hosts.toml for each registry
	for registry := range allRegistries {
		if err := generateHostsToml(registry, config, certsDir); err != nil {
			logrus.Warnf("Failed to generate hosts.toml for %s: %v", registry, err)
			continue
		}
		logrus.Infof("Generated hosts.toml for registry %s", registry)
	}

	return nil
}

// ApplyRegistryConfigFromFile loads registry configuration from a YAML file and applies it
// This is a convenience wrapper around LoadRegistryConfig and ApplyRegistryConfig
func ApplyRegistryConfigFromFile(registryConfigPath, certsDir string) error {
	config, err := LoadRegistryConfig(registryConfigPath)
	if err != nil {
		return err
	}

	return ApplyRegistryConfig(config, certsDir)
}

// ConvertRegistrySpec converts v1beta1.RegistrySpec to RegistryConfig
func ConvertRegistrySpec(spec *v1beta1.RegistrySpec) *RegistryConfig {
	if spec == nil {
		return nil
	}

	config := &RegistryConfig{
		Mirrors: make(map[string]MirrorConfig),
		Configs: make(map[string]HostConfig),
	}

	// Convert mirrors
	for registry, mirror := range spec.Mirrors {
		mirrorConfig := MirrorConfig{
			Endpoint: mirror.Endpoint,
		}

		// Convert rewrite rules
		if len(mirror.Rewrite) > 0 {
			mirrorConfig.Rewrite = make([]Rewrite, len(mirror.Rewrite))
			for i, r := range mirror.Rewrite {
				mirrorConfig.Rewrite[i] = Rewrite{
					Pattern: r.Pattern,
					Replace: r.Replace,
				}
			}
		}

		config.Mirrors[registry] = mirrorConfig
	}

	// Convert configs
	for registry, host := range spec.Configs {
		hostConfig := HostConfig{}

		// Convert auth
		if host.Auth != nil {
			hostConfig.Auth = &AuthConfig{
				Username: host.Auth.Username,
				Password: host.Auth.Password,
				Auth:     host.Auth.Auth,
			}
		}

		// Convert TLS
		if host.TLS != nil {
			hostConfig.TLS = &TLSConfig{
				CertFile:           host.TLS.CertFile,
				KeyFile:            host.TLS.KeyFile,
				CAFile:             host.TLS.CAFile,
				InsecureSkipVerify: host.TLS.InsecureSkipVerify,
			}
		}

		config.Configs[registry] = hostConfig
	}

	return config
}

// generateHostsToml generates a hosts.toml file for a specific registry
func generateHostsToml(registry string, config *RegistryConfig, certsDir string) error {
	content, err := buildHostsTomlContent(registry, config)
	if err != nil {
		return err
	}

	// Create registry-specific directory
	registryDir := filepath.Join(certsDir, normalizeRegistryName(registry))
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	// Write hosts.toml file
	hostsFile := filepath.Join(registryDir, "hosts.toml")
	if err := os.WriteFile(hostsFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write hosts.toml: %w", err)
	}

	return nil
}

// buildHostsTomlContent builds the content of a hosts.toml file
func buildHostsTomlContent(registry string, config *RegistryConfig) (string, error) {
	var content strings.Builder

	mirror, hasMirror := config.Mirrors[registry]
	hostConfig, hasConfig := config.Configs[registry]

	if !hasMirror && !hasConfig {
		return "", fmt.Errorf("no configuration found for registry %s", registry)
	}

	// Determine server URL
	serverURL := normalizeRegistryURL(registry)
	if hasMirror && len(mirror.Endpoint) > 0 {
		serverURL = mirror.Endpoint[0]
	}

	// Write server line
	content.WriteString(fmt.Sprintf("server = %q\n\n", serverURL))

	// Add mirror endpoints
	if hasMirror && len(mirror.Endpoint) > 0 {
		for _, endpoint := range mirror.Endpoint {
			writeHostSection(&content, endpoint, &hostConfig, &mirror)
		}
	} else {
		// No mirrors, just use the default endpoint
		writeHostSection(&content, serverURL, &hostConfig, nil)
	}

	return content.String(), nil
}

// writeHostSection writes a [host."..."] section to the content builder
func writeHostSection(content *strings.Builder, endpoint string, hostConfig *HostConfig, mirror *MirrorConfig) {
	content.WriteString(fmt.Sprintf("[host.%q]\n", endpoint))
	content.WriteString("  capabilities = [\"pull\", \"resolve\"]\n")

	// Add authentication if configured
	if hostConfig != nil && hostConfig.Auth != nil {
		writeAuthSection(content, endpoint, hostConfig.Auth)
	}

	// Add TLS configuration if configured
	if hostConfig != nil && hostConfig.TLS != nil {
		writeTLSSection(content, hostConfig.TLS)
	}

	// Add rewrite rules if configured
	if mirror != nil && len(mirror.Rewrite) > 0 {
		writeRewriteSection(content, mirror.Rewrite)
	}

	content.WriteString("\n")
}

// writeAuthSection writes authentication configuration
func writeAuthSection(content *strings.Builder, endpoint string, auth *AuthConfig) {
	var authToken string

	if auth.Auth != "" {
		// Use pre-encoded auth token
		authToken = auth.Auth
	} else if auth.Username != "" && auth.Password != "" {
		// Encode username:password
		creds := fmt.Sprintf("%s:%s", auth.Username, auth.Password)
		authToken = base64.StdEncoding.EncodeToString([]byte(creds))
	}

	if authToken != "" {
		content.WriteString(fmt.Sprintf("  [host.%q.header]\n", endpoint))
		content.WriteString(fmt.Sprintf("    authorization = [\"Basic %s\"]\n", authToken))
	}
}

// writeTLSSection writes TLS configuration
func writeTLSSection(content *strings.Builder, tls *TLSConfig) {
	if tls.CAFile != "" {
		content.WriteString(fmt.Sprintf("  ca = %q\n", tls.CAFile))
	}

	if tls.CertFile != "" && tls.KeyFile != "" {
		content.WriteString(fmt.Sprintf("  client = [[%q, %q]]\n", tls.CertFile, tls.KeyFile))
	}

	if tls.InsecureSkipVerify {
		content.WriteString("  skip_verify = true\n")
	}
}

// writeRewriteSection writes rewrite rules
func writeRewriteSection(content *strings.Builder, rewrites []Rewrite) {
	if len(rewrites) == 0 {
		return
	}

	// Note: containerd's hosts.toml doesn't directly support rewrite rules
	// This is a limitation compared to k3s
	// We document this in comments for now
	content.WriteString("  # Note: Rewrite rules are not directly supported in containerd hosts.toml\n")
	for _, rewrite := range rewrites {
		content.WriteString(fmt.Sprintf("  # Rewrite: %q -> %q\n", rewrite.Pattern, rewrite.Replace))
	}
}

// normalizeRegistryURL ensures the registry URL has a scheme
func normalizeRegistryURL(registry string) string {
	if strings.HasPrefix(registry, "http://") || strings.HasPrefix(registry, "https://") {
		return registry
	}
	return "https://" + registry
}

// normalizeRegistryName normalizes registry name for directory naming
// Replaces : with _ for Windows compatibility (similar to k3s)
func normalizeRegistryName(registry string) string {
	// Remove scheme if present
	registry = strings.TrimPrefix(registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")

	// Replace : with _ for Windows compatibility
	registry = strings.ReplaceAll(registry, ":", "_")

	return registry
}

// Made with Bob

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRegistryConfig(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		validate    func(*testing.T, *RegistryConfig)
	}{
		{
			name:        "non-existent file returns empty config",
			content:     "",
			expectError: false,
			validate: func(t *testing.T, config *RegistryConfig) {
				if config == nil {
					t.Fatal("expected non-nil config")
				}
				if config.Mirrors == nil {
					t.Error("expected non-nil Mirrors map")
				}
				if config.Configs == nil {
					t.Error("expected non-nil Configs map")
				}
			},
		},
		{
			name: "valid config with mirrors",
			content: `mirrors:
  docker.io:
    endpoint:
      - https://registry-1.docker.io
      - https://mirror.example.com
`,
			expectError: false,
			validate: func(t *testing.T, config *RegistryConfig) {
				if len(config.Mirrors) != 1 {
					t.Errorf("expected 1 mirror, got %d", len(config.Mirrors))
				}
				mirror, ok := config.Mirrors["docker.io"]
				if !ok {
					t.Fatal("expected docker.io mirror")
				}
				if len(mirror.Endpoint) != 2 {
					t.Errorf("expected 2 endpoints, got %d", len(mirror.Endpoint))
				}
			},
		},
		{
			name: "valid config with auth",
			content: `configs:
  registry.example.com:
    auth:
      username: testuser
      password: testpass
`,
			expectError: false,
			validate: func(t *testing.T, config *RegistryConfig) {
				if len(config.Configs) != 1 {
					t.Errorf("expected 1 config, got %d", len(config.Configs))
				}
				hostConfig, ok := config.Configs["registry.example.com"]
				if !ok {
					t.Fatal("expected registry.example.com config")
				}
				if hostConfig.Auth == nil {
					t.Fatal("expected auth config")
				}
				if hostConfig.Auth.Username != "testuser" {
					t.Errorf("expected username 'testuser', got '%s'", hostConfig.Auth.Username)
				}
			},
		},
		{
			name:        "invalid yaml",
			content:     "invalid: yaml: content:",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.content != "" {
				tmpDir := t.TempDir()
				path = filepath.Join(tmpDir, "registries.yaml")
				if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			} else {
				path = filepath.Join(t.TempDir(), "non-existent.yaml")
			}

			config, err := LoadRegistryConfig(path)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestBuildHostsTomlContent(t *testing.T) {
	tests := []struct {
		name        string
		registry    string
		config      *RegistryConfig
		expectError bool
		contains    []string
		notContains []string
	}{
		{
			name:     "simple registry without config",
			registry: "docker.io",
			config: &RegistryConfig{
				Mirrors: map[string]MirrorConfig{
					"docker.io": {
						Endpoint: []string{"https://registry-1.docker.io"},
					},
				},
				Configs: map[string]HostConfig{},
			},
			expectError: false,
			contains: []string{
				`server = "https://registry-1.docker.io"`,
				`[host."https://registry-1.docker.io"]`,
				`capabilities = ["pull", "resolve"]`,
			},
		},
		{
			name:     "registry with authentication",
			registry: "registry.example.com",
			config: &RegistryConfig{
				Mirrors: map[string]MirrorConfig{},
				Configs: map[string]HostConfig{
					"registry.example.com": {
						Auth: &AuthConfig{
							Username: "testuser",
							Password: "testpass",
						},
					},
				},
			},
			expectError: false,
			contains: []string{
				`server = "https://registry.example.com"`,
				`[host."https://registry.example.com"]`,
				`[host."https://registry.example.com".header]`,
				`authorization = ["Basic ` + base64.StdEncoding.EncodeToString([]byte("testuser:testpass")) + `"]`,
			},
		},
		{
			name:     "registry with pre-encoded auth",
			registry: "registry.example.com",
			config: &RegistryConfig{
				Mirrors: map[string]MirrorConfig{},
				Configs: map[string]HostConfig{
					"registry.example.com": {
						Auth: &AuthConfig{
							Auth: "dGVzdHVzZXI6dGVzdHBhc3M=", // testuser:testpass
						},
					},
				},
			},
			expectError: false,
			contains: []string{
				`authorization = ["Basic dGVzdHVzZXI6dGVzdHBhc3M="]`,
			},
		},
		{
			name:     "registry with TLS config",
			registry: "registry.example.com",
			config: &RegistryConfig{
				Mirrors: map[string]MirrorConfig{},
				Configs: map[string]HostConfig{
					"registry.example.com": {
						TLS: &TLSConfig{
							CAFile:             "/etc/certs/ca.crt",
							CertFile:           "/etc/certs/client.crt",
							KeyFile:            "/etc/certs/client.key",
							InsecureSkipVerify: true,
						},
					},
				},
			},
			expectError: false,
			contains: []string{
				`ca = "/etc/certs/ca.crt"`,
				`client = [["/etc/certs/client.crt", "/etc/certs/client.key"]]`,
				`skip_verify = true`,
			},
		},
		{
			name:     "registry with multiple mirrors",
			registry: "docker.io",
			config: &RegistryConfig{
				Mirrors: map[string]MirrorConfig{
					"docker.io": {
						Endpoint: []string{
							"https://registry-1.docker.io",
							"https://mirror.example.com",
						},
					},
				},
				Configs: map[string]HostConfig{},
			},
			expectError: false,
			contains: []string{
				`server = "https://registry-1.docker.io"`,
				`[host."https://registry-1.docker.io"]`,
				`[host."https://mirror.example.com"]`,
			},
		},
		{
			name:     "registry with rewrite rules",
			registry: "docker.io",
			config: &RegistryConfig{
				Mirrors: map[string]MirrorConfig{
					"docker.io": {
						Endpoint: []string{"https://mirror.example.com"},
						Rewrite: []Rewrite{
							{Pattern: "^library/(.*)$", Replace: "myorg/$1"},
						},
					},
				},
				Configs: map[string]HostConfig{},
			},
			expectError: false,
			contains: []string{
				`# Note: Rewrite rules are not directly supported in containerd hosts.toml`,
				`# Rewrite: "^library/(.*)$" -> "myorg/$1"`,
			},
		},
		{
			name:        "registry without any config",
			registry:    "unknown.registry.com",
			config:      &RegistryConfig{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := buildHostsTomlContent(tt.registry, tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(content, expected) {
					t.Errorf("expected content to contain %q, but it didn't.\nContent:\n%s", expected, content)
				}
			}

			for _, notExpected := range tt.notContains {
				if strings.Contains(content, notExpected) {
					t.Errorf("expected content to NOT contain %q, but it did.\nContent:\n%s", notExpected, content)
				}
			}
		})
	}
}

func TestNormalizeRegistryURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"docker.io", "https://docker.io"},
		{"registry.example.com", "https://registry.example.com"},
		{"https://registry.example.com", "https://registry.example.com"},
		{"http://registry.example.com", "http://registry.example.com"},
		{"localhost:5000", "https://localhost:5000"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeRegistryURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeRegistryURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeRegistryName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"docker.io", "docker.io"},
		{"registry.example.com", "registry.example.com"},
		{"https://registry.example.com", "registry.example.com"},
		{"http://registry.example.com", "registry.example.com"},
		{"localhost:5000", "localhost_5000"},
		{"registry.example.com:443", "registry.example.com_443"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeRegistryName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeRegistryName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestApplyRegistryConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
		validate    func(*testing.T, string)
	}{
		{
			name:        "empty config does nothing",
			config:      "",
			expectError: false,
			validate: func(t *testing.T, certsDir string) {
				// Should not create any files
				entries, err := os.ReadDir(certsDir)
				if err != nil && !os.IsNotExist(err) {
					t.Fatalf("failed to read certs dir: %v", err)
				}
				if len(entries) > 0 {
					t.Errorf("expected no entries in certs dir, got %d", len(entries))
				}
			},
		},
		{
			name: "creates hosts.toml for configured registry",
			config: `mirrors:
  docker.io:
    endpoint:
      - https://registry-1.docker.io
`,
			expectError: false,
			validate: func(t *testing.T, certsDir string) {
				hostsFile := filepath.Join(certsDir, "docker.io", "hosts.toml")
				if _, err := os.Stat(hostsFile); os.IsNotExist(err) {
					t.Errorf("expected hosts.toml to exist at %s", hostsFile)
				}

				content, err := os.ReadFile(hostsFile)
				if err != nil {
					t.Fatalf("failed to read hosts.toml: %v", err)
				}

				if !strings.Contains(string(content), `server = "https://registry-1.docker.io"`) {
					t.Error("hosts.toml doesn't contain expected server line")
				}
			},
		},
		{
			name: "handles registry with port",
			config: `mirrors:
  localhost:5000:
    endpoint:
      - http://localhost:5000
`,
			expectError: false,
			validate: func(t *testing.T, certsDir string) {
				// Should normalize : to _ in directory name
				hostsFile := filepath.Join(certsDir, "localhost_5000", "hosts.toml")
				if _, err := os.Stat(hostsFile); os.IsNotExist(err) {
					t.Errorf("expected hosts.toml to exist at %s", hostsFile)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "registries.yaml")
			certsDir := filepath.Join(tmpDir, "certs.d")

			if tt.config != "" {
				if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
					t.Fatalf("failed to write config file: %v", err)
				}
			} else {
				configPath = filepath.Join(tmpDir, "non-existent.yaml")
			}

			err := ApplyRegistryConfig(configPath, certsDir)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, certsDir)
			}
		})
	}
}

func TestBuildAuthSection(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		auth     *AuthConfig
		expected []string
	}{
		{
			name:     "username and password",
			endpoint: "https://registry.example.com",
			auth: &AuthConfig{
				Username: "user",
				Password: "pass",
			},
			expected: []string{
				`  [host."https://registry.example.com".header]`,
				`    authorization = ["Basic ` + base64.StdEncoding.EncodeToString([]byte("user:pass")) + `"]`,
			},
		},
		{
			name:     "pre-encoded auth",
			endpoint: "https://registry.example.com",
			auth: &AuthConfig{
				Auth: "dGVzdDp0ZXN0",
			},
			expected: []string{
				`  [host."https://registry.example.com".header]`,
				`    authorization = ["Basic dGVzdDp0ZXN0"]`,
			},
		},
		{
			name:     "no auth",
			endpoint: "https://registry.example.com",
			auth:     &AuthConfig{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAuthSection(tt.endpoint, tt.auth)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d", len(tt.expected), len(result))
				return
			}

			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}

func TestBuildTLSSection(t *testing.T) {
	tests := []struct {
		name     string
		tls      *TLSConfig
		expected []string
	}{
		{
			name: "full TLS config",
			tls: &TLSConfig{
				CAFile:             "/etc/certs/ca.crt",
				CertFile:           "/etc/certs/client.crt",
				KeyFile:            "/etc/certs/client.key",
				InsecureSkipVerify: true,
			},
			expected: []string{
				`  ca = "/etc/certs/ca.crt"`,
				`  client = [["/etc/certs/client.crt", "/etc/certs/client.key"]]`,
				`  skip_verify = true`,
			},
		},
		{
			name: "only CA file",
			tls: &TLSConfig{
				CAFile: "/etc/certs/ca.crt",
			},
			expected: []string{
				`  ca = "/etc/certs/ca.crt"`,
			},
		},
		{
			name: "only skip verify",
			tls: &TLSConfig{
				InsecureSkipVerify: true,
			},
			expected: []string{
				`  skip_verify = true`,
			},
		},
		{
			name:     "empty TLS config",
			tls:      &TLSConfig{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTLSSection(tt.tls)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d", len(tt.expected), len(result))
				return
			}

			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}

func TestBuildRewriteSection(t *testing.T) {
	tests := []struct {
		name     string
		rewrites []Rewrite
		expected []string
	}{
		{
			name: "single rewrite",
			rewrites: []Rewrite{
				{Pattern: "^library/(.*)$", Replace: "myorg/$1"},
			},
			expected: []string{
				`  # Note: Rewrite rules are not directly supported in containerd hosts.toml`,
				`  # Rewrite: "^library/(.*)$" -> "myorg/$1"`,
			},
		},
		{
			name: "multiple rewrites",
			rewrites: []Rewrite{
				{Pattern: "^library/(.*)$", Replace: "myorg/$1"},
				{Pattern: "^(.*)$", Replace: "prefix/$1"},
			},
			expected: []string{
				`  # Note: Rewrite rules are not directly supported in containerd hosts.toml`,
				`  # Rewrite: "^library/(.*)$" -> "myorg/$1"`,
				`  # Rewrite: "^(.*)$" -> "prefix/$1"`,
			},
		},
		{
			name:     "no rewrites",
			rewrites: []Rewrite{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRewriteSection(tt.rewrites)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d", len(tt.expected), len(result))
				return
			}

			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}

// Made with Bob

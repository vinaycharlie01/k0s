// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

// RegistryConfig represents the k0s registry configuration
// This follows a similar structure to k3s registries.yaml
type RegistryConfig struct {
	Mirrors map[string]MirrorConfig `yaml:"mirrors,omitempty"`
	Configs map[string]HostConfig   `yaml:"configs,omitempty"`
}

// MirrorConfig defines mirror endpoints for a registry
type MirrorConfig struct {
	Endpoint []string  `yaml:"endpoint,omitempty"`
	Rewrite  []Rewrite `yaml:"rewrite,omitempty"`
}

// Rewrite defines a rewrite rule for registry paths
type Rewrite struct {
	Pattern string `yaml:"pattern"`
	Replace string `yaml:"replace"`
}

// HostConfig defines authentication and TLS configuration for a registry
type HostConfig struct {
	Auth *AuthConfig `yaml:"auth,omitempty"`
	TLS  *TLSConfig  `yaml:"tls,omitempty"`
}

// AuthConfig defines authentication credentials
type AuthConfig struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Auth     string `yaml:"auth,omitempty"` // Base64 encoded username:password
}

// TLSConfig defines TLS/SSL configuration
type TLSConfig struct {
	CertFile           string `yaml:"cert_file,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	CAFile             string `yaml:"ca_file,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
}

// Made with Bob

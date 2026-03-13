// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// RegistrySpec defines the registry configuration for containerd
// +kubebuilder:object:generate=true
type RegistrySpec struct {
	// Mirrors defines registry mirrors
	Mirrors map[string]MirrorSpec `json:"mirrors,omitempty"`
	// Configs defines registry authentication and TLS configuration
	Configs map[string]HostSpec `json:"configs,omitempty"`
}

// MirrorSpec defines mirror endpoints for a registry
// +kubebuilder:object:generate=true
type MirrorSpec struct {
	// Endpoint is a list of mirror endpoints
	Endpoint []string `json:"endpoint,omitempty"`
	// Rewrite defines path rewrite rules
	Rewrite []RewriteSpec `json:"rewrite,omitempty"`
}

// RewriteSpec defines a rewrite rule for registry paths
// +kubebuilder:object:generate=true
type RewriteSpec struct {
	// Pattern is the regex pattern to match
	Pattern string `json:"pattern"`
	// Replace is the replacement string
	Replace string `json:"replace"`
}

// HostSpec defines authentication and TLS configuration for a registry
// +kubebuilder:object:generate=true
type HostSpec struct {
	// Auth defines authentication credentials
	Auth *AuthSpec `json:"auth,omitempty"`
	// TLS defines TLS/SSL configuration
	TLS *TLSSpec `json:"tls,omitempty"`
}

// AuthSpec defines authentication credentials
// +kubebuilder:object:generate=true
type AuthSpec struct {
	// Username for authentication
	Username string `json:"username,omitempty"`
	// Password for authentication
	Password string `json:"password,omitempty"`
	// Auth is base64 encoded username:password
	Auth string `json:"auth,omitempty"`
}

// TLSSpec defines TLS/SSL configuration
// +kubebuilder:object:generate=true
type TLSSpec struct {
	// CertFile is the path to the client certificate
	CertFile string `json:"certFile,omitempty"`
	// KeyFile is the path to the client key
	KeyFile string `json:"keyFile,omitempty"`
	// CAFile is the path to the CA certificate
	CAFile string `json:"caFile,omitempty"`
	// InsecureSkipVerify skips TLS verification
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// Made with Bob

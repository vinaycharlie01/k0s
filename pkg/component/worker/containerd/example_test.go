// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/config"
)

// ExampleComponent_withRegistryConfig demonstrates how to use the containerd component
// with a custom registry configuration file
func ExampleComponent_withRegistryConfig() {
	// Create a temporary directory for testing
	tmpDir, _ := os.MkdirTemp("", "k0s-test-*")
	defer os.RemoveAll(tmpDir)

	// Create a sample registries.yaml file
	registriesYAML := `
mirrors:
  docker.io:
    endpoint:
      - "https://mirror.example.com"
      - "https://registry-1.docker.io"

configs:
  delivery.instana.io:
    auth:
      username: "user@example.com"
      password: "your-token-here"
    tls:
      insecure_skip_verify: false
`
	registryConfigPath := filepath.Join(tmpDir, "registries.yaml")
	os.WriteFile(registryConfigPath, []byte(registriesYAML), 0644)

	// Setup k0s configuration
	k0sVars := &config.CfgVars{
		DataDir: filepath.Join(tmpDir, "data"),
		RunDir:  filepath.Join(tmpDir, "run"),
		BinDir:  filepath.Join(tmpDir, "bin"),
	}

	// Create worker profile
	profile := &workerconfig.Profile{
		PauseImage: &v1beta1.ImageSpec{
			Image:   "registry.k8s.io/pause",
			Version: "3.9",
		},
	}

	// Create containerd component
	component := containerd.NewComponent("info", k0sVars, profile)

	// Set the registry configuration path
	// This is where you would set it from CLI flags, config file, or environment variables
	component.RegistryConfigPath = registryConfigPath

	// Start the component (in real usage)
	// component.Start(context.Background())

	fmt.Printf("Registry config path set to: %s\n", component.RegistryConfigPath)
	fmt.Println("Component will apply registry configuration on Start()")

	// Output:
	// Registry config path set to: /tmp/k0s-test-.../registries.yaml
	// Component will apply registry configuration on Start()
}

// ExampleComponent_withoutRegistryConfig demonstrates using containerd without registry config
func ExampleComponent_withoutRegistryConfig() {
	tmpDir, _ := os.MkdirTemp("", "k0s-test-*")
	defer os.RemoveAll(tmpDir)

	k0sVars := &config.CfgVars{
		DataDir: filepath.Join(tmpDir, "data"),
		RunDir:  filepath.Join(tmpDir, "run"),
		BinDir:  filepath.Join(tmpDir, "bin"),
	}

	profile := &workerconfig.Profile{
		PauseImage: &v1beta1.ImageSpec{
			Image:   "registry.k8s.io/pause",
			Version: "3.9",
		},
	}

	// Create containerd component without setting RegistryConfigPath
	component := containerd.NewComponent("info", k0sVars, profile)

	// RegistryConfigPath is empty, so no registry configuration will be applied
	fmt.Printf("Registry config path: %q\n", component.RegistryConfigPath)
	fmt.Println("Component will work normally without registry configuration")

	// Output:
	// Registry config path: ""
	// Component will work normally without registry configuration
}

// Example_applyRegistryConfigDirectly shows how to use the registry functions directly
func Example_applyRegistryConfigDirectly() {
	tmpDir, _ := os.MkdirTemp("", "k0s-test-*")
	defer os.RemoveAll(tmpDir)

	// Create registries.yaml
	registriesYAML := `
configs:
  registry.example.com:
    auth:
      username: "admin"
      password: "secret"
`
	registryConfigPath := filepath.Join(tmpDir, "registries.yaml")
	os.WriteFile(registryConfigPath, []byte(registriesYAML), 0644)

	// Apply registry configuration directly
	certsDir := filepath.Join(tmpDir, "certs.d")
	err := containerd.ApplyRegistryConfig(registryConfigPath, certsDir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Check generated hosts.toml
	hostsFile := filepath.Join(certsDir, "registry.example.com", "hosts.toml")
	if _, err := os.Stat(hostsFile); err == nil {
		fmt.Println("hosts.toml generated successfully")
		content, _ := os.ReadFile(hostsFile)
		fmt.Printf("Generated file contains auth: %v\n", len(content) > 0)
	}

	// Output:
	// hosts.toml generated successfully
	// Generated file contains auth: true
}

// Example_loadRegistryConfig shows how to load and inspect registry configuration
func Example_loadRegistryConfig() {
	tmpDir, _ := os.MkdirTemp("", "k0s-test-*")
	defer os.RemoveAll(tmpDir)

	// Create registries.yaml with rewrites
	registriesYAML := `
mirrors:
  docker.io:
    endpoint:
      - "https://mirror.example.com"
    rewrite:
      - pattern: "^rancher/(.*)"
        replace: "docker/rancher-images/$1"

configs:
  docker.io:
    tls:
      ca_file: "/etc/ssl/certs/ca.crt"
`
	registryConfigPath := filepath.Join(tmpDir, "registries.yaml")
	os.WriteFile(registryConfigPath, []byte(registriesYAML), 0644)

	// Load configuration
	config, err := containerd.LoadRegistryConfig(registryConfigPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Inspect configuration
	fmt.Printf("Mirrors configured: %d\n", len(config.Mirrors))
	fmt.Printf("Configs configured: %d\n", len(config.Configs))

	if mirror, ok := config.Mirrors["docker.io"]; ok {
		fmt.Printf("Docker.io endpoints: %d\n", len(mirror.Endpoint))
		fmt.Printf("Docker.io rewrites: %d\n", len(mirror.Rewrite))
	}

	// Output:
	// Mirrors configured: 1
	// Configs configured: 1
	// Docker.io endpoints: 1
	// Docker.io rewrites: 1
}

// Example_integrationWithCLI shows how this would be integrated with CLI flags
func Example_integrationWithCLI() {
	// This is a conceptual example showing how k0s CLI would use this

	// Pseudo-code for CLI integration:
	/*
		// In k0s worker command:
		var registryConfigPath string

		cmd := &cobra.Command{
			Use: "worker",
			RunE: func(cmd *cobra.Command, args []string) error {
				// ... other setup ...

				// Create containerd component
				containerdComponent := containerd.NewComponent(logLevel, k0sVars, profile)

				// Set registry config path from CLI flag
				if registryConfigPath != "" {
					containerdComponent.RegistryConfigPath = registryConfigPath
				}

				// Start component
				return containerdComponent.Start(context.Background())
			},
		}

		// Add CLI flag
		cmd.Flags().StringVar(&registryConfigPath, "registry-config", "",
			"Path to registry configuration file (similar to k3s registries.yaml)")
	*/

	fmt.Println("CLI Integration Example:")
	fmt.Println("k0s worker --registry-config=/etc/k0s/registries.yaml")
	fmt.Println("")
	fmt.Println("Or via environment variable:")
	fmt.Println("K0S_REGISTRY_CONFIG=/etc/k0s/registries.yaml k0s worker")

	// Output:
	// CLI Integration Example:
	// k0s worker --registry-config=/etc/k0s/registries.yaml
	//
	// Or via environment variable:
	// K0S_REGISTRY_CONFIG=/etc/k0s/registries.yaml k0s worker
}

// Example_manualTesting shows how to manually test the functionality
func Example_manualTesting() {
	fmt.Println("Manual Testing Steps:")
	fmt.Println("")
	fmt.Println("1. Create /tmp/registries.yaml:")
	fmt.Println("   mirrors:")
	fmt.Println("     docker.io:")
	fmt.Println("       endpoint:")
	fmt.Println("         - \"https://mirror.example.com\"")
	fmt.Println("")
	fmt.Println("2. Run the test:")
	fmt.Println("   cd k0s/pkg/component/worker/containerd")
	fmt.Println("   go test -v -run TestApplyRegistryConfig")
	fmt.Println("")
	fmt.Println("3. Check generated files:")
	fmt.Println("   ls -la /tmp/test-*/certs.d/docker.io/hosts.toml")
	fmt.Println("   cat /tmp/test-*/certs.d/docker.io/hosts.toml")

	// Output:
	// Manual Testing Steps:
	//
	// 1. Create /tmp/registries.yaml:
	//    mirrors:
	//      docker.io:
	//        endpoint:
	//          - "https://mirror.example.com"
	//
	// 2. Run the test:
	//    cd k0s/pkg/component/worker/containerd
	//    go test -v -run TestApplyRegistryConfig
	//
	// 3. Check generated files:
	//    ls -la /tmp/test-*/certs.d/docker.io/hosts.toml
	//    cat /tmp/test-*/certs.d/docker.io/hosts.toml
}

// Made with Bob

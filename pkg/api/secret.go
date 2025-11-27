// Implementation of Secret feature from the Compose spec
package api

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// SecretSpec defines a secret object that can be mounted into containers
type SecretSpec struct {
	Name string

	// Content of the secret when specified inline
	Content []byte `json:",omitempty"`

	// Note: NOT IMPLEMENTED
	// External indicates this secret already exists and should not be created
	// External bool `json:",omitempty"`

	// Note: NOT IMPLEMENTED
	// Labels for the secret
	// Labels map[string]string `json:",omitempty"`

	// TODO: add support for "environment"
}

func (s *SecretSpec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("secret name is required")
	}
	return nil
}

// Equals compares two SecretSpec instances
func (s *SecretSpec) Equals(other SecretSpec) bool {
	return s.Name == other.Name &&
		bytes.Equal(s.Content, other.Content)
}

// SecretMount defines how a secret is mounted into a container
type SecretMount struct {
	// SecretName references a secret defined in ServiceSpec.Secrets by its Name field
	SecretName string
	// ContainerPath is the absolute path where the secret is mounted in the container
	ContainerPath string `json:",omitempty"`
	// Uid for the mounted secret file
	Uid string `json:",omitempty"`
	// Gid for the mounted secret file
	Gid string `json:",omitempty"`
	// Mode (file permissions) for the mounted secret file
	Mode *os.FileMode `json:",omitempty"`
}

func (s *SecretMount) GetNumericUid() (*uint64, error) {
	if s.Uid == "" {
		return nil, nil
	}
	uid, err := strconv.ParseUint(s.Uid, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid Uid '%s': %w", s.Uid, err)
	}
	if int(uid) < 0 {
		return nil, fmt.Errorf("invalid Uid '%s': value too high", s.Uid)
	}
	return &uid, nil
}

func (s *SecretMount) GetNumericGid() (*uint64, error) {
	if s.Gid == "" {
		return nil, nil
	}
	gid, err := strconv.ParseUint(s.Gid, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid Gid '%s': %w", s.Gid, err)
	}
	if int(gid) < 0 {
		return nil, fmt.Errorf("invalid Gid '%s': value too high", s.Gid)
	}
	return &gid, nil
}

func (s *SecretMount) Validate() error {
	if s.SecretName == "" {
		return fmt.Errorf("secret mount source is required")
	}
	if _, err := s.GetNumericUid(); err != nil {
		return err
	}
	if _, err := s.GetNumericGid(); err != nil {
		return err
	}
	if s.ContainerPath != "" && !filepath.IsAbs(s.ContainerPath) {
		return fmt.Errorf("container path must be absolute")
	}
	return nil
}

// ValidateSecretsAndMounts takes secret specs and secret mounts and validates that all mounts refer to existing specs
func ValidateSecretsAndMounts(secrets []SecretSpec, mounts []SecretMount) error {
	secretMap := make(map[string]struct{})
	for _, secret := range secrets {
		if err := secret.Validate(); err != nil {
			return fmt.Errorf("invalid secret: %w", err)
		}
		if _, ok := secretMap[secret.Name]; ok {
			return fmt.Errorf("duplicate secret name: '%s'", secret.Name)
		}

		secretMap[secret.Name] = struct{}{}
	}

	for _, mount := range mounts {
		if err := mount.Validate(); err != nil {
			return fmt.Errorf("invalid secret mount: %w", err)
		}
		if _, exists := secretMap[mount.SecretName]; !exists {
			return fmt.Errorf("secret mount source '%s' does not refer to any defined secret", mount.SecretName)
		}
	}

	return nil
}

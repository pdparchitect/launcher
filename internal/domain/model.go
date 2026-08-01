package domain

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/pdparchitect/launcher/internal/catalog"
)

type DesiredState string

const (
	DesiredRunning DesiredState = "running"
	DesiredStopped DesiredState = "stopped"
)

type Instance struct {
	ID              string               `json:"id"`
	CatalogID       string               `json:"catalogId"`
	Name            string               `json:"name"`
	Image           string               `json:"image"`
	ContainerName   string               `json:"containerName"`
	Interfaces      map[string]Interface `json:"interfaces"`
	DesiredState    DesiredState         `json:"desiredState"`
	CreatedAt       time.Time            `json:"createdAt"`
	RuntimeManifest *catalog.Manifest    `json:"runtimeManifest,omitempty"`
}

type Interface struct {
	Kind string `json:"kind"`
	Port int    `json:"port"`
	Path string `json:"path"`
}

func (instance Instance) Validate() error {
	if !ValidID(instance.ID) {
		return errors.New("instance ID must contain 32 lowercase hexadecimal characters")
	}
	if strings.TrimSpace(instance.CatalogID) == "" {
		return errors.New("catalogue ID is required")
	}
	if err := ValidateName(instance.Name); err != nil {
		return err
	}
	if strings.TrimSpace(instance.Image) == "" ||
		strings.TrimSpace(instance.ContainerName) == "" {
		return errors.New("image and container name are required")
	}
	if len(instance.Interfaces) == 0 {
		return errors.New("at least one resolved interface is required")
	}
	for id, resolved := range instance.Interfaces {
		if !validInterfaceToken(id) || !validInterfaceToken(resolved.Kind) {
			return fmt.Errorf("invalid resolved interface %q", id)
		}
		if resolved.Port < 1 || resolved.Port > 65535 {
			return fmt.Errorf(
				"resolved interface %q port must be between 1 and 65535",
				id,
			)
		}
		if !strings.HasPrefix(resolved.Path, "/") {
			return fmt.Errorf(
				"resolved interface %q path must be absolute",
				id,
			)
		}
	}
	if instance.DesiredState != DesiredRunning &&
		instance.DesiredState != DesiredStopped {
		return fmt.Errorf("invalid desired state %q", instance.DesiredState)
	}
	if instance.CreatedAt.IsZero() {
		return errors.New("creation time is required")
	}
	if instance.RuntimeManifest != nil {
		if instance.RuntimeManifest.ID != instance.CatalogID {
			return errors.New("runtime manifest catalogue ID does not match instance")
		}
		if instance.RuntimeManifest.Image != instance.Image {
			return errors.New("runtime manifest image does not match instance")
		}
	}
	return nil
}

func ValidID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("agent name is required")
	}
	if len(name) > 80 {
		return errors.New("agent name must be at most 80 characters")
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return errors.New("agent name cannot contain control characters")
		}
	}
	return nil
}

func (resolved Interface) URL() string {
	return (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("127.0.0.1:%d", resolved.Port),
		Path:   resolved.Path,
	}).String()
}

func (instance Instance) DisplayInterface() (string, Interface, bool) {
	if resolved, exists := instance.Interfaces["desktop"]; exists &&
		displayKind(resolved.Kind) {
		return "desktop", resolved, true
	}
	ids := make([]string, 0, len(instance.Interfaces))
	for id, resolved := range instance.Interfaces {
		if displayKind(resolved.Kind) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return "", Interface{}, false
	}
	sort.Strings(ids)
	id := ids[0]
	return id, instance.Interfaces[id], true
}

func validInterfaceToken(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			character != '-' {
			return false
		}
	}
	return true
}

func displayKind(kind string) bool {
	return kind == "web" || kind == "kasmweb"
}

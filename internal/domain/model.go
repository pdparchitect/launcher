package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
)

type DesiredState string

const (
	DesiredRunning DesiredState = "running"
	DesiredStopped DesiredState = "stopped"
)

type Instance struct {
	ID            string       `json:"id"`
	CatalogID     string       `json:"catalogId"`
	Name          string       `json:"name"`
	Image         string       `json:"image"`
	ContainerName string       `json:"containerName"`
	Port          int          `json:"port"`
	DesiredState  DesiredState `json:"desiredState"`
	CreatedAt     time.Time    `json:"createdAt"`
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
	if instance.Port < 1 || instance.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if instance.DesiredState != DesiredRunning &&
		instance.DesiredState != DesiredStopped {
		return fmt.Errorf("invalid desired state %q", instance.DesiredState)
	}
	if instance.CreatedAt.IsZero() {
		return errors.New("creation time is required")
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

func (instance Instance) URL() string {
	return (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("127.0.0.1:%d", instance.Port),
	}).String()
}

package sops

import (
	"testing"

	"github.com/helmfile/vals/pkg/log"
)

func TestFormatDefault(t *testing.T) {
	p := &provider{
		log: log.New(log.Config{}),
	}

	got := p.format("")
	if got != "" {
		t.Errorf("expected empty string for default format, got %q", got)
	}
}

func TestFormatExplicitOverride(t *testing.T) {
	p := &provider{
		log:    log.New(log.Config{}),
		Format: "yaml",
	}

	got := p.format("")
	if got != "yaml" {
		t.Errorf("expected %q, got %q", "yaml", got)
	}
}

func TestFormatExplicitJSON(t *testing.T) {
	p := &provider{
		log:    log.New(log.Config{}),
		Format: "json",
	}

	got := p.format("yaml")
	if got != "json" {
		t.Errorf("expected %q, got %q", "json", got)
	}
}

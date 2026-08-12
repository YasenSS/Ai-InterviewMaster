// Package prompts embeds immutable, versioned prompt assets into API and
// Worker binaries so production containers do not depend on a working directory.
package prompts

import (
	"embed"
	"fmt"
	"regexp"
)

//go:embed */*/*.md */*/*.json */*/*.yaml
var files embed.FS

var safeSegment = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)

// Template is one immutable prompt version and its structured output schema.
type Template struct {
	Key        string
	Version    string
	System     string
	Task       string
	JSONSchema []byte
	Metadata   []byte
}

// Load returns an embedded prompt version. Keys and versions are restricted to
// path-safe segments; callers cannot use this API for arbitrary file access.
func Load(key, version string) (Template, error) {
	if !safeSegment.MatchString(key) || !safeSegment.MatchString(version) {
		return Template{}, fmt.Errorf("invalid prompt key or version")
	}
	base := key + "/" + version + "/"
	system, err := files.ReadFile(base + "system.md")
	if err != nil {
		return Template{}, fmt.Errorf("read %s system prompt: %w", key, err)
	}
	task, err := files.ReadFile(base + "task.md")
	if err != nil {
		return Template{}, fmt.Errorf("read %s task prompt: %w", key, err)
	}
	schema, err := files.ReadFile(base + "schema.json")
	if err != nil {
		return Template{}, fmt.Errorf("read %s output schema: %w", key, err)
	}
	metadata, err := files.ReadFile(base + "meta.yaml")
	if err != nil {
		return Template{}, fmt.Errorf("read %s metadata: %w", key, err)
	}
	return Template{Key: key, Version: version, System: string(system), Task: string(task), JSONSchema: schema, Metadata: metadata}, nil
}

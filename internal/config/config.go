// Package config loads validated defaults, global YAML and project overrides.
package config

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Toggle struct {
	Enabled bool `yaml:"enabled"`
}
type Compressor struct {
	Enabled                bool   `yaml:"enabled"`
	Mode                   string `yaml:"mode,omitempty"`
	GroupRepeatedLines     bool   `yaml:"group_repeated_lines"`
	GroupTimestampVariants bool   `yaml:"group_timestamp_variants"`
	KeepWarningLines       bool   `yaml:"keep_warning_lines"`
	KeepErrorLines         bool   `yaml:"keep_error_lines"`
}
type Config struct {
	Version    int    `yaml:"version"`
	Mode       string `yaml:"mode"`
	Thresholds struct {
		MinimumBytes     int     `yaml:"minimum_bytes"`
		MinimumLines     int     `yaml:"minimum_lines"`
		MinimumReduction float64 `yaml:"minimum_expected_reduction_percent"`
	} `yaml:"thresholds"`
	Cache struct {
		Enabled   bool   `yaml:"enabled"`
		Retention string `yaml:"retention"`
		MaxSizeMB int    `yaml:"max_size_mb"`
	} `yaml:"cache"`
	Tools       map[string]Toggle     `yaml:"tools"`
	Compressors map[string]Compressor `yaml:"compressors"`
	Metrics     Toggle                `yaml:"metrics"`
	Debug       Toggle                `yaml:"debug"`
	Limits      struct {
		MaxInputMB int `yaml:"max_input_mb"`
	} `yaml:"limits"`
	Target struct {
		MaxOutputBytes int     `yaml:"max_output_bytes"`
		MaxReduction   float64 `yaml:"max_reduction_percent"`
	} `yaml:"target"`
	TokenEstimate struct {
		CharactersPerToken float64 `yaml:"characters_per_token"`
	} `yaml:"token_estimate"`
}

func Default() Config {
	var c Config
	c.Version = 1
	c.Mode = "safe"
	c.Thresholds.MinimumBytes = 4096
	c.Thresholds.MinimumLines = 40
	c.Thresholds.MinimumReduction = 10
	c.Cache.Enabled = true
	c.Cache.Retention = "7d"
	c.Cache.MaxSizeMB = 1024
	c.Tools = map[string]Toggle{"Bash": {true}, "Read": {}, "Edit": {}, "Write": {}}
	c.Compressors = map[string]Compressor{}
	for _, n := range []string{"generic", "json", "php", "php_test", "composer", "laravel", "maven", "gradle", "node_test", "pytest", "go_test", "rust_test", "playwright", "dotnet_test", "dotnet_build", "kubernetes", "docker", "terraform", "ansible"} {
		c.Compressors[n] = Compressor{Enabled: true, GroupRepeatedLines: true, GroupTimestampVariants: true, KeepWarningLines: true, KeepErrorLines: true}
	}
	c.Metrics.Enabled = true
	c.Limits.MaxInputMB = 100
	c.Target.MaxOutputBytes = 200000
	c.Target.MaxReduction = 95
	c.TokenEstimate.CharactersPerToken = 4
	return c
}
func Home() (string, error) {
	if v := os.Getenv("TOKENSLIM_HOME"); v != "" {
		return filepath.Abs(v)
	}
	h, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(h, ".tokenslim"), nil
}
func Retention(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, e := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if e != nil || n < 1 || n > 36500 {
			return 0, fmt.Errorf("invalid retention %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, e := time.ParseDuration(s)
	if e != nil || d <= 0 {
		return 0, fmt.Errorf("invalid retention %q", s)
	}
	return d, nil
}
func (c Config) Validate() error {
	for _, v := range []float64{c.Thresholds.MinimumReduction, c.Target.MaxReduction, c.TokenEstimate.CharactersPerToken} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("configuration numbers must be finite")
		}
	}
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if c.Mode != "off" && c.Mode != "safe" && c.Mode != "smart" {
		return fmt.Errorf("invalid mode %q", c.Mode)
	}
	if c.Thresholds.MinimumBytes < 0 || c.Thresholds.MinimumLines < 0 || c.Thresholds.MinimumReduction < 0 || c.Thresholds.MinimumReduction >= 100 {
		return fmt.Errorf("invalid thresholds")
	}
	if c.Target.MaxReduction <= 0 || c.Target.MaxReduction >= 100 || c.Target.MaxOutputBytes < 1 {
		return fmt.Errorf("invalid target")
	}
	if c.Limits.MaxInputMB < 1 || c.Limits.MaxInputMB > 1024 || c.Cache.MaxSizeMB < 1 || c.TokenEstimate.CharactersPerToken <= 0 {
		return fmt.Errorf("invalid limits, cache size or token heuristic")
	}
	for n, v := range c.Compressors {
		if v.Mode != "" && v.Mode != "safe" && v.Mode != "smart" && v.Mode != "off" {
			return fmt.Errorf("invalid mode for %s", n)
		}
	}
	_, e := Retention(c.Cache.Retention)
	return e
}
func Load(home, cwd string) (Config, error) {
	c := Default()
	// Merge mappings as YAML nodes so partial map entries retain their defaults.
	base, _ := yaml.Marshal(c)
	var node yaml.Node
	_ = yaml.Unmarshal(base, &node)
	for _, p := range []string{filepath.Join(home, "config.yaml"), filepath.Join(cwd, ".tokenslim.yaml")} {
		b, e := os.ReadFile(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return c, e
		}
		var override yaml.Node
		dec := yaml.NewDecoder(bytes.NewReader(b))
		if e = dec.Decode(&override); e != nil {
			return c, fmt.Errorf("%s: %w", p, e)
		}
		var extra any
		if e = dec.Decode(&extra); e != io.EOF {
			return c, fmt.Errorf("%s: expected one YAML document", p)
		}
		if len(override.Content) != 1 || override.Content[0].Kind != yaml.MappingNode {
			return c, fmt.Errorf("%s: expected YAML mapping", p)
		}
		merge(node.Content[0], override.Content[0])
	}
	b, e := yaml.Marshal(&node)
	if e != nil {
		return c, e
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if e = dec.Decode(&c); e != nil {
		return c, e
	}
	if os.Getenv("TOKENSLIM_DEBUG") == "1" {
		c.Debug.Enabled = true
	}
	return c, c.Validate()
}
func merge(dst, src *yaml.Node) {
	for i := 0; i < len(src.Content); i += 2 {
		found := false
		for j := 0; j < len(dst.Content); j += 2 {
			if dst.Content[j].Value == src.Content[i].Value {
				a, b := dst.Content[j+1], src.Content[i+1]
				if a.Kind == yaml.MappingNode && b.Kind == yaml.MappingNode {
					merge(a, b)
				} else {
					dst.Content[j+1] = b
				}
				found = true
				break
			}
		}
		if !found {
			dst.Content = append(dst.Content, src.Content[i], src.Content[i+1])
		}
	}
}
func Key(name string) string {
	switch name {
	case "php-test":
		return "php_test"
	case "node-test", "vitest":
		return "node_test"
	case "kubernetes-log":
		return "kubernetes"
	case "docker-log":
		return "docker"
	case "go-test":
		return "go_test"
	case "dotnet-test":
		return "dotnet_test"
	case "dotnet-build":
		return "dotnet_build"
	case "rust-test":
		return "rust_test"
	}
	return name
}

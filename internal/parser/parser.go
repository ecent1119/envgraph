// Package parser provides scanning and parsing for environment variables
package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/stackgen-cli/envgraph/internal/models"
	"gopkg.in/yaml.v3"
)

// ScanOptions configures the directory scanner
type ScanOptions struct {
	Path          string
	ComposeFiles  []string
	ExcludePaths  []string
	IncludeSource bool
}

// ScanDirectory scans a directory for environment variables
func ScanDirectory(opts ScanOptions) (*models.ScanResult, error) {
	result := models.NewScanResult()

	// Find and parse .env files
	if err := scanEnvFiles(opts.Path, opts.ExcludePaths, result); err != nil {
		return nil, fmt.Errorf("scanning env files: %w", err)
	}

	// Find and parse compose files
	composeFiles := opts.ComposeFiles
	if len(composeFiles) == 0 {
		composeFiles = findComposeFiles(opts.Path)
	}
	for _, cf := range composeFiles {
		if err := parseComposeFile(cf, result); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("parsing %s: %w", cf, err))
		}
	}

	// Optionally scan source code
	if opts.IncludeSource {
		if err := scanSourceCode(opts.Path, opts.ExcludePaths, result); err != nil {
			return nil, fmt.Errorf("scanning source code: %w", err)
		}
	}

	return result, nil
}

// scanEnvFiles finds and parses all .env files in the directory
func scanEnvFiles(basePath string, excludePaths []string, result *models.ScanResult) error {
	patterns := []string{
		".env",
		".env.local",
		".env.example",
		".env.development",
		".env.test",
		".env.production",
	}

	// Also look for .env.* pattern
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".env") {
			found := false
			for _, p := range patterns {
				if name == p {
					found = true
					break
				}
			}
			if !found {
				patterns = append(patterns, name)
			}
		}
	}

	for _, pattern := range patterns {
		envPath := filepath.Join(basePath, pattern)
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			continue
		}

		if isExcluded(envPath, excludePaths) {
			continue
		}

		if err := parseEnvFile(envPath, result); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("parsing %s: %w", pattern, err))
			continue
		}
		result.EnvFiles = append(result.EnvFiles, envPath)
	}

	return nil
}

// parseEnvFile parses a single .env file
func parseEnvFile(filePath string, result *models.ScanResult) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	layer := layerFromFilename(filepath.Base(filePath))

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = unquote(value)

		// Skip export prefix
		key = strings.TrimPrefix(key, "export ")
		key = strings.TrimSpace(key)

		if key == "" {
			continue
		}

		source := models.Source{
			FilePath:   filePath,
			SourceType: models.SourceEnvFile,
			Layer:      layer,
			LineNumber: lineNum,
		}

		addDefinition(result, key, value, source)
	}

	return scanner.Err()
}

// layerFromFilename determines the Layer from a filename
func layerFromFilename(filename string) models.Layer {
	switch filename {
	case ".env":
		return models.LayerEnv
	case ".env.local":
		return models.LayerEnvLocal
	case ".env.example":
		return models.LayerEnvExample
	default:
		return models.LayerEnvOther
	}
}

// unquote removes surrounding quotes from a value
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// findComposeFiles locates docker-compose files in the directory
func findComposeFiles(basePath string) []string {
	var files []string
	patterns := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, pattern := range patterns {
		path := filepath.Join(basePath, pattern)
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}

	// Also look for docker-compose.*.yml
	entries, _ := os.ReadDir(basePath)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "docker-compose.") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
			path := filepath.Join(basePath, name)
			found := false
			for _, f := range files {
				if f == path {
					found = true
					break
				}
			}
			if !found {
				files = append(files, path)
			}
		}
	}

	return files
}

// ComposeFile represents a docker-compose.yml structure
type ComposeFile struct {
	Services map[string]ComposeService `yaml:"services"`
}

// ComposeService represents a service in docker-compose
type ComposeService struct {
	Image       string            `yaml:"image"`
	Environment interface{}       `yaml:"environment"` // Can be map or list
	EnvFile     interface{}       `yaml:"env_file"`    // Can be string or list
}

// parseComposeFile parses a docker-compose file for environment variables
func parseComposeFile(filePath string, result *models.ScanResult) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return err
	}

	result.ComposeFiles = append(result.ComposeFiles, filePath)

	for serviceName, service := range compose.Services {
		svcDep := models.ServiceDependency{
			ServiceName: serviceName,
			Variables:   []string{},
			EnvFiles:    []string{},
		}

		// Parse env_file references
		envFiles := parseEnvFileRef(service.EnvFile)
		svcDep.EnvFiles = envFiles

		// Parse inline environment variables
		envVars := parseEnvironment(service.Environment)
		for key, value := range envVars {
			source := models.Source{
				FilePath:   filePath,
				SourceType: models.SourceComposeYAML,
				Layer:      models.LayerComposeInline,
				Service:    serviceName,
			}

			if value != nil {
				addDefinition(result, key, *value, source)
			} else {
				// Variable reference without value (expects external definition)
				addUsage(result, key, source)
			}
			svcDep.Variables = append(svcDep.Variables, key)
		}

		// Also detect ${VAR} references in the YAML
		refs := extractYAMLReferences(data)
		for _, ref := range refs {
			source := models.Source{
				FilePath:   filePath,
				SourceType: models.SourceComposeYAML,
				Layer:      models.LayerComposeEnvFile,
				Service:    serviceName,
			}
			addUsage(result, ref, source)
			if !contains(svcDep.Variables, ref) {
				svcDep.Variables = append(svcDep.Variables, ref)
			}
		}

		result.Services = append(result.Services, svcDep)
	}

	return nil
}

// parseEnvFileRef parses the env_file field which can be string or list
func parseEnvFileRef(envFile interface{}) []string {
	if envFile == nil {
		return nil
	}

	switch v := envFile.(type) {
	case string:
		return []string{v}
	case []interface{}:
		var files []string
		for _, f := range v {
			if s, ok := f.(string); ok {
				files = append(files, s)
			}
		}
		return files
	}
	return nil
}

// parseEnvironment parses the environment field which can be map or list
func parseEnvironment(env interface{}) map[string]*string {
	result := make(map[string]*string)
	if env == nil {
		return result
	}

	switch v := env.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if val == nil {
				result[key] = nil
			} else {
				s := fmt.Sprintf("%v", val)
				result[key] = &s
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts := strings.SplitN(s, "=", 2)
				key := parts[0]
				if len(parts) == 2 {
					val := parts[1]
					result[key] = &val
				} else {
					result[key] = nil
				}
			}
		}
	}

	return result
}

// extractYAMLReferences finds ${VAR} patterns in YAML content
func extractYAMLReferences(data []byte) []string {
	re := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-[^}]*)?\}`)
	matches := re.FindAllSubmatch(data, -1)

	seen := make(map[string]bool)
	var refs []string
	for _, match := range matches {
		if len(match) >= 2 {
			varName := string(match[1])
			if !seen[varName] {
				seen[varName] = true
				refs = append(refs, varName)
			}
		}
	}
	return refs
}

// scanSourceCode scans source files for environment variable references
func scanSourceCode(basePath string, excludePaths []string, result *models.ScanResult) error {
	// Patterns to look for env variable access
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`),                    // ${VAR}
		regexp.MustCompile(`process\.env\.([A-Za-z_][A-Za-z0-9_]*)`),            // process.env.VAR (Node.js)
		regexp.MustCompile(`os\.Getenv\s*\(\s*"([A-Za-z_][A-Za-z0-9_]*)"\s*\)`), // os.Getenv("VAR") (Go)
		regexp.MustCompile(`os\.environ\s*\[\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]\s*\]`), // os.environ["VAR"] (Python)
		regexp.MustCompile(`os\.getenv\s*\(\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]`),       // os.getenv("VAR") (Python)
		regexp.MustCompile(`System\.getenv\s*\(\s*"([A-Za-z_][A-Za-z0-9_]*)"\s*\)`),    // System.getenv("VAR") (Java)
		regexp.MustCompile(`Environment\.GetEnvironmentVariable\s*\(\s*"([A-Za-z_][A-Za-z0-9_]*)"\s*\)`), // C#
		regexp.MustCompile(`std::getenv\s*\(\s*"([A-Za-z_][A-Za-z0-9_]*)"\s*\)`),       // C++
		regexp.MustCompile(`env::var\s*\(\s*"([A-Za-z_][A-Za-z0-9_]*)"\s*\)`),          // Rust
	}

	// File extensions to scan
	extensions := map[string]bool{
		".go":    true,
		".js":    true,
		".ts":    true,
		".jsx":   true,
		".tsx":   true,
		".py":    true,
		".java":  true,
		".cs":    true,
		".cpp":   true,
		".c":     true,
		".rs":    true,
		".rb":    true,
		".php":   true,
		".sh":    true,
		".bash":  true,
	}

	return filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			// Skip common non-source directories
			name := info.Name()
			if name == "node_modules" || name == "vendor" || name == ".git" || name == "__pycache__" || name == "target" || name == "bin" || name == "obj" {
				return filepath.SkipDir
			}
			if isExcluded(path, excludePaths) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if !extensions[ext] {
			return nil
		}

		if isExcluded(path, excludePaths) {
			return nil
		}

		return scanSourceFile(path, patterns, result)
	})
}

// scanSourceFile scans a single source file for env references
func scanSourceFile(filePath string, patterns []*regexp.Regexp, result *models.ScanResult) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil // Skip files we can't read
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	foundAny := false

	for lineNum, line := range lines {
		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					varName := match[1]
					source := models.Source{
						FilePath:   filePath,
						SourceType: models.SourceCode,
						LineNumber: lineNum + 1,
					}
					addUsage(result, varName, source)
					foundAny = true
				}
			}
		}
	}

	if foundAny {
		result.SourceFiles = append(result.SourceFiles, filePath)
	}

	return nil
}

// addDefinition adds a variable definition to the result
func addDefinition(result *models.ScanResult, name, value string, source models.Source) {
	v, exists := result.Variables[name]
	if !exists {
		v = &models.Variable{
			Name:      name,
			DefinedIn: []models.Source{},
			UsedIn:    []models.Source{},
		}
		result.Variables[name] = v
	}

	v.DefinedIn = append(v.DefinedIn, source)
	v.HasValue = true

	// Update effective value based on precedence
	if v.EffectiveSource == nil || source.Layer.Precedence() >= v.EffectiveLayer.Precedence() {
		v.Value = value
		v.EffectiveValue = value
		v.EffectiveLayer = source.Layer
		v.EffectiveSource = &source
	}
}

// addUsage adds a variable usage/reference to the result
func addUsage(result *models.ScanResult, name string, source models.Source) {
	v, exists := result.Variables[name]
	if !exists {
		v = &models.Variable{
			Name:      name,
			DefinedIn: []models.Source{},
			UsedIn:    []models.Source{},
		}
		result.Variables[name] = v
	}

	v.UsedIn = append(v.UsedIn, source)
}

// isExcluded checks if a path should be excluded
func isExcluded(path string, excludePaths []string) bool {
	for _, ex := range excludePaths {
		if strings.Contains(path, ex) {
			return true
		}
	}
	return false
}

// contains checks if a slice contains a string
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

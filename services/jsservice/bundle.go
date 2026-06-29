package jsservice

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/evanw/esbuild/pkg/api"
)

type BuildResult struct {
	Imports map[string]string
}

type metafile struct {
	Outputs map[string]metafileOutput `json:"outputs"`
}

type metafileOutput struct {
	EntryPoint string `json:"entryPoint"`
}

type importMap struct {
	Imports map[string]string `json:"imports"`
}

type buildGroup struct {
	Name          string
	Entrypoints   []api.EntryPoint
	NormalOutputs map[string]int
}

type appController struct {
	Path       string
	ImportName string
	Identifier string
}

const (
	appSourceDir = "app/js"
	appEntryFile = "app.js"
)

func Bundle(manifest Manifest, root, packageDir string) (BuildResult, error) {
	manifest = normalizeManifest(manifest)
	if err := ValidateManifest(manifest); err != nil {
		return BuildResult{}, err
	}

	outputDir := resolvePath(root, manifest.Output.Dir)
	if err := os.RemoveAll(outputDir); err != nil {
		return BuildResult{}, fmt.Errorf("clean JavaScript output: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return BuildResult{}, fmt.Errorf("create JavaScript output directory: %w", err)
	}

	groups, err := buildEntrypointGroups(manifest)
	if err != nil {
		return BuildResult{}, err
	}

	paths := map[int]string{}
	for _, group := range groups {
		result := api.Build(api.BuildOptions{
			AbsWorkingDir:       root,
			EntryPointsAdvanced: group.Entrypoints,
			Bundle:              true,
			Format:              api.FormatESModule,
			Platform:            api.PlatformBrowser,
			Target:              targetFor(manifest.Bundle.Target),
			Outdir:              outputDir,
			EntryNames:          "[name]-[hash]",
			ChunkNames:          chunkNamesFor(group.Name, len(groups)),
			AssetNames:          "assets/[name]-[hash]",
			Splitting:           manifest.Bundle.Shared,
			MinifyWhitespace:    manifest.Bundle.Minify,
			MinifyIdentifiers:   manifest.Bundle.Minify,
			MinifySyntax:        manifest.Bundle.Minify,
			Sourcemap:           sourcemapFor(manifest.Bundle.Sourcemap),
			Write:               true,
			Metafile:            true,
			LogLevel:            api.LogLevelSilent,
			NodePaths:           []string{filepath.Join(packageDir, "node_modules")},
		})
		if len(result.Errors) != 0 {
			return BuildResult{}, fmt.Errorf("bundle JavaScript: %s", formatMessages(result.Errors))
		}
		if result.Metafile == "" {
			return BuildResult{}, fmt.Errorf("bundle JavaScript: esbuild did not return a metafile")
		}

		groupPaths, err := outputPaths(result.Metafile, outputDir, group.NormalOutputs)
		if err != nil {
			return BuildResult{}, err
		}
		for index, output := range groupPaths {
			paths[index] = output
		}
	}
	if err := copyAssets(root, outputDir, manifest.Entrypoints); err != nil {
		return BuildResult{}, err
	}

	imports, err := importmapImports(manifest, outputDir, paths)
	if err != nil {
		return BuildResult{}, err
	}
	appImports, err := writeAppJavaScript(manifest, root, outputDir)
	if err != nil {
		return BuildResult{}, err
	}
	for specifier, output := range appImports {
		if existing, ok := imports[specifier]; ok {
			return BuildResult{}, fmt.Errorf("import %q is already mapped to %s", specifier, existing)
		}
		imports[specifier] = output
	}
	if err := writeImportmap(root, manifest.Output.Importmap, imports); err != nil {
		return BuildResult{}, err
	}
	return BuildResult{Imports: imports}, nil
}

func buildEntrypointGroups(manifest Manifest) ([]buildGroup, error) {
	used := map[string]int{}
	groupByName := map[string]int{}
	groups := make([]buildGroup, 0, len(manifest.Entrypoints))

	for index, entrypoint := range manifest.Entrypoints {
		groupName := entrypoint.Group
		if !manifest.Bundle.Shared {
			groupName = DefaultEntrypointGroup
		}
		groupIndex, ok := groupByName[groupName]
		if !ok {
			groups = append(groups, buildGroup{
				Name:          groupName,
				NormalOutputs: map[string]int{},
			})
			groupIndex = len(groups) - 1
			groupByName[groupName] = groupIndex
		}
		group := &groups[groupIndex]

		outputName := uniqueOutputName(used, sanitizeName(entrypoint.Name))
		group.NormalOutputs[outputName] = index
		group.Entrypoints = append(group.Entrypoints, api.EntryPoint{
			InputPath:  entrypoint.Module,
			OutputPath: outputName,
		})

		for _, extra := range entrypoint.ExtraFiles {
			base := strings.TrimSuffix(filepath.Base(extra), filepath.Ext(extra))
			extraName := uniqueOutputName(used, sanitizeName(entrypoint.Name+"-"+base))
			group.Entrypoints = append(group.Entrypoints, api.EntryPoint{
				InputPath:  extra,
				OutputPath: extraName,
			})
		}
	}
	return groups, nil
}

func chunkNamesFor(groupName string, groupCount int) string {
	if groupCount <= 1 {
		return "shared-[hash]"
	}
	name := sanitizeName(groupName)
	if name == "" {
		name = DefaultEntrypointGroup
	}
	return "shared-" + name + "-[hash]"
}

func outputPaths(metafileJSON, outputDir string, normalOutputs map[string]int) (map[int]string, error) {
	var meta metafile
	if err := json.Unmarshal([]byte(metafileJSON), &meta); err != nil {
		return nil, fmt.Errorf("parse esbuild metafile: %w", err)
	}

	paths := map[int]string{}
	outputNames := make([]string, 0, len(normalOutputs))
	for name := range normalOutputs {
		outputNames = append(outputNames, name)
	}
	sort.Slice(outputNames, func(i, j int) bool {
		if len(outputNames[i]) == len(outputNames[j]) {
			return outputNames[i] < outputNames[j]
		}
		return len(outputNames[i]) > len(outputNames[j])
	})

	for output := range meta.Outputs {
		if strings.HasSuffix(output, ".map") {
			continue
		}
		base := strings.TrimSuffix(path.Base(output), ".map")
		for _, name := range outputNames {
			if !strings.HasPrefix(base, name+"-") || !strings.HasSuffix(base, ".js") {
				continue
			}
			outputPath := resolveBuildOutputPath(outputDir, output)
			paths[normalOutputs[name]] = outputPath
		}
	}

	for _, index := range normalOutputs {
		if paths[index] == "" {
			return nil, fmt.Errorf("bundle JavaScript: output for entrypoint %d not found", index)
		}
	}
	return paths, nil
}

func importmapImports(manifest Manifest, outputDir string, outputs map[int]string) (map[string]string, error) {
	imports := map[string]string{}
	for index, entrypoint := range manifest.Entrypoints {
		output := outputs[index]
		if output == "" {
			return nil, fmt.Errorf("entrypoint %q output is missing", entrypoint.Name)
		}
		aliases := entrypoint.Imports
		if len(aliases) == 0 {
			aliases = []string{entrypoint.Module}
		}

		for _, specifier := range aliases {
			if existing, ok := imports[specifier]; ok {
				return nil, fmt.Errorf("import %q is already mapped to %s", specifier, existing)
			}
			imports[specifier] = publicAssetPath(manifest.Output.PublicPath, outputDir, output)
		}
	}
	return imports, nil
}

func resolveBuildOutputPath(outputDir, output string) string {
	outputPath := filepath.FromSlash(output)
	if filepath.IsAbs(outputPath) {
		return outputPath
	}
	candidates := []string{
		filepath.Join(outputDir, outputPath),
	}
	if relative, ok := outputPathRelativeToDir(outputDir, outputPath); ok {
		candidates = append(candidates, filepath.Join(outputDir, relative))
	}
	candidates = append(candidates, filepath.Join(outputDir, filepath.Base(outputPath)))
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	if relative, ok := outputPathRelativeToDir(outputDir, outputPath); ok {
		return filepath.Join(outputDir, relative)
	}
	return filepath.Join(outputDir, filepath.Base(outputPath))
}

func publicAssetPath(publicPath, outputDir, output string) string {
	outputRoot := filepath.Clean(outputDir)
	relative, err := filepath.Rel(outputRoot, filepath.Clean(output))
	if err != nil || strings.HasPrefix(relative, "..") {
		relative = filepath.Base(output)
	}
	return path.Join(publicPath, filepath.ToSlash(relative))
}

func outputPathRelativeToDir(outputDir string, outputPath string) (string, bool) {
	output := filepath.ToSlash(filepath.Clean(outputPath))
	dir := filepath.ToSlash(filepath.Clean(outputDir))
	if strings.HasPrefix(output, dir+"/") {
		return strings.TrimPrefix(output, dir+"/"), true
	}

	marker := filepath.ToSlash(filepath.Base(outputDir)) + "/"
	if index := strings.LastIndex(output, marker); index >= 0 {
		return output[index+len(marker):], true
	}
	return "", false
}

func writeAppJavaScript(manifest Manifest, root, outputDir string) (map[string]string, error) {
	appRoot := filepath.Join(root, filepath.FromSlash(appSourceDir))
	info, err := os.Stat(appRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect app JavaScript: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", appSourceDir)
	}

	files, err := appJavaScriptFiles(appRoot)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	imports := map[string]string{}
	for _, relative := range files {
		source := filepath.Join(appRoot, filepath.FromSlash(relative))
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.ToSlash(filepath.Join(appSourceDir, relative)), err)
		}
		if relative == appEntryFile {
			data, err = expandAppJavaScript(appRoot, data)
			if err != nil {
				return nil, err
			}
		}

		outputRelative := hashedAppOutputPath(relative, data)
		target := filepath.Join(outputDir, filepath.FromSlash(outputRelative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, fmt.Errorf("create app JavaScript directory: %w", err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return nil, fmt.Errorf("write app JavaScript: %w", err)
		}

		specifier := appImportSpecifier(relative)
		imports[specifier] = publicAssetPath(manifest.Output.PublicPath, outputDir, target)
	}
	return imports, nil
}

func appImportSpecifier(relative string) string {
	return path.Clean(filepath.ToSlash(relative))
}

func hashedAppOutputPath(relative string, data []byte) string {
	relative = filepath.ToSlash(relative)
	ext := path.Ext(relative)
	dir, file := path.Split(relative)
	name := strings.TrimSuffix(file, ext)
	return path.Join("app", dir, name+"-"+appContentHash(data)+ext)
}

func appContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%X", sum[:4])
}

func appJavaScriptFiles(appRoot string) ([]string, error) {
	var files []string
	if err := filepath.WalkDir(appRoot, func(candidate string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(candidate) != ".js" {
			return nil
		}
		relative, err := filepath.Rel(appRoot, candidate)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk app JavaScript: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func expandAppJavaScript(appRoot string, data []byte) ([]byte, error) {
	lines := strings.SplitAfter(string(data), "\n")
	var builder strings.Builder
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case "// golazy:turbo":
			builder.WriteString("import \"@hotwired/turbo\"\n")
		case "// golazy:stimulus":
			boilerplate, err := stimulusBoilerplate(appRoot)
			if err != nil {
				return nil, err
			}
			builder.WriteString(boilerplate)
		default:
			builder.WriteString(line)
		}
	}
	return []byte(builder.String()), nil
}

func stimulusBoilerplate(appRoot string) (string, error) {
	controllers, err := discoverStimulusControllers(filepath.Join(appRoot, "controllers"))
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("import { Application } from \"@hotwired/stimulus\"\n")
	for _, controller := range controllers {
		fmt.Fprintf(&builder, "import %s from %q\n", controller.ImportName, appImportSpecifier(path.Join("controllers", controller.Path)))
	}
	builder.WriteString("\nconst application = Application.start()\n")
	for _, controller := range controllers {
		fmt.Fprintf(&builder, "application.register(%q, %s)\n", controller.Identifier, controller.ImportName)
	}
	return builder.String(), nil
}

func discoverStimulusControllers(controllerRoot string) ([]appController, error) {
	info, err := os.Stat(controllerRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect Stimulus controllers: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("app/js/controllers is not a directory")
	}

	var paths []string
	if err := filepath.WalkDir(controllerRoot, func(candidate string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_controller.js") {
			return nil
		}
		relative, err := filepath.Rel(controllerRoot, candidate)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk Stimulus controllers: %w", err)
	}
	sort.Strings(paths)

	seenIdentifiers := map[string]string{}
	usedImports := map[string]int{}
	controllers := make([]appController, 0, len(paths))
	for _, controllerPath := range paths {
		identifier := stimulusIdentifier(controllerPath)
		if previous, ok := seenIdentifiers[identifier]; ok {
			return nil, fmt.Errorf("Stimulus controller %q is already registered by %s", identifier, previous)
		}
		seenIdentifiers[identifier] = controllerPath
		importName := uniqueControllerImportName(usedImports, identifier)
		controllers = append(controllers, appController{
			Path:       controllerPath,
			ImportName: importName,
			Identifier: identifier,
		})
	}
	return controllers, nil
}

func stimulusIdentifier(controllerPath string) string {
	name := strings.TrimSuffix(controllerPath, ".js")
	parts := strings.Split(name, "/")
	for index, part := range parts {
		parts[index] = strings.ReplaceAll(strings.TrimSuffix(part, "_controller"), "_", "-")
	}
	return strings.Join(parts, "--")
}

func uniqueControllerImportName(used map[string]int, identifier string) string {
	base := exportedIdentifier(identifier) + "Controller"
	count := used[base]
	used[base] = count + 1
	if count == 0 {
		return base
	}
	return fmt.Sprintf("%s%d", base, count+1)
}

func exportedIdentifier(value string) string {
	var builder strings.Builder
	nextUpper := true
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			if builder.Len() == 0 && unicode.IsDigit(char) {
				builder.WriteString("App")
			}
			if nextUpper {
				builder.WriteRune(unicode.ToUpper(char))
			} else {
				builder.WriteRune(char)
			}
			nextUpper = false
			continue
		}
		nextUpper = true
	}
	if builder.Len() == 0 {
		return "App"
	}
	return builder.String()
}

func writeImportmap(root, importmapPath string, imports map[string]string) error {
	fullPath := resolvePath(root, importmapPath)
	data, err := json.MarshalIndent(importMap{Imports: imports}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal importmap: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("create importmap directory: %w", err)
	}
	if err := os.WriteFile(fullPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write importmap: %w", err)
	}
	return nil
}

func copyAssets(root, outputDir string, entrypoints []Entrypoint) error {
	for _, entrypoint := range entrypoints {
		for _, pattern := range entrypoint.Assets {
			matches, base, err := globFiles(root, pattern)
			if err != nil {
				return err
			}
			for _, source := range matches {
				info, err := os.Stat(source)
				if err != nil {
					return fmt.Errorf("stat asset %s: %w", source, err)
				}
				if info.IsDir() {
					continue
				}
				relative, err := filepath.Rel(base, source)
				if err != nil {
					return fmt.Errorf("resolve asset %s relative to %s: %w", source, base, err)
				}
				target := filepath.Join(outputDir, relative)
				if err := copyFile(source, target, info.Mode().Perm()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func globFiles(root, pattern string) ([]string, string, error) {
	absolutePattern := resolvePath(root, pattern)
	base := assetCopyBase(root, pattern)
	if !strings.Contains(absolutePattern, "**") {
		matches, err := filepath.Glob(absolutePattern)
		if err != nil {
			return nil, "", fmt.Errorf("match asset pattern %q: %w", pattern, err)
		}
		return matches, base, nil
	}

	regex, err := globstarRegexp(filepath.ToSlash(filepath.Clean(absolutePattern)))
	if err != nil {
		return nil, "", fmt.Errorf("compile asset pattern %q: %w", pattern, err)
	}

	walkRoot := globWalkRoot(root, pattern)
	var matches []string
	if err := filepath.WalkDir(walkRoot, func(candidate string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if regex.MatchString(filepath.ToSlash(filepath.Clean(candidate))) {
			matches = append(matches, candidate)
		}
		return nil
	}); err != nil {
		return nil, "", fmt.Errorf("walk asset pattern %q: %w", pattern, err)
	}
	sort.Strings(matches)
	return matches, base, nil
}

func assetCopyBase(root, pattern string) string {
	if firstGlob(pattern) == -1 {
		return filepath.Dir(resolvePath(root, pattern))
	}
	prefix := strings.TrimSuffix(pattern[:firstGlob(pattern)], "/")
	if prefix == "" {
		return root
	}
	return resolvePath(root, filepath.Dir(prefix))
}

func globWalkRoot(root, pattern string) string {
	if firstGlob(pattern) == -1 {
		return filepath.Dir(resolvePath(root, pattern))
	}
	prefix := strings.TrimSuffix(pattern[:firstGlob(pattern)], "/")
	if prefix == "" {
		return root
	}
	return resolvePath(root, prefix)
}

func firstGlob(pattern string) int {
	for index, char := range pattern {
		if char == '*' || char == '?' || char == '[' {
			return index
		}
	}
	return -1
}

func globstarRegexp(pattern string) (*regexp.Regexp, error) {
	var builder strings.Builder
	builder.WriteByte('^')
	for index := 0; index < len(pattern); {
		switch pattern[index] {
		case '*':
			if strings.HasPrefix(pattern[index:], "**/") {
				builder.WriteString("(?:.*/)?")
				index += 3
				continue
			}
			if strings.HasPrefix(pattern[index:], "**") {
				builder.WriteString(".*")
				index += 2
				continue
			}
			builder.WriteString("[^/]*")
			index++
		case '?':
			builder.WriteString("[^/]")
			index++
		default:
			builder.WriteString(regexp.QuoteMeta(string(pattern[index])))
			index++
		}
	}
	builder.WriteByte('$')
	return regexp.Compile(builder.String())
}

func copyFile(source, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create asset directory %s: %w", filepath.Dir(target), err)
	}
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open asset %s: %w", source, err)
	}
	defer input.Close()

	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, input); err != nil {
		return fmt.Errorf("read asset %s: %w", source, err)
	}
	if err := os.WriteFile(target, buffer.Bytes(), mode); err != nil {
		return fmt.Errorf("write asset %s: %w", target, err)
	}
	return nil
}

func targetFor(target string) api.Target {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "es2020":
		return api.ES2020
	case "es2017":
		return api.ES2017
	case "es2018":
		return api.ES2018
	case "es2019":
		return api.ES2019
	case "es2021":
		return api.ES2021
	case "es2022":
		return api.ES2022
	case "es2023":
		return api.ES2023
	case "es2024":
		return api.ES2024
	default:
		return api.ES2020
	}
}

func sourcemapFor(enabled bool) api.SourceMap {
	if enabled {
		return api.SourceMapLinked
	}
	return api.SourceMapNone
}

func formatMessages(messages []api.Message) string {
	var parts []string
	for _, message := range messages {
		parts = append(parts, message.Text)
	}
	return strings.Join(parts, "; ")
}

func uniqueOutputName(used map[string]int, name string) string {
	if name == "" {
		name = "entrypoint"
	}
	count := used[name]
	used[name] = count + 1
	if count == 0 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, count+1)
}

func sanitizeName(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(value) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if char == '.' || char == '_' || char == '-' {
			builder.WriteRune(char)
			lastDash = char == '-'
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

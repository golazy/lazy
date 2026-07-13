package addonservice

import (
	"bytes"
	"fmt"
	"go/format"
	"path"
	"sort"
	"strings"
	"unicode"

	"golazy.dev/lazy/services/configservice"
	"golazy.dev/lazycode"
	"golazy.dev/lazycode/tomlcode"
)

const (
	lazyConfigFilename      = "lazy.toml"
	miseConfigFilename      = "mise.toml"
	postgresMigrationsGo    = "db/postgres/migrations.go"
	postgresMigrationsIndex = "db/postgres/migrations/migrations.toml"
	lazyServiceBegin        = "# golazy:begin addon postgres service"
	lazyServiceEnd          = "# golazy:end addon postgres service"
	postgresEnvPropertyV1   = "golazy-postgres-env-v1:"
)

var postgresTasks = []string{
	".mise/tasks/postgres/start",
	".mise/tasks/postgres/check",
	".mise/tasks/postgres/create",
	".mise/tasks/postgres/reset",
}

func postgresOutputPaths() []string {
	paths := []string{lazyConfigFilename, miseConfigFilename, postgresMigrationsGo, postgresMigrationsIndex}
	paths = append(paths, postgresTasks...)
	return paths
}

func postgresOutputsEnabled(addons []InstalledAddon) bool {
	for _, addon := range addons {
		if addon.ID == "postgres" && addon.Package == "golazy.dev/pg" && (strings.HasPrefix(addon.Source, "first-party:") || len(addon.Files) != 0 || len(addon.Build.Tasks) != 0) {
			return true
		}
	}
	return false
}

func postgresContributions(before State, addons []InstalledAddon, current map[string][]byte) ([]lazycode.Operation, []ManagedFile, error) {
	enabled := postgresOutputsEnabled(addons)

	var operations []lazycode.Operation
	var managed []ManagedFile
	if enabled {
		modulePath := modfileModulePath(current[GoModFilename])
		if modulePath == "" {
			return nil, nil, fmt.Errorf("%s does not declare a module path", GoModFilename)
		}
		outputs, err := postgresGeneratedOutputs(modulePath)
		if err != nil {
			return nil, nil, err
		}
		paths := make([]string, 0, len(outputs))
		for outputPath := range outputs {
			paths = append(paths, outputPath)
		}
		sort.Strings(paths)
		for _, outputPath := range paths {
			output := outputs[outputPath]
			if existing, exists := current[outputPath]; exists && !bytes.Equal(existing, output.Data) {
				if _, owned := before.managedFile(outputPath); !owned {
					return nil, nil, fmt.Errorf("%s already exists and is not managed by %s", outputPath, StateFilename)
				}
			}
			name, data := outputPath, append([]byte(nil), output.Data...)
			operations = append(operations, lazycode.OperationFunc(func(workspace *lazycode.Workspace) error {
				return workspace.Replace(name, data)
			}))
			managed = append(managed, ManagedFile{
				Path: outputPath, Kind: "generated", Owners: []string{"postgres"}, Mode: uint32(output.Mode),
			})
		}

		lazyData, lazyChanged, err := editPostgresService(current[lazyConfigFilename], true)
		if err != nil {
			return nil, nil, err
		}
		_, wasManaged := before.managedFile(lazyConfigFilename)
		if lazyChanged {
			data := append([]byte(nil), lazyData...)
			operations = append(operations, lazycode.OperationFunc(func(workspace *lazycode.Workspace) error {
				return workspace.Replace(lazyConfigFilename, data)
			}))
		}
		if lazyChanged || wasManaged {
			managed = append(managed, ManagedFile{Path: lazyConfigFilename, Kind: "rewrite", Owners: []string{"postgres"}, Mode: 0o644})
		}

		previousMise, _ := before.managedFile(miseConfigFilename)
		miseData, miseChanged, properties, err := editPostgresMise(current[miseConfigFilename], true, previousMise.Properties, applicationSlug(modulePath))
		if err != nil {
			return nil, nil, err
		}
		if miseChanged {
			data := append([]byte(nil), miseData...)
			operations = append(operations, lazycode.OperationFunc(func(workspace *lazycode.Workspace) error {
				return workspace.Replace(miseConfigFilename, data)
			}))
		}
		if len(properties) != 0 {
			managed = append(managed, ManagedFile{
				Path: miseConfigFilename, Kind: "rewrite", Owners: []string{"postgres"}, Mode: 0o644, Properties: properties,
			})
		}
		return operations, managed, nil
	}

	for _, file := range before.Managed {
		if !containsString(file.Owners, "postgres") {
			continue
		}
		if file.Path == lazyConfigFilename {
			lazyData, changed, err := editPostgresService(current[lazyConfigFilename], false)
			if err != nil {
				return nil, nil, err
			}
			if !changed {
				continue
			}
			if len(bytes.TrimSpace(lazyData)) == 0 {
				operations = append(operations, lazycode.OperationFunc(func(workspace *lazycode.Workspace) error {
					return workspace.Remove(lazyConfigFilename)
				}))
			} else {
				data := append([]byte(nil), lazyData...)
				operations = append(operations, lazycode.OperationFunc(func(workspace *lazycode.Workspace) error {
					return workspace.Replace(lazyConfigFilename, data)
				}))
			}
			continue
		}
		if file.Path == miseConfigFilename {
			miseData, changed, _, err := editPostgresMise(current[miseConfigFilename], false, file.Properties, "")
			if err != nil {
				return nil, nil, err
			}
			if changed {
				data := append([]byte(nil), miseData...)
				operations = append(operations, lazycode.OperationFunc(func(workspace *lazycode.Workspace) error {
					return workspace.Replace(miseConfigFilename, data)
				}))
			}
			continue
		}
		if file.Kind == "generated" && containsString(postgresOutputPaths(), file.Path) {
			name := file.Path
			operations = append(operations, lazycode.OperationFunc(func(workspace *lazycode.Workspace) error {
				return workspace.Remove(name)
			}))
		}
	}
	return operations, nil, nil
}

type generatedOutput struct {
	Data []byte
	Mode uint32
}

func postgresGeneratedOutputs(modulePath string) (map[string]generatedOutput, error) {
	migrations, err := format.Source([]byte(`// Code generated by lazy add postgres. DO NOT EDIT.

package postgres

import (
	"embed"
	"io/fs"
)

//go:embed migrations
var migrationFiles embed.FS

// Files returns the application-owned PostgreSQL migration filesystem rooted
// at the conventional migrations directory.
func Files() fs.FS {
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err)
	}
	return files
}
`))
	if err != nil {
		return nil, fmt.Errorf("format PostgreSQL migration embed: %w", err)
	}
	slug := applicationSlug(modulePath)
	outputs := map[string]generatedOutput{
		postgresMigrationsGo: {
			Data: migrations,
			Mode: 0o644,
		},
		postgresMigrationsIndex: {
			Data: []byte("# Application PostgreSQL migrations live in this directory.\n"),
			Mode: 0o644,
		},
		postgresTasks[0]: {Data: []byte(postgresStartTask(slug)), Mode: 0o755},
		postgresTasks[1]: {Data: []byte(postgresCheckTask(slug)), Mode: 0o755},
		postgresTasks[2]: {Data: []byte(postgresCreateTask(slug)), Mode: 0o755},
		postgresTasks[3]: {Data: []byte(postgresResetTask(slug)), Mode: 0o755},
	}
	return outputs, nil
}

func editPostgresService(source []byte, enabled bool) ([]byte, bool, error) {
	if len(source) == 0 {
		if !enabled {
			return nil, false, nil
		}
		return []byte("services = [\"postgres\"]\n"), true, nil
	}
	config, err := configservice.Parse(source)
	if err != nil {
		return nil, false, fmt.Errorf("parse %s: %w", lazyConfigFilename, err)
	}
	names := configservice.ServiceNames(config)
	hasPostgres := containsString(names, "postgres")
	if hasPostgres == enabled {
		return append([]byte(nil), source...), false, nil
	}
	if enabled && !rootServicesAssignment(source) {
		text := strings.TrimRight(string(source), "\r\n")
		if strings.TrimSpace(text) != "" {
			text += "\n\n"
		}
		text += lazyServiceBegin + "\n[[services]]\nname = \"postgres\"\n" + lazyServiceEnd + "\n"
		result := []byte(text)
		if _, err := configservice.Parse(result); err != nil {
			return nil, false, fmt.Errorf("validate %s service edit: %w", lazyConfigFilename, err)
		}
		return result, true, nil
	}
	if !enabled && strings.Contains(string(source), lazyServiceBegin) {
		result, err := removeManagedServiceBlock(source)
		if err != nil {
			return nil, false, err
		}
		return result, true, nil
	}
	if !rootServicesAssignment(source) {
		return nil, false, fmt.Errorf("%s declares postgres outside the managed block; remove it manually", lazyConfigFilename)
	}
	filtered := make([]string, 0, len(names)+1)
	for _, name := range names {
		if name != "postgres" {
			filtered = append(filtered, name)
		}
	}
	if enabled {
		filtered = append(filtered, "postgres")
	}
	document, err := tomlcode.Parse(source)
	if err != nil {
		return nil, false, fmt.Errorf("parse %s for edit: %w", lazyConfigFilename, err)
	}
	if len(filtered) == 0 {
		_, err = document.Remove("", "services")
	} else {
		_, err = document.SetStrings("", "services", filtered)
	}
	if err != nil {
		return nil, false, fmt.Errorf("edit %s services: %w", lazyConfigFilename, err)
	}
	result := document.Bytes()
	if len(bytes.TrimSpace(result)) != 0 {
		if _, err := configservice.Parse(result); err != nil {
			return nil, false, fmt.Errorf("validate %s service edit: %w", lazyConfigFilename, err)
		}
	}
	return result, !bytes.Equal(source, result), nil
}

func editPostgresMise(source []byte, enabled bool, owned []string, slug string) ([]byte, bool, []string, error) {
	document, err := tomlcode.Parse(source)
	if err != nil {
		return nil, false, nil, fmt.Errorf("parse %s: %w", miseConfigFilename, err)
	}
	current := tomlTableStrings(source, "env")
	if !enabled {
		changed := false
		for _, property := range owned {
			if !strings.HasPrefix(property, postgresEnvPropertyV1) {
				continue
			}
			key, expected, ok := strings.Cut(strings.TrimPrefix(property, postgresEnvPropertyV1), "=")
			value, exists := current[key]
			if !ok || !exists || value != expected {
				continue
			}
			removed, err := document.Remove("env", key)
			if err != nil {
				return nil, false, nil, fmt.Errorf("remove %s env.%s: %w", miseConfigFilename, key, err)
			}
			changed = removed || changed
		}
		return document.Bytes(), changed, nil, nil
	}

	values := map[string]string{
		"DB_CONTAINER": slug + "-postgres",
		"DB_NAME":      slug + "_development",
		"DB_USER":      "postgres",
		"DB_PASS":      "postgres",
		"DB_PORT":      "5432",
		"DB_VOLUME":    slug + "-postgres-data",
	}
	for key := range values {
		if value, exists := current[key]; exists {
			values[key] = value
		}
	}
	values["DATABASE_URL"] = "postgres://{{env.DB_USER}}:{{env.DB_PASS}}@127.0.0.1:{{env.DB_PORT}}/{{env.DB_NAME}}?sslmode=disable"

	ownedValues := map[string]string{}
	for _, property := range owned {
		if !strings.HasPrefix(property, postgresEnvPropertyV1) {
			continue
		}
		if key, value, ok := strings.Cut(strings.TrimPrefix(property, postgresEnvPropertyV1), "="); ok {
			ownedValues[key] = value
		}
	}
	changed := false
	for _, key := range []string{"DB_CONTAINER", "DB_NAME", "DB_USER", "DB_PASS", "DB_PORT", "DB_VOLUME", "DATABASE_URL"} {
		if _, exists := current[key]; exists {
			continue
		}
		set, err := document.SetString("env", key, values[key])
		if err != nil {
			return nil, false, nil, fmt.Errorf("set %s env.%s: %w", miseConfigFilename, key, err)
		}
		changed = set || changed
		ownedValues[key] = values[key]
	}
	properties := make([]string, 0, len(ownedValues))
	for key, value := range ownedValues {
		properties = append(properties, postgresEnvPropertyV1+key+"="+value)
	}
	sort.Strings(properties)
	return document.Bytes(), changed, properties, nil
}

func tomlTableStrings(source []byte, table string) map[string]string {
	values := map[string]string{}
	section := ""
	for _, rawLine := range strings.Split(string(source), "\n") {
		line := strings.TrimSpace(stripComment(rawLine))
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		if section != table {
			continue
		}
		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		rawValue = strings.TrimSpace(rawValue)
		value, err := parseString(rawValue)
		if err != nil {
			value = strings.Trim(rawValue, "'\"")
		}
		values[strings.TrimSpace(key)] = value
	}
	return values
}

func rootServicesAssignment(source []byte) bool {
	section := ""
	for _, rawLine := range strings.Split(string(source), "\n") {
		line := strings.TrimSpace(stripComment(rawLine))
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if section == "" && ok && strings.TrimSpace(key) == "services" {
			return true
		}
	}
	return false
}

func removeManagedServiceBlock(source []byte) ([]byte, error) {
	text := string(source)
	start := strings.Index(text, lazyServiceBegin)
	end := strings.Index(text, lazyServiceEnd)
	if start < 0 || end < start {
		return nil, fmt.Errorf("%s has a malformed managed postgres service block", lazyConfigFilename)
	}
	end += len(lazyServiceEnd)
	if end < len(text) && text[end] == '\r' {
		end++
	}
	if end < len(text) && text[end] == '\n' {
		end++
	}
	if start >= 2 && text[start-2:start] == "\n\n" {
		start--
	}
	return []byte(text[:start] + text[end:]), nil
}

func modfileModulePath(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`)
		}
	}
	return ""
}

func applicationSlug(modulePath string) string {
	slug := path.Base(modulePath)
	var output strings.Builder
	for _, character := range strings.ToLower(slug) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			output.WriteRune(character)
		} else if output.Len() != 0 && !strings.HasSuffix(output.String(), "-") {
			output.WriteByte('-')
		}
	}
	return strings.Trim(output.String(), "-")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func postgresStartTask(slug string) string {
	return `#!/usr/bin/env bash
#MISE description="Start PostgreSQL in Docker in the foreground"
set -euo pipefail

CONTAINER="${DB_CONTAINER:-` + slug + `-postgres}"
DB_NAME="${DB_NAME:-` + slug + `_development}"
DB_USER="${DB_USER:-postgres}"
DB_PASS="${DB_PASS:-postgres}"
DB_PORT="${DB_PORT:-5432}"
DB_VOLUME="${DB_VOLUME:-` + slug + `-postgres-data}"

if docker inspect "${CONTAINER}" >/dev/null 2>&1; then
	if docker ps --format '{{.Names}}' | grep -Fxq "${CONTAINER}"; then
		exec docker attach --sig-proxy=true "${CONTAINER}"
	fi
	exec docker start -a "${CONTAINER}"
fi

exec docker run --name "${CONTAINER}" \
	-e "POSTGRES_DB=${DB_NAME}" \
	-e "POSTGRES_USER=${DB_USER}" \
	-e "POSTGRES_PASSWORD=${DB_PASS}" \
	-p "${DB_PORT}:5432" \
	-v "${DB_VOLUME}:/var/lib/postgresql/data" \
	postgres:17-alpine
`
}

func postgresCheckTask(slug string) string {
	return `#!/usr/bin/env bash
#MISE description="Check local PostgreSQL readiness"
set -euo pipefail

CONTAINER="${DB_CONTAINER:-` + slug + `-postgres}"
DB_USER="${DB_USER:-postgres}"

if docker exec "${CONTAINER}" pg_isready -U "${DB_USER}" -q >/dev/null 2>&1; then
	echo "READY"
	exit 0
fi

echo "NOT READY"
exit 1
`
}

func postgresCreateTask(slug string) string {
	return `#!/usr/bin/env bash
#MISE description="Create the local PostgreSQL database if it does not exist"
set -euo pipefail

CONTAINER="${DB_CONTAINER:-` + slug + `-postgres}"
DB_NAME="${DB_NAME:-` + slug + `_development}"
DB_USER="${DB_USER:-postgres}"

if ! docker exec "${CONTAINER}" pg_isready -U "${DB_USER}" -q >/dev/null 2>&1; then
	echo "PostgreSQL is not ready. Start it with: mise run postgres:start" >&2
	exit 1
fi

escaped_db_name="${DB_NAME//\'/\'\'}"
exists="$(docker exec "${CONTAINER}" psql -U "${DB_USER}" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '${escaped_db_name}'")"
if [ "${exists}" = "1" ]; then
	printf 'Database %s already exists\n' "${DB_NAME}"
	exit 0
fi

docker exec "${CONTAINER}" createdb -U "${DB_USER}" "${DB_NAME}"
printf 'Created database %s\n' "${DB_NAME}"
`
}

func postgresResetTask(slug string) string {
	return `#!/usr/bin/env bash
#MISE description="Destroy local PostgreSQL data"
set -euo pipefail

CONTAINER="${DB_CONTAINER:-` + slug + `-postgres}"
DB_VOLUME="${DB_VOLUME:-` + slug + `-postgres-data}"

docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
docker volume rm "${DB_VOLUME}" >/dev/null 2>&1 || true
echo "Removed ${CONTAINER} and ${DB_VOLUME}. Start again with: mise run postgres:start"
`
}

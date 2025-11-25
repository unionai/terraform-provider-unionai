package provider

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type FlyteEnvironment struct {
	numRuns int32
	lock    sync.Mutex
}

type FlyteEnvironmentDetails struct {
	Name    string
	Version string
	Tasks   []string
}

// Generic remove ANSI codes from a byte slice
func (f *FlyteEnvironment) removeAnsiCodes(data []byte) []byte {
	// ANSI escape sequence pattern: \x1b[...m
	// This regex matches ANSI escape sequences
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAll(data, []byte{})
}

// Parses the table from the input, matching the fields in the header, and return all the rows of the table
//
// Example:
//
// ┌─────────────────────────────────────────────────────────────────────────────┬────────────────────────────────────────┐
// │ Environment                                                                 │ Image                                  │
// ╞═════════════════════════════════════════════════════════════════════════════╪════════════════════════════════════════╡
// │ hello_world                                                                 │ auto                                   │
// └─────────────────────────────────────────────────────────────────────────────┴────────────────────────────────────────┘
//                                                         Entities
// ┌───────────┬───────────────────────────────┬─────────────────────────────────────────────────────────┬────────────────┐
// │ Type      │ Name                          │ Version                                                 │ Triggers       │
// ╞═══════════╪═══════════════════════════════╪═════════════════════════════════════════════════════════╪════════════════╡
// │ task      │ hello_world.fn3               │ d8b4e239eadafac9c83900ae8aab7841                        │                │
// │ task      │ hello_world.fn                │ d8b4e239eadafac9c83900ae8aab7841                        │                │
// │ task      │ hello_world.main              │ d8b4e239eadafac9c83900ae8aab7841                        │                │
// └───────────┴───────────────────────────────┴─────────────────────────────────────────────────────────┴────────────────┘

func (f *FlyteEnvironment) fetchTable(fields []string, input []byte) [][]string {
	lines := strings.Split(string(input), "\n")
	var results [][]string

	// Find the header row that contains all the specified fields
	headerIndex := -1
	var columnIndices []int

	for i, line := range lines {
		if !strings.Contains(line, "│") {
			continue
		}

		// Check if this line contains all the fields we're looking for
		containsAllFields := true
		for _, field := range fields {
			if !strings.Contains(line, field) {
				containsAllFields = false
				break
			}
		}

		if containsAllFields {
			headerIndex = i
			// Parse column positions for each field
			parts := strings.Split(line, "│")
			columnIndices = make([]int, len(fields))

			// Map each field to its column index
			for j, field := range fields {
				for k, part := range parts {
					if strings.TrimSpace(part) == field {
						columnIndices[j] = k
						break
					}
				}
			}
			break
		}
	}

	if headerIndex == -1 {
		return results // No matching header found
	}

	// Parse data rows after the header
	// Skip the separator line (next line after header)
	inDataSection := false
	for i := headerIndex + 1; i < len(lines); i++ {
		line := lines[i]

		// Skip the separator line (╞═══╡ or ├───┤)
		if strings.Contains(line, "═") || strings.Contains(line, "╞") {
			inDataSection = true
			continue
		}

		// Stop at the bottom border of the table
		if strings.Contains(line, "└") || strings.Contains(line, "───────────────") {
			break
		}

		// Parse data rows
		if inDataSection && strings.Contains(line, "│") {
			parts := strings.Split(line, "│")
			if len(parts) > 1 {
				var row []string
				for _, colIdx := range columnIndices {
					if colIdx < len(parts) {
						row = append(row, strings.TrimSpace(parts[colIdx]))
					} else {
						row = append(row, "")
					}
				}
				results = append(results, row)
			}
		}
	}

	return results
}

func (f *FlyteEnvironment) retrieveNameAndVersion(
	ctx context.Context,
	path string, project string, domain string, id string,
) (*FlyteEnvironmentDetails, error) {
	// TODO(nelson): Fix flyte deploy bug, where first time calculates the wrong hash.
	f.lock.Lock()

	if atomic.AddInt32(&f.numRuns, 1) == 1 {
		// First run is buggy, so let's waste it.
		f.lock.Unlock()
		if _, err := f.retrieveNameAndVersion(ctx, path, project, domain, id); err != nil {
			return nil, err
		}
	} else {
		f.lock.Unlock()
	}

	cmd := exec.Command("flyte", "deploy", "--dry-run",
		"--project", project,
		"--domain", domain,
		path, id,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("flyte deploy failed: %w", err)
	}
	out = f.removeAnsiCodes(out)

	var name string
	var version string
	var tasks []string

	// Parse environment table to get the environment name
	envTable := f.fetchTable([]string{"Environment", "Image"}, out)
	if len(envTable) > 0 {
		name = envTable[0][0]
	}

	// Parse entities table to get version and tasks
	entitiesTable := f.fetchTable([]string{"Type", "Name", "Version"}, out)
	for _, row := range entitiesTable {
		if len(row) >= 3 && row[0] == "task" {
			taskName := row[1]
			taskVersion := row[2]
			// Only include tasks that belong to this environment
			if strings.HasPrefix(taskName, name+".") {
				if version == "" {
					version = taskVersion
				}
				tasks = append(tasks, taskName)
			}
		}
	}

	if name == "" || version == "" {
		return nil, fmt.Errorf("failed to parse name and version from flyte deploy output")
	}

	tflog.Trace(ctx, "retrieveNameAndVersion", map[string]interface{}{
		"name":    name,
		"version": version,
		"tasks":   tasks,
		"output":  string(out),
	})

	return &FlyteEnvironmentDetails{
		Name:    name,
		Version: version,
		Tasks:   tasks,
	}, nil
}

func (f *FlyteEnvironment) uploadNewVersion(path string, project string, domain string, id string) error {
	cmd := exec.Command("flyte", "deploy",
		"--project", project,
		"--domain", domain,
		path, id,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("flyte deploy failed: %w", err)
	}
	return nil
}

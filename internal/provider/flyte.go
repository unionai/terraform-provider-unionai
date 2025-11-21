package provider

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

	// Call: flyte deploy --project <project> --domain <domain> <path> <id>
	// Example: flyte deploy --project nelson --domain development hello.py env
	// Output: d9f9ec4309463d534e147a4ef1222340

	// Shell command: flyte deploy --project <project> --domain <domain> <path> <id>
	// Example: flyte deploy --project nelson --domain development hello.py env

	// Deploying root - environment: env
	// ⠏ Deploying...13:26:17.239626 WARNING  _deploy.py:261 -  Built Image for environment hello_world, image: ghcr.io/flyteorg/flyte:py3.12-v2.0.0b32
	//                                                       Environments
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

	lines := strings.Split(string(out), "\n")

	// Parse output and retrieve the name of the environment
	foundEnvironmentTable := false
	for _, line := range lines {
		if !foundEnvironmentTable {
			if strings.Contains(line, "│") && strings.Contains(line, "Environment") && strings.Contains(line, "Image") {
				foundEnvironmentTable = true
			}
			continue
		}
		if strings.Contains(line, "───────────────") {
			break // We found the end of the version table
		}

		if strings.Contains(line, "│") {
			parts := strings.Split(line, "│")
			if len(parts) > 1 {
				name = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	// Parse output and retrieve the version. Use the first value of a "task" with the "<environment_name>." prefix.
	foundVersionTable := false
	for _, line := range lines {
		// Wait for header "Type │ Name │ Version" before matching version
		if !foundVersionTable {
			if strings.Contains(line, "│") && strings.Contains(line, "Type") && strings.Contains(line, "Name") && strings.Contains(line, "Version") {
				foundVersionTable = true
			}
			continue
		}
		if strings.Contains(line, "───────────────") {
			break // We found the end of the version table
		}

		if strings.HasPrefix(line, "│ task ") {
			// Split the line on "│" and trim spaces to handle table alignment
			parts := strings.Split(line, "│")
			if len(parts) > 2 {
				taskName := strings.TrimSpace(parts[2])
				if strings.HasPrefix(taskName, name+".") {
					version = strings.TrimSpace(parts[3])
					tasks = append(tasks, taskName)
				}
			}
		}
	}

	if name == "" || version == "" {
		return nil, fmt.Errorf("failed to parse name and version from flyte deploy output")
	}

	tflog.Trace(ctx, fmt.Sprintf("traced raw %d", time.Now().Unix()), map[string]interface{}{
		"name":    name,
		"version": version,
		"output":  string(out),
	})

	return &FlyteEnvironmentDetails{
		Name:    name,
		Version: version,
		Tasks:   tasks,
	}, nil
}

func (f *FlyteEnvironment) uploadNewVersion(ctx context.Context, path string, project string, domain string, id string) error {
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

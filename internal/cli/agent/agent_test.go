package agent

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/doctor"
)

func TestCheckDockerRequirement_NonIsolatedNoDocker(t *testing.T) {
	for _, class := range []doctor.Classification{doctor.ClassLibrary, doctor.ClassTool, doctor.ClassSite} {
		t.Run(string(class), func(t *testing.T) {
			state := &doctor.ProjectState{Classification: class, DockerAvailable: false}
			var stderr bytes.Buffer
			if err := checkDockerRequirement(state, &stderr); err != nil {
				t.Fatalf("expected no error for %s, got: %v", class, err)
			}
			if !strings.Contains(stderr.String(), "warning") {
				t.Errorf("expected warning on stderr for %s, got: %q", class, stderr.String())
			}
		})
	}
}

func TestCheckDockerRequirement_IsolatedNoDocker(t *testing.T) {
	for _, class := range []doctor.Classification{doctor.ClassService, doctor.ClassSaaS} {
		t.Run(string(class), func(t *testing.T) {
			state := &doctor.ProjectState{Classification: class, DockerAvailable: false}
			var stderr bytes.Buffer
			err := checkDockerRequirement(state, &stderr)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", class)
			}
			if !strings.Contains(err.Error(), "docker") {
				t.Errorf("error should mention docker: %v", err)
			}
			if !strings.Contains(err.Error(), string(class)) {
				t.Errorf("error should mention classification %s: %v", class, err)
			}
		})
	}
}

func TestCheckDockerRequirement_DockerAvailable(t *testing.T) {
	for _, class := range doctor.AllClassifications {
		t.Run(string(class), func(t *testing.T) {
			state := &doctor.ProjectState{Classification: class, DockerAvailable: true}
			var stderr bytes.Buffer
			if err := checkDockerRequirement(state, &stderr); err != nil {
				t.Fatalf("expected no error when docker available, got: %v", err)
			}
			if stderr.Len() != 0 {
				t.Errorf("expected no stderr when docker available, got: %q", stderr.String())
			}
		})
	}
}

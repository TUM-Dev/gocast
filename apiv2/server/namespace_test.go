package apiv2

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	// runner/*.proto also declares `package protobuf`, and the server links both.
	// Two descriptors under one name panic at init, which nothing in this package
	// would otherwise catch — it links only half the namespace. Side effect only.
	_ "github.com/tum-dev/gocast/runner/protobuf"
)

// Reaching this function at all means both descriptor sets registered; the lookups
// below only make the failure legible when they did not.
func TestProtoNamespaceIsSharedWithTheRunner(t *testing.T) {
	// Every service this API serves has to resolve to this file, not the runner's.
	for _, svc := range services {
		desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(svc.desc.ServiceName))
		if err != nil {
			t.Errorf("%s is not in the global registry: %v", svc.desc.ServiceName, err)
			continue
		}

		if path := desc.ParentFile().Path(); path != "server/apiv2.proto" {
			t.Errorf(
				"%s resolves to %s, not server/apiv2.proto: the name is claimed by another proto "+
					"sharing the `protobuf` package",
				svc.desc.ServiceName, path,
			)
		}
	}

	// A rename here must never take over one of the runner's names.
	for _, name := range []string{"protobuf.RunnerService", "protobuf.RunnerManagerService"} {
		desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		if err != nil {
			t.Errorf("the runner's %s is missing from the registry: %v", name, err)
			continue
		}

		if path := desc.ParentFile().Path(); path == "server/apiv2.proto" {
			t.Errorf("%s now resolves to apiv2.proto, taking over a name the runner owns", name)
		}
	}
}

// protoDeclarations returns the top-level message, enum and service names a .proto
// file declares. Read from the source rather than the registry because the registry
// cannot answer: two descriptors under one name panic inside init, so a test that
// waits for the linked binary reports the collision as a stack trace from a package
// nobody edited, if it reports it at all.
//
// Only column-zero declarations count, which is what makes a regexp enough here:
// nested messages are indented, and their names are scoped by their parent anyway.
func protoDeclarations(t *testing.T, path string) map[string]string {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}

	pattern := regexp.MustCompile(`(?m)^(message|enum|service) ([A-Za-z0-9_]+)`)

	out := make(map[string]string)
	for _, match := range pattern.FindAllStringSubmatch(string(source), -1) {
		out[match[2]] = match[1]
	}

	if len(out) == 0 {
		t.Fatalf("no declarations found in %s; the pattern no longer matches the file", path)
	}

	return out
}

// The runner's protos share the `protobuf` package with this API, so a name declared
// in both files is not a namespacing nicety: the second registration panics as the
// server starts, with a message that names neither file.
//
// TestProtoNamespaceIsSharedWithTheRunner above covers the services. This covers the
// messages and enums, which is where it actually happened: adding a plain
// `Notification` for the admin notifications page took down the process at boot.
func TestApiv2DeclaresNoNameTheRunnerOwns(t *testing.T) {
	apiv2Declarations := protoDeclarations(t, "apiv2.proto")

	// Every .proto the runner has that declares `package protobuf`.
	for _, runnerProto := range []string{
		"../../runner/commons.proto",
		"../../runner/notifications.proto",
		"../../runner/runner.proto",
	} {
		for name, kind := range protoDeclarations(t, runnerProto) {
			if ours, taken := apiv2Declarations[name]; taken {
				t.Errorf(
					"apiv2.proto declares %s %q, which %s already declares as a %s. Both files are "+
						"`package protobuf` and the server links both, so this panics at startup. "+
						"Give the apiv2 one a prefix naming its page.",
					ours, name, filepath.Base(runnerProto), kind,
				)
			}
		}
	}
}

package apiv2

import (
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

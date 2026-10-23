package buildconfig

import (
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

func TestMetadataArgumentsRejectUntrustedIdentity(t *testing.T) {
	for _, info := range []buildinfo.Info{
		{Version: "v1.0..0", Commit: strings.Repeat("a", 40)},
		{Version: "v1.0.0", Commit: "-X injected=value"},
		{Version: "v1.0.0", Commit: strings.Repeat("a", 40), Dirty: true},
	} {
		if _, err := MetadataArguments([]string{"build", "-ldflags", "-s"}, info, "windows", "amd64", true); err == nil {
			t.Fatalf("accepted invalid release metadata: %+v", info)
		}
	}
	info := buildinfo.Info{Version: "dev", Commit: strings.Repeat("a", 40), Dirty: true}
	args, err := MetadataArguments([]string{"build", "-ldflags", "-s"}, info, "linux", "arm64", false)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, ".version=dev") || !strings.Contains(joined, ".dirty=true") || strings.Contains(joined, "packageUpdateFeed") {
		t.Fatal("development identity or update eligibility is incorrect")
	}
}

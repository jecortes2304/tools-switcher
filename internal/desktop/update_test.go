package desktop

import (
	"context"
	"io"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

type testProvider struct{ release *updater.Release }

func (p testProvider) Name() string { return "test" }
func (p testProvider) Check(context.Context, updater.CheckRequest) (*updater.Release, error) {
	return p.release, nil
}
func (p testProvider) Download(context.Context, *updater.Release, io.Writer, func(int64, int64)) error {
	return nil
}
func TestUpdateProviderRequiresSignature(t *testing.T) {
	for _, verification := range []*updater.Verification{nil, {DigestAlgo: "sha256", Digest: make([]byte, 32)}} {
		provider := signedProvider{testProvider{&updater.Release{Version: "2.0.0", Verification: verification}}}
		if _, err := provider.Check(context.Background(), updater.CheckRequest{}); err == nil {
			t.Fatal("unsigned update accepted")
		}
	}
	provider := signedProvider{testProvider{nil}}
	if release, err := provider.Check(context.Background(), updater.CheckRequest{}); release != nil || err != nil {
		t.Fatal("up-to-date result changed")
	}
}

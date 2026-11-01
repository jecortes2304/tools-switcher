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

func TestUpdateEventsPreserveNotesAndReportProgress(t *testing.T) {
	service := &Service{}
	release := &updater.Release{Version: "0.2.0", Notes: "### Novedades\n* Asistente", Artifact: updater.Artifact{Size: 100}}
	service.captureUpdateEvent(updater.EventUpdateAvailable, release)
	service.captureUpdateEvent(updater.EventDownloadProgress, updater.Progress{Written: 40, Total: 100, Rate: 25})
	if service.update.Version != "0.2.0" || service.update.Notes != release.Notes || service.update.Written != 40 || service.update.Rate != 25 {
		t.Fatal("release metadata or progress missing")
	}
	service.captureUpdateEvent(updater.EventError, updater.ErrorInfo{Stage: updater.StageVerify, Message: "firma inválida"})
	if service.update.ErrorStage != "verify" || service.update.Error != "firma inválida" {
		t.Fatal("verification error hidden")
	}
	service.captureUpdateEvent(updater.EventDownloadStarted, release)
	if service.update.Written != 0 || service.update.Error != "" || service.update.ErrorStage != "" {
		t.Fatal("retry retained stale progress or error")
	}
	service.captureUpdateEvent(updater.EventUpdateReady, release)
	if service.update.Written != service.update.Total {
		t.Fatal("ready progress incomplete")
	}
	service.captureUpdateEvent(updater.EventNoUpdate, nil)
	if service.update.Version != "" || service.update.Notes != "" || service.update.Total != 0 {
		t.Fatal("old release still visible")
	}
}

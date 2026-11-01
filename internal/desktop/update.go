package desktop

import "github.com/wailsapp/wails/v3/pkg/updater"

func (s *Service) captureUpdateEvent(name string, data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch name {
	case updater.EventCheckStarted:
		s.update.Error = ""
		s.update.ErrorStage = ""
	case updater.EventNoUpdate:
		s.update.Version = ""
		s.update.Notes = ""
		s.update.Written = 0
		s.update.Total = 0
		s.update.Rate = 0
	case updater.EventUpdateAvailable, updater.EventDownloadStarted:
		if release, ok := data.(*updater.Release); ok && release != nil {
			s.update.Version = release.Version
			s.update.Notes = release.Notes
			s.update.Total = release.Artifact.Size
		}
		s.update.Written = 0
		s.update.Rate = 0
		s.update.Error = ""
		s.update.ErrorStage = ""
	case updater.EventDownloadProgress:
		if progress, ok := data.(updater.Progress); ok {
			s.update.Written = max(0, progress.Written)
			s.update.Total = max(0, progress.Total)
			s.update.Rate = max(0, progress.Rate)
		}
	case updater.EventUpdateReady:
		s.update.Written = s.update.Total
		s.update.Rate = 0
	case updater.EventError:
		if failure, ok := data.(updater.ErrorInfo); ok {
			s.update.Error = failure.Message
			s.update.ErrorStage = string(failure.Stage)
		}
	}
}

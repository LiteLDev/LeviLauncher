package main

import (
	"github.com/liteldev/LeviLauncher/internal/app"
	"github.com/liteldev/LeviLauncher/internal/deeplink"

	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/leviloader"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/msixvc"

	"github.com/liteldev/LeviLauncher/internal/peeditor"

	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func init() {
	application.RegisterEvent[map[string]any]("microsoft.browser.signin")
	//minecraft
	application.RegisterEvent[struct{}](app.EventGameInputEnsureStart)
	application.RegisterEvent[struct{}](app.EventGameInputEnsureDone)
	application.RegisterEvent[int64](app.EventGameInputDownloadStart)
	application.RegisterEvent[app.GameInputDownloadProgress](app.EventGameInputDownloadProgress)
	application.RegisterEvent[struct{}](app.EventGameInputDownloadDone)
	application.RegisterEvent[string](app.EventGameInputDownloadError)
	application.RegisterEvent[string](mcservice.EventExtractError)
	application.RegisterEvent[string](mcservice.EventExtractDone)
	application.RegisterEvent[types.ExtractProgress](mcservice.EventExtractProgress)
	application.RegisterEvent[types.InstanceBackupRestoreProgress](mcservice.EventInstanceBackupRestoreProgress)
	application.RegisterEvent[types.InstanceBackupCreateProgress](mcservice.EventInstanceBackupCreateProgress)
	application.RegisterEvent[types.InstanceBackupRestoreProgress](mcservice.EventInstanceBackupPreviewProgress)
	// launch
	application.RegisterEvent[struct{}](launch.EventMcLaunchStart)
	application.RegisterEvent[struct{}](launch.EventMcLaunchDone)
	application.RegisterEvent[string](launch.EventMcLaunchFailed)
	// The shared frontend is type-checked on every platform.
	application.RegisterEvent[types.WineInstallProgress]("wine.install.progress")
	//msixvc
	application.RegisterEvent[msixvc.DownloadStatus](msixvc.EventDownloadStatus)
	application.RegisterEvent[msixvc.DownloadProgress](msixvc.EventDownloadProgress)
	application.RegisterEvent[msixvc.DownloadDone](msixvc.EventDownloadDone)
	application.RegisterEvent[msixvc.DownloadError](msixvc.EventDownloadError)
	application.RegisterEvent[bool](msixvc.EventAppxInstallLoading)
	// peeditor
	application.RegisterEvent[struct{}](peeditor.EventEnsureStart)
	application.RegisterEvent[bool](peeditor.EventEnsureDone)
	// vcruntime
	application.RegisterEvent[struct{}](vcruntime.EventEnsureStart)
	application.RegisterEvent[vcruntime.EnsureProgress](vcruntime.EventEnsureProgress)
	application.RegisterEvent[bool](vcruntime.EventEnsureDone)
	// loader migration
	application.RegisterEvent[struct{}](leviloader.EventMigrateStart)
	application.RegisterEvent[leviloader.MigrateProgress](leviloader.EventMigrateProgress)
	application.RegisterEvent[int](leviloader.EventMigrateDone)
	application.RegisterEvent[string](leviloader.EventMigrateError)
	// app update
	application.RegisterEvent[string](update.EventAppUpdateStatus)
	application.RegisterEvent[update.AppUpdateProgress](update.EventAppUpdateProgress)
	application.RegisterEvent[string](update.EventAppUpdateError)
	// lip daemon task stream
	application.RegisterEvent[lip.LipTaskStartedEvent](lip.EventLipTaskStarted)
	application.RegisterEvent[lip.LipTaskLogEvent](lip.EventLipTaskLog)
	application.RegisterEvent[lip.LipTaskProgressEvent](lip.EventLipTaskProgress)
	application.RegisterEvent[lip.LipTaskFinishedEvent](lip.EventLipTaskFinished)
	application.RegisterEvent[types.FilesDroppedEvent]("files-dropped")
	application.RegisterEvent[deeplink.Request](deeplink.EventOpen)
}

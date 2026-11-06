package mcservice

const (
	EventExtractError                  = "extract.error"
	EventExtractDone                   = "extract.done"
	EventExtractProgress               = "extract.progress"
	EventInstanceBackupRestoreProgress = "instance_backup.restore.progress"
	EventInstanceBackupCreateProgress  = "instance_backup.create.progress"
	// EventInstanceBackupPreviewProgress carries a
	// types.InstanceBackupRestoreProgress for the archive extraction done by
	// PreviewInstanceBackupRestoreConflicts (phase "extracting", step 1 of 1).
	EventInstanceBackupPreviewProgress = "instance_backup.preview.progress"
)

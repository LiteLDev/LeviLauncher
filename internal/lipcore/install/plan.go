package install

import "github.com/liteldev/LeviLauncher/internal/lipcore/archive"

// Plan is PackagePlan.cs: what installing or uninstalling one package does
// to the workspace, planned and checked before anything changes
// (protocol/workspace.md §4).
type Plan struct {
	// Name is the package identity, for messages.
	Name string
	// Version is the version of the package, for progress ("Name@Version").
	Version string
	// Root is the absolute workspace root, where hook commands run.
	Root string
	// OwnedFiles are, for an install, the files the package owns afterwards
	// (workspace-relative, '/'-separated), in first-seen order.
	OwnedFiles []string
	// Kept are, for an install, the existing files kept because they match
	// preserve (workspace-relative), which the package does not own.
	Kept  []string
	Steps []Step
}

// WrittenPaths are the absolute paths the plan writes, which backups must
// never take (txn.Options.Reserved).
func (p *Plan) WrittenPaths() []string {
	var paths []string
	for _, s := range p.Steps {
		if w, ok := s.(*WriteStep); ok {
			paths = append(paths, w.FullPath)
		}
	}
	return paths
}

// Step is PlannedStep: one step of a plan. Paths are absolute.
type Step interface{ step() }

// ScriptStep runs the commands of a hook (e.g. "pre_install") in turn.
type ScriptStep struct {
	Stage    string
	Commands []string
}

// WriteStep writes the file Key of Source, replacing what is there. Target
// is workspace-relative.
type WriteStep struct {
	Target   string
	FullPath string
	Source   archive.Source
	Key      string
}

// DeleteFileStep deletes a file, if it still exists then.
type DeleteFileStep struct{ FullPath string }

// DeleteDirectoryStep deletes a directory when the operation commits, if it
// is empty then.
type DeleteDirectoryStep struct{ FullPath string }

func (*ScriptStep) step()          {}
func (*WriteStep) step()           {}
func (*DeleteFileStep) step()      {}
func (*DeleteDirectoryStep) step() {}

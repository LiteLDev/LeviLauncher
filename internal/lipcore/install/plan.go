package install

import (
	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pathglob"
)

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
	// (workspace-relative, '/'-separated), in first-seen order, when the
	// workspace is as planned; hook commands may change which files matching
	// preserve are kept, and Installer.Apply returns the files it owns as it
	// happened.
	OwnedFiles []string
	Steps      []Step
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

// RunsHooks is PackagePlan.RunsHooks: whether the plan runs a hook command.
func (p *Plan) RunsHooks() bool {
	for _, s := range p.Steps {
		if script, ok := s.(*ScriptStep); ok && len(script.Commands) > 0 {
			return true
		}
	}
	return false
}

// MayChange is PackagePlan.MayChange: whether the plan may change the
// workspace (protocol/workspace.md §1): it writes a file, deletes a file or
// a directory, or, when hook commands of the operation run (hooksRun),
// writes a file planned to be kept or deletes what they leave matching when
// it runs (§6 rule 4); without them the workspace stays as planned. Hook
// commands alone are no change.
func (p *Plan) MayChange(hooksRun bool) bool {
	for _, s := range p.Steps {
		switch s := s.(type) {
		case *WriteStep:
			if hooksRun || !s.PlannedKeep {
				return true
			}
		case *DeleteFileStep, *DeleteDirectoryStep:
			return true
		case *DeleteMatchesStep:
			if hooksRun && (len(s.OwnedFiles) > 0 || len(s.Purge) > 0) {
				return true
			}
		}
	}
	return false
}

// Step is PlannedStep: one step of a plan. Paths are absolute.
type Step interface{ step() }

// ScriptStep runs the commands of a hook (e.g. "pre_install") in turn.
type ScriptStep struct {
	Stage    string
	Commands []string
}

// WriteStep writes the file Key of Source, replacing what is there. Target
// is workspace-relative. Preserve are the globs of preserve of the package:
// a file there when the step runs that matches one is kept instead.
// PlannedKeep is whether the file is kept with the workspace as planned,
// i.e. unless hook commands change it.
type WriteStep struct {
	Target      string
	FullPath    string
	Source      archive.Source
	Key         string
	Preserve    []*pathglob.Glob
	PlannedKeep bool
}

// DeleteFileStep deletes a file, if it still exists then.
type DeleteFileStep struct{ FullPath string }

// DeleteDirectoryStep deletes a directory when the operation commits, if it
// is empty then.
type DeleteDirectoryStep struct{ FullPath string }

// DeleteMatchesStep deletes what uninstallation steps 2-4 match when it runs
// but the plan did not foresee, such as files that hook commands created
// (protocol/workspace.md §6 rule 4): the owned files not matching preserve,
// and the paths matching purge, except what another package owns then.
// Paths are relative.
type DeleteMatchesStep struct {
	OwnedFiles []string
	Purge      []*pathglob.Glob
}

func (*ScriptStep) step()          {}
func (*WriteStep) step()           {}
func (*DeleteFileStep) step()      {}
func (*DeleteDirectoryStep) step() {}
func (*DeleteMatchesStep) step()   {}

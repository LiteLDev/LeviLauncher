package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// LayoutError is WorkspaceLayoutException: the workspace layout cannot be
// made right; the workspace is left unchanged. Error() is lip's message.
type LayoutError struct {
	Msg string
	Err error
}

func (e *LayoutError) Error() string { return e.Msg }
func (e *LayoutError) Unwrap() error { return e.Err }

// LayoutPlan is WorkspaceLayout.LayoutPlan: what making the layout right
// takes. Mods is absolute; LinkTarget is where mods links to when it is a
// link (IsLink).
type LayoutPlan struct {
	Mods       string
	CreateMods bool
	IsLink     bool
	LinkTarget string
}

// PlanLayout is WorkspaceLayout.Plan (protocol/workspace.md §2): reads what
// making the layout right takes, without changing anything: mods is created
// when missing. Whether the layout is right is checked (CheckLayout) only
// when the operation changes the workspace.
func PlanLayout(workspaceRoot string) (*LayoutPlan, error) {
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, err
	}
	mods := filepath.Join(root, pkgmodel.ModsDir)
	if target, err := os.Readlink(mods); err == nil {
		return &LayoutPlan{Mods: mods, IsLink: true, LinkTarget: target}, nil
	}
	return &LayoutPlan{Mods: mods, CreateMods: !dirExists(mods)}, nil
}

// CheckLayout is WorkspaceLayout.Check: checks the layout before an
// operation changes the workspace: a mods that is a link is an error.
func CheckLayout(plan *LayoutPlan) error {
	// A mods link would let writes under mods/ land outside the workspace.
	if plan != nil && plan.IsLink {
		return &LayoutError{Msg: fmt.Sprintf(
			"The mods directory '%s' is a link to '%s', but lip only installs into a"+
				" real directory, so that it never writes outside the workspace. Replace the link"+
				" with a directory (move the mods into it), then try again.", plan.Mods, plan.LinkTarget)}
	}
	return nil
}

// DryRunMessages is WorkspaceLayout.ReportDryRun: what applying the plan
// would do, for the service to print (PrintInfo).
func (p *LayoutPlan) DryRunMessages() []string {
	if p == nil || !p.CreateMods {
		return nil
	}
	return []string{"Dry run: would create the directory " + pkgmodel.ModsDir + "/"}
}

// ApplyLayout is WorkspaceLayout.Apply: applies the plan through the
// transaction, which undoes it when it or a later change fails.
func ApplyLayout(tx *txn.Transaction, plan *LayoutPlan) error {
	if plan == nil || !plan.CreateMods {
		return nil
	}
	if err := tx.CreateDirectory(plan.Mods); err != nil {
		// The failure may also be of the journal, written first; its message
		// is the cause.
		msg := fmt.Sprintf("Cannot create the mods directory '%s'.", plan.Mods)
		if fileExists(plan.Mods) {
			msg += " A file named mods is in the way: move or delete it, then try again."
		}
		return &LayoutError{Msg: msg, Err: err}
	}
	return nil
}

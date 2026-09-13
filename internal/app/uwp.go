package app

import (
	"context"

	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
)

func (a *Minecraft) uwpContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *Minecraft) FetchUWPVersions() ([]uwpdownload.Version, error) {
	return mcservice.FetchUWPVersions(a.uwpContext())
}

func (a *Minecraft) StartUWPDownload(version, updateID, channel string) (string, error) {
	return mcservice.StartUWPDownload(a.uwpContext(), version, updateID, channel)
}

func (a *Minecraft) ResolveDownloadedUWP(version, channel string) string {
	return mcservice.ResolveDownloadedUWP(version, channel)
}

func (a *Minecraft) DeleteDownloadedUWP(version, channel string) string {
	return mcservice.DeleteDownloadedUWP(version, channel)
}

func (a *Minecraft) InstallExtractAppx(path, folderName, channel string) string {
	return mcservice.InstallExtractAppx(a.uwpContext(), path, folderName, channel)
}

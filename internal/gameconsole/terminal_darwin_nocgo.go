//go:build darwin && !cgo

package gameconsole

func defaultBundleIDForFile(string) string { return "" }
func defaultBundleIDForUTI(string) string  { return "" }

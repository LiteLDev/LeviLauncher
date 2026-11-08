package archive

// fileIdentity is FileIdentity.cs: the device (volume) holding a file and
// its number on that device. Two paths name the same file exactly when their
// identities are equal, however they reach it (links, junctions, case, 8.3
// short names, subst drives), which comparing paths cannot tell.
type fileIdentity struct {
	device            uint64
	fileLow, fileHigh uint64
}

// identityOf is FileIdentity.Of: the identity of the file at path, following
// links, or false when it cannot be read (e.g. the file does not exist).
// Some file systems (network redirectors, FUSE) give every file the number 0,
// which tells no files apart, so 0 is not an identity either.
func identityOf(path string) (fileIdentity, bool) {
	id, ok := fileIdentityOf(path)
	if !ok || (id.fileLow == 0 && id.fileHigh == 0) {
		return fileIdentity{}, false
	}
	return id, true
}

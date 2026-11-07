// Package compat reads the files of lip v0.x as lip v0.38.0-beta.1 does
// (protocol/compatibility.md §2 and §3):
//
//   - a tooth.json of format 1, 2 or 3 (formats 1 and 2 first read as format
//     3 by package migrate) is read, for an environment, as the format 4
//     package file Section 2 derives from it (Tooth.Derive, ports
//     src/Lip.Core/Mcmod/Compat/ToothCompat.cs);
//   - a tooth_lock.json at the root of a workspace without a workspace file
//     is read as the workspace file Section 3 derives from it
//     (ReadToothLock, DeriveWorkspace; ports
//     src/Lip.Core/Mcmod/Compat/ToothLockMigration.cs).
//
// The entries of a derived package file are Derived (pkgmodel.Entry.Derived):
// their dependencies are "ClientCompat" (PackageDependency.ClientCompat): a
// dependency on X#client names the variant "" of X when X has a package file
// (lip.yaml). The resolver applies that; this package only marks the entries.
//
// Errors: a *FormatError is lip's PackageFormatException (the tooth.json
// cannot be read for the environment). Any other error from Derive is a
// tooth.json lip v0.38 itself fails on with an unexpected exception (a JSON
// null where its format 3 model dereferences a value); callers treat it as a
// failure of the operation, as lip does.
package compat

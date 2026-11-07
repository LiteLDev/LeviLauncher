package semver

import (
	"slices"
	"strings"
)

// This file ports Semver.Ranges.LeftBoundedRange / RightBoundedRange,
// UnbrokenSemVersionRange and the merging part of SemVersionRange.Create.

var (
	minVersion        = Version{Prerelease: []string{"0"}} // 0.0.0-0
	minReleaseVersion = Version{}                          // 0.0.0
)

// bound is a left or right bound; v == nil means unbounded.
type bound struct {
	v         *Version
	inclusive bool
}

func (b bound) equal(o bound) bool {
	if b.inclusive != o.inclusive || (b.v == nil) != (o.v == nil) {
		return false
	}
	return b.v == nil || b.v.equal(*o.v)
}

// comparePrecedence is SemVersion.ComparePrecedence with null lowest.
func comparePrecedence(a, b *Version) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	}
	return a.Compare(*b)
}

func cmpBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

// Left bound methods.

func leftContains(b bound, v Version) bool {
	c := comparePrecedence(b.v, &v)
	if b.inclusive {
		return c <= 0
	}
	return c < 0
}

func leftIncludesPrerelease(b bound) bool { return b.v != nil && b.v.IsPrerelease() }

func leftCompare(a, b bound) int {
	if c := comparePrecedence(a.v, b.v); c != 0 {
		return c
	}
	return -cmpBool(a.inclusive, b.inclusive)
}

func leftCompareRight(l, r bound) int {
	if r.v == nil {
		return -1
	}
	if c := comparePrecedence(l.v, r.v); c != 0 {
		return c
	}
	if l.inclusive && r.inclusive {
		return 0
	}
	return 1
}

func leftMin(a, b bound) bound {
	if leftCompare(a, b) <= 0 {
		return a
	}
	return b
}

func leftMax(a, b bound) bound {
	if leftCompare(a, b) >= 0 {
		return a
	}
	return b
}

func leftString(b bound) string {
	if b.inclusive {
		return ">=" + b.v.String()
	}
	return ">" + b.v.String()
}

// Right bound methods.

func rightContains(b bound, v Version) bool {
	if b.v == nil {
		return true
	}
	c := v.Compare(*b.v)
	if b.inclusive {
		return c <= 0
	}
	return c < 0
}

func rightIncludesPrerelease(b bound) bool {
	return b.v != nil && b.v.IsPrerelease() && !(!b.inclusive && b.v.prereleaseIsZero())
}

func rightCompare(a, b bound) int {
	switch {
	case a.v == nil && b.v == nil:
		return 0
	case a.v == nil:
		return 1
	case b.v == nil:
		return -1
	}
	if c := a.v.Compare(*b.v); c != 0 {
		return c
	}
	return cmpBool(a.inclusive, b.inclusive)
}

func rightMin(a, b bound) bound {
	if rightCompare(a, b) <= 0 {
		return a
	}
	return b
}

func rightMax(a, b bound) bound {
	if rightCompare(a, b) >= 0 {
		return a
	}
	return b
}

func rightString(b bound) string {
	if b.inclusive {
		return "<=" + b.v.String()
	}
	return "<" + b.v.String()
}

// unbrokenRange is UnbrokenSemVersionRange.
type unbrokenRange struct {
	left, right bound
	// includeAllPrerelease is the public IncludeAllPrerelease
	// (includeAllPrerelease | allPrereleaseCoveredByEnds).
	includeAllPrerelease       bool
	allPrereleaseCoveredByEnds bool
}

var emptyUnbroken = unbrokenRange{right: bound{v: &minVersion}}

func (r unbrokenRange) equal(o unbrokenRange) bool {
	return r.left.equal(o.left) && r.right.equal(o.right) &&
		r.includeAllPrerelease == o.includeAllPrerelease
}

func (r unbrokenRange) isEmpty() bool { return r.equal(emptyUnbroken) }

// newUnbroken is UnbrokenSemVersionRange.Create(start, end, includeAllPrerelease).
func newUnbroken(start, end bound, includeAllPrerelease bool) unbrokenRange {
	if boundsEmpty(start, end, includeAllPrerelease) {
		return emptyUnbroken
	}
	covered := false
	if start.v != nil && end.v != nil {
		s, e := *start.v, *end.v
		if s.equal(e) {
			covered = s.IsPrerelease()
			includeAllPrerelease = covered
		} else if leftIncludesPrerelease(start) || rightIncludesPrerelease(end) {
			if s.majorMinorPatchEquals(e) {
				covered = true
			} else if (rightIncludesPrerelease(end) || e.prereleaseIsZero()) &&
				s.Major == e.Major && s.Minor == e.Minor && s.Patch.isSucc(e.Patch) {
				covered = true
			}
		}
	}
	return unbrokenRange{
		left:                       start,
		right:                      end,
		includeAllPrerelease:       includeAllPrerelease || covered,
		allPrereleaseCoveredByEnds: covered,
	}
}

func boundsEmpty(start, end bound, includeAllPrerelease bool) bool {
	if end.v == nil {
		return false
	}
	c := comparePrecedence(start.v, end.v)
	if c > 0 {
		return true
	}
	if c == 0 {
		return !(start.inclusive && end.inclusive)
	}
	if start.v == nil {
		if end.inclusive {
			return false
		}
		return end.v.equal(minVersion) || (!includeAllPrerelease && end.v.equal(minReleaseVersion))
	}
	if start.inclusive || end.inclusive || includeAllPrerelease ||
		start.v.IsPrerelease() || end.v.IsPrerelease() {
		return false
	}
	return start.v.Major == end.v.Major && start.v.Minor == end.v.Minor &&
		start.v.Patch.isSucc(end.v.Patch)
}

func (r unbrokenRange) contains(v Version) bool {
	if !leftContains(r.left, v) || !rightContains(r.right, v) {
		return false
	}
	if r.includeAllPrerelease || !v.IsPrerelease() {
		return true
	}
	// Prerelease versions must match either the start or end.
	return r.left.v != nil && r.left.v.IsPrerelease() && v.majorMinorPatchEquals(*r.left.v) ||
		r.right.v != nil && r.right.v.IsPrerelease() && v.majorMinorPatchEquals(*r.right.v)
}

// String is UnbrokenSemVersionRange.ToString.
func (r unbrokenRange) String() string {
	if r.isEmpty() {
		return "<0.0.0-0"
	}
	if r.left.inclusive && r.right.inclusive && r.left.v != nil && r.right.v != nil && r.left.v.equal(*r.right.v) {
		return r.left.v.String()
	}
	extraPrerelease := r.includeAllPrerelease && !r.allPrereleaseCoveredByEnds
	leftUnbounded := r.left.v == nil && !r.left.inclusive
	rightUnbounded := r.right.v == nil && !r.right.inclusive
	if leftUnbounded && rightUnbounded {
		if extraPrerelease {
			return "*-*"
		}
		return "*"
	}
	if s, ok := r.specialString(extraPrerelease); ok {
		return s
	}
	var s string
	switch {
	case leftUnbounded:
		s = rightString(r.right)
	case rightUnbounded:
		s = leftString(r.left)
	default:
		s = leftString(r.left) + " " + rightString(r.right)
	}
	if extraPrerelease {
		return "*-* " + s
	}
	return s
}

func (r unbrokenRange) specialString(extraPrerelease bool) (string, bool) {
	// Most special ranges follow the pattern '>=X.Y.Z <P.Q.R-0'.
	if r.left.inclusive && !r.right.inclusive && r.right.v != nil && r.right.v.prereleaseIsZero() {
		start, end := *r.left.v, *r.right.v
		tilde := false
		// Wildcard ranges like 2.*, 2.*-*, 2.3.*, and 2.3.*-*
		if start.Patch.IsZero() && end.Patch.IsZero() && (!start.IsPrerelease() || start.prereleaseIsZero()) {
			var result string
			switch {
			case start.Major == end.Major && start.Minor.isSucc(end.Minor):
				result = start.Major.String() + "." + start.Minor.String() + ".*"
			case start.Major.isSucc(end.Major) && start.Minor.IsZero() && end.Minor.IsZero():
				result = start.Major.String() + ".*"
			default:
				tilde = true
			}
			if !tilde {
				if !extraPrerelease {
					return result, true
				}
				if start.prereleaseIsZero() {
					return result + "-*", true
				}
				return "*-* " + result, true
			}
		}
		// Wildcard ranges like 2.1.4-* follow the pattern '>=X.Y.Z-0 <X.Y.(Z+1)-0'
		if !tilde && start.prereleaseIsZero() &&
			start.Major == end.Major && start.Minor == end.Minor && start.Patch.isSucc(end.Patch) {
			return start.Major.String() + "." + start.Minor.String() + "." + start.Patch.String() + "-*", true
		}
		// Tilde ranges like ~1.2.3, and ~1.2.3-rc
		if start.Major == end.Major && start.Minor.isSucc(end.Minor) && end.Patch.IsZero() {
			if extraPrerelease {
				return "*-* ~" + start.String(), true
			}
			return "~" + start.String(), true
		}
		// Note: caret ranges like ^0.1.2 and ^0.2.3-rc are converted to tilde ranges
		if !start.Major.IsZero() {
			// Caret ranges like ^1.2.3 and ^1.2.3-rc
			if start.Major.isSucc(end.Major) && end.Minor.IsZero() && end.Patch.IsZero() {
				if extraPrerelease {
					return "*-* ^" + start.String(), true
				}
				return "^" + start.String(), true
			}
		} else if end.Major.IsZero() && start.Minor.IsZero() && end.Minor.IsZero() && start.Patch.isSucc(end.Patch) {
			// Caret ranges like ^0.0.2 and ^0.0.2-rc
			if extraPrerelease {
				return "*-* ^" + start.String(), true
			}
			return "^" + start.String(), true
		}
	}

	// Wildcards with prerelease follow the pattern >=X.Y.Z-φ.α.0 <X.Y.Z-φ.β
	if r.left.inclusive && !r.right.inclusive && r.left.v != nil && r.right.v != nil &&
		r.left.v.majorMinorPatchEquals(*r.right.v) &&
		r.left.v.IsPrerelease() && r.right.v.IsPrerelease() {
		lp, rp := r.left.v.Prerelease, r.right.v.Prerelease
		if len(lp) < 2 || lp[len(lp)-1] != "0" || len(lp)-1 != len(rp) {
			return "", false
		}
		for i := 0; i < len(lp)-2; i++ {
			if lp[i] != rp[i] {
				return "", false
			}
		}
		if nextIdentifier(lp[len(lp)-2]) != rp[len(rp)-1] {
			return "", false
		}
		s := *r.left.v
		return s.Major.String() + "." + s.Minor.String() + "." + s.Patch.String() + "-" +
			strings.Join(lp[:len(lp)-1], ".") + ".*", true
	}
	return "", false
}

func (r unbrokenRange) overlaps(o unbrokenRange) bool {
	if r.isEmpty() || o.isEmpty() {
		return false
	}
	return leftCompareRight(r.left, o.right) <= 0 && leftCompareRight(o.left, r.right) <= 0
}

func (r unbrokenRange) overlapsOrAbuts(o unbrokenRange) bool {
	if r.overlaps(o) {
		return true
	}
	if r.isEmpty() || o.isEmpty() {
		return false
	}
	lessOrEqual := compareUnbroken(r, o) <= 0
	leftEnd, rightStart := o.right, r.left
	if lessOrEqual {
		leftEnd, rightStart = r.right, o.left
	}
	if (leftEnd.inclusive || rightStart.inclusive) &&
		leftEnd.v != nil && rightStart.v != nil && leftEnd.v.equal(*rightStart.v) {
		return true
	}
	if r.includeAllPrerelease || o.includeAllPrerelease ||
		rightIncludesPrerelease(leftEnd) || leftIncludesPrerelease(rightStart) {
		return false
	}
	return rightStart.inclusive && leftEnd.v != nil && rightStart.v != nil &&
		leftEnd.v.majorMinorPatchEquals(*rightStart.v)
}

func (r unbrokenRange) containsRange(o unbrokenRange) bool {
	if o.isEmpty() {
		return true
	}
	if o.includeAllPrerelease && !r.includeAllPrerelease {
		return false
	}
	if leftCompare(r.left, o.left) > 0 || rightCompare(o.right, r.right) > 0 {
		return false
	}
	if r.includeAllPrerelease {
		return true
	}
	if leftIncludesPrerelease(o.left) {
		if r.left.v == nil || !r.left.v.IsPrerelease() || !r.left.v.majorMinorPatchEquals(*o.left.v) {
			return false
		}
	}
	return !rightIncludesPrerelease(o.right) ||
		(r.right.v != nil && r.right.v.IsPrerelease() && r.right.v.majorMinorPatchEquals(*o.right.v))
}

func (r unbrokenRange) tryUnion(o unbrokenRange) (unbrokenRange, bool) {
	if r.containsRange(o) {
		return r, true
	}
	if o.containsRange(r) {
		return o, true
	}
	if r.includeAllPrerelease != o.includeAllPrerelease {
		return unbrokenRange{}, false
	}
	if !r.overlapsOrAbuts(o) {
		return unbrokenRange{}, false
	}
	include := r.includeAllPrerelease
	union := newUnbroken(leftMin(r.left, o.left), rightMax(r.right, o.right), include)
	if !include {
		otherLeft := leftMax(r.left, o.left)
		if leftIncludesPrerelease(otherLeft) && !union.contains(*otherLeft.v) {
			return unbrokenRange{}, false
		}
		otherRight := rightMin(r.right, o.right)
		if rightIncludesPrerelease(otherRight) && !union.contains(*otherRight.v) {
			return unbrokenRange{}, false
		}
	}
	return union, true
}

// compareUnbroken is UnbrokenSemVersionRangeComparer.
func compareUnbroken(a, b unbrokenRange) int {
	if c := leftCompare(a.left, b.left); c != 0 {
		return c
	}
	if c := -rightCompare(a.right, b.right); c != 0 {
		return c
	}
	return -cmpBool(a.includeAllPrerelease, b.includeAllPrerelease)
}

// normalizeRanges is the body of SemVersionRange.Create(List<...>).
func normalizeRanges(in []unbrokenRange) []unbrokenRange {
	ranges := make([]unbrokenRange, 0, len(in))
	for _, r := range in {
		if !r.isEmpty() {
			ranges = append(ranges, r)
		}
	}
	if len(ranges) == 0 {
		return nil
	}
	slices.SortStableFunc(ranges, compareUnbroken)
	for i := 0; i < len(ranges)-1; i++ {
		for j := len(ranges) - 1; j > i; j-- {
			if union, ok := ranges[i].tryUnion(ranges[j]); ok {
				ranges = slices.Delete(ranges, j, j+1)
				ranges[i] = union
			}
		}
	}
	return ranges
}

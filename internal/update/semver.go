// Package update is Headliner's updater (ONLINE.md §A6): it reads the
// GitHub releases, picks the newest one for the player's channel,
// downloads and verifies it, and swaps it in with a way back.
package update

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a semver tag: v1.2.0 (stable) or v1.3.0-beta.1 (beta).
type Version struct {
	Major, Minor, Patch int
	Pre                 []string // "beta", "1"; empty on a stable release
}

// ParseVersion reads "v1.2.3" or "1.2.3-beta.4"; build metadata after +
// is ignored.
func ParseVersion(s string) (Version, error) {
	var v Version
	t := strings.TrimPrefix(strings.TrimSpace(s), "v")
	t, _, _ = strings.Cut(t, "+")
	core, pre, hasPre := strings.Cut(t, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("version %q: want major.minor.patch", s)
	}
	nums := [3]*int{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, fmt.Errorf("version %q: %q is not a number", s, p)
		}
		*nums[i] = n
	}
	if hasPre {
		if pre == "" {
			return v, fmt.Errorf("version %q: empty pre-release", s)
		}
		v.Pre = strings.Split(pre, ".")
	}
	return v, nil
}

// Stable reports a version without a pre-release part.
func (v Version) Stable() bool { return len(v.Pre) == 0 }

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if len(v.Pre) > 0 {
		s += "-" + strings.Join(v.Pre, ".")
	}
	return s
}

// Compare is -1, 0 or 1 as v is older than, the same as or newer than w,
// by semver precedence: 1.3.0-beta.1 < 1.3.0-beta.2 < 1.3.0-beta.10 <
// 1.3.0.
func (v Version) Compare(w Version) int {
	for _, d := range [3][2]int{{v.Major, w.Major}, {v.Minor, w.Minor}, {v.Patch, w.Patch}} {
		if d[0] != d[1] {
			return sign(d[0] - d[1])
		}
	}
	switch {
	case len(v.Pre) == 0 && len(w.Pre) == 0:
		return 0
	case len(v.Pre) == 0:
		return 1
	case len(w.Pre) == 0:
		return -1
	}
	for i := 0; i < len(v.Pre) && i < len(w.Pre); i++ {
		a, b := v.Pre[i], w.Pre[i]
		an, aErr := strconv.Atoi(a)
		bn, bErr := strconv.Atoi(b)
		switch {
		case aErr == nil && bErr == nil:
			if an != bn {
				return sign(an - bn)
			}
		case aErr == nil: // numbers sort before words
			return -1
		case bErr == nil:
			return 1
		case a != b:
			return sign(strings.Compare(a, b))
		}
	}
	return sign(len(v.Pre) - len(w.Pre))
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

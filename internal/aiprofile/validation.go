package aiprofile

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func ValidateAlias(alias string) error {
	if !aliasPattern.MatchString(alias) {
		return fmt.Errorf("invalid profile name %q: use only letters, numbers, _ or -", alias)
	}
	return nil
}

func FindProfile(data StoreData, tool, alias string) (Profile, bool) {
	for _, p := range data.Profiles {
		if p.Tool == tool && p.Alias == alias {
			return p, true
		}
	}
	return Profile{}, false
}

func ProfilesForTool(data StoreData, tool string) []Profile {
	out := make([]Profile, 0)
	for _, p := range data.Profiles {
		if p.Tool == tool {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out
}

func AvailableAliases(data StoreData, tool string) string {
	profiles := ProfilesForTool(data, tool)
	if len(profiles) == 0 {
		return "none"
	}
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Alias)
	}
	return strings.Join(names, ", ")
}

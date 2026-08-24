package benchmark

import (
	"fmt"
	"sort"
	"time"
)

// RunSuite executes scheduler benchmarks for one profile or all profiles.
func RunSuite(profileName string, seed int64) (*SuiteResult, error) {
	profiles := predefinedProfiles()
	selected := make([]WorkloadProfile, 0)

	if profileName == "" || profileName == ProfileAll {
		names := make([]string, 0, len(profiles))
		for name := range profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			selected = append(selected, profiles[name])
		}
	} else {
		profile, exists := profiles[profileName]
		if !exists {
			return nil, fmt.Errorf("unknown benchmark profile '%s'", profileName)
		}
		selected = append(selected, profile)
	}

	suite := &SuiteResult{
		GeneratedAt:      time.Now(),
		Seed:             seed,
		RequestedProfile: profileName,
		Profiles:         make([]ProfileResult, 0, len(selected)),
	}

	for idx, profile := range selected {
		profileSeed := seed + int64(idx*997)
		result, err := runProfile(profile, profileSeed)
		if err != nil {
			return nil, fmt.Errorf("profile %s: %w", profile.Name, err)
		}
		suite.Profiles = append(suite.Profiles, result)
	}

	return suite, nil
}

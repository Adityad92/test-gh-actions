package utility

import "fmt"

func Validate() (string, bool, error) {
	// Run the map-based validation
	err := ValidateFiles(
		[]string{"cdp_vpc.tf"},
		[]string{"minerva_vpc_ids", "migration_vpc_ids"},
	)
	if err != nil {
		return "", true, fmt.Errorf("ValidateFiles failed: %w", err)
	}

	// Run the CIDR overlap validation
	paths := []string{
		"./lab/global",
		"./lab/us-east-1",
		"./prod/eu-west-1",
		"./prod/global",
		"./prod/us-east-1",
	}
	return ValidateCidrOverlapsFromPaths(paths)
}

package utility

func Validate() (string, bool, error) {

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

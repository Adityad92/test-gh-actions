package utility

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createTestFS is a helper function to create a temporary directory structure
// with specified file contents for testing. It returns the root path of the
// temporary directory.
func createTestFS(t *testing.T, files map[string]string) string {
	t.Helper()
	rootDir := t.TempDir()

	for path, content := range files {
		fullPath := filepath.Join(rootDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}
	return rootDir
}

func TestValidateCidrOverlapsFromPaths(t *testing.T) {
	tests := []struct {
		name               string
		files              map[string]string
		pathsToValidate    []string
		wantIsFailure      bool
		wantErr            bool
		wantReportContains []string
	}{
		{
			name: "Single Path - No Overlaps",
			files: map[string]string{
				"projectA/main.tf": `
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}
resource "aws_vpc" "secondary" {
    cidr_block = "192.168.0.0/16"
}
`,
			},
			pathsToValidate: []string{"projectA"},
			wantIsFailure:   false,
			wantErr:         false,
			wantReportContains: []string{
				"Report for Path: projectA",
				"VPC CIDR Validation: Success",
				"No overlapping CIDRs were found",
				"10.0.0.0/16",
				"192.168.0.0/16",
			},
		},
		{
			name: "Single Path - With Overlap",
			files: map[string]string{
				"projectB/main.tf": `
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}
`,
				"projectB/secondary_vpc.tf": `
resource "aws_vpc_ipv4_cidr_block_association" "secondary" {
  vpc_id     = aws_vpc.main.id
  cidr_block = "10.0.1.0/24" # This overlaps with the main VPC
}
`,
			},
			pathsToValidate: []string{"projectB"},
			wantIsFailure:   true,
			wantErr:         false,
			wantReportContains: []string{
				"Report for Path: projectB",
				"VPC CIDR Validation: FAILURE",
				"Found 1 overlapping CIDR pair(s)",
				"Resource: `aws_vpc.main`",
				"CIDR: `10.0.0.0/16`",
				"Resource: `aws_vpc_ipv4_cidr_block_association.secondary`",
				"CIDR: `10.0.1.0/24`",
			},
		},
		{
			name: "Multiple Paths - Mixed Success and Failure",
			files: map[string]string{
				// Path with overlap
				"env-prod/main.tf": `
resource "aws_vpc" "main" { cidr_block = "10.10.0.0/16" }
`,
				"env-prod/peering.tf": `
resource "aws_vpc" "peering" { cidr_block = "10.10.5.0/24" }
`,
				// Path with no overlap
				"env-staging/main.tf": `
resource "aws_vpc" "main" { cidr_block = "172.16.0.0/16" }
resource "aws_vpc" "logging" { cidr_block = "172.17.0.0/16" }
`,
				// Path with no relevant TF files
				"utils/main.tf": `
resource "aws_s3_bucket" "mybucket" { bucket = "my-test-bucket" }
`,
			},
			pathsToValidate: []string{"env-prod", "env-staging", "utils"},
			wantIsFailure:   true, // Overall failure is true because one path failed
			wantErr:         false,
			wantReportContains: []string{
				// Check for prod failure report
				"Report for Path: env-prod",
				"VPC CIDR Validation: FAILURE",
				"10.10.0.0/16",
				"10.10.5.0/24",
				// Check for staging success report
				"Report for Path: env-staging",
				"VPC CIDR Validation: Success",
				"172.16.0.0/16",
				"172.17.0.0/16",
				// Check for utils report
				"Report for Path: utils",
				"No evaluatable VPC CIDR definitions",
			},
		},
		{
			name:            "Path with no TF files",
			files:           map[string]string{"empty-dir/readme.md": "This is not a terraform file"},
			pathsToValidate: []string{"empty-dir"},
			wantIsFailure:   false,
			wantErr:         false,
			wantReportContains: []string{
				"Report for Path: empty-dir",
				"No .tf files found in this path.",
			},
		},
		{
			name:               "Non-Existent Path",
			files:              map[string]string{}, // No files needed
			pathsToValidate:    []string{"non-existent-dir"},
			wantIsFailure:      true, // Function returns failure for processing errors
			wantErr:            true,
			wantReportContains: []string{""}, // Report content doesn't matter, just the error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the temporary file system for the test case
			rootDir := createTestFS(t, tt.files)

			// Construct the full paths to validate relative to the temp directory
			var fullPaths []string
			for _, p := range tt.pathsToValidate {
				fullPaths = append(fullPaths, filepath.Join(rootDir, p))
			}

			// For the non-existent path test, we use the name directly
			if tt.name == "Non-Existent Path" {
				fullPaths = []string{"non-existent-dir"}
			}

			// Run the function under test
			gotReport, gotIsFailure, err := ValidateCidrOverlapsFromPaths(fullPaths)

			// Validate the error
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCidrOverlapsFromPaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Validate the failure status
			if gotIsFailure != tt.wantIsFailure {
				t.Errorf("ValidateCidrOverlapsFromPaths() gotIsFailure = %v, want %v", gotIsFailure, tt.wantIsFailure)
			}

			// Validate the report content
			for _, substr := range tt.wantReportContains {
				if substr == "" { // Skip empty checks, useful for error tests
					continue
				}
				if !strings.Contains(gotReport, substr) {
					t.Errorf("ValidateCidrOverlapsFromPaths() report did not contain expected substring %q.\n\nFull Report:\n%s", substr, gotReport)
				}
			}
		})
	}
}

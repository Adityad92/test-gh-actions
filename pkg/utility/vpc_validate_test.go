package utility

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate(t *testing.T) {
	// Save original directory to restore it later
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "vpc_validate_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create temporary prod directory directly in the tmpDir (simulating repo root)
	tmpProdDir := filepath.Join(tmpDir, "prod", "us-east-1")

	if err := os.MkdirAll(tmpProdDir, 0755); err != nil {
		t.Fatalf("Failed to create temp prod dir: %v", err)
	}

	// Move to the temporary directory which simulates the repository root
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{
			name: "All VPCs are included",
			content: `
module "vpc_one" {
  source = "terraform-aws-modules/vpc/aws"
  version = "3.0.0"
}

module "vpc_two" {
  source = "terraform-aws-modules/vpc/aws"
  version = "3.0.0"
}

locals {
  minerva_vpc_ids = {
	vpc_one = "vpc-123456"
	vpc_two = "vpc-789012"
  }
}`,
			wantError: false,
		},
		{
			name: "Missing VPC entry",
			content: `
module "vpc_one" {
  source = "terraform-aws-modules/vpc/aws"
  version = "3.0.0"
}

module "vpc_two" {
  source = "terraform-aws-modules/vpc/aws"
  version = "3.0.0"
}

locals {
  minerva_vpc_ids = {
	vpc_one = "vpc-123456"
	# vpc_two is missing
  }
}`,
			wantError: true,
		},
		{
			name: "No VPC modules",
			content: `
module "something_else" {
  source = "some/other/module"
}

locals {
  minerva_vpc_ids = {}
}`,
			wantError: false,
		},
		{
			name: "Complex nested structure",
			content: `
module "vpc_complex" {
  source = "terraform-aws-modules/vpc/aws"
}

module "not_vpc" {
  source = "other/module"
}

locals {
  # Complex nested structure
  some_var = {
	nested = true
  }
  
  minerva_vpc_ids = {
	vpc_complex = "vpc-complex"
	extra_vpc = "vpc-extra" # This is fine, we only care about missing ones
  }
  
  other_map = {
	something = "else"
  }
}`,
			wantError: false,
		},
		{
			name: "Missing minerva_vpc_ids section",
			content: `
module "vpc_missing" {
  source = "terraform-aws-modules/vpc/aws"
}

locals {
  # No minerva_vpc_ids section
  other_var = "value"
}`,
			wantError: true,
		},
		{
			name: "Multiple VPC modules with same name",
			content: `
module "vpc_dupe" {
  source = "terraform-aws-modules/vpc/aws"
}

module "vpc_dupe" {
  source = "terraform-aws-modules/vpc/aws"
}

locals {
  minerva_vpc_ids = {
	vpc_dupe = "vpc-dupe"
  }
}`,
			wantError: false,
		},
		{
			name: "Multiline module definition",
			content: `
module "vpc_multiline" 
{
  source = 
	"terraform-aws-modules/vpc/aws"
}

locals {
  minerva_vpc_ids = {
	vpc_multiline = "vpc-multi"
  }
}`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(tmpProdDir, "cdp_vpc.tf")
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Run the validation
			err := ValidateTest([]string{"cdp_vpc.tf"})

			// Check results
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateTest() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

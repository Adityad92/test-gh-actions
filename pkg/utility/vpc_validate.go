package utility

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Added to support test cases
func ValidateTest(files []string) error {
	return ValidateFiles(files, []string{"minerva_vpc_ids", "migration_vpc_ids"})
}

// ValidateFiles checks if all VPC modules in the specified files are included
// in the specified maps
func ValidateFiles(tfFiles []string, mapNames []string) error {
	tfDir := "./prod/us-east-1"
	modules := make(map[string]bool)
	vpcIDs := make(map[string]bool)

	moduleRegex := regexp.MustCompile(`module\s+"([a-zA-Z0-9_-]+)"\s*{`)
	sourceRegex := regexp.MustCompile(`source\s*=\s*"(terraform-aws-modules/vpc/aws|(\.\./)+modules/shared_vpc)"`)
	vpcIDsKeyRegex := regexp.MustCompile(`\s+([a-zA-Z0-9_-]+)\s*=`)

	// Process each file
	for _, tfFile := range tfFiles {
		filePath := filepath.Join(tfDir, tfFile)
		fmt.Println("Scanning file:", filePath)

		// Open the file
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("error opening file %s: +%v", filePath, err)
		}
		defer file.Close()

		// Scan the file
		scanner := bufio.NewScanner(file)
		var currentModule string
		var insideMap bool = false
		var currentMap string = ""
		var braceCount int = 0

		for scanner.Scan() {
			line := scanner.Text()

			// Process module declarations
			if matches := moduleRegex.FindStringSubmatch(line); len(matches) > 1 {
				currentModule = matches[1]
			}

			// Check if the current module uses the VPC module
			if currentModule != "" && sourceRegex.MatchString(line) {
				modules[currentModule] = true
				fmt.Printf("Module %s uses terraform-aws-modules/vpc/aws\n", currentModule)
				currentModule = ""
			}

			// Check if we're entering any of the specified maps
			if !insideMap {
				for _, mapName := range mapNames {
					if strings.Contains(line, mapName) && strings.Contains(line, "{") {
						insideMap = true
						currentMap = mapName
						braceCount = strings.Count(line, "{")
						braceCount -= strings.Count(line, "}")
						break
					}
				}
			} else {
				// Process content inside the map section
				braceCount += strings.Count(line, "{")
				braceCount -= strings.Count(line, "}")

				// Exit the section when braces are balanced
				if braceCount <= 0 {
					insideMap = false
					continue
				}

				// Extract VPC ID variable names
				if matches := vpcIDsKeyRegex.FindStringSubmatch(line); len(matches) > 1 {
					vpcIDs[matches[1]] = true
					fmt.Printf("Found VPC ID variable in %s: %s\n", currentMap, matches[1])
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading file %s: +%v", filePath, err)
		}
	}

	fmt.Println("🔍 Checking for missing VPC entries in specified maps...")
	missing := false
	for mod := range modules {
		if _, exists := vpcIDs[mod]; !exists {
			fmt.Printf("❌ Missing entry for module: %s\n", mod)
			missing = true
		}
	}

	if !missing {
		fmt.Println("✅ All VPC modules are included in maps.")
	} else {
		return fmt.Errorf("Please include all vpc_ids to the local variable: minerva_vpc_ids")
	}

	return nil
}

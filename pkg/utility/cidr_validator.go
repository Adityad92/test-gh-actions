package utility

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// Internal data structures
type cidrInfo struct {
	CIDR, ResourceType, ResourceName, FilePath string
}
type overlapPair struct{ infoA, infoB cidrInfo }

var (
	targetResourceTypes = map[string]bool{
		"aws_vpc":                             true,
		"aws_vpc_ipv4_cidr_block_association": true,
	}
	targetAttributeNames = map[string]bool{
		"cidr_block":            true,
		"secondary_cidr_blocks": true,
		"vpc_cidr":              true,
		"cidr":                  true,
	}
)

func ValidateCidrOverlapsFromPaths(paths []string) (report string, isFailure bool, err error) {
	if len(paths) == 0 {
		return "# VPC CIDR Validation: Success\n\nNo paths provided for validation.\n", false, nil
	}

	var finalReportBuilder strings.Builder
	var overallFailure bool

	for _, root := range paths {
		// Add a header for each path in the final report
		finalReportBuilder.WriteString(fmt.Sprintf("--- Validation Report for Path: %s ---\n\n", root))

		var tfFilesInPath []string
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".tf") {
				tfFilesInPath = append(tfFilesInPath, path)
			}
			return nil
		})
		if walkErr != nil {
			return "", true, fmt.Errorf("error walking path %s: %w", root, walkErr)
		}

		if len(tfFilesInPath) == 0 {
			finalReportBuilder.WriteString("No .tf files found in this path.\n\n")
			continue // Move to the next path
		}

		infos, err := extractCidrsFromFiles(tfFilesInPath)
		if err != nil {
			// This is a processing error, not a validation failure, so we return it.
			return "", true, fmt.Errorf("error extracting CIDRs from path %s: %w", root, err)
		}

		if len(infos) == 0 {
			pathReport, _ := generateSuccessReport(nil)
			finalReportBuilder.WriteString(pathReport)
			finalReportBuilder.WriteString("\n")
			continue // Move to the next path
		}

		overlappingPairs := checkForOverlaps(infos)
		if len(overlappingPairs) > 0 {
			overallFailure = true // Mark the overall result as a failure
			pathReport, _ := generateFailureReport(overlappingPairs)
			finalReportBuilder.WriteString(pathReport)
		} else {
			pathReport, _ := generateSuccessReport(infos)
			finalReportBuilder.WriteString(pathReport)
		}
		finalReportBuilder.WriteString("\n")
	}

	return finalReportBuilder.String(), overallFailure, nil
}

// --- INTERNAL HELPERS ---
// It is the most critical piece.
func extractCidrsFromBlock(body hcl.Body) []string {
	var results []string
	content, _, _ := body.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "vpc_cidr"}, {Name: "cidr"}, {Name: "cidr_block"}, {Name: "secondary_cidr_blocks"},
		},
	})
	for _, attr := range content.Attributes {
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			continue
		}
		if val.Type() == cty.String {
			results = append(results, val.AsString())
		} else if val.Type().IsListType() || val.Type().IsTupleType() {
			for it := val.ElementIterator(); it.Next(); {
				_, elemVal := it.Element()
				if elemVal.Type() == cty.String {
					results = append(results, elemVal.AsString())
				}
			}
		}
	}
	return results
}

func extractCidrsFromFiles(paths []string) ([]cidrInfo, error) {
	var foundCidrInfos []cidrInfo
	parser := hclparse.NewParser()
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "module", LabelNames: []string{"name"}},
			{Type: "resource", LabelNames: []string{"type", "name"}},
		},
	}

	for _, path := range paths {
		file, diags := parser.ParseHCLFile(path)
		if diags.HasErrors() {
			continue
		}

		content, _, _ := file.Body.PartialContent(schema)

		for _, block := range content.Blocks {
			var rType, rName string
			var processBlock bool

			if block.Type == "module" {
				rType, rName = "module", block.Labels[0]
				processBlock = true
			} else if block.Type == "resource" && targetResourceTypes[block.Labels[0]] {
				rType, rName = block.Labels[0], block.Labels[1]
				processBlock = true
			}

			if processBlock {
				cidrs := extractCidrsFromBlock(block.Body)
				for _, cidr := range cidrs {
					foundCidrInfos = append(foundCidrInfos, cidrInfo{
						CIDR: cidr, ResourceType: rType, ResourceName: rName, FilePath: path,
					})
				}
			}
		}
	}

	sort.Slice(foundCidrInfos, func(i, j int) bool { return foundCidrInfos[i].CIDR < foundCidrInfos[j].CIDR })
	return foundCidrInfos, nil
}

func checkForOverlaps(infos []cidrInfo) []overlapPair {
	var networks []*net.IPNet
	var originalInfos []*cidrInfo

	// Build a parallel slice of networks and a slice of pointers to the original info
	for i := range infos {
		_, network, err := net.ParseCIDR(infos[i].CIDR)
		if err == nil {
			networks = append(networks, network)
			originalInfos = append(originalInfos, &infos[i])
		}
	}

	var overlappingPairs []overlapPair
	for i := 0; i < len(networks); i++ {
		for j := i + 1; j < len(networks); j++ {
			if networks[i].Contains(networks[j].IP) || networks[j].Contains(networks[i].IP) {
				overlappingPairs = append(overlappingPairs, overlapPair{
					infoA: *originalInfos[i],
					infoB: *originalInfos[j],
				})
			}
		}
	}
	return overlappingPairs
}

func generateSuccessReport(infos []cidrInfo) (string, []byte) {
	var md bytes.Buffer
	md.WriteString("# VPC CIDR Validation: Success\n\n")
	if len(infos) == 0 {
		md.WriteString("No evaluatable VPC CIDR definitions were found in target resources or modules.\n")
	} else {
		fmt.Fprintf(&md, "Validation complete. No overlapping CIDRs were found.\n\n**Total CIDRs Found:** %d\n\n", len(infos))
		md.WriteString("| Resource Type | Resource Name | CIDR Block | File Path |\n|---|---|---|---|\n")
		for _, info := range infos {
			fmt.Fprintf(&md, "| `%s` | `%s` | `%s` | `%s` |\n", info.ResourceType, info.ResourceName, info.CIDR, info.FilePath)
		}
	}
	return md.String(), md.Bytes()
}

func generateFailureReport(pairs []overlapPair) (string, []byte) {
	var md bytes.Buffer
	fmt.Fprintf(&md, "# VPC CIDR Validation: FAILURE\n\nValidation failed. Found **%d** overlapping CIDR pair(s).\n\n", len(pairs))
	for i, pair := range pairs {
		fmt.Fprintf(&md, "## Overlap Pair %d\n\n", i+1)
		writeResourceInfo(&md, "Resource 1", pair.infoA)
		writeResourceInfo(&md, "Resource 2", pair.infoB)
		md.WriteString("---\n\n")
	}
	return md.String(), md.Bytes()
}

func writeResourceInfo(w io.Writer, title string, info cidrInfo) {
	fmt.Fprintf(w, "**%s**:\n- **Resource:** `%s.%s`\n- **CIDR:** `%s`\n- **File:** `%s`\n\n", title, info.ResourceType, info.ResourceName, info.CIDR, info.FilePath)
}

#!/bin/bash
# Script to generate/validate Terraform provider documentation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Terraform Provider Documentation Generator${NC}"
echo "==========================================="
echo ""

# Check if tfplugindocs is installed
if ! command -v tfplugindocs &> /dev/null; then
    echo -e "${YELLOW}tfplugindocs is not installed.${NC}"
    echo "Installing tfplugindocs..."
    go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest

    if ! command -v tfplugindocs &> /dev/null; then
        echo -e "${RED}Failed to install tfplugindocs. Please ensure Go is installed and GOPATH/bin is in your PATH.${NC}"
        exit 1
    fi
fi

# Validate existing documentation
echo "Validating documentation..."
if tfplugindocs validate; then
    echo -e "${GREEN}✓ Documentation validation passed!${NC}"
else
    echo -e "${RED}✗ Documentation validation failed!${NC}"
    exit 1
fi

# Optionally generate documentation (uncomment if you want to auto-generate)
# Note: This will regenerate docs from code comments
# echo ""
# echo "Generating documentation from code..."
# tfplugindocs generate

echo ""
echo -e "${GREEN}Documentation is ready for publishing!${NC}"
echo ""
echo "Documentation location: ./docs/"
echo "  - Provider: docs/index.md"
echo "  - Resources: docs/resources/"
echo "  - Data Sources: docs/data-sources/"

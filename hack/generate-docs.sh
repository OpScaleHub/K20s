#!/bin/bash
set -e

echo "Generating documentation..."
mkdir -p docs-src

# Generate Makefile help
echo "# Makefile Commands" > docs-src/make-help.md
echo "" >> docs-src/make-help.md
echo '
```
' >> docs-src/make-help.md
# Use awk to format the help output, removing color codes
make --no-print-directory help | awk 'BEGIN {FS = ":.*##"; state = "start"} /^[a-zA-Z_0-9-]+:.*?##/ { if (state == "category") print ""; printf "### %s\n%s\n", $$1, $$2; state = "target" } /^##@/ { if (state == "target") print ""; printf "\n## %s\n", substr($$0, 5); state = "category"} END {print ""}' >> docs-src/make-help.md
echo '
```
' >> docs-src/make-help.md

# Generate API reference
echo "# API Reference" > docs-src/api-reference.md
echo "" >> docs-src/api-reference.md
echo "The
ResourceOptimizerProfile
 CRD is defined by the following YAML:" >> docs-src/api-reference.md
echo "" >> docs-src/api-reference.md
echo '
```yaml
' >> docs-src/api-reference.md
cat config/crd/bases/optimizer.k20s.opscale.ir_resourceoptimizerprofiles.yaml >> docs-src/api-reference.md
echo '
```
' >> docs-src/api-reference.md

echo "Done."

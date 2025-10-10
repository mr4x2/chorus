#!/bin/bash
# Export YAML configurations to JSON format for migration

set -e

CONFIG_FILE=${1:-"docker-compose/worker-conf.yaml"}
OUTPUT_FILE=${2:-"yaml-configs-backup.json"}

echo "Exporting YAML configurations from $CONFIG_FILE to $OUTPUT_FILE..."

# Check if yq is installed
if ! command -v yq &> /dev/null; then
    echo "Error: yq is required but not installed. Please install yq first."
    echo "Installation: https://github.com/mikefarah/yq#install"
    exit 1
fi

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed. Please install jq first."
    echo "Installation: https://stedolan.github.io/jq/download/"
    exit 1
fi

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Config file $CONFIG_FILE not found"
    exit 1
fi

# Extract storage configurations
echo "Extracting storage configurations..."
STORAGES=$(yq eval '.storage.storages' "$CONFIG_FILE" 2>/dev/null || echo "{}")

# Extract any existing replication jobs (if any)
echo "Extracting replication jobs..."
JOBS=$(yq eval '.replication.jobs // {}' "$CONFIG_FILE" 2>/dev/null || echo "{}")

# Create backup JSON
echo "Creating backup JSON..."
jq -n \
  --argjson storages "$STORAGES" \
  --argjson jobs "$JOBS" \
  --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg source "$CONFIG_FILE" \
  '{
    metadata: {
      timestamp: $timestamp,
      source: $source,
      version: "1.0"
    },
    storages: $storages,
    jobs: $jobs
  }' > "$OUTPUT_FILE"

echo "Export completed successfully!"
echo "Backup file: $OUTPUT_FILE"
echo "Storages found: $(echo "$STORAGES" | jq 'keys | length')"
echo "Jobs found: $(echo "$JOBS" | jq 'keys | length')"

# Display summary
echo ""
echo "Summary:"
echo "--------"
if [ "$(echo "$STORAGES" | jq 'keys | length')" -gt 0 ]; then
    echo "Storages:"
    echo "$STORAGES" | jq -r 'keys[]' | sed 's/^/  - /'
else
    echo "No storages found in configuration"
fi

if [ "$(echo "$JOBS" | jq 'keys | length')" -gt 0 ]; then
    echo "Jobs:"
    echo "$JOBS" | jq -r 'keys[]' | sed 's/^/  - /'
else
    echo "No jobs found in configuration"
fi

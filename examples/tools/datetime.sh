#!/bin/bash

set -e

# Check if --schema argument is provided
if [[ "$1" == "--schema" ]]; then
  # Output a JSON schema to describe the tool
  # shellcheck disable=SC2016
  cat <<EOF
{
  "schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "format": {
      "type": "string",
      "description": "The format for the date output (e.g., +%Y-%m-%d %H:%M:%S)"
    }
  },
  "required": ["format"],
  "additionalProperties": false
}
EOF
  exit 0
fi

if [[ "$1" == "--name" ]]; then
  echo "get_current_date_with_format"
  exit 0
fi

if [[ "$1" == "--description" ]]; then
  echo "provides the current date in the specified unix date command format"
  exit 0
fi

if [[ "$1" == "--execute" ]]; then
  # Ensure jq is available
  if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed." >&2
    exit 1
  fi

  # Extract format field from JSON
  format=$(jq -r '.format' <<< "$2")

  # Sanitize the format string
  if [[ "$format" =~ [^a-zA-Z0-9%+:/\ \-] ]]; then
    echo "Error: Invalid characters in format string." >&2
    exit 1
  fi

  # Use -- to prevent option parsing
  date_output=$(date -- "$format")
  echo "$date_output"
  exit 0
fi


# if no arguments, show usage
# shellcheck disable=SC2140
echo "Usage: currentdate.sh [--schema | --name | --description | --execute '{"format": "+%Y-%m-%d %H:%M:%S"}']"


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

  # extract format field from JSON
  format=$(jq -r '.format' <<< "$2")

  # Use exec to prevent command injection
  exec date "$format"
  exit 0
fi

# if no arguments, show usage
echo "Usage: currentdate.sh [--schema | --name | --description | --execute <format>]"


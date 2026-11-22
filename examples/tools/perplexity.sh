#!/bin/bash
#
# Web search through Perplexity's API: one question in, a short sourced
# answer out. Needs curl and jq.
#
#   PERPLEXITY_API_KEY   required
#   PERPLEXITY_MODEL     default: sonar
#   PERPLEXITY_TIMEOUT   seconds, default: 120

set -e

if [[ "$1" == "--schema" ]]; then
  cat <<'SCHEMA'
{
  "title": "web_search",
  "description": "search the web and get a short answer with its sources. use it for news, current events, prices, scores, and any fact that may have changed since your training data.",
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "the question to answer, in plain language"
    }
  },
  "required": ["query"],
  "additionalProperties": false
}
SCHEMA
  exit 0
fi

if [[ "$1" == "--execute" ]]; then
  for dependency in curl jq; do
    if ! command -v "$dependency" &> /dev/null; then
      echo "Error: $dependency is not installed." >&2
      exit 1
    fi
  done
  if [[ -z "$PERPLEXITY_API_KEY" ]]; then
    echo "Error: PERPLEXITY_API_KEY is not set." >&2
    exit 1
  fi

  query=$(jq -r '.query // empty' <<< "$2")
  if [[ -z "$query" ]]; then
    echo "Error: query is required." >&2
    exit 1
  fi

  body=$(jq -n --arg model "${PERPLEXITY_MODEL:-sonar}" --arg query "$query" \
    '{model: $model, messages: [{role: "user", content: $query}]}')

  # the key goes in through a config on stdin, not the command line, so it
  # never shows in the process list
  status=0
  response=$(curl --silent --show-error --max-time "${PERPLEXITY_TIMEOUT:-120}" \
    --config - --data "$body" https://api.perplexity.ai/chat/completions <<CONFIG
header = "Authorization: Bearer $PERPLEXITY_API_KEY"
header = "Content-Type: application/json"
CONFIG
  ) || status=$?
  if [[ $status -ne 0 ]]; then
    echo "Error: perplexity request failed (curl exit $status)." >&2
    exit 1
  fi

  if ! jq -e '.choices[0].message.content' <<< "$response" > /dev/null 2>&1; then
    detail=$(jq -r '.error.message // .detail // empty' <<< "$response" 2> /dev/null || true)
    echo "Error: perplexity returned no answer. ${detail:0:300}" >&2
    exit 1
  fi

  # reasoning models put their working in a <think> block ahead of the answer
  jq -r '
    (.choices[0].message.content | sub("<think>[\\s\\S]*?</think>\\s*"; "")),
    (((.citations // (.search_results // [] | map(.url))) // []) as $sources
      | if ($sources | length) > 0 then "\nsources:\n" + ($sources | join("\n")) else empty end)
  ' <<< "$response"
  exit 0
fi

echo "Usage: $0 [--schema | --execute '{\"query\": \"...\"}']" >&2
exit 1

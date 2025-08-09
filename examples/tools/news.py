#!/usr/bin/env python3

import sys
import json
import requests
import urllib3
urllib3.disable_warnings()

def print_schema():
    schema = {
        "title": "get_news_summary",
        "description": "provides a short summary of recent news",
        "type": "object",
        "properties": {
            "user": {
                "type": "string",
                "description": "The user requesting the summary"
            },
            "topic": {
                "type": "string",
                "description": "topic",
            },
        },
        "required": ["user", "topic"],
        "additionalProperties": False,
    }
    print(json.dumps(schema, indent=2))

# hacker news rss feed, can be whatever
DOCUMENT_URL = "https://hnrss.org/frontpage.jsonfeed?points=150&count=3"  
DOCUMENT_KEY = ""  

def getsummary(user,topic):
    headers = {
        'Authorization': f'Bearer {DOCUMENT_KEY}',
        'User-Agent': 'document_script',
        'Content-Type': 'application/json',
    }

    
    try:
        response = requests.get(DOCUMENT_URL, headers=headers, verify=False)
        if response.status_code == 200:
           try:
                response_data = response.json()
                return json.dumps(response_data, indent=2)
           except json.JSONDecodeError:
                return "Error: Failed to parse JSON response"
        else:
            return f"Error: Unable to fetch summary (status code: {response.status_code})"
    except requests.RequestException as e:
        return f"Error: Request failed ({str(e)})"

def main():
    if len(sys.argv) < 2:
        print("Usage: links.py [--schema | --execute <json>]")
        sys.exit(1)

    option = sys.argv[1]

    if option == "--schema":
        print_schema()
    elif option == "--execute":
        if len(sys.argv) < 3:
            print("Error: Missing JSON input for execution")
            sys.exit(1)
        try:
            data = json.loads(sys.argv[2])
            user = data.get("user")
            term = data.get("topic")
            result = getsummary(user,term)
            print(result)
        except json.JSONDecodeError:
            print("Error: Invalid JSON input")
            sys.exit(1)
    else:
        print("Usage: links.py [--schema | --execute <json>]")
        sys.exit(1)

if __name__ == "__main__":
    main()
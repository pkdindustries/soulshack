#!/usr/bin/env python3

import sys
import json
import os
import shutil
import subprocess
import pwd
import time

def print_schema():
    schema = {
        "name": "system_resource_monitor",
        "description": "provides system resource usage such as CPU, memory, disk, process, Docker container, and load average information",
        "type": "object",
        "properties": {
            "resource": {
                "type": "string",
                "description": "The type of system resource to monitor (e.g., 'cpu', 'memory', 'disk', 'processes', 'docker', 'all', 'loadavg')",
                "enum": ["cpu", "memory", "disk", "processes", "docker", "all", "loadavg", "uptime"]
            }
        },
        "required": ["resource"],
        "additionalProperties": False
    }
    print(json.dumps(schema, indent=2))

def execute(resource_json):
    try:
        # Parse the JSON input to extract the resource type
        data = json.loads(resource_json)
        resource_type = data.get("resource")
        if not resource_type:
            print("Error: 'resource' is required in the input JSON.", file=sys.stderr)
            sys.exit(1)
        
        if resource_type == "memory" or resource_type == "all":
            # Get memory information using free command
            try:
                result = subprocess.run(["free", "-m"], capture_output=True, text=True, check=True)
                if result.stdout:
                    print(result.stdout.strip())
                else:
                    print("No memory information found.")
            except subprocess.CalledProcessError:
                print("Error: Failed to run 'free' command.", file=sys.stderr)
                sys.exit(1)
        if resource_type == "disk" or resource_type == "all":
            # Get disk space information using df command
            try:
                result = subprocess.run(["df", "-h"], capture_output=True, text=True, check=True)
                if result.stdout:
                    print(result.stdout.strip())
                else:
                    print("No disk information found.")
            except subprocess.CalledProcessError:
                print("Error: Failed to run 'df' command.", file=sys.stderr)
                sys.exit(1)
        if resource_type == "processes" or resource_type == "all":
            # Get process information using ps command, including system and user time
            try:
                result = subprocess.run(["ps", "-eo", "pid,ppid,user,args,%mem,%cpu"], capture_output=True, text=True, check=True)
                if result.stdout:
                    print(result.stdout.strip())
                else:
                    print("No processes found.")
            except subprocess.CalledProcessError:
                print("Error: Failed to run 'ps' command.", file=sys.stderr)
                sys.exit(1)
        if resource_type == "docker" or resource_type == "all":
            # Get Docker container information using the docker command line
            try:
                result = subprocess.run(["docker", "ps"], capture_output=True, text=True, check=True)
                if result.stdout:
                    print(result.stdout.strip())
                else:
                    print("No running Docker containers found.")
            except FileNotFoundError:
                print("Error: Docker is not installed or not in PATH.", file=sys.stderr)
                sys.exit(1)
            except subprocess.CalledProcessError:
                print("Error: Failed to run Docker command.", file=sys.stderr)
                sys.exit(1)
        if resource_type == "loadavg" or resource_type == "uptime" or resource_type == "cpu" or resource_type == "all":
            # Get load average and system uptime using uptime command
            try:
                uptime_result = subprocess.run(["uptime"], capture_output=True, text=True, check=True)
                if uptime_result.stdout:
                    uptime_info = uptime_result.stdout.strip()
                    print(f"Uptime and Load Average: {uptime_info}")
            except FileNotFoundError:
                print("Error: 'uptime' command is not found.", file=sys.stderr)
                sys.exit(1)
            except subprocess.CalledProcessError:
                print("Error: Failed to run 'uptime' command.", file=sys.stderr)
                sys.exit(1)
        if resource_type not in ["cpu", "memory", "disk", "processes", "docker", "all", "loadavg", "uptime"]:
            print(f"Error: Unknown resource type '{resource_type}'. Supported types are {resource_type}.", file=sys.stderr)
            sys.exit(1)
    except json.JSONDecodeError:
        print("Error: Invalid JSON input.", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: system.py [--schema | --execute <resource_json>]", file=sys.stderr)
        sys.exit(1)

    command = sys.argv[1]
    if command == "--schema":
        print_schema()
    elif command == "--execute" and len(sys.argv) == 3:
        execute(sys.argv[2])
    else:
        print("Usage: system.py [--schema | --execute <resource_json>]", file=sys.stderr)
        sys.exit(1)

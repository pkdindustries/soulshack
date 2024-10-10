#!/usr/bin/env python3

import sys
import json
import os
import shutil
import subprocess

def print_schema():
    schema = {
        "$schema": "http://json-schema.org/draft-07/schema#",
        "type": "object",
        "properties": {
            "resource": {
                "type": "string",
                "description": "The type of system resource to monitor (e.g., 'cpu', 'memory', 'disk', 'processes', 'docker')"
            }
        },
        "required": ["resource"],
        "additionalProperties": False
    }
    print(json.dumps(schema, indent=2))

def get_name():
    print("system_resource_monitor")

def get_description():
    print("provides system resource usage such as CPU, memory, disk, process, and Docker container information")

def execute(resource_json):
    try:
        # Parse the JSON input to extract the resource type
        data = json.loads(resource_json)
        resource_type = data.get("resource")
        if not resource_type:
            print("Error: 'resource' is required in the input JSON.", file=sys.stderr)
            sys.exit(1)
        
        # Provide system resource information based on the requested type
        if resource_type == "cpu":
            # Get CPU information using /proc/stat
            with open("/proc/stat", "r") as f:
                line = f.readline()
                if line.startswith("cpu "):
                    cpu_times = list(map(int, line.split()[1:]))
                    idle_time = cpu_times[3]
                    total_time = sum(cpu_times)
                    print(f"Total CPU Time: {total_time}")
                    print(f"Idle CPU Time: {idle_time}")
        elif resource_type == "memory":
            # Get memory information using /proc/meminfo
            with open("/proc/meminfo", "r") as f:
                meminfo = {}
                for line in f:
                    parts = line.split()
                    key = parts[0].rstrip(":")
                    value = int(parts[1])
                    meminfo[key] = value
                
                total_memory = meminfo.get("MemTotal", 0) / 1024
                free_memory = (meminfo.get("MemFree", 0) + meminfo.get("Buffers", 0) + meminfo.get("Cached", 0)) / 1024
                used_memory = total_memory - free_memory
                print(f"Total Memory: {total_memory:.2f} MB")
                print(f"Used Memory: {used_memory:.2f} MB")
                print(f"Free Memory: {free_memory:.2f} MB")
        elif resource_type == "disk":
            # Get disk space information for all mounted volumes
            with open("/proc/mounts", "r") as f:
                mounts = [line.split()[1] for line in f if line.startswith("/dev/")]
            
            for mount in mounts:
                total, used, free = shutil.disk_usage(mount)
                print(f"Mount Point: {mount}")
                print(f"  Total Disk Space: {total / (1024 * 1024 * 1024):.2f} GB")
                print(f"  Used Disk Space: {used / (1024 * 1024 * 1024):.2f} GB")
                print(f"  Free Disk Space: {free / (1024 * 1024 * 1024):.2f} GB")
        elif resource_type == "processes":
            # Get process information using /proc
            processes = []
            for pid in os.listdir("/proc"):
                if pid.isdigit():
                    try:
                        with open(f"/proc/{pid}/stat", "r") as f:
                            stat_info = f.readline().split()
                            process_name = stat_info[1].strip("()")
                            process_state = stat_info[2]
                            user_time = int(stat_info[13]) / os.sysconf(os.sysconf_names['SC_CLK_TCK'])
                            system_time = int(stat_info[14]) / os.sysconf(os.sysconf_names['SC_CLK_TCK'])
                            processes.append((pid, process_name, process_state, user_time, system_time))
                    except FileNotFoundError:
                        # Process might have ended before we could read it
                        continue
            print("PID	NAME	STATE	USER_TIME	SYSTEM_TIME")
            for pid, name, state, u_time, s_time in processes:
                print(f"{pid}	{name}	{state}	{u_time:.2f}s	{s_time:.2f}s")
        elif resource_type == "docker":
            # Get Docker container information using the docker command line
            try:
                result = subprocess.run(["docker", "ps", "--format", "{{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}"], capture_output=True, text=True, check=True)
                if result.stdout:
                    print("CONTAINER ID	IMAGE	STATUS	NAME")
                    print(result.stdout.strip())
                else:
                    print("No running Docker containers found.")
            except FileNotFoundError:
                print("Error: Docker is not installed or not in PATH.", file=sys.stderr)
                sys.exit(1)
            except subprocess.CalledProcessError:
                print("Error: Failed to run Docker command.", file=sys.stderr)
                sys.exit(1)
        else:
            print(f"Error: Unknown resource type '{resource_type}'. Supported types are 'cpu', 'memory', 'disk', 'processes', 'docker'.", file=sys.stderr)
            sys.exit(1)
    except json.JSONDecodeError:
        print("Error: Invalid JSON input.", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: system_resource_monitor.py [--schema | --name | --description | --execute <resource_json>]", file=sys.stderr)
        sys.exit(1)

    command = sys.argv[1]
    if command == "--schema":
        print_schema()
    elif command == "--name":
        get_name()
    elif command == "--description":
        get_description()
    elif command == "--execute" and len(sys.argv) == 3:
        execute(sys.argv[2])
    else:
        print("Usage: system_resource_monitor.py [--schema | --name | --description | --execute <resource_json>]", file=sys.stderr)
        sys.exit(1)
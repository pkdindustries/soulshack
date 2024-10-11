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
        "$schema": "http://json-schema.org/draft-07/schema#",
        "type": "object",
        "properties": {
            "resource": {
                "type": "string",
                "description": "The type of system resource to monitor (e.g., 'cpu', 'memory', 'disk', 'processes', 'docker', 'full')",
                "enum": ["cpu", "memory", "disk", "processes", "docker", "full"]
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
        if resource_type == "cpu" or resource_type == "full":
            # Get CPU information using /proc/stat
            with open("/proc/stat", "r") as f:
                line = f.readline()
                if line.startswith("cpu "):
                    cpu_times = list(map(int, line.split()[1:]))
                    idle_time = cpu_times[3]
                    total_time = sum(cpu_times)
                    print(f"Total CPU Time: {total_time}")
                    print(f"Idle CPU Time: {idle_time}")
        if resource_type == "memory" or resource_type == "full":
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
        if resource_type == "disk" or resource_type == "full":
            # Get disk space information for all mounted volumes
            with open("/proc/mounts", "r") as f:
                mounts = [line.split()[1] for line in f if line.startswith("/dev/")]
            
            for mount in mounts:
                total, used, free = shutil.disk_usage(mount)
                print(f"Mount Point: {mount}")
                print(f"  Total Disk Space: {total / (1024 * 1024 * 1024):.2f} GB")
                print(f"  Used Disk Space: {used / (1024 * 1024 * 1024):.2f} GB")
                print(f"  Free Disk Space: {free / (1024 * 1024 * 1024):.2f} GB")
        if resource_type == "processes" or resource_type == "full":
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
                            uid = os.stat(f"/proc/{pid}").st_uid
                            username = pwd.getpwuid(uid).pw_name
                            processes.append((pid, username, process_name, process_state, user_time, system_time))
                    except FileNotFoundError:
                        # Process might have ended before we could read it
                        continue
            print("PID	USERNAME	NAME	STATE	USER_TIME	SYSTEM_TIME")
            for pid, username, name, state, u_time, s_time in processes:
                print(f"{pid}	{username}	{name}	{state}	{u_time:.2f}s	{s_time:.2f}s")
        if resource_type == "docker" or resource_type == "full":
            # Get Docker container information using the docker command line
            try:
                result = subprocess.run(["docker", "ps", "--format", "{{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}"], capture_output=True, text=True, check=True)
                if result.stdout:
                    print("CONTAINER ID	IMAGE	STATUS	NAME	CPU %	MEMORY USAGE / LIMIT	UPTIME")
                    container_ids = [line.split()[0] for line in result.stdout.strip().split('\n')]
                    for container_id in container_ids:
                        stats_result = subprocess.run(["docker", "stats", container_id, "--no-stream", "--format", "{{.CPUPerc}}\t{{.MemUsage}}\t{{.ID}}"], capture_output=True, text=True, check=True)
                        if stats_result.stdout:
                            stats = stats_result.stdout.strip().split('\t')
                            cpu_usage = stats[0]
                            mem_usage = stats[1]
                            uptime_result = subprocess.run(["docker", "inspect", "-f", "{{.State.StartedAt}}", container_id], capture_output=True, text=True, check=True)
                            if uptime_result.stdout:
                                started_at = uptime_result.stdout.strip()
                                started_at_epoch = time.mktime(time.strptime(started_at.split('.')[0], "%Y-%m-%dT%H:%M:%S"))
                                uptime_seconds = time.time() - started_at_epoch
                                uptime = time.strftime("%H:%M:%S", time.gmtime(uptime_seconds))
                                container_info = [line for line in result.stdout.strip().split('\n') if container_id in line][0]
                                print(f"{container_info}\t{cpu_usage}\t{mem_usage}\t{uptime}")
                else:
                    print("No running Docker containers found.")
            except FileNotFoundError:
                print("Error: Docker is not installed or not in PATH.", file=sys.stderr)
                sys.exit(1)
            except subprocess.CalledProcessError:
                print("Error: Failed to run Docker command.", file=sys.stderr)
                sys.exit(1)
        if resource_type not in ["cpu", "memory", "disk", "processes", "docker", "full"]:
            print(f"Error: Unknown resource type '{resource_type}'. Supported types are 'cpu', 'memory', 'disk', 'processes', 'docker', 'full'.", file=sys.stderr)
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
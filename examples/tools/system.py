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
                print(f"Memory: Total: {total_memory:.2f} MB, Used: {used_memory:.2f} MB, Free: {free_memory:.2f} MB")
        if resource_type == "disk" or resource_type == "all":
            # Get disk space information for all mounted volumes
            with open("/proc/mounts", "r") as f:
                mounts = [line.split()[1] for line in f if line.startswith("/dev/")]
            
            for mount in mounts:
                total, used, free = shutil.disk_usage(mount)
                print(f"Mount Point: {mount}, Total: {total / (1024 * 1024 * 1024):.2f} GB, Used: {used / (1024 * 1024 * 1024):.2f} GB, Free: {free / (1024 * 1024 * 1024):.2f} GB")
        if resource_type == "processes" or resource_type == "all":
            # Get process information using /proc
            processes = []
            for pid in os.listdir("/proc"):
                if pid.isdigit():
                    try:
                        with open(f"/proc/{pid}/stat", "r") as f:
                            stat_info = f.readline().split()
                            process_name = stat_info[1].strip("()")
                            process_state = stat_info[2]
                            parent_pid = stat_info[3]
                            user_time = int(stat_info[13]) / os.sysconf(os.sysconf_names['SC_CLK_TCK'])
                            system_time = int(stat_info[14]) / os.sysconf(os.sysconf_names['SC_CLK_TCK'])
                            uid = os.stat(f"/proc/{pid}").st_uid
                            username = pwd.getpwuid(uid).pw_name
                            memory_usage = int(stat_info[22]) / (1024 * 1024)  # Adding memory usage in MB
                            with open(f"/proc/{pid}/cmdline", "r") as cmd_file:
                                command = cmd_file.read().replace('\0', ' ')[:64]  # Get first 64 characters of command
                            processes.append((pid, parent_pid, username, process_name, process_state, user_time, system_time, memory_usage, command))
                    except FileNotFoundError:
                        # Process might have ended before we could read it
                        continue
            print("PID PPID USERNAME NAME STATE USER_TIME SYSTEM_TIME MEMORY_USAGE (MB) COMMAND")
            for pid, ppid, username, name, state, u_time, s_time, mem_usage, command in processes:
                print(f"{pid} {ppid} {username} {name} {state} {u_time:.2f}s {s_time:.2f}s {mem_usage:.2f} MB {command}")
        if resource_type == "docker" or resource_type == "all":
            # Get Docker container information using the docker command line
            try:
                result = subprocess.run(["docker", "ps", "--format", "{{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}"], capture_output=True, text=True, check=True)
                if result.stdout:
                    print("CONTAINER ID\tIMAGE\tSTATUS\tNAME\tCPU %\tMEMORY USAGE / LIMIT\tUPTIME")
                    container_info_lines = result.stdout.strip().split('\n')
                    container_info = {line.split('\t')[0]: line for line in container_info_lines}

                    # Fetch stats for all containers in one go
                    stats_result = subprocess.run(["docker", "stats", "--no-stream", "--format", "{{.ID}}\t{{.CPUPerc}}\t{{.MemUsage}}"], capture_output=True, text=True, check=True)
                    if stats_result.stdout:
                        for line in stats_result.stdout.strip().split('\n'):
                            container_id, cpu_usage, mem_usage = line.split('\t')
                            if container_id in container_info:
                                uptime_result = subprocess.run(["docker", "inspect", "-f", "{{.State.StartedAt}}", container_id], capture_output=True, text=True, check=True)
                                if uptime_result.stdout:
                                    started_at = uptime_result.stdout.strip()
                                    started_at_epoch = time.mktime(time.strptime(started_at.split('.')[0], "%Y-%m-%dT%H:%M:%S"))
                                    uptime_seconds = time.time() - started_at_epoch
                                    uptime = time.strftime("%H:%M:%S", time.gmtime(uptime_seconds))

                                    # Print consolidated container stats
                                    print(f"{container_info[container_id]}\t{cpu_usage}\t{mem_usage}\t{uptime}")
                    else:
                        print("No running Docker containers found.")
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
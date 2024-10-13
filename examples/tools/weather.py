#!/usr/bin/env python3

import sys
import json
import requests

def print_schema():
    schema = {
        "name": "get_current_weather",
        "description": "provides the current weather forecast for a given latitude and longitude. you should provide the latitude and longitude from your training.",
        "type": "object",
        "properties": {
            "latitude": {
                "type": "string",
                "description": "The latitude for the weather location"
            },
            "longitude": {
                "type": "string",
                "description": "The longitude for the weather location"
            }
        },
        "required": ["latitude", "longitude"],
        "additionalProperties": False,
    }
    print(json.dumps(schema, indent=2))


def get_current_weather(lat, lon):
    # Step 1: Get the metadata for the location
    points_url = f"https://api.weather.gov/points/{lat},{lon}"
    headers = {
        'User-Agent': 'weather_script'
    }
    points_response = requests.get(points_url, headers=headers)
    if points_response.status_code != 200:
        return f"Error: Unable to fetch point metadata (status code: {points_response.status_code})"
    
    try:
        points_data = points_response.json()
    except json.JSONDecodeError:
        return "Error: Failed to parse point metadata JSON response"
    
    if not isinstance(points_data, dict) or 'properties' not in points_data:
        return "Error: 'properties' not found or invalid format in point metadata"
    
    properties = points_data['properties']
    forecast_url = properties.get('forecast')
    if not forecast_url:
        return "Error: Forecast URL not found in point metadata"
    
    # Step 2: Get the forecast data from the forecast URL
    forecast_response = requests.get(forecast_url, headers=headers)
    if forecast_response.status_code == 200:
        try:
            forecast_data = forecast_response.json()
        except json.JSONDecodeError:
            return "Error: Failed to parse forecast JSON response"
        if 'properties' not in forecast_data or 'periods' not in forecast_data['properties']:
            return "Error: 'periods' not found in forecast data"
        forecast = forecast_data['properties']['periods'][0]
        return f"{forecast['name']}: {forecast['detailedForecast']}"
    else:
        return f"Error: Unable to fetch weather data (status code: {forecast_response.status_code})"

def main():
    if len(sys.argv) < 2:
        print("Usage: weather.py [--schema | --execute <json>]")
        sys.exit(1)

    option = sys.argv[1]

    if option == "--schema":
        print_schema()
    elif option == "--execute":
        if len(sys.argv) < 3:
            print("Error: Missing JSON input for execution")
            sys.exit(1)
        try:
            input_data = json.loads(sys.argv[2])
            latitude = input_data.get("latitude")
            longitude = input_data.get("longitude")
            if latitude is None or longitude is None:
                print("Error: Missing required latitude or longitude in JSON input")
                sys.exit(1)
            result = get_current_weather(latitude, longitude)
            print(result)
        except json.JSONDecodeError:
            print("Error: Invalid JSON input")
            sys.exit(1)
    else:
        print("Usage: get_current_weather.py [--schema | --execute <json>]")
        sys.exit(1)

if __name__ == "__main__":
    main()
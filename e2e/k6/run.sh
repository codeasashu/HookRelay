#!/bin/bash

# Default values
uri=${BASE_URL:-"http://localhost"}
vus="10"
rate="10"
duration="1m"
endpoint_id=""
api_key=""
project_id=""
queue_url=""
aws_access_key=""
aws_secret_key=""

# Function to display help
usage() {
	echo "Usage: $0 [options]"
	echo ""
	echo "Options:"
	echo "-u, --uri           Base URL for outgoing, ingest URL for incoming. Default: http://localhost"
	echo "-v, --vus           Set how many virtual users should execute the test concurrently. Default: 10"
	echo "-r, --rate          Set how many requests should be sent per second. Default: 10"
	echo "-d, --duration      Set how long the test should run. Default: 1m"
	echo "-h, --help          Display this help message."
}

# Parse command-line arguments
while [[ "$#" -gt 0 ]]; do
	case $1 in
	-u | --uri)
		uri="$2"
		shift
		;;
	-v | --vus)
		vus="$2"
		shift
		;;
	-r | --rate)
		rate="$2"
		shift
		;;
	-d | --duration)
		duration="$2"
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "Unknown parameter passed: $1"
		usage
		exit 1
		;;
	esac
	shift
done

export VUS="$vus"
export RATE="$rate"
export DURATION="$duration"
export BASE_URL="$uri"

k6 run index.js

#!/bin/bash

# Create a simple web server that responds to requests
# This is our target container for the combined netem and iptables example

# Remove container if it exists
docker rm -f combined-server 2>/dev/null

# Run a simple nginx server
docker run -d --name combined-server \
  --rm \
  -p 8080:80 \
  nginx:alpine

# Show logs from the server
docker logs -f combined-server
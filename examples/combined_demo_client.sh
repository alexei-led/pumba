#!/bin/bash

# Create a client container that sends requests to the server
# We'll use curl to make periodic requests to demonstrate the effect of our network chaos

# Remove container if it exists
docker rm -f combined-client 2>/dev/null

# Create a client that makes HTTP requests to the server
docker run -d --name combined-client \
  --rm \
  --network host \
  alpine:latest \
  sh -c "while true; do time wget -q -O- http://localhost:8080 > /dev/null && echo 'Request completed' || echo 'Request failed'; sleep 2; done"

# Show logs from the client
docker logs -f combined-client
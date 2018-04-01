#!/bin/bash

if [ -z "$GITHUB_TOKEN" ]; then echo "missing environment variable GITHUB_TOKEN" && exit 1; fi

docker run --rm \
           --interactive \
           --tty \
           -v "$(pwd):$(pwd)" \
           -w "$(pwd)" \
           -it muccg/github-changelog-generator -u alexei-led -p pumba \
           -t "$GITHUB_TOKEN"

#!/bin/bash
distdir=.dist
user=${1}
repo=${2}

if [ -z "$GITHUB_TOKEN" ]; then
  echo "Need to set GITHUB_TOKEN environment variable"; exit 1
fi

if [ -z "$RELEASE_TAG" ]; then
  RELEASE_TAG=$(git describe --tags)
  if [ $? -ne 0 ]
  then
    echo "Failed to setup RELEASE_TAG from 'git describe --tags'" >&2; exit 1
  fi
fi

# see https://github.com/aktau/github-release for the tool commands
# edit release details (release is automatically created for annotated tag by GitHub)
github-release release \
  --security-token ${GITHUB_TOKEN} \
  --user aktau \
  --repo gofinance \
  --tag ${RELEASE_TAG}

# upload files
( cd ${distdir} || exit
for f in *; do
  github-release upload \
    --security-token ${GITHUB_TOKEN} \
    --user ${user} \
    --repo ${repo} \
    --tag ${RELEASE_TAG} \
    --name $f \
    --file $f
done
)

if [ $? -ne 0 ]
then
  echo "Something went wrong with publishing a new release '${RELEASE_TAG}' to GitHub" >&2; exit 1
fi

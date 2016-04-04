#!/bin/bash
distdir=.dist

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

# see https://github.com/tcnksm/ghr for the tool commands
ghr -t ${GITHUB_TOKEN} -u ${1} -r ${2} --replace ${RELEASE_TAG} ${distdir}/
if [ $? -ne 0 ]
then
  echo "Something went wrong with publishing a new release '${RELEASE_TAG}' to GitHub" >&2; exit 1
fi

#!/bin/bash
distdir=.dist

# see https://github.com/tcnksm/ghr for toll commands
ghr -t ${GITHUB_TOKEN} -u ${1} -r ${2} --replace `git describe --tags` ${distdir}/

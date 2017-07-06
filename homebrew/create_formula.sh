#!/bin/bash

version=$(cat ../VERSION)
sed "s/{{ VERSION }}/$version/g" pumba.rb.tpl | tee pumba.rb

files=( pumba_darwin_amd64 pumba_darwin_386 )

for f in "${files[@]}"; do
  curl -L https://github.com/gaia-adm/pumba/releases/download/$version/$f > $TMPDIR/$f
  sha256="$(shasum -a 256 $TMPDIR/$f | awk '{print $1}')"
  sed -i '' -e "s/{{ ${f}_SHA256 }}/${sha256}/g" pumba.rb
done

diff $(brew --repo homebrew/core)/Formula/pumba.rb pumba.rb
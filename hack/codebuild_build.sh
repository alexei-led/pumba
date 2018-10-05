#!/bin/sh

# update build cache
echo "==> create/update image: ${FULL_REPO_NAME}/builder:${CODEBUILD_GIT_BRANCH}" 
docker build -t ${FULL_REPO_NAME}/builder:${CODEBUILD_GIT_BRANCH} --target builder \
  --cache-from ${FULL_REPO_NAME}/builder:${CODEBUILD_GIT_BRANCH} \
  --file docker/Dockerfile .

echo "==> create/update image: ${FULL_REPO_NAME}/build-and-test:${CODEBUILD_GIT_BRANCH}" 
docker build -t ${FULL_REPO_NAME}/build-and-test:${CODEBUILD_GIT_BRANCH} --target build-and-test \
  --cache-from ${FULL_REPO_NAME}/builder:${CODEBUILD_GIT_BRANCH} \
  --cache-from ${FULL_REPO_NAME}/build-and-test:${CODEBUILD_GIT_BRANCH} \
  --build-arg CODECOV_TOKEN=$CODECOV_TOKEN \
  --build-arg VCS_COMMIT_ID=${CODEBUILD_GIT_COMMIT} \
  --build-arg VCS_BRANCH_NAME=${CODEBUILD_GIT_BRANCH} \
  --build-arg VCS_SLUG="alexei-led/pumba" \
  --file docker/Dockerfile .

echo "==> create/update image: ${FULL_REPO_NAME}/github-release:${CODEBUILD_GIT_BRANCH}"
if [ "${CODEBUILD_GIT_BRANCH}" == "master" ]; then
  echo "==> going to create a GitHub release ..."
  RELEASE="true"
fi
docker build -t ${FULL_REPO_NAME}/github-release:${CODEBUILD_GIT_BRANCH} --target github-release \
  --cache-from ${FULL_REPO_NAME}/builder:${CODEBUILD_GIT_BRANCH} \
  --cache-from ${FULL_REPO_NAME}/build-and-test:${CODEBUILD_GIT_BRANCH} \
  --cache-from ${FULL_REPO_NAME}/github-release:${CODEBUILD_GIT_BRANCH} \
  --build-arg CODECOV_TOKEN=$CODECOV_TOKEN \
  --build-arg VCS_COMMIT_ID=${CODEBUILD_GIT_COMMIT} \
  --build-arg VCS_BRANCH_NAME=${CODEBUILD_GIT_BRANCH} \
  --build-arg VCS_SLUG="alexei-led/pumba" \
  --build-arg GITHUB_TOKEN=${GITHUB_TOKEN} \
  --build-arg RELEASE=${RELEASE} \
  --build-arg RELEASE_TAG=${CODEBUILD_GIT_MOST_RECENT_TAG} \
  --build-arg TAG_MESSAGE="${CODEBUILD_GIT_TAG_MESSAGE}" \
  --file docker/Dockerfile .

# Build and push target image
echo "==> create/update final image: ${FULL_REPO_NAME}:${CODEBUILD_GIT_BRANCH}"
if [ "$NO_CACHE" == "true" ]; then
  echo "==> NO CACHE mode"
  NO_CACHE_OPT="--no-cache"
fi
docker build -t ${FULL_REPO_NAME}:${CODEBUILD_GIT_BRANCH} ${NO_CACHE_OPT} \
  --cache-from ${FULL_REPO_NAME}/builder:${CODEBUILD_GIT_BRANCH} \
  --cache-from ${FULL_REPO_NAME}/build-and-test:${CODEBUILD_GIT_BRANCH} \
  --cache-from ${FULL_REPO_NAME}/github-release:${CODEBUILD_GIT_BRANCH} \
  --cache-from ${FULL_REPO_NAME}:${CODEBUILD_GIT_BRANCH} \
  --build-arg CODECOV_TOKEN=$CODECOV_TOKEN \
  --build-arg VCS_COMMIT_ID=${CODEBUILD_GIT_COMMIT} \
  --build-arg VCS_BRANCH_NAME=${CODEBUILD_GIT_BRANCH} \
  --build-arg VCS_SLUG="alexei-led/pumba" \
  --build-arg GITHUB_TOKEN=${GITHUB_TOKEN} \
  --build-arg RELEASE=${RELEASE} \
  --build-arg RELEASE_TAG=${CODEBUILD_GIT_MOST_RECENT_TAG} \
  --build-arg TAG_MESSAGE="${CODEBUILD_GIT_TAG_MESSAGE}" \
  --file docker/Dockerfile .


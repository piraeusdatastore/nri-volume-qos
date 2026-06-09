#!/bin/bash -e
die() {
	echo "$@" >&2
	exit 1
}

REPO="https://github.com/piraeusdatastore/nri-volume-qos"

VERSION="${1#v}"
if [ -z "$VERSION" ]; then
	die "Usage: $0 <version>   (e.g. $0 1.2.3)"
fi

# only plain x.y.z releases are supported here; tag pre-releases by hand
if ! echo "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
	die "'$VERSION' does not match the expected format <major>.<minor>.<patch>"
fi

# refuse if the tag already exists
if git rev-parse --verify --quiet "v$VERSION" >/dev/null; then
	die "git tag v$VERSION already exists"
fi

# refuse to release from a dirty working tree
if ! git diff-index --quiet HEAD --; then
	die "refusing to create a release from a dirty working tree"
fi

date_today="$(date +%Y-%m-%d)"

# comparison link for the new version: against the previous tag, or its tag page for the first release
prev="$(git tag --list 'v*' --sort=-v:refname | head -n1)"
if [ -n "$prev" ]; then
	version_link="$REPO/compare/$prev...v$VERSION"
else
	version_link="$REPO/releases/tag/v$VERSION"
fi

# roll [Unreleased] into the new version, leaving a fresh empty [Unreleased] on top
sed -i "s|^## \[Unreleased\]\$|## [Unreleased]\n\n## [$VERSION] - $date_today|" CHANGELOG.md
# point the Unreleased link at the new tag and add the release's own link
sed -i "s|^\[Unreleased\]:.*|[Unreleased]: $REPO/compare/v$VERSION...HEAD\n[$VERSION]: $version_link|" CHANGELOG.md

# commit and tag the release
git commit --signoff -m "Release v$VERSION" CHANGELOG.md
git tag -s -m "Release v$VERSION" "v$VERSION"

echo
echo "Created release commit and annotated tag v$VERSION."
echo "Publish it (triggers the build + release workflows) with:"
echo "    git push origin HEAD v$VERSION"

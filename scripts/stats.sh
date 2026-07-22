#!/bin/sh
# Download and traffic stats for payk. Requires gh with push access.
set -e
REPO=yusupkhemraev/payk

echo "== releases =="
gh api "repos/$REPO/releases" --jq '
  [.[] | .tag_name as $t | .assets[] | select(.name != "checksums.txt")
   | {tag: $t, n: .download_count}]
  | group_by(.tag) | map({tag: .[0].tag, downloads: (map(.n) | add)})
  | .[] | "\(.tag)\t\(.downloads)"'
echo "total:\t$(gh api "repos/$REPO/releases" --jq '[.[].assets[] | select(.name != "checksums.txt") | .download_count] | add')"

echo "\n== repo =="
gh api "repos/$REPO" --jq '"stars: \(.stargazers_count) · forks: \(.forks_count) · watchers: \(.subscribers_count)"'

echo "\n== traffic (14d, lags ~1 day) =="
gh api "repos/$REPO/traffic/views" --jq '"views:  \(.count) (\(.uniques) unique)"'
gh api "repos/$REPO/traffic/clones" --jq '"clones: \(.count) (\(.uniques) unique, includes CI)"'
echo "referrers:"
gh api "repos/$REPO/traffic/popular/referrers" --jq '.[] | "  \(.referrer): \(.count) (\(.uniques) unique)"'

# Instructions for Claude

## Workflow for fixes

For every fix (whether from a GitHub issue, a review comment, or a direct
request), always:

1. Commit the change on the designated working branch.
2. Push the branch to `origin`.
3. Open a pull request against `master` describing what was changed and why.
4. Merge the pull request into `master` (squash merge, matching the
   existing history style).

Do all four steps without asking — it is the default flow for this repo.

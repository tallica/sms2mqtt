Create a new release. The argument is the version bump type: `patch`, `minor`, or `major`. If no argument is given, decide the bump type based on the unreleased changelog entries.

## Steps

1. **Determine the new version**
   - Read the latest version from the most recent tagged section in `CHANGELOG.md`
   - Read the `[Unreleased]` section of `CHANGELOG.md` to understand what changed
   - If `$ARGUMENTS` is `patch`, `minor`, or `major` — use that; otherwise infer from changes:
     - Breaking/incompatible changes (new required config, removed flags, changed APIs) → `major`
     - New features, new tools, new flags → `minor`
     - Bug fixes only → `patch`
   - Compute the new version string

2. **Update `CHANGELOG.md`**
   - Rename `## [Unreleased]` → `## [X.Y.Z] - YYYY-MM-DD` (today's date)
   - Leave a fresh empty `## [Unreleased]` above it
   - Add compare link at the bottom: `[X.Y.Z]: https://github.com/tallica/sms2mqtt/compare/vPREV...vX.Y.Z`
   - Update the `[Unreleased]` link to point to `vX.Y.Z...HEAD`

3. **Build** — run `make build` and confirm it succeeds before committing

4. **Commit, tag, push**
   ```
   git add CHANGELOG.md
   git commit -m "release: vX.Y.Z"
   git tag vX.Y.Z
   git push && git push origin vX.Y.Z
   ```

   Pushing the tag triggers the `release` job in `.github/workflows/ci.yml`: after test, lint and audit pass, GoReleaser builds linux amd64/arm64/armv7 archives and publishes a GitHub release whose notes are the tag's `CHANGELOG.md` section. The binary version comes from the tag via ldflags.

Report the new version and tag when done.

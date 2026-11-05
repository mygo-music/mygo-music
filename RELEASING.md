# Releasing

Nothing the release workflow builds is signed or notarized, and the app has no auto-update. On macOS the app is signed ad hoc, which Apple silicon needs to run it, and people allow it once in System Settings → Privacy & Security; Windows SmartScreen asks them to choose More info → Run anyway.

1. Commit pending work, bring main up to date with origin, and run `go test ./internal/...`. Others push to main too, so a release cut from a stale main leaves out their work:

   ```bash
   git pull --rebase origin main
   ```

2. Bump `version` in `mygo.json`, and add a `## 0.1.8` section at the top of `CHANGELOG.md` saying, in Chinese, what changed for people using the app since the last release (`git log v0.1.7..HEAD`, after the pull). Commit both in a commit titled just the version, e.g. `0.1.8`. The release page takes that section as its notes, and the app shows it when someone asks for updates in its settings; the release workflow fails without one.
3. Push main first, and tag only once that push has gone through. Pushing both at once still pushes the tag when main is rejected, and the tag starts a release build of a commit that isn't on main:

   ```bash
   git push origin main
   git tag v0.1.8
   git push origin v0.1.8
   ```

   If main is rejected, pull again, add what came in to the changelog, amend the version commit, test, and push main again before tagging. If a tag went up on the wrong commit anyway, cancel its run, then delete the tag and the draft it made before tagging again:

   ```bash
   git push origin :refs/tags/v0.1.8
   gh release delete v0.1.8 --yes
   ```

4. `.github/workflows/release.yml` builds macOS, Windows and Linux and uploads them to a draft release. Wait for every job to pass:

   ```bash
   gh run watch "$(gh run list --workflow release.yml -L1 --json databaseId -q '.[0].databaseId')" --exit-status
   ```

   If a job fails, don't publish; fix it first.

5. Publish the draft and mark it latest:

   ```bash
   gh release edit v0.1.8 --draft=false --latest
   ```

6. Check the app finds it. The app doesn't update itself: in the previous version's settings, 检查更新 reads GitHub's latest release and offers its page. It only sees a release once it is published (step 5), as drafts aren't `latest`.

7. Deploy the website. It reads the latest release when it's prerendered, so it has to be rebuilt after publishing:

   ```bash
   cd website && bun run deploy
   ```

   Then check the website's download page shows the new version.

## The speech recognition libraries

Podcast captions run sherpa-onnx, whose libraries the app downloads with the model from the `sherpa-onnx-<version>` release, not from the app's own. They only change with sherpa-onnx's version, in `scripts/sherpa`. Then pack them again, commit the `internal/speech/natives.go` it writes, and upload them as a prerelease, which never becomes the latest release that the app's update check looks for. Push the commit first: the release's tag is made from the default branch. The macOS libraries are packed as sherpa-onnx built them, as the app has no Developer ID to sign them with:

```bash
go run ./scripts/sherpa
gh release create sherpa-onnx-2.0.1 --prerelease --latest=false build/sherpa/*
```

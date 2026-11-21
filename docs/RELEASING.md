# Releasing Headliner

For the owner. Plain steps; each one says what you should see.

## Once: let GitHub sign the releases

Every update is checked against a signature, so nobody can slip a fake
update in. The signing key was made on your laptop and lives at:

    %USERPROFILE%\.headliner-signing\release.key

Keep that file safe and private: never commit it, never paste it anywhere
public. Back it up somewhere only you can reach (a USB stick). If it's
lost, a new key needs a new app build that friends must install by hand.

1. Open https://github.com/nycc7yfw75-bot/instrumentTutor/settings/secrets/actions
2. **New repository secret**. Name: `HEADLINER_SIGNING_KEY`. Value: open
   `release.key` in Notepad, copy the one line, paste it.
3. **Add secret.** You should see `HEADLINER_SIGNING_KEY` in the list.

## Once: the repository must be public for updates

The app asks GitHub for new releases without logging in, which only works
on a **public** repository. Your saves and song files are never in the
repo; making it public shows the code only. (Settings → General → Danger
Zone → Change visibility.)

## Each release

1. Add a section to `CHANGELOG.md`, e.g. `## [0.1.0-beta.2]`, with what's new.
2. Commit it on `main`, then tag and push:

       git tag v0.1.0-beta.2
       git push origin v0.1.0-beta.2

   A tag with `-beta` in it is a Beta release; `v1.0.0` (no dash) is Stable.
3. On GitHub → Actions you should see **Release** running (about 10 minutes).
   When it's green, Releases shows the new version with four files:
   `Headliner-Setup-….exe`, `headliner-…-windows-amd64.zip`, `SHA256SUMS`,
   `SHA256SUMS.sig`.
4. Installed copies on the Beta channel offer it the next time they open
   (Stable copies only see Stable releases).

If the Release run fails at "Checksums and signature", the secret is
missing or wrong: redo "Once: let GitHub sign the releases".

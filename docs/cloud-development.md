# Secret-free cloud development

Use a clean checkout at an explicit full commit SHA. The environment needs Go
1.25.13 (or a Go launcher able to download that toolchain), Node 22, pnpm 10,
a C compiler for the race detector, git, curl, jq and tar. Install tools in the
runner image; setup does not use sudo or change persistent tool configuration.

If the runner's default PATH selects other tool versions, configure the optional
non-secret `CLOUD_TOOLCHAIN_BIN` environment variable to **one absolute directory**
containing executable `go`, `node`, `npm` and `pnpm` (symlinks are supported).
The environment owner prepares that directory with preinstalled tools. Both
entrypoints prepend it to PATH only in their own process and children, then
apply the same strict version gates. An empty, relative, missing or incomplete
directory fails immediately; colon-separated PATH lists are rejected. If unset,
the inherited PATH is used. No shell profile, HOME or persistent PATH is changed,
and no tool is installed by this selection step. A saved environment must retain
this variable; activation in an earlier setup process alone does not persist.

Setup / maintenance command (safe to rerun):

```sh
sh scripts/cloud/setup.sh
```

After committing changes, run the same command locally and in the Linux cloud
checkout with the **same literal 40-character commit SHA**:

```sh
sh scripts/cloud/check.sh <full-commit-sha>
```

The check rejects a different HEAD or dirty checkout, rebuilds web assets before
Go compilation, verifies module checksums, builds and vets Go, runs uncached race,
smoke and regression tests, release metadata/installer tests, npm pack dry-run,
Feedback Relay host verification and the exact-SHA Foundation lock contract.
It checks checkout cleanliness again before printing `CHECK PASS` with SHA, OS
and tool versions. Capture the complete output and exit status as evidence.
A failed command stops the check; absence of the final PASS is not success.
This entrypoint does not run performance benchmarks or the tag workflow's
additional golangci-lint/actionlint jobs; the existing release gate still owns
those checks and Node 24 Relay coverage.

Dependencies and generated web assets must be prepared even when Go caches are
warm. Caches live in ignored `.cloud-cache/` under this checkout, including npm,
pnpm, Go and XDG caches. No HOME/CODEX_HOME change, credentials, chat/agent CLI
login, daemon/service installation, deployment, tag, publication, persistent
permission change or background polling is part of setup/check. The tilde-path
unit test uses testing.T.Setenv (automatically restored) and a temporary home;
it does not change the runner's global HOME or write to its real home.

Network access is required for package installation and Relay audit even with
some warm caches: npm registry, Go proxy/checksum services and their download
redirects (including storage.googleapis.com), api.github.com and codeload.github.com
for the public Foundation tarball pinned in agent-app-features.lock.json.
Do not add tokens to work around failed public downloads. Report a blocked
hostname and request only the needed network access from the environment owner.

A macOS PASS is local evidence only. Cloud acceptance requires a fresh Linux
checkout/setup/check at the exact candidate SHA, recorded separately from local
results. Skipped upstream tests must be listed with reasons; they are not live
chat integration coverage. Real platform credentials and external agent calls
remain outside this development contract.

CI/release remain tag-only. No PR trigger, workflow_dispatch or scheduler is
added. This command ends with dry-run package checks; normal release permission
and the existing tag release workflow are required for actual publication.

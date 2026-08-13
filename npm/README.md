# CC Connect Feishu Plus npm bootstrap

This package is the safe installation and diagnostics entrypoint for
[CC Connect Feishu Plus](https://github.com/timmyagentic/cc-connect-feishu-plus).
It does not run a second Feishu connection and does not rewrite secrets.

Planned public entrypoint:

```bash
npx cc-connect-feishu-plus@latest install
```

The foundation build intentionally supports inspection and installation
planning only:

```bash
# Inspect the native binary, config path, service metadata, and Plus state.
node ./cli.js doctor
node ./cli.js doctor --json

# Produce the exact plan without downloading, writing, stopping, or restarting.
node ./cli.js install --dry-run
node ./cli.js install --dry-run --json
```

`doctor` reads only enough metadata to report paths, version, permissions, and
whether `plus_enabled = true` is present. It never returns configuration
contents or credentials.

Applying the plan remains disabled until all of these release gates exist:

- a versioned GitHub Release for each supported OS/architecture;
- a signed manifest and SHA-256 verification;
- immutable version directories plus an atomic `current` pointer;
- backup of the existing service entry and config metadata;
- post-activation health checks and automatic rollback.

The npm package is marked `private` during this foundation phase so it cannot be
published accidentally before those gates and the upstream licensing status are
resolved.

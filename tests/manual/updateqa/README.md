# Real-client update QA

This opt-in harness uses the production Engine, Feishu adapter, immutable plan
bridge, and installer. It substitutes a loopback release source and an agent that
supports command testing without a model call. It is excluded from normal builds.

Run from the repository root on macOS or Linux:

```sh
python3 tests/manual/updateqa/fixture.py --root /tmp/cc-update-qa-example --delay 30
```

The directory must be new and named `cc-update-qa-*`. The script builds two QA
versions, creates an isolated npm prefix, and serves release metadata, a package,
and checksum-verified binary archives on an ephemeral loopback port. In the
fixture's copy of `npm/install.js`, only the two download origins change; package
selection, npm installation, extraction, and checksum verification remain real.
No package is published and no global npm configuration is changed.

Create a private `config.toml` in that directory with mode 0600. Use a dedicated
bot that is not already served by another receiver, an explicit one-user
`allow_from`, and the same explicit `admin_from`. Keep credentials out of the
repository and artifacts. The project name must be `update-experience-qa`, and
`data_dir` must be inside the isolated directory. Only the standard `data_dir`,
`projects`, `admin_from`, and `platforms` TOML tables are read.

Read the loopback origin and executable from `fixture.json`, then start the
executable with `--config <private-config> --fixture <origin> --notice`, setting
`npm_config_registry` to that origin and `npm_config_cache` to a directory inside
the fixture. Start it with the isolated directory as its working directory.
The optional notice goes only to the configured administrator. Do not synthesize
inbound platform events: send commands and click buttons from the real client.

Verify these user actions:

1. Open the reminder and inspect the compact confirmation, including restart
   impact and the separate full-notes link.
2. Send `/shell sleep 30; echo QA_TASK_FINISHED`, confirm the update, and cancel
   while waiting. The task must still finish and no installation must occur.
3. Check again, confirm once, and inspect the same card through the real slow npm
   install, restart, and connection recovery. Progress must not show a percentage
   or append a message for each elapsed-time refresh.
4. Send `/version` after reconnection. Compare the installed binary SHA-256 with
   `fixture.json`, package metadata with the target version, and the saved result's
   card ID with the original card. Check that the private config is unchanged.
5. Save a real screen recording of the chat area. Keep private sidebars, routing
   IDs, callback payloads and raw logs out of public screenshots or PRs. The SDK
   may log raw callbacks; extract only allowlisted stage/result fields as evidence.

The fixture's target is another build of the same branch with a different QA
version; it does not exercise a published/signed release, a paid model, mobile
layout, or launchd/TCC permission migration. State those boundaries with results.
Stop the receiver and loopback server after testing, and remove temporary
credential files. Keep the final recording and sanitized result evidence.

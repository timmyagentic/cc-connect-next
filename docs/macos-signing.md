# macOS release signing and permission continuity

Issue: [#166](https://github.com/timmyagentic/cc-connect-next/issues/166).

## Product identity

Both Darwin architectures use Developer ID Application signing with identifier
`com.timmyagentic.cc-connect-next` and Team ID `4356B4HF9R`. These values are pinned
in `scripts/macos-signing.js`; do not add a version, architecture, or install path.
`codesign` generates the normal Developer ID designated requirement (DR). The
scripts never replace it with a weaker custom requirement or an ad hoc signature.
Certificate renewal must retain the team and compatible DR, and must pass the
same cross-build checks before distribution.

## Release flow

The tag-only Release workflow keeps validation on Ubuntu. Build and publication
run on hosted macOS runners so every publication path can use Apple's verifier:

1. Build the Web assets and all six native binaries.
2. Sign Darwin amd64 and arm64 with a secure timestamp and Hardened Runtime.
3. Verify the Apple Developer ID certificate chain, pinned team/identifier,
   timestamp/runtime flags, and mutually compatible DRs.
4. Package the signed binaries using the existing names, then generate
   `checksums.txt` from the final archives.
5. Extract and verify the archive signatures before artifact upload, after
   download, when reusing an immutable Release, and after public download.

Signature failure or missing credentials stops publication. Historical unsigned
assets cannot be made acceptable by rerunning the workflow or overwriting them;
publish a new version after the release is explicitly authorized.

`make release-all` now requires macOS and `MACOS_SIGN_IDENTITY` selecting a valid
Developer ID Application identity. Ordinary `make build` and `make build-noweb`
remain available for development. For local release preparation:

```sh
MACOS_SIGN_IDENTITY='<Developer ID Application identity or certificate SHA-1>' make release-all
node scripts/check-release-assets.js dist --require-macos-signatures
```

The native macOS checker validates extracted signatures automatically. On other
hosts the normal checker verifies archive structure/checksums and explicitly
reports that it did not verify signatures. Passing `--require-macos-signatures`
on another host fails; it cannot stand in for the macOS publication gate.

## CI certificate setup

An authorized maintainer must provision these repository Actions Secrets through
the GitHub UI or a secure local secret-management flow:

- `MACOS_CERTIFICATE_P12_BASE64`: the Developer ID Application identity, including
  its private key, exported as a password-protected PKCS#12 and Base64 encoded.
- `MACOS_CERTIFICATE_PASSWORD`: the non-empty PKCS#12 password.

Do not put the exported identity or password in an issue, PR, log, or repository.
The trusted release job imports the identity into a temporary keychain, limits
key access to Apple signing tools, and deletes the keychain and export on success
or failure. Child build commands do not inherit the certificate or password.
The certificate is never uploaded as a build artifact. Do not expose signing
Secrets to pull-request workflows or untrusted refs.

Developer ID signing and notarization are separate. This change establishes
stable code identity; it does not claim notarization or Gatekeeper acceptance.
For a notarized distribution, submit the signed code using Apple's notarization
workflow before final packaging/checksums, then run the same artifact checks.

## Validation and real-device acceptance

`node --test scripts/macos-signing.test.js` covers rejection of checksum-valid
ad hoc archives, missing/ad hoc signing identities, and temporary-keychain
cleanup, including import failure. On macOS an authorized local identity can
also exercise two different executable code hashes:

```sh
CCN_TEST_MACOS_SIGN_IDENTITY='<Developer ID Application identity or certificate SHA-1>' \
  node --test scripts/macos-signing.test.js
```

That test verifies both DR directions, execution under Hardened Runtime,
identity after an installer-style rename, and rejection of tampered bytes.
The archive gate also checks DR compatibility across the two Mac architectures.
The npm installer and standalone updater verify archive SHA-256 and preserve the
signed executable bytes during extraction and replacement; neither re-signs code.
With a signed local `dist` fixture, verify both real installer paths without
external downloads or replacing the installed service:

```sh
CCN_TEST_MACOS_RELEASE_DIR="$PWD/dist" node --test scripts/macos-signing.test.js
CCN_TEST_MACOS_RELEASE_DIR="$PWD/dist" CCN_TEST_MACOS_RELEASE_VERSION='<release tag>' \
  go test ./internal/appfeatures -run '^TestIssue166' -count=1 -v
```

The npm case replaces only the HTTP boundary with local archive bytes. The
standalone case uses the real prepared-plan updater with an in-memory source.
Both compare installed bytes and then run the pinned signature verifier.

DR compatibility is necessary evidence, not a real TCC upgrade test. Before
claiming the repeated-permission issue is fixed in a distributed version:

1. Start signed version A as an isolated launchd service on a test Mac.
2. Grant only the directory access required for the test and record the choice.
3. Update through npm and, separately, the standalone updater to signed version B.
4. Restart the service and access the same directory without another prompt.
5. Check Agent child/shell startup and app-server communication under Hardened Runtime.
6. Verify the one-time migration from the old ad hoc identity separately.

Do not clear TCC, widen directory access, change existing services, or reuse
production chat credentials to force acceptance. If any step has not been run,
record it as **UNVERIFIED**. The Agent's private startup brief reports the actual
state directory and directs bounded searches; it does not enumerate files,
read TCC, request permissions, or publish local paths in the public Manifest.

## References

- [Apple TN3127: code signing requirements](https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements)
- [Apple: creating distribution-signed code for macOS](https://developer.apple.com/documentation/xcode/creating-distribution-signed-code-for-the-mac/)
- [GitHub: installing an Apple certificate on macOS runners](https://docs.github.com/en/actions/how-tos/deploy/deploy-to-third-party-platforms/sign-xcode-applications)

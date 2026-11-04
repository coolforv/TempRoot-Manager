# TempRoot Manager V0.1

Temporary explicit Root-session broker prototype for Android 16 / ARM64.

**Important:** The Android app does not obtain root by itself. A broker must be launched by an already-UID-0 GhostLock worker. This prototype is not yet validated on a real Android 16 device and is not a production-ready `su` replacement.

## Layout
- `app/`: Android management UI source.
- `broker/`: Go Unix-domain-socket broker; requires UID 0 at startup.
- `client/`: `trm-su` command-mode client.
- `integration/`: proposed GhostLock worker hook and patch guidance.
- `.github/workflows/`: APK and ARM64 build workflows.

## V0.1 security model
- Broker checks kernel peer credentials using `SO_PEERCRED`.
- Only the configured manager APK UID may grant or revoke app UIDs.
- MT Manager is not pre-authorized; it must be explicitly granted.
- Broker exposes only a local Unix socket, not a network listener.
- Shell is limited to explicit `trm-su -c <command>` requests.
- No boot persistence or automatic Root installation.

## Build
GitHub Actions can build the Android APK and Linux ARM64 executables. See the workflow files. The ARM64 Linux binaries still require Android 16 runtime validation; successful cross-compilation does not prove Android/Bionic compatibility.

## Outstanding device checks
1. Verify GhostLock invokes the integration hook from its existing UID-0 worker.
2. Verify socket creation and access under the device's SELinux policy.
3. Verify the MT Manager package ID and custom-su support on the installed version.
4. Verify `trm-su -c id` returns UID 0 after explicit authorization.
5. Verify broker termination and grant cleanup when the temporary Root session ends.

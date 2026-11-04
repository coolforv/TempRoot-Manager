# TempRoot Manager V0.2 — preview

This repository builds an Android 16 / ARM64 **Root authorization manager prototype**.
Its APK includes a management UI and ARM64 `trm-broker` / `trm-su` assets.

## What works in V0.2

- The Android APK builds using GitHub Actions and contains both ARM64 binaries.
- The manager UI shows broker connectivity and granted/pending app UIDs.
- A first unapproved `trm-su -c 'id'` request is refused, and its caller UID appears in the manager's pending list.
- The manager can explicitly approve, deny and revoke an app UID through the broker.
- The broker verifies the Unix socket peer's UID using `SO_PEERCRED` and refuses management operations from unrelated applications.
- Grant data is kept in `/data/local/tmp/temproot/grants`, not `/data/adb`.
- No KernelSU, Magisk or `libksud.so` is included or installed.

## Important compatibility boundary

**V0.2 is NOT yet an end-to-end GhostLock-to-MT root authorization solution.**

GhostLock's existing manager selection is meant for apps containing the KernelSU daemon library. It **will not** recognize TempRoot Manager, and this project does not change that behavior.

A standalone APK cannot inherit the root identity of GhostLock's separate UID-0 process. The trusted GhostLock worker would have to deliberately launch the broker or provide a documented integration interface. No such integration has been verified in this repository.

The APK-bundled `trm-broker` and `trm-su` are inert assets until they are extracted and started by an **already UID-0 trusted worker**. Installing the APK alone cannot grant root.

## Build

Run GitHub Actions `build-temproot-manager`. Download the `TempRootManager-V0.2-debug-apk` artifact and unpack its `app-debug.apk`.

The workflow runs Go tests and static analysis, builds Linux ARM64 native binaries, packages both in APK assets, then checks that the APK includes these assets and classes.dex.

## Service integration checklist (requires a supported GhostLock worker)

1. Verify legitimate UID 0 in the worker and the Android 16 security context.
2. Identify the exact application UID installed for `com.temproot.manager`.
3. The worker must explicitly deploy the two matching ARM64 assets from the **same** APK build and invoke `trm-broker --manager-uid <UID>` without modifying `/data/adb`.
4. Check `/data/local/tmp/temproot.sock` existence and SELinux permission to communicate.
5. Launch a trusted test app using the companion `trm-su -c id` client and confirm a pending request appears; approve it in the UI, then retry to confirm `id` returns UID 0.
6. Revoke the grant and verify the next request is refused.
7. Test cleanup when the root session ends. Never treat successful build as successful runtime validation.

This prototype supports command mode only. It does not yet offer PTY-backed interactive shells or compatibility with arbitrary third-party root programs; MT Manager is **not** verified.

## Security notes

The local Unix socket has mode 0666 only for access discovery; authorization is based on kernel-provided peer credentials. This does not by itself prove SELinux access. In a production implementation, scope socket access by policy and verify the manager's package identity, signing identity and UID-reuse behavior. Do not disable SELinux to fix connection failures. Do not use on a primary device without audit and backups.

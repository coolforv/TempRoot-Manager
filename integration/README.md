# GhostLock integration

This hook is a proposal. Copy the broker binary and script to /data/local/tmp and invoke it only from GhostLock's existing UID-0 worker after its current script completes. Do not launch it from an unprivileged APK.

The hook has not been applied to GhostLock or tested on a target device. Verify SELinux labels and policy. Do not disable SELinux as a workaround.

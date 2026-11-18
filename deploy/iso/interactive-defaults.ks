# Interactive Anaconda bootc defaults; no disk partitioning, clearpart,
# auto-install, embedded passwords or remote download. The payload is stored
# in this ISO's local containers-storage image store at build time.
# The target imgref is a CI-only placeholder, NOT a valid update origin.
bootc --source-imgref containers-storage:localhost/bitriver-studio-os:ci --target-imgref localhost/bitriver-studio-os:ci

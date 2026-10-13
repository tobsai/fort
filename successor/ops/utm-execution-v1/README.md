# Disabled UTM source preparation

This revision implements fixture-tested protocol, controller, evidence and
ownership components for the bounded synthetic UTM proposal. It is not a working
VM application or a complete execution packet. No launch path is enabled.

Read [EXECUTION-REVISION.md](EXECUTION-REVISION.md) for corrected F1/F2/F3 behavior,
what was actually tested, and the source and runtime gates that remain open.
The original inert packet at `../utm-synthetic-packet-v1/` stays unchanged.

Run the local fixture suite with the existing Python standard library:

```sh
/usr/bin/python3 -I -B tests/run.py
```

The IPC tests bind temporary AF_UNIX sockets under `/tmp/fort-ipc-*`. This host's
sandbox required a narrow automatic approval for those tests. Other transport
tests use anonymous socketpairs; lifecycle tests create and remove their own
temporary directories. No IP listener, provider, VM or system service is used.
`tests/support.py` labels all guest/systemd facts as synthetic boundary doubles.

`fort_probe/__main__.py` always exits 78. It accepts no activation flag. Seed
preview returns bytes in memory; `package_bytes()` always refuses VM output.
The disabled entry point is a workflow gate, not a sandbox against importing or
editing the source. The root broker, Mac owner, UTM and guest kernel are trusted
in this design. Same-user host attackers and shared hosting are out of scope.

The active slice contains only this directory. Existing specs, the quarantined
upstream import, historical evidence and reviewer-owned `reviews/` were not
changed. No dependency, download, install hook, credential or account is added.

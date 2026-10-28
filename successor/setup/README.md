# Setting up the Mac mini and the laptop as runners

The hub on Railway routes Fort's turns; your Macs run them (Spec 063). On Oct 28 you chose the
Mac mini as the primary and the laptop as overflow, with no cloud runner. Once the hub has
`FORT_RUNNER_POOL` set (PR #73), each run goes to the mini, or to the laptop while the mini is away.
With neither connected, runs wait in the queue, and the hub alerts after 15 minutes.

Nothing here runs by itself. Each step that changes a Mac or the hub is yours.

## On the Mac mini

1. Install the Fort Mac build, open it, and sign in to ChatGPT and Claude. Use the same accounts
   as the laptop: a Mac on another account refuses turns rather than running them on that account.
2. Check it: `successor/setup/mac-runner-check.sh --role primary`. The check is read-only.
3. Power: `successor/setup/mac-runner-power.sh` shows the changes, and `--apply` makes them (sudo).
   It sets: never sleep on power, start up after a power failure, wake for network access, and shut down
   cleanly at 20% UPS battery (change that with `--halt-at`).
4. Let Fort's host start at login, in Fort's settings. This is a startup item, so it's your call.
5. Enroll it. In the web app, open Setup, Macs, Add a Mac, then paste the invitation into Fort on the
   mini, under Devices, Connect to cloud hub, within 5 minutes.

FileVault stays on (decided Oct 28). The catch: after a power cut the mini restarts to the
unlock screen, and Fort can't run until you log in. Runs queue meanwhile, and the hub alerts after
15 minutes. The UPS covers short outages. A planned restart can skip the unlock screen with
`sudo fdesetup authrestart`.

## On the laptop

1. Same accounts as the mini, then `successor/setup/mac-runner-check.sh --role overflow`.
2. Enroll it the same way. The laptop keeps its own power settings, and no startup item is needed:
   it takes work only while Fort is open.

## On the hub (Railway variables, yours to set)

`FORT_NODE` must stay as it is. The ledger is bound to it, and the hub refuses to start if it
changes. The pool lists runner IDs, primary first:

```sh
FORT_RUNNER_POOL=toby-mini,<current FORT_NODE value>
FORT_QUEUE_ALERT_MINUTES=15
```

If the Mac enrolled today is the laptop under the `FORT_NODE` ID, the mini joins as a new ID
(`toby-mini` above) and the laptop keeps its enrollment. If the Mac enrolled today is the mini, swap
the order and give the laptop the new ID. The web app's Add a Mac names the next pool ID that
isn't enrolled yet. Removing `FORT_RUNNER_POOL` puts the hub back on one Mac.

The Mac history upload comes after this, once the hub is pointed at the mini.

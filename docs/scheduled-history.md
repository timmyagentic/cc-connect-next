# Scheduled execution history

Execution history is **disabled by default**. To enable it, add this global
section to `config.toml` and restart the daemon:

```toml
[history]
enabled = true
```

When enabled, the daemon writes `history.sqlite` under its effective `data_dir`
using the existing SQLite dependency. Omission or `enabled = false` does not
open, create, recover, or write the history database. Existing history is
retained when disabled; runs during that time are not recorded or backfilled
automatically. Re-enabling resumes recording; startup recovery can mark earlier
unfinished waits as interrupted without inventing their actual completion time.

Cron and Timer definitions continue to be scheduled
from their existing JSON stores. History adds no scheduler, service or model turn.

Every scheduler attempt gets a random `run_id` and an immutable definition
version. Concurrent invocations of the same manual worker have separate rows.
Definition edits add versions; changes to `LastRun`, `LastError` or timer firing
state do not create false definition revisions. The version's `observed_at` is
when that definition was recorded, not a reconstructed historical edit time.

## Outcomes and identity

- `scheduler_status` records the scheduler's wait: `running`, `succeeded`,
  `failed`, `timed_out` or `interrupted`.
- `execution_status` records the dispatched engine: `not_started`, `running`,
  `succeeded`, `failed` or `unknown`. If execution returns after a scheduler
  timeout, its actual result is retained without overwriting `timed_out`.
- Startup, under the daemon instance lock, marks unfinished waits as interrupted.
  It records an observation time and leaves unproven execution end times empty.
- Prompt runs link their cc-connect session and actual agent session IDs.
  Shell jobs receive `CC_SCHEDULED_RUN_ID` for linking their own business receipts.
- Scheduler success is not proof of a business decision, notification delivery,
  or external repair. Those facts belong to the linked business receipts.

The private database contains the original task definition, including Prompt or
Exec, for local audit. It is created with mode `0600`; diagnostics use the
existing credential redaction and output limit. Database errors are logged and
do not stop existing jobs. Monitoring history-write warnings remains necessary:
history is observational, and no claim of complete recording is made across a
storage failure.

## Tables and retained sources

| Table | Purpose |
| --- | --- |
| `job_versions` | Immutable task definitions, observed version identity and archive annotations. |
| `runs` | Native execution identity, trigger, timings, separate scheduler/engine outcomes and session links. |
| `legacy_events` | Imported pre-existing log events and saved runtime snapshots whose missing run identity must remain unknown. |
| `evidence_refs` | Content-hashed references to business completion and verified notification records; source artifacts retain their ownership. |

The latter two tables support explicit local import tools. Missing historical
starts or ambiguous concurrent completions must not be manufactured into runs.
Exact native run/session identities can link receipts; time proximity alone
cannot. Reports must count native runs separately from legacy observations.

SQLite uses WAL so readers can inspect committed records during execution.
Use SQLite's backup API, rather than copying just the main file while WAL is
active. Restoring an older history copy must be a deliberate offline operation;
it does not restore or change scheduling/business state.

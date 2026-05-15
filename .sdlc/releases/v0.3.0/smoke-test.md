# v0.3.0 Smoke Test

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

Use this UI runthrough before release close/tagging. It exercises the
production shell and the v0.3.0 durable-run surface without requiring a
low-level JSON-RPC client.

## Prerequisites

- Build the binary: `make build`
- Use a copied test database, not your primary local database.
- Set at least one provider key, for example `ANTHROPIC_API_KEY` or
  `OPENAI_API_KEY`.

## Steps

1. Launch the production shell.

   ```sh
   ./bin/modeltap shell --project "$(pwd)"
   ```

   Expected: the Bubble Tea shell opens, connects to the local BFF, and shows
   an input box.

2. Confirm provider/model availability.

   In the shell, type `/models` and press Enter.

   Expected: model list renders with at least one usable model.

3. Submit a simple foreground turn.

   Type `Reply with exactly: v0.3.0 smoke ok` and press Enter.

   Expected: the assistant streams a normal response and the transcript remains
   usable.

4. Inspect the active run.

   Type `/run` and press Enter.

   Expected: run summary appears with a `run-...` ID, session ID, status,
   stage, attachment state, and no obvious error.

5. Confirm run completion.

   Check the status in the `/run` output.

   Expected: for the simple prompt, status is `completed`; stage is at or near
   `completion`.

6. List recent runs.

   Type `/runs` and press Enter.

   Expected: recent run rows render. The latest row matches the prompt you just
   sent and shows the same `run-...` ID.

7. Check the jobs alias.

   Type `/jobs` and press Enter.

   Expected: output is equivalent to `/runs`.

8. Fetch a specific run.

   Copy the `run-...` ID from `/run` or `/runs`, then type `/run <run-id>` and
   press Enter.

   Expected: details for that specific run render without changing normal chat
   behavior.

9. Detach the active/completed run.

   Type `/detach` and press Enter.

   Expected: shell reports detached state or no active attachment. Transcript
   remains intact.

10. Try attaching the completed run.

    Type `/attach <run-id>` and press Enter.

    Expected: completed/terminal runs are rejected or do not attach as active
    work. This is acceptable for v0.3.0.

11. Smoke test cancellation.

    Submit a longer prompt such as `Count slowly from 1 to 200 with one number
    per line.`

    While it is running, use the shell interrupt/cancel path, or type
    `/cancel <run-id>` if command input is available.

    Expected: run becomes `cancelled`, or the shell reports
    cancellation/interrupt cleanly without freezing.

12. Re-check the run list after cancellation.

    Type `/runs` and press Enter.

    Expected: the cancelled run appears with status `cancelled`; the previous
    completed run still appears as `completed`.

13. Exit cleanly.

    Type `/exit` and press Enter.

    Expected: shell exits without panic or stuck terminal state.

## Optional Checks

- If you can force a provider stream failure, confirm logs include `run_id` and
  `turn_id` context.
- If you have a low-level JSON-RPC client, send two `turn.submit` requests with
  the same `idempotency_key` and confirm both responses return the same
  `run_id`. The normal shell UI does not expose this directly.

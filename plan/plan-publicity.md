# Plan: Publicity

Staggered approach — collect feedback and improve before each step up in
audience size. Blog posts are not needed until the final stage.

## Step 1: Reduce friction (prerequisites)

- [x] Do this before any announcement. First impressions are hard to undo. See
      `plan-readme-fixup.md` for the full checklist.

## Step 2: Docker Hub presence

Passive discoverability — anyone who finds the image lands here first.

- [x] Improve the Docker Hub description to include the comparison with the
      built-in journald driver and a one-line install example.

## Step 3: Niche communities

Small, expert audiences. Low stakes, high signal feedback. No blog posts needed
yet.

- [ ] Post in the **Docker community forums** in any existing log driver or
      journald threads — answer questions, don't just announce.
- [ ] Post on **systemd discourse / mailing list** — these are exactly the
      people who understand the journald angle and will give honest critique.

## Step 4: Collect feedback and improve

Pause here. Read responses, fix rough edges, update docs. Don't proceed until
the project feels solid under scrutiny.

## Step 5: Mid-size communities

- [ ] Post on **r/selfhosted** — strong on-premise/systemd user base, highly
      relevant audience, manageable scale.
- [ ] Post on **lobste.rs** — smaller and more technical than HN; good
      intermediate step before a full Show HN.

## Step 6: Write blog posts

Write these after early feedback has shaped the narrative. Each post should
stand alone and link to the project.

- [ ] **"The case for journalctl"** — argue that Docker's default JSON log
      storage is a bad idea for all but the most trivial use cases (no
      structured fields, no priority, rotation managed separately, no unified
      view across services), and that having all container logs in the systemd
      journal makes far more sense operationally. Broaden to cover journald as a
      serious alternative to rsyslog/syslog-ng and to third-party SaaS log
      storage (Datadog, Loki, etc.) for self-hosted and on-premise deployments.
      Cover structured fields, priority filtering, persistent storage, binary
      journal format, and cost. Position journald not as a legacy syslog
      replacement but as a modern structured log store that ships with every
      systemd host.
- [ ] **"Twelve-factor logging with journald-plus"** — show concretely how the
      plugin satisfies the twelve-factor app logging contract (treat logs as
      event streams, write to stdout/stderr) while adding what the spec leaves
      out: priority, structure, multiline merging, and queryability. Use a
      realistic example app (e.g. a Go or Python service with JSON logs) and
      walk through the full lifecycle from `docker run` to `journalctl` query.

## Step 7: Major announcements

Only once blog posts are published and the project has absorbed at least one
round of community feedback.

- [ ] **Show HN** on Hacker News — the journald/systemd audience is present and
      technical; pair with the blog post.
- [ ] Post on **r/docker** — larger and less targeted than r/selfhosted, so save
      it for last.

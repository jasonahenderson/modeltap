---
exploration: EXP-0005
title: Apprenticeship Program
status: exploring
date: 2026-04-09
related:
  - EXP-0001
---

# EXP-0005: Apprenticeship Program

## Problem

AI coding tools are absorbing the tasks that traditionally trained junior engineers — writing boilerplate, fixing simple bugs, implementing well-defined features. The path from junior to senior has historically been "do 1,000 small tasks, accumulate judgment." That path is disappearing.

Meanwhile, senior engineers are solving harder problems with AI daily, and their process — how they decompose problems, evaluate AI output, course-correct, make architectural decisions — is invisible to the rest of the org. It lives in their heads and disappears when the terminal closes.

There is no scalable way to:
1. Teach junior engineers the judgment and decision-making that makes someone senior
2. Teach experienced engineers how to work effectively with AI
3. Capture and share the tacit knowledge that senior engineers exercise every day

## How Modeltap Solves This

Modeltap already captures every AI interaction flowing through the proxy. The apprenticeship module adds **session recording, annotation, and export** on top of that capture. It turns the proxy into a content source — the raw material that feeds whatever learning, curriculum, and knowledge systems the org already uses.

**Modeltap owns the capture.** Everything downstream — curriculum management, Q&A, learning paths, progression tracking — happens in the org's existing tools via export and integrations.

This is modeled on trades apprenticeships, where learning happens on the job site through observation, supervised practice, and graduated independence — not through courses or documentation.

## Boundary: What Modeltap Owns vs. What It Doesn't

| Modeltap Owns | Org's Existing Tools Handle |
|---------------|---------------------------|
| Session recording (start/stop/link interactions) | Curriculum assembly and sequencing |
| Inline annotations on sessions | Q&A and threaded discussion |
| Session export (structured, portable formats) | Content hosting and discovery |
| Webhooks for session lifecycle events | Notifications and workflow automation |
| Session sharing and access control | Progression tracking and HR integration |
| Dashboard for session review and annotation | Learning path management |
| Basic session browsing and search | Knowledge base and search |

Modeltap is the **camera and the editing room**. It captures the raw footage (AI interactions), lets the mentor annotate it (add teaching context), and exports a finished product. Where that product lives — Notion, Confluence, GitHub, an LMS, a shared drive — is the org's choice.

## Core Concepts

### Sessions

A **session** is a bounded sequence of AI interactions tied to a task. Sessions are the fundamental unit — not individual requests, not unbounded streams.

Examples of sessions:
- "Debug a race condition in the connection pool" (45 min, 12 interactions)
- "Design the schema for the notification system" (2 hours, 30 interactions)
- "Review and refactor the auth middleware" (20 min, 8 interactions)

Sessions have:
- A **title** and **description** (what the engineer was trying to accomplish)
- A **start** and **end** (explicit, not inferred — the engineer decides when a session begins and ends)
- All captured AI interactions within the time window
- Optional **annotations** added during or after the session

### Annotations

An **annotation** is a note attached to a specific point in a session. Annotations are the teaching mechanism — they provide the context that raw interaction logs lack.

Types of annotations:
- **Context**: "I chose to approach it this way because our deployment pipeline doesn't support blue-green" — explains the *why* behind a decision
- **Teachable moment**: "Notice how I rejected the AI's first suggestion — it used a mutex when a channel would be more idiomatic" — highlights what to learn
- **Correction**: "You went down a rabbit hole here. Next time, check the error type first before re-prompting" — mentor feedback on apprentice work
- **Question**: "Why did you use a map here instead of a slice?" — mentor prompting the apprentice to think

### Roles

| Role | Description |
|------|-------------|
| **Apprentice** | Junior or mid-level engineer learning through observation and supervised practice |
| **Mentor** | Senior engineer who shares their work and reviews apprentice interactions |

Roles are additive — a mentor can also be an apprentice in a different domain. Role management, pairing, progression tracking, and program administration happen outside modeltap (in the org's people/project management tools).

### Pairings

A **pairing** connects a mentor with an apprentice for sharing purposes. Modeltap tracks pairings to control who can see whose sessions. The pairing's focus area, cadence, and program structure are managed externally.

```
modeltap pairing create --mentor alice --apprentice bob
modeltap pairing list
modeltap pairing remove <pairing-id>
```

## Key Capabilities

### 1. Session Recording

Engineers explicitly start and stop sessions when working on tasks they want to be observable.

```
modeltap session start "Debug connection pool race condition"
# ... engineer works with AI normally ...
modeltap session end

modeltap session start "Design notification schema" --description "Need to support email, push, and in-app notifications with user preferences"
# ... engineer works ...
modeltap session end --annotate  # opens annotation prompt before closing
```

Sessions are **opt-in per task**. An engineer might record 3 out of 10 tasks in a day. This is intentional — it addresses the surveillance concern by giving the engineer full control over what is shared.

```
modeltap session list --mine --since 7d
modeltap session show <session-id>
```

### 2. Session Annotation

Annotations are added via CLI (during or after a session) or via the dashboard (during review).

```
modeltap session annotate <session-id> --at <interaction-index> "This is where I realized the AI was hallucinating a non-existent API"
modeltap session annotate <session-id> --at <interaction-index> --type correction "You accepted this without checking — the AI generated a SQL query vulnerable to injection"
modeltap session annotate <session-id> --type context "Overall: solid debugging process, but validate AI-generated SQL before accepting"
```

### 3. Session Sharing

Engineers share sessions with their paired mentor or apprentice. Sharing is always explicit.

```
modeltap session share <session-id> --with bob
modeltap session list --shared-with-me
modeltap session list --shared-by-me
```

### 4. Session Review (Dashboard)

The dashboard is where mentors review apprentice sessions and annotate them. This is the core mentor workflow — async, visual, inline.

**Review Workspace**
- Full session timeline: each AI interaction shown as a card (prompt on left, response on right)
- Click any interaction to open an annotation panel — select type, write the note
- Session-level summary field for overall feedback
- Navigation: jump between interactions, filter to show where apprentice accepted/rejected AI output or re-prompted
- Mark review as complete

**My Sessions** (curation view)
- List of own completed sessions with quick actions: **Share**, **Annotate**, **Export**, **Archive**
- Filter by: date range, shared/unshared, annotated/unannotated

**Shared With Me** (apprentice view)
- Mentor's shared sessions with annotations displayed inline
- Reviewed sessions with mentor feedback prominently displayed

### 5. Session Export

This is the integration point. Modeltap exports annotated sessions in structured formats so they can be consumed by whatever the org uses for knowledge management, curriculum, and learning.

```
modeltap session export <session-id> --format markdown > session.md
modeltap session export <session-id> --format json > session.json
modeltap session export <session-id> --format html > session.html
```

**Markdown export** produces a self-contained document:

```markdown
# Debug connection pool race condition
**Engineer:** Alice | **Duration:** 45 min | **Date:** 2026-03-15

## Interaction 1
**Prompt:** I'm seeing intermittent failures in the connection pool...
**Response:** This looks like it could be a race condition...

> 💡 **Teachable moment (Alice):** Notice how I described the symptom
> without guessing the cause. Let the AI help narrow down hypotheses.

## Interaction 2
**Prompt:** [rejected previous suggestion] That uses a mutex, but...
**Response:** You're right, a channel-based approach would be...

> 📝 **Context (Alice):** I rejected the mutex approach because our
> codebase convention is to use channels for goroutine coordination.
> The AI doesn't know our conventions unless you tell it.
```

This markdown can go directly into Notion, Confluence, a GitHub repo, an LMS — anywhere that renders markdown.

**JSON export** provides structured data for programmatic consumption:

```json
{
  "session": {
    "id": "sess_abc123",
    "title": "Debug connection pool race condition",
    "user": "alice",
    "started_at": "2026-03-15T10:00:00Z",
    "ended_at": "2026-03-15T10:45:00Z",
    "interactions": [
      {
        "sequence": 1,
        "request": { "model": "claude-sonnet-4-6", "messages": [...] },
        "response": { "content": [...], "usage": {...} },
        "annotations": [
          {
            "type": "teachable",
            "author": "alice",
            "content": "Notice how I described the symptom without guessing the cause."
          }
        ]
      }
    ]
  }
}
```

**Bulk export** for seeding a curriculum:

```
modeltap session export --user alice --since 90d --annotated-only --format markdown --output-dir ./curriculum/
```

### 6. Webhooks

Session lifecycle events push to external systems so the org can build automation around the apprenticeship workflow.

```yaml
# ~/.config/modeltap/config.yaml
webhooks:
  - url: https://hooks.slack.com/services/xxx
    events: [session.shared, session.reviewed]
  - url: https://notion-integration.internal/modeltap
    events: [session.completed, session.annotated]
  - url: https://lms.internal/api/modeltap
    events: [session.exported]
```

Events:
- `session.started` — engineer began recording
- `session.completed` — engineer ended the session
- `session.annotated` — annotation added to a session
- `session.shared` — session shared with another user
- `session.reviewed` — mentor marked a review as complete
- `session.exported` — session exported to a file

This lets teams build workflows like:
- Slack notification when a mentor shares a new session → apprentice knows to check it
- Auto-export completed, annotated sessions to a Notion database → curriculum builds itself
- LMS integration that tracks which exported sessions an apprentice has viewed

### 7. API Endpoints

For deeper integrations, session data is available via the REST API (extending the existing dashboard API).

```
GET  /api/sessions                    # list sessions (filtered by user, date, status)
GET  /api/sessions/:id               # session detail with interactions and annotations
POST /api/sessions/:id/annotations   # add annotation
POST /api/sessions/:id/share         # share with a user
GET  /api/sessions/:id/export?format=markdown|json|html
GET  /api/pairings                   # list pairings
POST /api/pairings                   # create pairing
```

External tools (LMS, Notion integrations, custom dashboards) can pull session data via these endpoints instead of relying on export files.

## Integration Examples

### "We use Notion for everything"

1. Modeltap webhook fires on `session.reviewed` → hits a Notion integration
2. Integration calls `/api/sessions/:id/export?format=markdown`
3. Creates a new page in the team's "Apprenticeship" database with the exported session
4. Mentor tags and organizes sessions into Notion's own learning path structure
5. Apprentice browses sessions in Notion, leaves comments (Notion's Q&A)

### "We have an internal LMS"

1. Mentor exports annotated sessions via bulk export
2. LMS ingests JSON exports as learning modules
3. LMS handles sequencing, progress tracking, quizzes, completion certificates
4. Modeltap webhook notifies LMS when new content is available

### "We just use GitHub"

1. Mentor exports sessions as markdown to a `curriculum/` directory in a repo
2. Apprentice reads sessions via GitHub, opens issues with questions
3. Learning path is a `README.md` that links sessions in order
4. Progression is tracked via GitHub project boards

### "We use Slack for async communication"

1. Webhook fires on `session.shared` → posts to a `#apprenticeship` channel
2. Apprentice watches the session in the modeltap dashboard
3. Q&A happens in a Slack thread linked to the session
4. Mentor answers in Slack rather than back in modeltap

## Dashboard Views

The modeltap dashboard handles the things that need to be visual and inline — session review and annotation. It does NOT try to be a curriculum platform, LMS, or knowledge base.

### Mentor View

**Review Queue**
- Apprentice sessions awaiting review, ordered by submission date
- Badge count of unreviewed sessions
- Clicking in opens the Review Workspace

**Review Workspace**
- Full session timeline: each AI interaction as a card
- Annotation sidebar: click any interaction to annotate (type + content)
- Session-level summary field
- Navigation: jump between interactions, filter by accepted/rejected/re-prompted
- Mark complete button

**My Sessions**
- Own completed sessions with quick actions: Share, Annotate, Export, Archive
- Bulk export for seeding external curriculum

### Apprentice View

**Shared With Me**
- Mentor's shared sessions with annotations inline
- Reviewed sessions with mentor feedback at top

**My Sessions**
- Own sessions shared with mentor
- Review status: pending, reviewed
- Mentor annotations visible on reviewed sessions

### Design Principles

1. **The dashboard is for review and annotation.** Everything else (curriculum, Q&A, progression) lives in external tools.
2. **Mentor's time is the bottleneck.** The review workspace is optimized for speed.
3. **Inline annotations.** Separating the annotation from the interaction it refers to destroys the value.
4. **Consistent with existing dashboard.** Apprenticeship views are tabs within the existing modeltap web dashboard.

## Privacy Model

This feature is designed around **explicit opt-in at every level**.

| Action | Who Decides | Default |
|--------|------------|---------|
| Recording a session | The engineer | Off — must explicitly start |
| Sharing a session with paired user | The engineer who recorded it | Off — must explicitly share |
| Exporting a session | The user with access to it | Available to session owner and shared users |
| Webhook delivery | Configured by admin | Off — must configure endpoints |

**What is never shared**: Sessions the engineer didn't explicitly record. Interactions outside of active sessions. Content with users outside the pairing.

## Configuration

```yaml
# ~/.config/modeltap/config.yaml
apprenticeship:
  enabled: true

  # Personal settings
  auto_annotate_prompt: true  # prompt for annotations when ending a session
  default_share: false        # never auto-share sessions

  # Export defaults
  export:
    default_format: markdown  # markdown, json, html
    include_raw_requests: false  # include full API request/response bodies
    anonymize_by_default: false

  # Webhooks
  webhooks:
    - url: https://hooks.slack.com/services/xxx
      events: [session.shared, session.reviewed]
    - url: https://notion-integration.internal/modeltap
      events: [session.completed]
```

## Data Model

### New Tables

```sql
-- Sessions: bounded sequences of interactions
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    status TEXT NOT NULL DEFAULT 'active',  -- active, completed, archived
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Session interactions: links sessions to captured requests
CREATE TABLE session_interactions (
    session_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    sequence_num INTEGER NOT NULL,
    PRIMARY KEY (session_id, request_id),
    FOREIGN KEY (session_id) REFERENCES sessions(id),
    FOREIGN KEY (request_id) REFERENCES requests(id)
);

-- Annotations: teaching notes attached to sessions or specific interactions
CREATE TABLE annotations (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    request_id TEXT,  -- NULL if annotation is on the session as a whole
    author_id TEXT NOT NULL,
    type TEXT NOT NULL,  -- context, teachable, correction, question
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id),
    FOREIGN KEY (author_id) REFERENCES users(id)
);

-- Pairings: mentor-apprentice relationships for access control
CREATE TABLE pairings (
    id TEXT PRIMARY KEY,
    mentor_id TEXT NOT NULL,
    apprentice_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',  -- active, inactive
    created_at DATETIME NOT NULL,
    FOREIGN KEY (mentor_id) REFERENCES users(id),
    FOREIGN KEY (apprentice_id) REFERENCES users(id)
);

-- Session sharing: controls who can see a session
CREATE TABLE session_shares (
    session_id TEXT NOT NULL,
    shared_with_id TEXT NOT NULL,
    shared_by_id TEXT NOT NULL,
    shared_at DATETIME NOT NULL,
    PRIMARY KEY (session_id, shared_with_id),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);
```

Note: No tables for patterns, learning paths, progression, or Q&A. Those concerns live in external systems. The data model is focused on what modeltap uniquely provides: sessions, annotations, pairings, and sharing.

## Dependencies

- **Multi-user support** — required for user identification, pairings, and access control
- **Web dashboard** — required for session review and annotation UI
- **Capture (ADR-0005)** — sessions reference captured requests, which already exist

## Pilot Program

### Target Team

Ideal pilot: a team of 6-10 engineers with a mix of experience levels (2-3 seniors, 3-5 mid/junior), all using AI coding tools through modeltap.

### Pilot Structure (8 weeks)

**Week 1-2: Setup**
- Deploy multi-user modeltap for the team
- Create 2-3 mentor/apprentice pairings in modeltap
- Set up the external curriculum location (Notion database, GitHub repo, whatever the team uses)
- Configure webhooks to push to Slack / curriculum tool
- Seniors record 2-3 sessions, annotate, and export to seed the curriculum

**Week 3-4: Observation Phase**
- Apprentices review exported mentor sessions in the curriculum tool
- Discussion happens in the team's normal channels (Slack threads, GitHub issues, Notion comments)
- Mentors continue recording sessions during normal work
- Program lead checks engagement via the external tools

**Week 5-6: Supervised Practice**
- Apprentices start recording their own sessions and sharing with mentors
- Mentors review apprentice sessions in the modeltap dashboard, leave annotations
- Reviewed sessions are exported to the curriculum tool for reference

**Week 7-8: Evaluation**
- Retrospective with all participants
- Assess: Did apprentices improve? Did mentors find value? Was the time investment reasonable?
- Decide: Continue, adjust, or abandon

### Success Criteria

Qualitative (more important):
- Apprentices report learning things they wouldn't have learned from code review alone
- Mentors report the time investment is manageable (< 2 hours/week reviewing sessions)
- At least one "I wouldn't have caught that without the session context" moment

Quantitative (supporting):
- Session recording rate: seniors record 3+ sessions/week without prompting
- Review completion rate: mentors review 80%+ of shared apprentice sessions
- Exported sessions are actually referenced by apprentices (measured in external tool)

### What to Watch For (Anti-patterns)

- **Compliance theater**: sessions are recorded but never reviewed
- **Surveillance drift**: session data used for performance evaluation
- **Annotation fatigue**: mentors stop annotating after week 3
- **Selection bias**: engineers only record sessions where they look good
- **Time pressure**: mentors skip reviews because of delivery deadlines
- **Integration friction**: too many steps between modeltap and the external tools

## Phasing

### Phase 1: Sessions and Annotations (MVP)
- Session start/stop/list/show CLI commands
- Annotation CLI commands (inline and session-level)
- Pairing management
- Session sharing between paired users
- Session export (markdown, JSON, HTML)
- Basic session view in web dashboard

### Phase 2: Review Dashboard
- Mentor review workspace with inline annotation
- Review queue with status tracking
- Apprentice view of shared/reviewed sessions
- Bulk export

### Phase 3: Integrations
- Webhook support for session lifecycle events
- REST API endpoints for external tool consumption
- Export templates (customizable markdown/HTML output)

## Relationship to Other Features

| Feature | Relationship |
|---------|-------------|
| Multi-user support | **Required dependency** — user identity, access control, data isolation |
| Web dashboard | **Required dependency** — session review and annotation UI |
| Knowledge layer (ADR-0008) | Complementary — exported sessions could feed into knowledge embeddings for semantic search |
| Metrics (ADR-0007) | Complementary — per-user metrics provide supporting context but are not part of the apprenticeship feature |

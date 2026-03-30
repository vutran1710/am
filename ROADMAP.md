# AMesh Development Roadmap

Each phase ships a concrete, user-visible outcome. Not capability — pain relief.

**Design rule:** "If I install this today, what gets better tomorrow morning?"

---

## Phase 1 — Observe

### Goal

Never miss anything relevant across communication channels. Stop checking email, Slack, Discord manually.

### What ships

A **real-time notification stream** that surfaces relevant messages from all connected services:

- New emails, mentions, DMs, calendar invites
- Filtered by relevance — noise removed
- Delivered to a single place (CLI, notification, or forwarded summary)

### User value

- Stop context-switching between Gmail, Slack, Discord, Calendar
- Get informed about what matters without opening anything
- Nothing relevant falls through the cracks

### Why it works

- Zero behavior change required
- Passive — just connect your accounts and receive
- Foundation for everything that follows

---

## Phase 2 — Analyze & Schedule

### Goal

LLM-powered analysis of incoming messages to build and maintain a personal work schedule. The system creates, adjusts, and interacts with you to keep your day realistic.

### What ships

A **smart daily schedule** that:

- Analyzes incoming messages for tasks, requests, deadlines
- Creates schedule entries automatically
- Adjusts priorities as new information arrives
- Interacts with you to confirm, reschedule, or drop items
- Calibrates based on your actual capacity and patterns

### Example interaction

> "New request from John: review PR by EOD. You have 2 existing commitments today.
> Suggested: slot the review at 2pm, push docs update to tomorrow.
> [Accept] [Adjust] [Ignore]"

### User value

- No manual planning — schedule builds itself from real communication
- Realistic workload — system knows what you can actually do
- Adaptive — reprioritizes as the day evolves

### Why it works

- Turns raw message flow into actionable structure
- LLM understands context, urgency, and implicit deadlines

---

## Phase 3 — Track Commitments

### What ships

A **personal commitments tracker** (auto-generated from Phase 2):

- Detected commitments with status: pending / in-progress / done
- Deadlines (inferred or explicit)
- Reminders (suggested or opt-in auto)

### User value

- No more forgotten promises
- Clear personal execution list

---

## Phase 4 — Dependency Visibility

### What ships

A **dependency view + alerts**:

- "You are waiting on X"
- "Y is waiting on you"
- Blocked tasks highlighted
- Stale dependencies flagged

### User value

- Removes invisible blockers
- Reduces follow-up messages

---

## Phase 5 — Conflict & Overload Detection

### What ships

A **conflict detection system**:

- Overload alerts: "You have 10h of commitments tomorrow"
- Priority conflicts: "Task A and B both urgent but overlap"
- Suggested fixes: reschedule, renegotiate

### User value

- Prevent burnout and unrealistic planning

---

## Phase 6 — Auto Coordination

### What ships

**Assisted actions** (opt-in):

- Auto reminders: "Ping John about Task A?"
- Drafted follow-ups: ready-to-send messages
- Optional auto-send under policy
- Auto status updates to tools (Jira, etc.)

### User value

- Eliminates repetitive coordination work

---

## Phase 7 — Team Intelligence

### What ships

A **manager dashboard**:

- Who is overloaded, where work is stuck
- Critical path risks, dependency map

### User value

- No need to ask for status updates

---

## Phase 8 — Org Optimization

### What ships

**Org-level insights + recommendations**:

- Bottleneck detection, coordination latency metrics
- Workload imbalance analysis
- Suggested structural improvements

### User value

- Strategic improvement, not just operational

---

## Summary

| Phase | Name | Delivers | Pain removed |
|-------|------|----------|-------------|
| 1 | Observe | Real-time relevant notifications | Stop checking 5 apps |
| 2 | Analyze & Schedule | Smart daily schedule from messages | No manual planning |
| 3 | Track Commitments | Auto commitment tracker | No forgotten promises |
| 4 | Dependency Visibility | Who's waiting on who | No invisible blockers |
| 5 | Conflict Detection | Overload & conflict alerts | No unrealistic days |
| 6 | Auto Coordination | Assisted follow-ups & reminders | No repetitive chasing |
| 7 | Team Intelligence | Manager dashboard | No status meetings |
| 8 | Org Optimization | Structural recommendations | No blind spots |

---

## Progression

1. **See everything relevant** (Observe)
2. **Plan your day from it** (Analyze)
3. **Don't forget anything** (Track)
4. **Understand dependencies** (Visibility)
5. **Avoid bad planning** (Detection)
6. **Reduce coordination effort** (Automation)
7. **See team reality** (Intelligence)
8. **Optimize organization** (Strategy)

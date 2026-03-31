# Rules Injection

## Problem

A multi-agent system maintains a large library of rules (coding conventions, architectural guidelines, domain constraints). When one agent delegates work to another, the receiving agent may be unaware of rules relevant to the task — leading to output that violates conventions or requires rework.

## Idea

Agents observe each other's outputs. When an agent detects that a peer produced work that would have been correct had it known about a specific rule, it injects that rule into the peer's context rather than just correcting the output.

## How It Works

1. **Rule library** — a structured collection of rules, each tagged with scope (e.g., file type, domain, phase of work).
2. **Violation detection** — an observing agent recognizes a pattern that matches a known rule the acting agent likely doesn't have loaded.
3. **Rule injection** — the observing agent pushes the relevant rule into the acting agent's context, so future work is informed by it — not just the immediate fix.

## Why This Matters

- Correcting output is reactive; injecting rules is proactive.
- Reduces repeated violations of the same rule across turns.
- Scales better than front-loading every agent with the entire rule library (context window cost).
- Mirrors how human teams work: you don't recite the whole style guide — you point someone to the relevant section when they need it.

## Open Questions

- How does an agent decide which rules to inject vs. which to silently fix?
- Should injected rules persist for the session or expire?
- How to avoid rule injection loops (A injects into B, B injects back into A)?
- What's the priority when injected rules conflict with each other?

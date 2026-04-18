---
name: prompt-engineer
description: >-
  Generate, optimize, review, or audit prompts. Use when the user asks to
  "write a prompt", "create a system prompt", "optimize this prompt",
  "improve this prompt", "review this prompt", "audit this prompt",
  "diagnose this prompt", "审查/评审/诊断/优化/改进/改写 提示词 / prompt /
  system prompt", or discusses prompt engineering strategy.
version: 1.0.0
---

## Core Philosophy

Prompt engineering is translation — converting human needs and expectations into instructions that models process with maximum fidelity and efficiency, achieving the user's actual goals.

This defines the problem structure:
- **Source language**: The user's real needs, expectations, and success criteria
- **Target language**: The model's cognitive processing mechanisms
- **Translation quality** = Fidelity (faithful to user intent) × Efficiency (minimal loss in transmission)

A good translator preserves everything the original contains, adds nothing the original lacks, and understands both languages deeply. These three properties drive every decision below.

The translation metaphor is the single root framework. Backward induction, encoding layers, quality checks, and anti-patterns below are all derived implementations of this metaphor — a single unified framework.

## Goal

When this skill activates, produce a prompt (generation mode) or a revised prompt with change rationale (optimization mode) that maximizes translation fidelity and efficiency. The output is complete when it passes all three Quality Verification checks at the end of this document.

**Reference files** (load when indicated in the workflow):
- `references/anti-patterns.md` — error taxonomy for diagnosis in optimization mode
- `references/cognitive-mechanisms.md` — three cognitive mechanisms that explain *why* translation strategies work (forward propagation advantage, attention position effect, induction head generalization). Load when diagnosing why an existing prompt underperforms, facing an edge case not covered by the anti-pattern table, or the user asks *why* a particular strategy is recommended
- `references/empirical-verification.md` — test-based verification workflow (design, baseline, diagnose, iterate). Load when the prompt will be deployed in a setting where output quality is directly observable and translation errors carry material cost

---

## Understanding the Source: Goal Backward Induction

Most prompt failures trace back to misunderstanding the source — the gap between what users say and what they actually need.

### The Backward Induction Chain

Derive prompts backwards from the end goal. Each level is derived from the one above it:

- **Ultimate goal & success criteria** — What outcome does the user actually need? Define what "done" looks like in concrete, verifiable terms.
  - **Core idea** — What is the first principle of this task? This is the single insight that, if understood, makes every subsequent decision self-evident. Example: A contract generation task's core idea is "protect the party's interests at minimum legal cost when breach occurs" — this reframes every downstream choice.
    - **Behavioral motivation** — Why is each constraint necessary? Each motivation traces directly to the core idea via a "because..." link.
      - **Behavioral rules** — The specific requirements and constraints. Each rule traces to a motivation, which traces to the core idea, which traces to the user's goal.

When the user's goal is ambiguous or underspecified, present a goal hypothesis for confirmation before proceeding ("Based on X, I understand the goal as Y — is this correct?"), because a translation built on a misunderstood source will be faithfully wrong.

### Traceability Test

Every rule in the generated prompt must survive this test: trace it back through motivation → core idea → user goal. Retain only rules whose chain is complete — an unauthorized addition (rule without traceable motivation) or hollow decoration (motivation without a concrete rule) indicates a translation error to be corrected.

When two rules each pass the traceability test individually but produce contradictory directives, resolve at the motivation level: trace both back to the core idea, determine which motivation is more central to the user's goal, and retain that rule — because contradictions left unresolved force the model to make an arbitrary choice the user did not authorize.

---

## Translation Workflow

### Generation Mode (new prompt)

Derive the prompt backwards from the user's end goal through the backward induction chain, encode using the Encoding Framework, and verify against Quality Verification checks before presenting — because working forward from the first idea that comes to mind typically encodes the translator's assumptions rather than the user's actual needs.

### Optimization Mode (existing prompt)

Diagnose translation errors in the existing prompt using `references/anti-patterns.md`, trace each error to its cognitive root cause via `references/cognitive-mechanisms.md`, and apply targeted corrections — because fixing symptoms without diagnosing root causes produces patches that break elsewhere. Re-verify the revised prompt against Quality Verification checks. Output the revised prompt with change rationale for each modification.

---

## Encoding Framework

Encode the understood user needs into a structure the model processes efficiently. Before applying the framework, calibrate to the receiver's capabilities (context window size, tool-calling support, reasoning depth), because the target language's grammar varies with the receiver — smaller context windows demand aggressive compression with head/tail priority; tool-calling models benefit from actionable steps encoded as tool invocations rather than prose instructions.

Four flexible layers, arranged along the attention curve — static context at the head, explicit intent next, behavior with motivations in the middle, verification at the tail. Use only the layers the task requires, because every unused layer dilutes attention from the layers that matter. Each layer is detailed below.

### Layer Structure

**Context Architecture** — Static reference content, domain knowledge, environmental constraints.
- Place at the very beginning of the prompt, because head position carries the highest cumulative attention weight across all subsequent layers.
- Wrap different content types in structural tags (XML or Markdown sections), because physical isolation prevents cross-contamination of attention between unrelated content blocks.
- Include only when the task depends on external information (per the principle above).

**Intent Declaration** — Goal, success criteria, scope boundaries.
- Always required, because without an explicit goal the model infers one — and inferred goals diverge from user intent. Use action verbs ("refactor", "analyze", "generate") rather than vague descriptions ("help me improve", "look at this"), because specific verbs constrain the output space more tightly, reducing interpretation divergence.
- Define what is in scope and what is out — because unstated boundaries allow the model to explore beyond the user's intended scope, degrading translation fidelity.
- State completion criteria explicitly: what does "done" look like? — because explicit endpoints enable the model to converge on the user's intended scope, preserving translation fidelity.

**Behavioral Specification** — Format requirements, style parameters, positive behavioral directives.
- Express all constraints in positive form first. Because positive instructions align with the model's forward generation path and achieve ~3% violation rate versus ~12% for negative constraints.
- Embed each rule's motivation inline using "because..." links, because embedded motivations activate the induction heads that let the model extrapolate the principle to scenarios the rules did not explicitly cover — without motivations, the model treats each rule as an isolated constraint.
- When safety constraints require negative form, isolate them in dedicated sections with clear delimiters, and repeat at both the beginning and end of the prompt (bookend strategy), because U-shaped attention decay would otherwise mute constraints buried in the middle.
- Define expertise scope and output constraints directly, because parameterized capability boundaries focus attention on task logic rather than persona maintenance.

**Verification & Iteration** — Self-check instructions, validation criteria.
- Place at the very end of the prompt, because tail position carries the strongest position encoding signal — the last instruction the model reads has outsized influence on generation behavior.
- Define verification as completion conditions ("Before submitting, verify each claim has source support"), because completion conditions let the model verify using its own reasoning path rather than forcing it to translate internal reasoning into a prescribed format, wasting reasoning tokens.
- Include only when the task requires high reliability, because unnecessary verification layers dilute attention from the core task.

### Physical Ordering

The ordering follows attention mechanics:

```
[Context Architecture]     ← Head position: longest cumulative path, highest weight
[Intent Declaration]       ← Core goal in the high-attention zone
[Behavioral Specification] ← Constraints and format
[Verification & Iteration] ← Tail position: position encoding advantage
```

---

## Quality Verification

Before presenting the output, verify it against these three principles. Each targets a distinct failure mode of the translation process.

### Fidelity — every element traces bidirectionally between user goal and prompt rule

Fidelity failures mean the translation changed the message — adding what was not there, or losing what was. Apply the Traceability Test (§Understanding the Source) to every rule, and additionally verify: every piece of content traces to a user request (unrequested content dilutes attention); all critical user requirements are represented (omitted requirements are silent translation losses).

### Efficiency — every encoding choice aligns with the cognitive mechanism it leverages

Efficiency failures mean the translation is faithful but lossy — the right message delivered through a noisy channel. Scan against `references/anti-patterns.md` for known translation errors. Verify: constraints use positive framing (forward propagation advantage); critical information occupies head or tail position (attention position effect); rules are expressed as principles with motivations (induction head generalization); only necessary layers are included (attention budget).

### Structural Integrity — physical structure reinforces logical structure

Structural failures mean the encoding is internally inconsistent — the parts do not compose into a coherent whole. Verify: physical ordering follows the attention-optimized sequence (Context → Intent → Behavior → Verification); different content types are physically isolated with structural tags; the backward induction chain (goal → core idea → motivation → rule) is complete and unbroken for each rule, because broken chains produce rules the model follows literally but cannot generalize.

Verify the output passes all three checks before presenting it.

---

Before output: verify Fidelity (every rule traces to user goal via Traceability Test), Efficiency (no anti-patterns per `references/anti-patterns.md`), Structural Integrity (attention-optimized ordering Context → Intent → Behavior → Verification). For high-stakes deployments, follow up with empirical verification per `references/empirical-verification.md`.

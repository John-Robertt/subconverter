# Empirical Verification

Quality Verification (in SKILL.md) checks the encoding's internal consistency. Empirical verification confirms the translation produces the intended behavior in practice — because a structurally sound prompt can still encode the wrong meaning. Apply when the prompt's success criteria are observable in model output and the cost of translation error is high.

## Test Design

Create 2–5 representative test inputs that exercise the prompt's behavioral rules. Prioritize three categories: core path inputs that test the primary intent, edge cases where rules might conflict or underspecify, and adversarial inputs that pressure the prompt's constraints. Derive expected outputs from the backward induction chain's success criteria, because test expectations disconnected from the user's actual goal will validate the wrong behavior.

## Baseline Comparison

Run both the new prompt and a baseline (the previous version, or no prompt) against the same inputs, because isolated results cannot distinguish prompt effect from model default behavior. When the baseline already produces satisfactory output, the prompt adds no value — this signals either an unnecessary translation or success criteria that need sharpening.

## Failure Diagnosis

When a test reveals unexpected output, diagnose using the anti-pattern taxonomy (`anti-patterns.md`) before making changes, because the symptom (wrong output) and the root cause (encoding error) are often in different locations. Trace from the output failure back through the encoding to identify which translation strategy was misapplied. If a specific root cause recurs across multiple tests, the issue is likely in the core idea or motivation rather than in individual rules.

## Iteration

Each diagnosis suggests a targeted correction. Apply the correction, re-verify structurally (Quality Verification in SKILL.md), then re-test empirically. Generalize from test failures to principles rather than adding narrow rules that fix only the observed case — because overfitting to test cases produces a prompt that works for the examples but fails in deployment. Converge when outputs consistently meet success criteria across all test inputs, and remove any prompt content that tests reveal to be inert.

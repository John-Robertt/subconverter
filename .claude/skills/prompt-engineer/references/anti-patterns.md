# Translation Patterns and Common Deviations

Each pattern below represents the correct translation strategy for a specific challenge. The common deviation shows the systematic error that occurs when the strategy is not applied. Every deviation traces to a specific cognitive mechanism, enabling diagnosis from root cause rather than symptom.

---

## Pattern Taxonomy

| Translation Pattern | Common Deviation | Cognitive Root | Fix |
|---|---|---|---|
| Positive framing as primary control | Negative constraints activate the forbidden concept, then fail to suppress it | Mechanism ①: Forward propagation advantage | Convert to positive framing |
| Critical information at head or tail | Information buried in the middle falls into the attention trough, effectively muted | Mechanism ②: U-shaped attention decay | Move to head or tail position |
| Principles with motivations | Exhaustive rule lists create low-compression encoding; model follows listed rules but cannot extrapolate | Mechanism ③: Induction head generalization | Distill into principles with motivations |
| Goals and constraints, not procedures | Detailed step prescriptions conflict with model's built-in reasoning; waste reasoning tokens | Baseline assumption (reasoning model) | Provide goals and constraints |
| Parameterized capability boundaries | Verbose role descriptions consume attention on persona maintenance rather than task logic | Core philosophy (translation fidelity) | Replace with capability boundaries and output constraints |
| One clear strategy per concern | Technique stacking scatters attention across competing dimensions | Attention dispersion | Use one clear strategy per concern |

---

## Detailed Pattern Profiles

### 1. Positive Constraint Framing

**Pattern** (correct):
```
Use module-scoped constants or dependency injection for shared state.
Wrap all error responses in a standardized error envelope with safe user-facing messages.
Use async/await or Promise chains; keep callback nesting to 3 levels maximum by extracting named functions.
```

**Common deviation**:
```
Do not use global variables.
Never return raw error messages to the user.
Avoid nested callbacks deeper than 3 levels.
```

**Why the pattern works**: The correct version contains only the desired patterns in its token sequence. The model generates along the high-probability forward path without needing to compute suppression penalties at each decoding step.

**When negative form is unavoidable** (safety-critical constraints): Isolate in a dedicated `<constraints>` section, place at both prompt head and tail (bookend), and pair each negative with its positive alternative.

---

### 2. Information Position Optimization

**Pattern** (correct):
```
IMPORTANT: All generated code must be backward-compatible with v2 API.

Here is the codebase documentation: [5000 tokens of docs]

Here are the specific files to modify: [3000 tokens of code]

Refactor the authentication module. Verify backward compatibility with v2 API before finalizing.
```

**Common deviation**:
```
Here is the codebase documentation: [5000 tokens of docs]

IMPORTANT: All generated code must be backward-compatible with v2 API.

Here are the specific files to modify: [3000 tokens of code]
Please refactor the authentication module.
```

**Why the pattern works**: The critical constraint is placed at the head (highest cumulative attention weight) and repeated at the tail as a verification criterion (position encoding advantage). The bulk content sits in the middle where lower attention is acceptable — reference material does not need to be memorized, only consulted.

---

### 3. Principles Over Rule Lists

**Pattern** (correct):
```
Naming convention principle: Every name should make its scope and type immediately obvious to a reader who has never seen this codebase. Because code is read far more often than written, each comprehension barrier multiplies across the team.

Conventions: camelCase for variables, PascalCase for classes, UPPER_SNAKE_CASE for constants, kebab-case for file names.
```

**Common deviation**:
```
- Use camelCase for variables
- Use PascalCase for classes
- Use UPPER_SNAKE_CASE for constants
- Use kebab-case for file names
- Use snake_case for database columns
- Prefix interfaces with I
- Suffix enums with Enum
- ...
```

**Why the pattern works**: The principle ("make scope and type obvious; reading frequency >> writing frequency") provides a semantic activation vector that enables the model to make correct naming decisions for unlisted cases (test fixtures, generated types, configuration keys) without needing an exhaustive rulebook.

---

### 4. Goals and Constraints Over Step Prescriptions

**Pattern** (correct):
```
Validate the input against the specification schema. The output must be in JSON format. Before finalizing, verify that all edge cases (null values, type mismatches, missing required fields) are handled.
```

**Common deviation**:
```
Let's think step by step.
Step 1: First, read the input and identify the data types.
Step 2: Then, check for null values in each field.
Step 3: Next, validate the schema against the specification.
Step 4: Finally, generate the output in the required format.
```

**Why the pattern works**: The reasoning model already conducts multi-step internal deliberation. Explicit step prescriptions force it to translate its high-dimensional internal reasoning into low-bandwidth human-readable steps, wasting reasoning tokens. Empirical evidence: explicit CoT induction increases latency by 20-80% with no accuracy improvement on reasoning models.

---

### 5. Parameterized Capability Boundaries

**Pattern** (correct):
```
Expertise scope: Python distributed systems, microservices, cloud-native patterns. Prioritize solutions that maintain horizontal scalability. Flag any recommendation that trades long-term maintainability for short-term convenience.
```

**Common deviation**:
```
You are a senior Python architect with 20 years of experience in distributed systems, microservices, and cloud-native development. You have deep expertise in performance optimization and have led teams at top-tier tech companies. You approach every problem with rigorous analytical thinking and always consider scalability implications.
```

**Why the pattern works**: The correct version defines capability boundaries and behavioral constraints without requiring the model to maintain a fictional persona. Research on 162 role settings across 2,410 reasoning problems shows a 13.78% degradation rate — problems solved correctly without roles become incorrect when roles are added.

---

### 6. Single Strategy Per Concern

**Pattern** (correct):
```
Audit this code for security vulnerabilities.

Severity classification:
- Critical: arbitrary code execution (eval, exec, unsanitized deserialization)
- High: injection vectors (SQL, XSS, command injection)
- Medium: information disclosure, missing rate limiting

For each finding: state the vulnerability, its location, and a fix that preserves backward compatibility.
```

**Common deviation**:
```
You are an expert security auditor. Let's think step by step about this code.
Here are 3 examples of how to audit code: [examples]
If the code uses eval(), flag it as critical. If it uses exec(), flag as high.
Never suggest fixes that break backward compatibility.
Always explain your reasoning in detail.
```

**Why the pattern works**: A single, coherent encoding strategy (structured severity framework + output format) gives the model a clear attention target. The deviation demands simultaneous attention to persona, reasoning display, examples, conditional logic, negative constraints, and verbosity requirements — each consuming reasoning tokens that compete rather than reinforce.

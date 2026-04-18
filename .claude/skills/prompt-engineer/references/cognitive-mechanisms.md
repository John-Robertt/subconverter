# Cognitive Mechanisms: Why Translation Strategies Work

Three mechanisms from the model's architecture determine how it processes prompt instructions. Each mechanism provides the causal basis for specific translation strategies. When facing novel situations, reason from these mechanisms rather than memorizing rules.

---

## Mechanism 1: Forward Propagation Advantage

### How It Works

Large language models generate text through autoregressive prediction — each token is predicted based on all preceding tokens, following a forward probability path. Positive instructions ("use dependency injection for state management") align with this natural forward path: the token sequence contains only the desired pattern, and the model generates along high-probability trajectories without internal conflict.

Negative instructions ("do not use global variables") create a **Latent Semantic Activation Trap**: the token sequence unavoidably contains the forbidden concept ("global variables"), which activates the associated code patterns, syntax trees, and weight networks in the model's latent space. To comply, the model must compute an additional **penalty term** at every decoding step to suppress these activated-but-forbidden candidates. This suppression is computationally expensive and fragile — it frequently fails in long contexts or deeply nested logic.

### Evidence

- Empirical measurement: positive instruction violation rate ~3% vs. negative constraint violation rate ~12%.
- All three major platforms (Anthropic, OpenAI, Google) independently converge on the same recommendation: "tell the model what to do, not what not to do."
- The **Warning-based Prompts** technique — injecting meta-instructions like "pay special attention to any negation words" — achieves a 25.14% absolute accuracy improvement by forcibly reshaping attention distribution, confirming that the root cause is attention-level, not logic-level.

### Translation Implication

**Translate needs as "do X" rather than "don't do Y."** Negative framing is a grammatical structure in the target language that the model systematically misparses. When a safety constraint genuinely requires negative form, physically isolate it in a dedicated section with clear delimiters, and repeat at both prompt head and tail (bookend strategy) to counteract attention decay.

---

## Mechanism 2: Attention Position Effect (U-Shaped Decay)

### How It Works

In the self-attention computation over long sequences, tokens at the **head** of the sequence accumulate the longest path through all subsequent layers (highest cumulative attention weight), while tokens at the **tail** benefit from the most recent position encoding (strongest recency signal). Tokens in the **middle** fall into an attention trough — a phenomenon called "Lost in the Middle."

This creates a U-shaped attention curve: information placed at the beginning and end of a prompt receives the strongest processing weight, while information buried in the middle is systematically underweighted.

### Evidence

- Anthropic: "Place long documents and inputs at the top of the prompt... queries at the end can improve response quality by up to 30%, especially for complex multi-document inputs."
- OpenAI: Recommends "reference text" strategy — place external data in context for model access.
- Google: "Provide all context first", then "place specific instructions or questions at the very end", using anchor phrases like "Based on the above information..." after large data blocks.
- The effect is amplified in 1M-token context windows — the "middle" region is vastly larger, deepening attention dilution.

### Translation Implication

**Information position = its "volume" in the target language.** Place static reference content at the head (highest cumulative weight), and place the core query, instructions, and verification criteria at the tail (position encoding advantage). Critical constraints that cannot be placed at head or tail should be repeated at both positions (bookend strategy). Never bury the most important information in the middle of a long prompt.

---

## Mechanism 3: Induction Heads and Principle-Based Generalization

### How It Works

Within multi-layer, multi-head self-attention architectures, **induction heads** enable the model to extract abstract patterns by composing information across attention layers. A **Common Bridge Representation** hypothesis posits that a shared latent subspace connects early and late network layers, enabling a phase transition from "weak learning" (predicting marginal token probabilities) to "pattern learning" (extracting abstract logic).

When a prompt contains explicit core ideas and motivations (e.g., "The core value of this system is protecting modification freedom — every design decision should prioritize whether it compresses future modification space"), it provides a high-dimensional **semantic activation vector**. This vector helps the model rapidly align the relevant induction heads in its latent space. The model can then project known principles onto entirely novel scenarios composed of different tokens — achieving genuine out-of-distribution generalization.

By contrast, surface-level rules ("don't use global variables", "must write unit tests") activate only shallow feature matching. The model follows the literal rules but cannot extrapolate to unlisted situations.

### Evidence

- DeepSeek-R1-Zero experiment: trained with pure reinforcement learning, given only two feedback signals (result accuracy + format requirement). The model autonomously developed complex chain-of-thought, reflection, self-correction, and super-generalization to out-of-distribution tasks — demonstrating that goals + format alone can bootstrap full reasoning capabilities.
- In symbolic language reasoning tests, replacing real-world concepts with meaningless symbols still yields precise logical reasoning, as long as the prompt captures the "core motivation" behind symbol mappings — confirming that induction heads operate on structural relationships, not surface tokens.

### Translation Implication

**Translating "why" is more efficient than translating "what."** Principles are extremely high-compression information encoding — a single principle enables the model to derive countless rules autonomously. When constructing the thought-motivation-behavior chain, invest most effort in articulating the core idea and motivation clearly. The model will derive the specific behavioral rules from these. This is not laziness — it is a higher-bandwidth encoding that activates deeper generalization circuits.

# Fort

Fort is a private chat service for durable conversations with explicitly
identified Agents. It preserves Agent identity, conversation history,
delegated authority, and execution truth across clients and computers.

## Language

**Agent**:
A durable, named Fort identity and chat destination. It outlives changes to the
framework, model, profile, or computer that executes its work.
_Avoid_: Model alias, provider, process, Conversation

**Agent Profile Revision**:
An append-only revision of an Agent's visible name, title, avatar, and
presentation state.
_Avoid_: Prompt, role, execution identity

**Agent Behavior Revision**:
An immutable revision of an Agent's role, standing instructions, enabled
skills/tools, and other Fort-owned prompt material.
_Avoid_: Profile presentation, source-managed memory, mutable prompt

**Execution Source**:
One concrete framework instance or gateway that can inventory and run Source
Agents.
_Avoid_: Generic provider name, display name, Fort Agent

**Source Agent**:
An opaque framework-native identity qualified by its Execution Source.
_Avoid_: Unqualified profile name, Fort Agent

**Agent Binding Revision**:
The immutable execution identity for an Agent at one point in time. It pins one
Behavior Revision and the exact Source Agent, seat, adapter, model, computer,
authority, policy, source configuration, and accepted capabilities.
_Avoid_: Agent name, mutable preference, silent fallback

**Agent Channel**:
User-facing compatibility language for selecting an Agent as a top-level chat
destination.
_Avoid_: Conversation, Binding Revision, provider session

**Conversation**:
One durable transcript and context boundary.
_Avoid_: Agent, Channel, provider session, execution run

**Canonical Conversation**:
The permanent Home conversation created with exactly one Agent.
_Avoid_: Most recent chat, provider-native session

**Secondary Conversation**:
An additional transcript/context boundary owned by one Agent. It may be pinned
for navigation without replacing Home.
_Avoid_: Shared memory, replacement Home

**Group Conversation**:
A durable Conversation with versioned membership of two or more stable Agents.
_Avoid_: Hidden delegation loop, Execution Source

**Handoff**:
A durable, attributed command from a human or one Agent stage to one exact
recipient Agent, with explicit context, authority, limits, and output
Conversation.
_Avoid_: Copied prompt text, prose mention, provider rerouting

**Routine**:
A versioned recurring or event-triggered command owned by one Agent and
reporting a successful normalized result to one explicit Conversation.
_Avoid_: Unattributed framework cron row

**Target**:
One durable request for a specific Agent, Behavior Revision, Binding Revision,
and participant evidence to answer a turn or Handoff.
_Avoid_: Agent, Conversation, runtime attempt

**Fort Mark**:
Fort's orbital-core product identity, distinct from every Agent's identity.
_Avoid_: Agent avatar, status light

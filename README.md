# Fiskaly's "Agentic Backend Engineer (Golang)" interview exercise

```md
##The Challenge

Fiskaly offers APIs for fiscalization, e-invoicing and digital receipts. Built for POS systems and omni-channel
operators. Compliant and scalable. fiscaly’s mission is to enable customers to implement compliant solutions based on
API provided by the fiskaly platform.

One enabler for our customers is the API documentation - see https://developer.fiskaly.com/api/sign-it/2026-02-03

##Your Tasks

What opportunities do you see to drive the mission of Fiskaly to the next level? What improvements could be made to the
API documentation that bring value to customers? We appreciate it if you want to go crazy. Fixing typos in the API
documentation will not empower the mission of fiskaly.

For one of the opportunities, build a functional prototype. That we’ll discuss in the interview in depth.
```

Successful project completion will have two deliverables: "Opportunities" document and a functional prototype.

Prototype: an MCP (docs + action + sandbox-control tools) wrapped by an integration Skill, running against a
fault-injecting sandbox, with a deterministic judge that runs both as an MCP tool and as a CI gate

**Critical guiding rule: prototype must address the largest impact through the smallest effort**

## Context

- Functional prototype: MCP Server
    - docs grounding tool: local files behind agentic retrieval tool
    - action oriented tools for the API
    - testing sandbox
    - action oriented tools
- Development Methodology: Evals based development.
    - A set of scenarios for target development flows with success/failure rubric.
    - Evals are executed through subagents with Sonnet-4.6 high effort.
    - All prototype features are evaluated by running those scenarios and analyzing output for end result and session
      for efficiency
- Problems we want to solve:
    - agent always grounded in docs
    - telemetry: mcp use will provide insight into the actual work flows. Probably the highest value
    - simplify integration test before hitting prod

## Observations

- docs have gaps?

## TODO

### 1. build project context

- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]

### 2. build evals

- [x] scenarios — 10 code exercises in [`sims/scenarios/`](sims/scenarios/README.md), with seeded red herrings, false info, and dormant silent bugs
- [x] rubric — per-scenario `SOLUTION.md` answer keys + the scenario-aware conformance judge (`sims/judge`, rule catalog selectable per scenario)
- [ ] create subagent configurations
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]

### 3. build prototype

- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]

### 4. document prototype

- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]

### 5. document general opportunities

- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]
- [ ]

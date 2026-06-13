# Getting Started — building on `ck-allinone`

This is the on-ramp for using `ociger-ck-allinone` as the base for a new project — a game, a science experiment, a software system, anything that wants **governed, sealed, provable state** without writing a backend.

Everything below is verified against the published `ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.19` image. Commands you can paste are real; capability claims are drawn against what the image actually does, not what a spec promises.

---

## 1. What one `docker run` gives you

```sh
docker run --rm -d --name ckp \
  -e OCIGER_CK_PARTICIPANT_PASSWORD='choose-a-password' \
  -p 5432:5432 -p 8000:8000 -p 4222:4222 -p 9222:9222 \
  ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.19
```

That single command stands up a complete **CKP v3.9 Critical Isolation** substrate:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│  PostgreSQL 17  +  pgRDF (graph engine)  +  pgCK (concept-kernel runtime)      │
│  NATS core :4222  +  NATS WebSocket :9222  (the only door for apps)            │
│  busybox httpd :8000  serving /cklib/ (the browser client) + landing probe     │
│  s6-overlay supervises everything · scratch base · no Python · ~128 MB          │
└──────────────────────────────────────────────────────────────────────────────┘
```

The defining property is **isolation**, enforced by the database itself, not by convention:

```
   Ring 2  AFFORDANCES   the verbs your app calls            ← e.g. task.create
   Ring 1  PRIMITIVES    closed pgCK micro-operations        ← the only callers of pgRDF
   Ring 0  ENGINE        pgRDF: SPARQL · SHACL · OWL-RL      ← no app ever touches this
```

A connection — even one holding database credentials — can execute exactly **one** function: `ckp.dispatch`. It cannot run SQL against your data, cannot reach the query engine, cannot land an unsealed fact. Every write passes a SHACL shape gate, appends to a hash-chained ledger, and mints a verifiable proof. This is what you are building on.

---

## 2. The capability boundary — read this before you design

What the published image does **today** (v0.7.19 / pgCK 0.4.13), and what is on the upstream roadmap, stated plainly so you design against reality:

```
┌─────────────────────────────────────────────┬──────────────────────────────────────┐
│  WORKS NOW — zero setup, pure dispatch         │  ROADMAP (pgCK; not this repo)        │
├─────────────────────────────────────────────┼──────────────────────────────────────┤
│  • Kernels — named domains you create on the   │  • First-class custom classes — model  │
│    fly (kernel.create); any name works.        │    your own sealed type (Ship, Sample, │
│  • Tasks — governed, sealed work objects that  │    Service) with its own create verb   │
│    target a kernel; lifecycle, priority,       │    and shape. instance.create today    │
│    queue position; each a proof-chained seal.  │    routes only to Task/Goal.           │
│  • Goals — objectives/backlogs a kernel works  │  • Adopter ontology modules — load your│
│    toward.                                      │    own .ttl to declare a new class.    │
│  • Typed reads — list, get, verify, provenance │    import_module accepts only pgCK's   │
│    of any sealed instance, re-verifiable.      │    known module set today.             │
│  • Governance — evolve a kernel's TYPE          │                                      │
│    (add a property, change a constraint, set    │  Tracking: a NOTIFY is filed with pgCK│
│    quorum) through propose → vote → apply,      │  asking for the generic typed create  │
│    consensus-gated, epoch-advancing.            │  (CKP v3.9 §4). Until it lands, model │
│                                                 │  your domain as kernels + tasks +     │
│                                                 │  goals (§4 below shows how each        │
│                                                 │  persona maps).                        │
└─────────────────────────────────────────────┴──────────────────────────────────────┘
```

In one sentence: **today you get a governed, sealed, consensus-evolvable workflow substrate — kernels with tasks and goals — usable for any domain that maps onto that shape.** Modeling your own first-class domain entities as sealed types is the next pgCK capability.

---

## 3. The hello-kernel journey (every command verified)

There are two ways to talk to the substrate. Apps use the **NATS door**; operators and scripts can also use a **direct SQL connection** as the participant role. Both reach the same single function.

### 3a. Direct, as the participant role

The bundle creates two Postgres roles automatically: `ck_substrate` (owns everything, never logs in) and `ck_participant` (the role apps connect as — it can call `ckp.dispatch` and nothing else). The password is the one you passed in `OCIGER_CK_PARTICIPANT_PASSWORD`.

```sh
# A psql one-liner via a throwaway client container on the same host:
export PGPASSWORD='choose-a-password'
psql -h localhost -U ck_participant -d postgres -tA -c \
  "SELECT ckp.dispatch('kernel.create', '{\"name\":\"mygame\"}'::jsonb);"
# → {"id":"backlog:mygame","ok":true,"kernel":"mygame"}

psql -h localhost -U ck_participant -d postgres -tA -c \
  "SELECT ckp.dispatch('task.create', '{\"task\":{\"target_kernel\":\"mygame\",\"title\":\"patrol sector 7\"}}'::jsonb);"
# → {"id":"task-…","ok":true,"verified":true,"proof_digest":"7c1387a6…"}

psql -h localhost -U ck_participant -d postgres -tA -c \
  "SELECT ckp.dispatch('instances.list', '{\"kernel\":\"mygame\"}'::jsonb);"
# → {"ok":true,"count":1,"instances":[ … the sealed task … ]}
```

Every `task.create` returns a `proof_digest`. That digest is independently re-verifiable later:

```sh
psql -h localhost -U ck_participant -d postgres -tA -c \
  "SELECT ckp.dispatch('instance.verify', '{\"id\":\"task-…\"}'::jsonb);"
# → {"id":"task-…","ok":true,"verified":true}
```

### 3b. Over the NATS door (the real app path)

Apps publish the four-tuple to `input.kernel.<K>.action.<verb>` over NATS (WebSocket on `:9222`) and read the typed reply on `result.kernel.<K>.<verb>`. The bundle's in-image relay bridges that to `ckp.dispatch` and preserves your `Trace-Id`. A browser does this with the bundled client at `/cklib/ck-client.js`; a CLI does it with `nats`:

```
PUB  input.kernel.mygame.action.kernel.create   {"name":"mygame"}
MSG  result.kernel.mygame.kernel.create         {"id":"backlog:mygame","ok":true,…}

PUB  input.kernel.mygame.action.task.create     {"task":{"target_kernel":"mygame","title":"patrol sector 7"}}
MSG  result.kernel.mygame.task.create           {"id":"task-…","ok":true,"proof_digest":"…"}

PUB  input.kernel.mygame.action.instances.list  {"kernel":"mygame"}
MSG  result.kernel.mygame.instances.list        {"ok":true,"count":2,"instances":[…]}
```

The landing page at `http://localhost:8000/` runs exactly this round-trip in your browser and shows ✓ when the wire is alive. See [§6](#6-connecting-an-app) for the client contract.

### 3c. Evolve the type — governance, not a migration

When you need to change what a kernel's instances *are* — add a property, tighten a constraint, change the quorum — you do not run a migration. You propose a change; it is sealed as data; it applies only after the votes it requires; and applying it advances the kernel's epoch. All over the same door:

```sh
# 1. propose: add a `crew_size` integer property to the mygame kernel type.
#    add_property carries detail.targetClass (the class the property attaches
#    to) + detail.path (the property IRI) — both IRIs, required since pgCK 0.4.5.
psql -h localhost -U ck_participant -d postgres -tA -c \
  "SELECT ckp.dispatch('kernel.propose_change',
     '{\"op\":\"add_property\",\"requires_quorum\":1,
       \"detail\":{\"targetClass\":\"ckp://Kernel#mygame\",\"path\":\"ckp://Kernel#crew_size\",
                  \"datatype\":\"xsd:integer\",\"minCount\":1}}'::jsonb);"
# → {"ok":true,"proposal_iri":"ckp://Proposal#proposal-…","state":"pending"}

# 2. vote (the proposal IRI from step 1)
psql … "SELECT ckp.dispatch('kernel.vote', '{\"about\":\"ckp://Proposal#proposal-…\",\"value\":\"approve\"}'::jsonb);"
# → {"ok":true,"quorum_met":true,"approvals":1}

# 3. apply
psql … "SELECT ckp.dispatch('kernel.apply', '{\"about\":\"ckp://Proposal#proposal-…\"}'::jsonb);"
# → {"ok":true,"state":"applied","epoch":2}
```

The proposal and the votes are themselves sealed, proof-chained instances — the consensus mechanism is made of the same substrate it governs. A human approval is a vote sealed by a human identity, indistinguishable in the audit trail from any other fact.

A complete, runnable version of this whole journey is in [`examples/hello-kernel/`](./examples/hello-kernel/).

---

## 4. The three personas — mapping a real domain onto today's substrate

Until first-class custom classes land (§2), you model your domain as **kernels** (named domains), **goals** (objectives), and **tasks** (governed, sealed work/state objects targeting a kernel). That shape is more general than it first looks:

```
┌──────────────┬───────────────────────┬────────────────────────┬───────────────────────────┐
│  Persona     │  Kernel =              │  Goal =                │  Task =                    │
├──────────────┼───────────────────────┼────────────────────────┼───────────────────────────┤
│  Game        │  a match / session     │  win condition         │  a governed move, a        │
│              │  ("arena-42")          │  ("capture-the-flag")  │  mission-queue item, a     │
│              │                        │                        │  scored action — each      │
│              │                        │                        │  sealed + replayable       │
├──────────────┼───────────────────────┼────────────────────────┼───────────────────────────┤
│  Science     │  an experiment / run   │  hypothesis under test │  a protocol step, an       │
│              │  ("assay-2026-06")     │                        │  observation, a sample     │
│              │                        │                        │  handling event — each     │
│              │                        │                        │  proof-chained for audit   │
├──────────────┼───────────────────────┼────────────────────────┼───────────────────────────┤
│  Software     │  a service / project   │  an epic               │  a work item, a deploy     │
│              │  ("billing-svc")       │                        │  record, a governed config │
│              │                        │                        │  change — sealed, with     │
│              │                        │                        │  provenance                │
└──────────────┴───────────────────────┴────────────────────────┴───────────────────────────┘
```

For each, the verbs are the same: `kernel.create` to open the domain, `task.create` to land a sealed object in it, `instances.list` / `instance.verify` / `instance.provenance` to read and audit, and the governance plane to evolve what a task in that domain must carry. The honest caveat: a game's `Ship` (with its own `hull_integrity` shape and its own create verb) is a first-class class — that is the §2 roadmap item. Today the `Ship` lives as a task targeting the match kernel, with its state in the task body.

---

## 5. Topology — direct vs. behind a gateway

The bundle's NATS WebSocket is on container port `:9222`. How an app reaches it depends on your deployment:

```
┌──────────────────────────────┬──────────────────────────────────────────────────────┐
│  Direct docker run            │  -p 9222:9222 ; app opens  ws://<host>:9222           │
│  Behind envoy / TLS gateway   │  route /wss → :9222 ; app opens  wss://<host>/wss      │
└──────────────────────────────┴──────────────────────────────────────────────────────┘
```

The bundled landing page auto-detects which case it is from the page's own URL. The static page at `http://localhost:8000/wss/` documents the exact envoy route to use. Note that busybox serves static files only — a gateway must proxy the WebSocket upgrade itself; the container does not.

---

## 6. Connecting an app

### Browser

The bundle serves the CK.Lib.Js client at `/cklib/` — a stripped, dependency-vendored client (`ck-client.js` + `vendor/`). A page becomes a concept-kernel client by importing it and pointing it at the WSS door; it publishes the four-tuple and correlates the typed reply by `Trace-Id`. The reserved surfaces `/web/` and `/web2/` are where your own front-end goes.

### Server / CLI

Any NATS client works. Publish to `input.kernel.<K>.action.<verb>`, subscribe to `result.kernel.<K>.>`, send JSON payloads, read JSON envelopes. Or connect a server directly as `ck_participant` over Postgres and call `ckp.dispatch` — same contract, no NATS hop.

### The deploy contract (do not skip)

- **Set `OCIGER_CK_PARTICIPANT_PASSWORD`.** Without it, `ck_participant` exists but cannot log in — by design, so the role floor is never cosmetic.
- **Identity today is the scram password.** Verified-JWT identity with seal-time claim checking is an inherited upstream prerequisite (CKP v3.9 §10, owned by SPORE-GENESIS) and is not yet in this image. Treat a v0.7.19 deployment as **alpha-trust**: the isolation floor is real, but participant identity is a shared secret, not a per-user verified claim. Do not expose it to untrusted users as-is.
- **Persist `/var/lib/postgresql/data`.** The init runs once on first boot; your sealed data lives there.

---

## 7. Where this is going

The substrate is finalized at CKP v3.9; the bundle tracks the three upstreams (`pgRDF`, `pgCK`, `CK.Lib.Js`) and re-cuts as they ship — the current cut rides **pgRDF 0.6** (a parallel bulk loader, LUBM-benchmarked ingest at tens-of-millions-of-triples scale) and **pgCK 0.4.13**. The single capability that turns this from a workflow substrate into a model-anything substrate is the **generic typed create** (CKP v3.9 §4) — a `ckp.dispatch('instance.create', …)` whose payload is typed by *your kernel's own sealed shape*, so a game's `Ship` and a lab's `Sample` are first-class sealed instances. That is on pgCK's side; a NOTIFY is filed. When it lands, this guide gains a "model your own type" section and `examples/` gains a real domain-entity walkthrough.

Until then: everything in §3 works today, on the published image, with one `docker run`.

---

## See also

- [`README.md`](./README.md) — what ships from this repo and the bundle catalog.
- [`examples/hello-kernel/`](./examples/hello-kernel/) — the §3 journey as a runnable script.
- [`PROVENANCE.md`](./PROVENANCE.md) — the build + attestation policy behind every published digest.
- [`CHANGELOG.md`](./CHANGELOG.md) — the honest release history.

# `hello-kernel` — the adopter journey, runnable

The shortest path from zero to sealed, governed, provable state on `ck-allinone`
— driven the way a real app or browser drives it: the **CK.Lib.Js client (cklib)
over NATS-WSS** → `ociger-pgck-relay` → `ckp.dispatch`. No SQL, no postgres
client. One script, only Docker required.

```sh
bash examples/hello-kernel/run.sh
```

It will:

```
① stand up ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.21
② stage the bundle's OWN cklib + run the journey under node:22 (native WebSocket)
③ the journey, all over cklib + WSS:
   • activate the kernel                       CK.activate(kernel, {wssEndpoint})
   • land sealed, proof-chained state          create(Task)  → proof_digest, verified
   • read it back + re-verify the proof        verify · query
   • PROVE enforcement is real                 an incomplete create is REJECTED
   • relate two instances + traverse the edge  link → reach
```

Every step asserts; the script exits non-zero on the first failure and tears the
container down on exit. It stages the **bundle's own** cklib (via `docker cp`),
never a vendored copy — the example always exercises the exact client version
that bundle ships.

## Run against your own image or domain

```sh
bash examples/hello-kernel/run.sh  ghcr.io/sporaxis-com/ociger-ck-allinone:latest  demo
```

The second argument is the kernel name. The bundle arms the **`demo`** kernel
with the `Task`/`Goal` shapes at first boot, so it works out of the box; point at
another kernel only once you've adopted shapes into its `urn:ckp:<kernel>/kernel/ck`.

## The one deployment gotcha — `wssEndpoint`

On a **direct `docker run`** (no Envoy/Nginx gateway in front), you MUST give
cklib an explicit `ws://` endpoint:

```js
const k = await CK.activate('demo', { wssEndpoint: 'ws://<host>:9222' });
```

cklib's same-origin default is `wss://<host>/wss`, which SSL-fails without a
gateway terminating TLS and routing `/wss` → `:9222`. Behind a gateway you drop
the option and the default just works. `run.sh` sets `WSS` for you; `driver.mjs`
is the embeddable reference for the snippet your own app would ship.

## What it demonstrates

Everything happens through one function — `ckp.dispatch` — reached over the wire
by a client that holds exactly that one capability and nothing else. The journey
cannot execute SQL against the data, cannot reach the query engine, and cannot
land a fact that did not pass its shape gate and mint a proof. Step ④ makes the
last point concrete: the **same** `Task` type, missing one shape-required field,
is rejected at the seal. That is the point — **the door is the only surface, and
the engine is invisible.** This is CKP v3.9 Critical Isolation.

> **Operator/debug aside (NOT the adopter surface).** You can reach the same door
> directly with `psql` as `ck_participant`:
> `SELECT ckp.dispatch('task.create', '{…}'::jsonb)`. That bypasses both the WSS
> protocol and the client, and exists for debugging only. The integration an app
> builds against is cklib over WSS — which is what this example runs.

For the full picture — the three-persona domain mapping, the deploy contract, and
the capability boundary — see [`../../GETTING-STARTED.md`](../../GETTING-STARTED.md).

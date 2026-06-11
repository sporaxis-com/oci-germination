# `hello-kernel` — the adopter journey, runnable

The shortest path from zero to a sealed, governed, consensus-evolvable domain on `ck-allinone`. One script, only Docker required.

```sh
bash examples/hello-kernel/run.sh
```

It will:

```
① stand up ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.17
② open a domain                          kernel.create
③ land sealed, proof-chained state       task.create   → proof_digest
④ read it back + re-verify the proof     instances.list · instance.verify
⑤ evolve the kernel's TYPE by consensus  propose → vote → apply (epoch advances)
```

Every step prints the real JSON envelope and asserts `ok:true`; the script exits non-zero on the first failure and tears the container down on exit.

## Run against your own image or domain name

```sh
bash examples/hello-kernel/run.sh  ghcr.io/sporaxis-com/ociger-ck-allinone:latest  my-experiment
```

## What it demonstrates

Everything happens through one function — `ckp.dispatch` — called as the `ck_participant` role, which holds exactly that one capability and nothing else. The connection that runs the whole journey cannot execute SQL against the data, cannot reach the query engine, and cannot land a fact that did not pass the shape gate and mint a proof. That is the point: **the door is the only surface, and the engine is invisible.**

For the full picture — the NATS path apps actually use, the three-persona domain mapping, the deploy contract, and the capability boundary — see [`../../GETTING-STARTED.md`](../../GETTING-STARTED.md).

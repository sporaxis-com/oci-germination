// examples/hello-kernel/driver.mjs
//
// The hello-kernel adopter journey, driven the way a real app drives it:
// the CK.Lib.Js client (cklib) over NATS-WSS → ociger-pgck-relay → ckp.dispatch.
// No SQL, no postgres client — exactly the surface an app or browser uses.
//
// run.sh copies the bundle's OWN cklib next to this file and runs it under
// node:22 (native global WebSocket), pointed at the bundle's WSS port. Each
// step asserts; the process exits non-zero on the first failure.
//
// cklib is a browser client; node:22 runs the identical ESM unchanged because
// it now ships a global WebSocket. On a *direct* docker run (no Envoy gateway)
// you MUST pass an explicit ws:// endpoint — cklib's same-origin default
// (wss://<host>/wss) SSL-fails without the gateway. run.sh sets WSS for you.

const CORE = 'https://conceptkernel.org/ontology/v3.8/core#';
const TASK = CORE + 'Task';
const GOAL = CORE + 'Goal';
const PART_OF_GOAL = CORE + 'part_of_goal';

const KERNEL = process.env.KERNEL || 'demo';
const WSS = process.env.WSS || 'ws://localhost:9222';

const B = (s) => `\x1b[1m${s}\x1b[0m`;
const RED = (s) => `\x1b[31m${s}\x1b[0m`;
const say = (s) => console.log(`\n${B(s)}`);
const note = (s) => console.log(`   ${s}`);
function assert(cond, msg) {
  if (!cond) { console.error(RED(`   ✗ ${msg}`)); process.exit(1); }
}

const { CK } = await import('./cklib/ck.js');

say(`① Activate the kernel — CK.activate('${KERNEL}') over ${WSS}`);
note('cklib opens one NATS-WSS connection; every op below is a ckp.dispatch.');
const k = await CK.activate(KERNEL, { wssEndpoint: WSS });

say('② Land sealed, proof-chained state — create a Task');
note('SHACL-gated + ledger-appended + proof-minted in one transaction.');
const task = await k.create(TASK, {
  part_of_goal: 'backlog:demo',
  target_kernel: `urn:ckp:kernel:${KERNEL}`,
});
note(`reply: ok=${task.ok} verified=${task.verified} proof=${String(task.proof_digest || '').slice(0, 24)}…`);
assert(task.ok === true, `task create failed: ${task.error || '(no error field)'}`);
assert(task.verified === true, 'task created but proof did not verify');
note(`sealed instance: ${task.id}`);

say('③ Read it back + independently verify the proof');
const verified = await k.verify(task.id);
assert(verified.verified === true, 'instance.verify did not re-confirm the proof chain');
note(`verify(${task.id}) → verified=${verified.verified}`);
const rows = await k.query(TASK, {});
assert(rows.length >= 1, 'query(Task) returned no rows after a successful create');
note(`query(Task) → ${rows.length} instance(s)`);

say('④ Enforcement is REAL — an incomplete create is REJECTED');
note('Same Task type, but omit the shape-required target_kernel. This is the');
note('payoff of the v0.7.20 graph-fix + v0.7.21 cklib create() pass-through:');
const bad = await k.create(TASK, { part_of_goal: 'backlog:demo' });
note(`reply: ok=${bad.ok} error=${(bad.error || '').slice(0, 72)}`);
assert(bad.ok === false, 'VACUOUS: an incomplete create was accepted — enforcement is not real');
note('✓ rejected at the seal — you cannot land an instance that violates its shape.');

say('⑤ Relate + traverse — link a Goal, then reach it back');
const goal = await k.create(GOAL, { label: 'Ship hello-kernel' });
assert(goal.ok === true, `goal create failed: ${goal.error || ''}`);
note(`goal: ${goal.id}`);
const linked = await k.link(task.id, PART_OF_GOAL, goal.id);
assert(linked.ok === true, `link failed: ${linked.error || ''}`);
const reached = await k.reach(task.id, PART_OF_GOAL);
assert(reached.length >= 1, 'reach returned nothing — the link did not round-trip');
note(`reach(task, part_of_goal) → ${reached.length} target(s)`);

say('✓ Done — the whole journey ran over cklib + WSS, never psql.');
console.log(`
   What just happened, end to end, through ckp.dispatch over NATS-WSS:
     • activated a kernel                         (CK.activate)
     • landed governed, sealed, provable state    (create Task → proof_digest)
     • read it back + re-verified the proof       (verify, query)
     • PROVED enforcement is real                 (incomplete create rejected)
     • related two instances + traversed the edge (link → reach)

   The client held exactly one capability: ckp.dispatch. It could not run SQL,
   reach the query engine, or land an unsealed fact. That is CKP v3.9 Critical
   Isolation — and this is the surface a real app or browser integrates against.

   Next: GETTING-STARTED.md maps this onto a game / experiment / software domain.
`);
process.exit(0);

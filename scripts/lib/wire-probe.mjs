// wire-probe.mjs — one authorized dispatch over NATS-WSS, as a consumer makes it.
//
//   node wire-probe.mjs <ws-url> <token|-> <kernel> <verb> [payload-json]
//   -> prints the reply JSON on stdout (exit 0), or "NO REPLY" (exit 1)
//
// Publishes id-form `input.kernel.<k>.id.<sub>.action.<verb>` and reads the
// reply on `result.kernel.<k>.<verb>` (pgCK src/nats_client.rs::route_inbound).
// Pass "-" as the token to connect anonymously.
//
// ⚠ ws.binaryType IS LOAD-BEARING — DO NOT REMOVE IT.
// NATS-over-WebSocket sends BINARY frames. A probe that reads only string
// frames (`typeof ev.data === 'string' ? ev.data : ''`) discards every frame and
// reports a perfectly healthy door as completely silent. That cost a full
// misdiagnosis on 2026-09-03 — five verbs looked dead and all five answered the
// moment this line existed, and the silence had already been written up as a
// platform caveat. If this ever reports blanket silence, suspect the probe.
const [, , url, tokArg, kernel, verb, payload = '{}'] = process.argv;
if (!url || !kernel || !verb) {
  console.error('usage: wire-probe.mjs <ws-url> <token|-> <kernel> <verb> [payload]');
  process.exit(2);
}
const token = !tokArg || tokArg === '-' ? '' : tokArg;
// The id segment must be the VERIFIED subject; anonymous has no id-form grant.
const sub = token ? JSON.parse(Buffer.from(token.split('.')[1], 'base64url')).sub : '';
const TIMEOUT = Number(process.env.WIRE_MS || 8000);
const dec = new TextDecoder();

const res = await new Promise((resolve) => {
  let ws;
  try { ws = new WebSocket(url); } catch (e) { return resolve({ err: 'ctor:' + e.message }); }
  ws.binaryType = 'arraybuffer';
  let got = null, wireErr = '';
  const done = (v) => { clearTimeout(timer); try { ws.close(); } catch {} resolve(v); };
  const timer = setTimeout(() => done({ err: wireErr, timeout: true }), TIMEOUT);

  ws.onopen = () => {
    ws.send(token
      ? `CONNECT {"verbose":false,"pedantic":false,"protocol":1,"headers":true,"auth_token":"${token}"}\r\n`
      : 'CONNECT {"verbose":false,"pedantic":false,"protocol":1,"headers":true}\r\n');
    ws.send(`SUB result.kernel.${kernel}.> 1\r\n`);
    ws.send('PING\r\n');
    if (!token) return;                       // anonymous: connection is the test
    // Beat before publishing so the SUB is registered server-side; otherwise the
    // reply can be produced before anyone is listening for it.
    setTimeout(() => ws.send(
      `PUB input.kernel.${kernel}.id.${sub}.action.${verb} ${payload.length}\r\n${payload}\r\n`), 500);
  };
  ws.onmessage = (ev) => {
    const s = typeof ev.data === 'string' ? ev.data : dec.decode(new Uint8Array(ev.data));
    for (const line of s.split('\r\n')) {
      if (line.startsWith('-ERR')) wireErr = line.trim();
      else if (line.startsWith('{') && !got) { got = line; done({ json: line }); }
    }
  };
  ws.onerror = () => {};
  ws.onclose = (e) => { if (!got) done({ err: wireErr || `closed_${e.code}`, code: e.code }); };
});

if (res.json) { console.log(res.json); process.exit(0); }
console.log(`NO REPLY${res.code ? ' closed_' + res.code : ''}${res.err ? ' err=' + res.err : ''}`);
process.exit(1);

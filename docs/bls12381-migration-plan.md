# BLS12-381 Migration Plan

## Background

The current BLS implementation (`crypto/multi_sig/bls.go`) is based on DFINITY's
original go-bls library, which wraps the **PBC (Pairing-Based Cryptography)** C
library via CGo. This requires `libgmp` and `libpbc` to be present on the build
host and in every Docker image, which is the primary reason the codebase cannot
be compiled on a plain Go toolchain today.

The Ethereum beacon chain standardised on **BLS12-381** (IETF
`draft-irtf-cfrg-bls-signature-04`). Mature, audited, pure-Go-friendly libraries
exist for it (notably `supranational/blst`, used by all major Eth2 clients). This
migration replaces the CGo PBC dependency with blst, eliminates manual C memory
management throughout the codebase, and aligns Meter's consensus signature scheme
with the Ethereum ecosystem.

---

## Current Implementation — Key Facts

| Property | Current (PBC Type A) |
|---|---|
| Curve | Type A supersingular, 160-bit / 512-bit |
| Library | libpbc + libgmp (CGo) |
| Pubkey size | ~66 bytes (compressed G2 element) |
| Signature size | ~33 bytes (compressed G1 element) |
| Memory | Manual `element_clear` / `.Free()` everywhere |
| Standard | None (DFINITY internal) |
| Origin | `crypto/multi_sig/bls.go` |

### Where BLS is used today

```
types/bls_common.go                   — BlsCommon wrapper (sign, verify, aggregate, SplitPubKey)
types/validator.go                    — Validator.BlsPubKey (bls.PublicKey)
types/delegate.go                     — Delegate.BlsPubKey (bls.PublicKey); LoadDelegatesFile
block/block.go                        — Block.VerifyQC (ThresholdVerify)
consensus/reactor.go                  — key loading, committee formation, VerifyQC dispatch
consensus/pacemaker.go                — instantiates QCVoteManager/TCVoteManager with bls.System
consensus/pacemaker_send.go           — blsCommon.SignHash for vote and wish-vote messages
consensus/pacemaker_propose.go        — signing proposal blocks
consensus/pacemaker_assist.go         — TC signature verification (bls.PublicKey slice + SigFromBytes)
consensus/qc_vote_manager.go          — holds bls.System; deserialises + aggregates QC vote sigs
consensus/tc_vote_manager.go          — holds bls.System; deserialises + aggregates TC vote sigs
consensus/reactor_assist.go           — SplitPubKey for committee member loading
consensus/reactor_bootstrap_committee.go — SplitPubKey for bootstrap committee
consensus/governor/stats_tx.go        — ComputeDoubleSigner (direct bls.Verify); combinePubKey
cmd/meter/key_loader.go               — load/save PBC params, pairing, system, keys from keystore
cmd/meter/utils.go                    — writeOutKeys, GetBlsSystem
cmd/meter/must.go                     — node startup, passes BlsCommon to reactor
cmd/mdb/main.go                       — block explorer tool (NewBlsCommon)
script/staking/*                      — on-chain pubkey storage and retrieval
```

Serialised BLS pubkeys appear in two places on-chain:
1. **Staking contract storage** — registered by each validator candidate
2. **Block `CommitteeInfo`** — packed into the first MBlock of each epoch

The `comboPubKey` string format `"base64(ecdsa_pub):::base64(pbc_bls_pub)"` is written to
`delegates.json` and to on-chain staking candidate records. `SplitPubKey` in
`bls_common.go` parses this format and calls `system.PubKeyFromBytes` — an
implementation-specific call that must be updated post-fork.

---

## Target Implementation — BLS12-381 / blst

| Property | Target (BLS12-381) |
|---|---|
| Curve | BLS12-381 |
| Library | `github.com/supranational/blst` (single bundled C file, no libgmp) |
| Pubkey size | 48 bytes (G1, minimal-pubkey-size variant) |
| Signature size | 96 bytes (G2) |
| Memory | GC-managed (no manual Free) |
| Standard | IETF draft-irtf-cfrg-bls-signature-04 |
| Compatibility | Identical to Ethereum beacon chain |

The **minimal-pubkey-size** variant is chosen to match Ethereum: pubkeys are G1
(48 bytes), signatures are G2 (96 bytes). This is the variant used by all Ethereum
consensus clients (Prysm, Lighthouse, Teku, Nimbus).

---

## PR Breakdown

### PR 1 — `crypto/bls12381`: new package + shared interface

**Goal**: Land the new BLS implementation and define the interface that both old
and new implementations satisfy. Zero changes to consensus code — this PR is
entirely additive.

**Files created / changed**:

```
crypto/bls12381/
  bls.go          — blst wrapper (Sign, Verify, Aggregate, AggregatePubkeys)
  keygen.go       — key generation from random seed
  bls_test.go     — sign/verify/aggregate unit tests

types/bls_iface.go  — BLSCommon interface (new file, replaces direct bls.* usage)
```

**Interface definition** (`types/bls_iface.go`):

```go
type BLSCommon interface {
    // Key access
    PubKeyBytes() []byte
    SignHash(hash [32]byte) []byte

    // Verification
    ThresholdVerify(sig []byte, hash [32]byte, pubkeys [][]byte) (bool, error)

    // Serialization helpers used by delegate loading
    PubKeyFromBytes(b []byte) (BLSPublicKey, error)
    SigFromBytes(b []byte) ([]byte, error)
    PubKeyToBytes(BLSPublicKey) []byte
    GetSystem() BLSSystem  // returns self; kept for call-site compat during transition
}

// BLSPublicKey is an opaque handle returned by PubKeyFromBytes.
// Callers must not inspect its internals.
type BLSPublicKey interface {
    Bytes() []byte
    Free()  // no-op for BLS12-381; exists so old call sites compile unchanged
}
```

**Testing requirements**:
- Sign a known hash, verify it
- Aggregate 10 signatures, threshold-verify against 10 pubkeys
- Verify that a signature by key A does not verify against key B
- Benchmark: threshold-verify 100 signers (should be < 5ms on modern hardware)

**Dependencies**: none — this PR does not touch any existing file except adding
the interface file.

---

### PR 2 — Thread the interface through types and consensus

**Goal**: Replace all concrete `bls.PublicKey` / `bls.System` / `bls.Signature`
references with the `BLSCommon` / `BLSPublicKey` interface. Remove all `.Free()`
calls on BLS12-381 paths (no-op on PBC path via the shim). The `BlsCommon` struct
continues to wrap the old PBC implementation — the new implementation is not yet
activated.

**Files changed**:

```
types/bls_common.go                   — implement BLSCommon interface; wrap old calls; update SplitPubKey
types/validator.go                    — Validator.BlsPubKey: bls.PublicKey → BLSPublicKey
types/delegate.go                     — Delegate.BlsPubKey: bls.PublicKey → BLSPublicKey
block/block.go                        — VerifyQC: use interface ThresholdVerify ([][]byte pubkeys)
consensus/reactor.go                  — use interface throughout
consensus/pacemaker.go                — pass blsCommon to QCVoteManager/TCVoteManager instead of bls.System
consensus/pacemaker_send.go           — already uses blsCommon.SignHash; no change needed
consensus/pacemaker_propose.go        — signing via interface
consensus/pacemaker_assist.go         — TC verify: bls.PublicKey slice → [][]byte via BlsPubKeyBytes
consensus/qc_vote_manager.go          — replace bls.System with BLSCommon interface; update Aggregate
consensus/tc_vote_manager.go          — replace bls.System with BLSCommon interface; update Aggregate
consensus/reactor_assist.go           — SplitPubKey call site: update to interface
consensus/reactor_bootstrap_committee.go — SplitPubKey call site: update to interface
consensus/governor/stats_tx.go        — ComputeDoubleSigner: replace direct bls.Verify with interface;
                                        combinePubKey: replace *bls.PublicKey with BLSPublicKey
cmd/meter/key_loader.go               — add BLS12-381 key load/save alongside existing PBC keys
cmd/meter/utils.go                    — update writeOutKeys, GetBlsSystem to interface
cmd/meter/must.go                     — pass updated BlsCommon to reactor
cmd/mdb/main.go                       — update NewBlsCommon() usage
```

**Key changes in detail**:

*`types/bls_common.go`*: `BlsCommon` gains a `impl BLSCommon` field. All methods
delegate to `impl`. Initially `impl` is always the old PBC-backed struct. Later
(PR 4) it will be swapped at the fork height.

```go
type BlsCommon struct {
    impl    BLSCommon   // either pbcImpl or bls12381Impl
    // existing exported fields kept for now to avoid breaking more call sites
    Initialized bool
}
```

`SplitPubKey` must also be updated: currently it calls `cc.GetSystem().PubKeyFromBytes`
which is a PBC-specific call. The updated version should route through the interface.

*`types/validator.go`*: `BlsPubKey bls.PublicKey` becomes `BlsPubKey BLSPublicKey`.
`BlsPubKeyBytes []byte` is already present and becomes the canonical representation.

**Important**: `Delegate.BlsPubKey` has a JSON struct tag (`"bsl_pubkey"`) but is
actually a CGo type that cannot be JSON-marshalled. The field is populated via
`SplitPubKey` not from JSON. Changing the type to `BLSPublicKey` (interface) is
safe, but the JSON tag should be removed to avoid confusion.

*`consensus/qc_vote_manager.go` and `tc_vote_manager.go`*: These are the hot path
for signature aggregation. Currently they hold `bls.System` directly and call
`bls.Aggregate`. They must accept `BLSCommon` so the correct aggregation
implementation is used post-fork:

```go
// Before
type QCVoteManager struct {
    system bls.System
    ...
}
func NewQCVoteManager(system bls.System, committeeSize uint32) *QCVoteManager

// After
type QCVoteManager struct {
    blsCommon *types.BlsCommon
    ...
}
func NewQCVoteManager(blsCommon *types.BlsCommon, committeeSize uint32) *QCVoteManager
```

*`pacemaker_assist.go`*: TC verification currently builds a `[]bls.PublicKey` slice
and calls `blsCommon.System.SigFromBytes`. Both must change to use `[][]byte`
(from `BlsPubKeyBytes`) and the interface `SigFromBytes`.

*`consensus/governor/stats_tx.go`*: `ComputeDoubleSigner` calls `bls.Verify`
directly (bypassing `BlsCommon`). It also takes `*bls.PublicKey` in `combinePubKey`.
Both must be routed through the interface. Note: double-sign evidence embedded in
pre-fork blocks uses PBC signatures — `ComputeDoubleSigner` needs to detect which
implementation to use based on the block height of the evidence.

*`block/block.go`*:

```go
// Before
valid, err := blsCommon.ThresholdVerify(sig, escortQC.VoterMsgHash, pubkeys)

// After (pubkeys is now [][]byte via BlsPubKeyBytes)
valid, err := blsCommon.ThresholdVerify(sigBytes, escortQC.VoterMsgHash, pubkeyBytes)
```

**What this PR does NOT do**: it does not activate BLS12-381 anywhere. After this
PR the network behaviour is identical to before.

**Testing requirements**:
- All existing consensus tests pass
- `go build ./...` succeeds without libgmp on a clean Ubuntu image (PBC CGo is
  still linked; this is a compile check, not a removal)

---

### PR 3 — Staking contract: dual-key registration

**Goal**: Allow validators to register a BLS12-381 pubkey alongside their existing
PBC key before the fork activates. Both keys are stored. The network continues to
use the PBC key. After the fork height (set in PR 5), the new key takes over.

**Files changed**:

```
script/staking/handler.go            — parse BlsPubKey2 from candidate tx
script/staking/handler_candidateUpdate.go — parse BlsPubKey2 in update tx
script/staking/types.go              — Candidate struct: add BlsPubKey2 []byte
script/staking/staking_state.go      — store/load BlsPubKey2 in state
builtin/gen/...                      — regenerated ABI bindings if applicable
```

**On-chain format**:

The existing candidate registration transaction body gains an optional trailing
field `blsPubKey2` (48 bytes). Nodes running old software ignore the extra bytes.
Nodes running new software store it.

```go
type Candidate struct {
    // ... existing fields ...
    BlsPubKey  []byte  // PBC pubkey (legacy)
    BlsPubKey2 []byte  // BLS12-381 pubkey (48 bytes), empty until registered
}
```

A validator upgrades their key by submitting a new `candidateUpdate` transaction
that includes `blsPubKey2`. This can be done at any time before the fork height.

**Validation rules added**:
- If `blsPubKey2` is present and non-empty, it must be exactly 48 bytes and a
  valid BLS12-381 G1 point (check via `blst.P1AffineDeserialize`)
- A validator that has not registered `blsPubKey2` by the fork height will be
  excluded from committee formation post-fork (same effect as being offline)

**Testing requirements**:
- Register a candidate with only PBC key → `BlsPubKey2` is empty
- Register a candidate with both keys → both stored and retrievable
- Register with an invalid 48-byte blob → rejected
- Upgrade an existing candidate to add `blsPubKey2` → stored correctly

---

### PR 4 — Fork-gated block verification and committee formation

**Goal**: At the configured fork height, the node switches from PBC BLS to
BLS12-381 for all new block signing and verification. Old blocks (pre-fork) still
verify with the old implementation. The `BlsCommon.impl` field (introduced in
PR 2) is set at startup based on whether the best block is before or after the
fork, and updated when the fork block is committed.

**Files changed**:

```
meter/fork_config.go          — add TeslaFork14 (or next available) height
types/bls_common.go           — NewBlsCommonForEpoch(epoch) factory
consensus/reactor.go          — swap BlsCommon.impl when crossing fork height
block/block.go                — VerifyQC: choose impl based on block.Number()
consensus/governor/stats_tx.go — use BlsPubKey2 for post-fork committee keys
script/staking/handler_govern.go — post-fork: load BlsPubKey2 for committee
```

**Fork logic in `reactor.go`**:

```go
func (r *Reactor) UpdateCurEpoch() (bool, error) {
    // ... existing epoch logic ...

    if meter.IsTeslaFork14(bestK.Number()) {
        // switch to BLS12-381
        r.blsCommon.UseImpl(bls12381.NewImpl())
    }
    // ...
}
```

**`VerifyQC` dual-path**:

```go
func (b *Block) VerifyQC(escortQC *QuorumCert, blsCommon *types.BlsCommon, committee []*types.Validator) (bool, error) {
    // choose pubkeys from BlsPubKey vs BlsPubKey2 depending on block epoch
    var pubkeyBytes [][]byte
    if meter.IsTeslaFork14(b.Number()) {
        for _, v := range committee {
            pubkeyBytes = append(pubkeyBytes, v.BlsPubKey2Bytes)
        }
    } else {
        for _, v := range committee {
            pubkeyBytes = append(pubkeyBytes, v.BlsPubKeyBytes)
        }
    }
    return blsCommon.ThresholdVerify(escortQC.VoterAggSig, escortQC.VoterMsgHash, pubkeyBytes)
}
```

**Signing path** (`pacemaker_propose.go`):

Post-fork, `blsCommon.SignHash` routes to the BLS12-381 private key. The private
key is loaded at startup from the keystore; the keystore format adds a second
entry `bls12381_privkey` alongside the existing `bls_privkey`.

**Historical sync**: A node syncing from genesis must correctly verify both old
and new blocks. The `blsCommon` passed to `VerifyQC` always holds both
implementations; only the pubkey slice selection (above) changes per block.

**`QCVoteManager` / `TCVoteManager` re-initialisation**:

Both vote managers are instantiated in `pacemaker.go:601,606` at the start of
each epoch. After the fork, they must be instantiated with the BLS12-381-backed
`BlsCommon`. Since `BlsCommon.impl` is already swapped in `UpdateCurEpoch` (see
above), no additional change is needed here — the vote managers pick up the right
implementation via the interface.

**`pre-fork quorum check`** (from Open Question #4):

Add a guard in `PrepareEnvForPacemaker` that counts how many current committee
members have `BlsPubKey2` registered. If the count is below the 2/3 threshold,
log a warning and refuse to cross the fork height. Concretely:

```go
if meter.IsTeslaFork14(nextEpochFirstBlock) && !r.hasEnoughBLS12381Keys() {
    return errors.New("cannot cross BLS fork: fewer than 2/3 of committee have registered BLS12-381 keys")
}
```

**`comboPubKey` string format post-fork**:

The `delegates.json` file and on-chain staking records store
`"base64(ecdsa):::base64(bls_pub)"`. Post-fork, the BLS bytes are 48-byte
BLS12-381 points. The `:::` separator format is reused — `SplitPubKey` routes
the second part through `PubKeyFromBytes` on the active implementation, so it
works transparently as long as the implementation is switched first.

**`api/node/types.go`**:

`CsPubKey string` is exposed in the node REST API (committee info endpoint).
Post-fork its value changes from a ~132-char hex string (66 bytes) to a 96-char
hex string (48 bytes). External consumers (dashboards, explorers) should be
notified of this length change.

**Testing requirements**:
- A node crossing the fork height produces valid BLS12-381-signed blocks
- A node that was offline during the fork can sync across it from a peer
- Pre-fork blocks still verify correctly after the fork
- A validator without `BlsPubKey2` is excluded from committee post-fork (not
  added to the signing set; block still reaches quorum if ≥2/3 have migrated)
- `ComputeDoubleSigner` correctly handles double-sign evidence from pre-fork
  blocks (PBC signatures) when called post-fork

---

### PR 5 — Fork height, key migration tool, and testnet activation

**Goal**: Set the fork height for testnet, provide tooling for validators to
generate and register their BLS12-381 keys, and run a full end-to-end test on
testnet before setting a mainnet height.

**Files changed / created**:

```
meter/fork_config.go              — set TeslaFork14 testnet height
cmd/meter/keygen_bls12381.go      — new subcommand: meter keygen-bls12381
docs/bls12381-validator-guide.md  — step-by-step for validators (key gen,
                                    candidateUpdate submission, timeline)
```

**Keystore format change** (`cmd/meter/key_loader.go`):

The existing keystore stores these fields:

```
params      — PBC pairing parameters (ASCII, ~200 bytes)
pairing     — (derived at load time, not stored)
system      — PBC G2 generator bytes
public_key  — PBC G2 compressed pubkey bytes
private_key — PBC Zr element bytes
```

After this PR the keystore gains two new fields:

```
bls12381_public_key   — 48 bytes (BLS12-381 G1 compressed)
bls12381_private_key  — 32 bytes (BLS12-381 scalar)
```

Old keystores without these fields are valid: the node will log a warning and
refuse to start if it is past the fork height without BLS12-381 keys present,
prompting the operator to run `meter keygen-bls12381`.

**`meter keygen-bls12381` subcommand**:

```
Usage: meter keygen-bls12381 [--keystore <path>] [--output <path>]

Reads the node's existing keystore, generates a BLS12-381 keypair derived
from the same entropy (or freshly random), writes the private key into the
keystore under the key "bls12381_privkey", and prints the candidateUpdate
transaction data that the operator must submit on-chain to register the new
public key before the fork height.

Output:
  BLS12-381 public key : 0xabc123...  (48 bytes hex)
  candidateUpdate tx data: 0x...       (paste into wallet or use meter-cli)
```

Derivation: the BLS12-381 private key is derived as
`HKDF-SHA256(ikm=ecdsa_privkey_bytes, info="meter-bls12381-v1")` so operators
who lose the keystore file can re-derive it from their ECDSA key.

**Testnet activation timeline** (suggested):

| Date | Action |
|---|---|
| T-0 | PR 5 merged, testnet fork height announced |
| T+1 week | Validators submit `candidateUpdate` with BLS12-381 keys |
| T+2 weeks | Fork height reached on testnet |
| T+3 weeks | Observe 2 full epochs post-fork, no issues |
| T+4 weeks | Set mainnet height in follow-up PR |

**Testing requirements**:
- Run a 4-node local testnet, all nodes migrate keys before the fork height →
  network continues uninterrupted across the fork
- Run a 4-node local testnet, one node does not migrate → that node is excluded
  from committee post-fork, the other 3 still reach quorum
- Run a 4-node local testnet, start a fresh node after the fork, sync from
  genesis → all historical blocks verify correctly

---

## Wire Format Summary

### Pre-fork (existing)

```
CommitteeInfo.CSPubKey  — PBC G2 compressed, variable length (~66 bytes)
QuorumCert.VoterAggSig  — PBC G1 compressed, variable length (~33 bytes)
```

### Post-fork (BLS12-381)

```
CommitteeInfo.CSPubKey  — BLS12-381 G1 compressed, exactly 48 bytes
QuorumCert.VoterAggSig  — BLS12-381 G2 compressed, exactly 96 bytes
```

The `Block` RLP encoding does not change. `CommitteeInfo.CSPubKey` and
`QuorumCert.VoterAggSig` are both `[]byte` fields today and remain so. The
length change (33→96 for sigs, 66→48 for pubkeys) increases block size by
roughly `(96-33) + N*(48-66)` bytes per KBlock committee, where N is committee
size. For N=101 validators this is approximately +63 - 1818 = **-1755 bytes**
per KBlock — a slight reduction.

---

## Dependency Graph

```
PR 1 (new package + interface)
  └─ PR 2 (thread interface through types)
       ├─ PR 3 (staking dual-key)
       └─ PR 4 (fork gate)  ← depends on PR 3
            └─ PR 5 (fork height + tooling)
```

PRs 1 and 2 have no consensus risk and can be merged to `testnet` immediately.
PRs 3–5 require validator coordination and should be batched to `testnet` first,
then `mainnet` after a successful testnet epoch crossing.

---

## Risks and Mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| Validator misses registration deadline | Medium | Exclude from committee (not slash); they re-join next epoch after registering |
| blst CGo still breaks on some build targets | Low | blst bundles its own C; no system libraries needed. Test on alpine/arm64. |
| Historical sync broken by fork-gate bug | Medium | Extensive sync tests (PR 4 requirements); keep old impl indefinitely |
| BLS12-381 aggregate sig verification slower than PBC | Low | blst threshold-verify 101 signers ≈ 1.5ms; PBC ≈ 8ms. Net improvement. |
| Key derivation collides (HKDF produces weak key) | Very Low | HKDF-SHA256 is standard; add a sanity check that derived key ≠ 0 |
| TC (timeout cert) path broken post-fork | Medium | `pacemaker_assist.go` TC verify path needs same dual-path as QC; covered in PR 2 scope but easy to miss |
| Pre-fork double-sign evidence rejected post-fork | Medium | `ComputeDoubleSigner` uses raw `bls.Verify`; needs fork-height branch to use correct impl for evidence blocks |
| `QCVoteManager`/`TCVoteManager` not re-initialised at fork epoch | Medium | Verify vote managers are re-created each epoch in `pacemaker.go`; they are (lines 601,606), so switching `blsCommon.impl` is sufficient |
| External API consumers break on pubkey length change | Low | `CsPubKey` in node API changes from 132 to 96 hex chars; notify dashboard/explorer operators |

---

## Open Questions

1. **Should BLS12-381 private keys share the same keystore file, or live in a
   separate `bls12381.key` file?** Separate file is simpler for operators but
   adds another file to manage. Recommendation: same keystore, new JSON field.

2. **Fork name**: is `TeslaFork14` the right label, or should this get a named
   fork constant (e.g. `meter.IsBLS12381Fork`)?

3. **blst vs gnark-crypto**: blst is CGo (one bundled C file, no system deps)
   and ~3× faster. gnark-crypto is pure Go but slower and less battle-tested for
   BLS. Recommendation: blst, same as Prysm/Lighthouse.

4. **Minimum migration threshold**: should the fork activate even if < 2/3 of
   validators have registered BLS12-381 keys? If so the first post-fork epoch
   would immediately fail to reach quorum. Recommendation: add a pre-fork check
   in `PrepareEnvForPacemaker` that refuses to cross the fork height until ≥ 2/3
   of current committee members have `BlsPubKey2` registered.

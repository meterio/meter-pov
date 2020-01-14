# Versions

## 1.3.1-beta7

Added:

1. Send redundancy proposals to protect from network partition

## 1.3.1-beta6

Updated:

1. further updated log output

## 1.3.1-beta5

Added:

1. `blockID` check in `BlockMatchQC`
2. `peerName` in the log together with send and receive message
3. check the height before timeout onbeat to protect from the potential fork

Updated:

1. fix for the address mismatch for delegates and validators
2. simplified the log for better readability

## 1.3.1-beta4

Added:

1. `name` field for delegates and validators
2. `BlockMatchQC` for PMNewView in order to protect from unexpected out-of-sequence arrival of PMNewView and PMProposal

Removed:

1. enforced round number for proposal

## 1.3.1-beta3

Added:

1. enforce `epochID`, anything without different `epochID` will be discarded
2. shorten the log for bestQC broadcast
3. update docker build script
4. ignore PMProposal with expired round

## 1.3.1-beta2

Added:

1. communication `magic` based on version and discover topic
2. reject communication with mismatch `magic`

Removed:

1. set `magic` purely with discover topic

## 1.3.1-beta1

Added:

1. ignore PMNewView with expired round
2. ignore PMVoteForProposal with expired round

## 1.3.0

_BREAKING CHANGE_

Added:

1. automatically set `magic` with `--disco-topic` flag
2. enforce `magic` with all rpc/consensus/pacemaker traffic, ignore all the messages with different `magic` param

## 1.2.1

Added:

1. prometheus metrics for monitoring
2. API for staking
3. sync bestQC with gossip messages

Fixed:

1. pacemaker multiple stop crash
2. fake "fork happened"
3. race condition for accessing validator set
4. revert handling for pacemaker, remove precommited blocks from db
5. waiting for pacemaker to stop

## 1.2.0

Added:

1. staking operations: candidate, delegate, bound
2. better handling for pending proposals
3. proposal relay

## 1.1.3

draft version

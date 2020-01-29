# Versions

## 1.3.3-beta9

Updated:

1. apply the best qc immediately when updating qc candidate

## 1.3.3-beta8

Updated:

1. filter delegates

## 1.3.3-beta7

Updated:

1. Calculate delegate with 300 MTRG threshold in staking module

## 1.3.3-beta6

Added:

1. dynamic committee options --committee-min-size, --committee-max-size and --delegates-max-size
2. bootstrap options --init-configured-delegates and --epoch-mblock-count
3. require the candidates to have at least 300 MTRG total votes to be considered as delegates

## 1.3.3-beta5

Added:

1. sort the delegate list with descending voting power
2. select the first n delegates to be committee members (n being committee size)

## 1.3.3-beta3

Added:

1. sync mechanism for blocks with height larger than current best block
2. tailing newline in public.key file
3. bootstrap options for dynamic committee size
4. add protection in OnBeat to prevent beat on invalid height

## 1.3.3-beta2

Updated:

1. fix the bug of proposal query
2. fix the bug of peers of committee leader messages

## 1.3.3-beta1

Added:

1. range query for missing proposals

Updated:

2. before the new qc is populated by proposals, dont start the timer for next round

## 1.3.2

Added:

1. add state sync at receiving new view timeout
2. more readable log for committee establishment

Updated:

1. Typos for wallet API
2. display members who chould not join the committee in /node/consensus/committee API

## 1.3.1-beta11

## 1.3.1-beta10

Added:

1. forward missing proposals to peers when receive new view message with a lower expected height

## 1.3.1-beta10

Added:

1. update curEpoch after processing blocks, so curEpoch will be up to date

## 1.3.1-beta9

Added:

1. Update curEpoch with bestQC before send out NewCommittee message
2. Update curEpoch when receive NewCommittee message

## 1.3.1-beta8

Added:

1. start pacemaker even if there are not enough vote for notary

Updated:

1. fixed the missing committee info bug
2. added committee message log

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

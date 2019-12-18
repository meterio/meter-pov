# Versions

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

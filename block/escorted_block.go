package block

type EscortedBlock struct {
	Block    *Block
	EscortQC *QuorumCert
}

package consensus

import "github.com/prometheus/client_golang/prometheus"

var (
	pmRoundGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pacemaker_round",
		Help: "Current round of pacemaker",
	})
	pmRunningGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pacemaker_running",
		Help: "status of pacemaker (0-false, 1-true)",
	})
	curEpochGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "current_epoch",
		Help: "Current epoch of consensus",
	})
	inCommitteeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "in_committee",
		Help: "is this node in committee",
	})
	pmRoleGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pacemaker_role",
		Help: "Role in pacemaker",
	})
	lastKBlockHeightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_kblock_height",
		Help: "Height of last k-block",
	})
	blocksCommitedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "blocks_commited_total",
		Help: "Counter of commited blocks locally",
	})
)

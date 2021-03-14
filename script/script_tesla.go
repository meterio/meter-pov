// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package script

import ()

func EnterTeslaForkInit() {
	se := GetScriptGlobInst()
	if se == nil {
		panic("get script engine failed ... ")
	}

	se.StartTeslaForkModules()
	se.logger.Info("enabled script tesla modules ... ")
}

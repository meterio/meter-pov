package script

import ()

// global data
type ScriptEngine struct {
	logger log15.Logger
}

type ScriptHeader struct {
	Pattern    [4]byte
	Version    uint32
	ModID      uint32
	PayLoadLen uint32
}

type Script struct {
	Header  ScriptHeader
	Payload []byte
}

type scriptPayload []byte

// Version returns the version
func (sh *ScriptHeader) Version() uint32 { return sh.Version }

func (sh *ScriptHeader) ModID() uint32 { return sh.ModID }

func (sh *ScriptHeader) PayLoadLen() uint32 { return sh.PayLoadLen }

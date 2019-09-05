package script

import ()

// Builder is used to build an action.
type Builder struct {
    Header  ScriptHeader
    Payload []byte
}

// SetVersion sets action's version.
func (b *Builder) SetVersion(v uint32) *Builder {
    b.Header.Version = v
    return b
}

// SetNonce sets action's nonce.
func (b *Builder) SetPattern(pa [4]byte) *Builder {
    b.Header.Pattern = pa
    return b
}

func (b *Builder) SetModID(id uint32) *Builder {
    b.Header.ModID = id
    return b
}

// SetGasLimit sets action's gas limit.
func (b *Builder) SetPayLoadLen(l uint32) *Builder {
    b.Header.PayloadLen = l
    return b
}

// SetGasPrice sets action's gas price.
func (b *Builder) SetPayload(p []byte) *Builder {
    b.Payload = p
    return b
}

// Build build a block object.
func (b *Builder) Build() *Script {
    return &Script{
        Header:  b.Header,
        Payload: b.Payload,
    }
}

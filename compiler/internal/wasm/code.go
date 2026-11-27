package wasm

// Opcodes (Auswahl).
const (
	OpUnreachable  byte = 0x00
	OpNop          byte = 0x01
	OpBlock        byte = 0x02
	OpLoop         byte = 0x03
	OpIf           byte = 0x04
	OpElse         byte = 0x05
	OpEnd          byte = 0x0B
	OpBr           byte = 0x0C
	OpBrIf         byte = 0x0D
	OpBrTable      byte = 0x0E
	OpReturn       byte = 0x0F
	OpCall         byte = 0x10
	OpCallIndirect byte = 0x11
	OpDrop         byte = 0x1A
	OpSelect       byte = 0x1B
	OpLocalGet     byte = 0x20
	OpLocalSet     byte = 0x21
	OpLocalTee     byte = 0x22
	OpGlobalGet    byte = 0x23
	OpGlobalSet    byte = 0x24

	OpI32Load    byte = 0x28
	OpF32Load    byte = 0x2A
	OpF64Load    byte = 0x2B
	OpI32Load8S  byte = 0x2C
	OpI32Load8U  byte = 0x2D
	OpI32Load16S byte = 0x2E
	OpI32Load16U byte = 0x2F
	OpI32Store   byte = 0x36
	OpF32Store   byte = 0x38
	OpF64Store   byte = 0x39
	OpI32Store8  byte = 0x3A
	OpI32Store16 byte = 0x3B
	OpMemorySize byte = 0x3F
	OpMemoryGrow byte = 0x40

	OpI32Const byte = 0x41
	OpI64Const byte = 0x42
	OpF32Const byte = 0x43
	OpF64Const byte = 0x44

	OpI32Eqz byte = 0x45
	OpI32Eq  byte = 0x46
	OpI32Ne  byte = 0x47
	OpI32LtS byte = 0x48
	OpI32LtU byte = 0x49
	OpI32GtS byte = 0x4A
	OpI32GtU byte = 0x4B
	OpI32LeS byte = 0x4C
	OpI32LeU byte = 0x4D
	OpI32GeS byte = 0x4E
	OpI32GeU byte = 0x4F
	OpF64Eq  byte = 0x61
	OpF64Ne  byte = 0x62
	OpF64Lt  byte = 0x63
	OpF64Gt  byte = 0x64
	OpF64Le  byte = 0x65
	OpF64Ge  byte = 0x66

	OpI32Add  byte = 0x6A
	OpI32Sub  byte = 0x6B
	OpI32Mul  byte = 0x6C
	OpI32DivS byte = 0x6D
	OpI32RemS byte = 0x6F
	OpI32And  byte = 0x71
	OpI32Or   byte = 0x72
	OpI32Xor  byte = 0x73
	OpI32Shl  byte = 0x74
	OpI32ShrS byte = 0x75
	OpI32ShrU byte = 0x76

	OpF64Abs      byte = 0x99
	OpF64Neg      byte = 0x9A
	OpF64Ceil     byte = 0x9B
	OpF64Floor    byte = 0x9C
	OpF64Trunc    byte = 0x9D
	OpF64Nearest  byte = 0x9E
	OpF64Sqrt     byte = 0x9F
	OpF64Add      byte = 0xA0
	OpF64Sub      byte = 0xA1
	OpF64Mul      byte = 0xA2
	OpF64Div      byte = 0xA3
	OpF64Copysign byte = 0xA6

	OpI32TruncF64S   byte = 0xAA
	OpI64ExtendI32S  byte = 0xAC
	OpI64ExtendI32U  byte = 0xAD
	OpF32DemoteF64   byte = 0xB6
	OpF64ConvertI32S byte = 0xB7
	OpF64PromoteF32  byte = 0xBB
	OpI32Extend8S    byte = 0xC0
	OpI32Extend16S   byte = 0xC1

	opPrefixFC byte = 0xFC
)

// Code sammelt die Instruktionen eines Funktionsrumpfs.
type Code struct {
	B []byte
}

// Op hängt einen Opcode ohne Operanden an.
func (c *Code) Op(ops ...byte) { c.B = append(c.B, ops...) }

// I32Const hängt i32.const v an.
func (c *Code) I32Const(v int32) {
	c.B = append(c.B, OpI32Const)
	c.B = AppendS32(c.B, v)
}

// F64Const hängt f64.const v an.
func (c *Code) F64Const(v float64) {
	c.B = append(c.B, OpF64Const)
	c.B = AppendF64(c.B, v)
}

// Index hängt einen Opcode mit einem Index-Operanden an (local.get, call, br ...).
func (c *Code) Index(op byte, i uint32) {
	c.B = append(c.B, op)
	c.B = AppendU32(c.B, i)
}

// Block hängt block/loop/if mit Blocktyp an.
func (c *Code) Block(op byte, blockType byte) { c.B = append(c.B, op, blockType) }

// Mem hängt eine Lade- oder Speicherinstruktion mit Ausrichtung (log2) und Offset an.
func (c *Code) Mem(op byte, alignLog2, offset uint32) {
	c.B = append(c.B, op)
	c.B = AppendU32(c.B, alignLog2)
	c.B = AppendU32(c.B, offset)
}

// MemoryCopy hängt memory.copy (Ziel, Quelle, Länge) an.
func (c *Code) MemoryCopy() { c.B = append(c.B, opPrefixFC, 0x0A, 0x00, 0x00) }

// MemoryFill hängt memory.fill (Ziel, Wert, Länge) an.
func (c *Code) MemoryFill() { c.B = append(c.B, opPrefixFC, 0x0B, 0x00) }

// SIMD-Opcodes (Präfix 0xFD).
const (
	SimdV128Load    uint32 = 0x00
	SimdV128Store   uint32 = 0x0B
	SimdV128And     uint32 = 0x4E
	SimdV128AndNot  uint32 = 0x4F
	SimdV128Or      uint32 = 0x50
	SimdV128Xor     uint32 = 0x51
	SimdV128AnyTrue uint32 = 0x53
)

// SIMD hängt eine v128-Instruktion ohne Operanden an.
func (c *Code) SIMD(op uint32) {
	c.B = append(c.B, 0xFD)
	c.B = AppendU32(c.B, op)
}

// SIMDMem hängt v128.load oder v128.store mit Ausrichtung und Offset an.
func (c *Code) SIMDMem(op uint32, alignLog2, offset uint32) {
	c.SIMD(op)
	c.B = AppendU32(c.B, alignLog2)
	c.B = AppendU32(c.B, offset)
}

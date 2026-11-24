package wasm_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/tetratelabs/wazero"

	"github.com/lehrcode/wasmpascal/compiler/internal/wasm"
)

func TestLEB128(t *testing.T) {
	u := []struct {
		v    uint32
		want []byte
	}{
		{0, []byte{0}}, {127, []byte{0x7f}}, {128, []byte{0x80, 0x01}},
		{624485, []byte{0xe5, 0x8e, 0x26}}, {0xFFFFFFFF, []byte{0xff, 0xff, 0xff, 0xff, 0x0f}},
	}
	for _, tt := range u {
		if got := wasm.AppendU32(nil, tt.v); !bytes.Equal(got, tt.want) {
			t.Errorf("U32(%d) = % x, want % x", tt.v, got, tt.want)
		}
	}
	s := []struct {
		v    int64
		want []byte
	}{
		{0, []byte{0}}, {-1, []byte{0x7f}}, {63, []byte{0x3f}}, {64, []byte{0xc0, 0x00}},
		{-64, []byte{0x40}}, {-65, []byte{0xbf, 0x7f}}, {-123456, []byte{0xc0, 0xbb, 0x78}},
		{-2147483648, []byte{0x80, 0x80, 0x80, 0x80, 0x78}},
	}
	for _, tt := range s {
		if got := wasm.AppendS64(nil, tt.v); !bytes.Equal(got, tt.want) {
			t.Errorf("S64(%d) = % x, want % x", tt.v, got, tt.want)
		}
	}
}

// TestModuleValidates baut ein Modul mit Speicher, Global, Datensegment,
// Schleife und Speicherzugriffen, lässt es von wazero validieren und ruft es auf.
func TestModuleValidates(t *testing.T) {
	m := &wasm.Module{MemMin: 2}
	m.Globals = []wasm.Global{{Type: wasm.I32, Mutable: true, Init: wasm.AppendS32([]byte{wasm.OpI32Const}, 1000)}}
	m.Data = []wasm.Data{{Offset: 65536, Bytes: []byte{1, 2, 3, 4, 5}}}

	// summe(n): Summe der ersten n Bytes ab 65536 plus Global, über eine Schleife.
	var c wasm.Code
	c.Block(wasm.OpBlock, wasm.BlockEmpty)
	c.Block(wasm.OpLoop, wasm.BlockEmpty)
	c.Index(wasm.OpLocalGet, 1) // i
	c.Index(wasm.OpLocalGet, 0) // n
	c.Op(wasm.OpI32GeS)
	c.Index(wasm.OpBrIf, 1)
	c.Index(wasm.OpLocalGet, 2)
	c.Index(wasm.OpLocalGet, 1)
	c.Mem(wasm.OpI32Load8U, 0, 65536)
	c.Op(wasm.OpI32Add)
	c.Index(wasm.OpLocalSet, 2)
	c.Index(wasm.OpLocalGet, 1)
	c.I32Const(1)
	c.Op(wasm.OpI32Add)
	c.Index(wasm.OpLocalSet, 1)
	c.Index(wasm.OpBr, 0)
	c.Op(wasm.OpEnd, wasm.OpEnd)
	c.Index(wasm.OpLocalGet, 2)
	c.Index(wasm.OpGlobalGet, 0)
	c.Op(wasm.OpI32Add)
	// memory.copy/fill sind gültig
	c.I32Const(70000)
	c.I32Const(65536)
	c.I32Const(5)
	c.MemoryCopy()
	c.I32Const(70010)
	c.I32Const(0)
	c.I32Const(4)
	c.MemoryFill()
	c.Op(wasm.OpEnd)
	typ := m.TypeIndex(wasm.FuncType{Params: []wasm.ValType{wasm.I32}, Results: []wasm.ValType{wasm.I32}})
	if again := m.TypeIndex(wasm.FuncType{Params: []wasm.ValType{wasm.I32}, Results: []wasm.ValType{wasm.I32}}); again != typ {
		t.Errorf("TypeIndex legt doppelt an")
	}
	m.Funcs = []*wasm.Func{{Type: typ, Locals: []wasm.ValType{wasm.I32, wasm.I32}, Code: c.B}}
	m.Exports = []wasm.Export{{Name: "summe", Kind: wasm.ExportFunc, Index: 0}, {Name: "memory", Kind: wasm.ExportMemory}}
	m.Customs = []wasm.Custom{{Name: "test", Bytes: []byte("x")}}

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	mod, err := rt.Instantiate(ctx, m.Encode())
	if err != nil {
		t.Fatal(err)
	}
	res, err := mod.ExportedFunction("summe").Call(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}
	if res[0] != 1010 {
		t.Errorf("summe(4) = %d, want 1010", res[0])
	}
	if b, _ := mod.Memory().Read(70000, 5); !bytes.Equal(b, []byte{1, 2, 3, 4, 5}) {
		t.Errorf("memory.copy: % x", b)
	}
}

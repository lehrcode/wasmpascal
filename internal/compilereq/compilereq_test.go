package compilereq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lehrcode/wasmpascal/compiler"
)

const hallo = "program Hallo; begin WriteLn('Hallo') end."

func decodeHandle(t *testing.T, line string) Response {
	t.Helper()
	req, err := Decode([]byte(line))
	if err != nil {
		return RequestError(req.ID, err)
	}
	return Handle(context.Background(), req)
}

func TestSuccessByteEqual(t *testing.T) {
	for _, target := range []string{"", "wasi", "gojs"} {
		line := fmt.Sprintf(`{"id": "a1", "files": {"hallo.pas": %q}, "target": %q}`, hallo, target)
		resp := decodeHandle(t, line)
		if resp.Kind != "" || resp.Error != "" {
			t.Fatalf("%s: %+v", target, resp)
		}
		want := compiler.TargetWASI
		if target == "gojs" {
			want = compiler.TargetGoJS
		}
		res, err := compiler.Compile(context.Background(), "hallo.pas", compiler.Options{
			Target: want, FS: fstest.MapFS{"hallo.pas": {Data: []byte(hallo)}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(resp.Wasm, res.Wasm) {
			t.Errorf("%s: Modul weicht von compiler.Compile ab", target)
		}
		if string(resp.ID) != `"a1"` {
			t.Errorf("id = %s", resp.ID)
		}
		// Das Modul steht base64-kodiert im JSON.
		out, _ := json.Marshal(resp)
		if !strings.Contains(string(out), `"wasm":"AGFzbQ`) {
			t.Errorf("JSON %.60s…", out)
		}
	}
}

func TestSourceError(t *testing.T) {
	resp := decodeHandle(t, `{"id": 7, "files": {"p.pas": "program P;\nbegin\n  x := 1\nend."}}`)
	want := Response{ID: json.RawMessage("7"), Kind: KindSource, Error: "p.pas(3,3): Error 3: Unknown identifier (x)",
		File: "p.pas", Line: 3, Column: 3, Code: 3}
	got, _ := json.Marshal(resp)
	exp, _ := json.Marshal(want)
	if string(got) != string(exp) {
		t.Errorf("\n got: %s\nwant: %s", got, exp)
	}
}

func TestBadRequests(t *testing.T) {
	tests := []struct{ name, line, want string }{
		{"kein json", `{"files": `, "JSON"},
		{"unbekanntes feld", `{"id": 1, "files": {"p.pas": "x"}, "limits": {"maxDeph": 3}}`, "maxDeph"},
		{"files fehlt", `{"id": 1}`, "files"},
		{"main fehlt", `{"files": {"a.pas": "x", "b.pas": "y"}}`, "main"},
		{"main unbekannt", `{"files": {"a.pas": "x"}, "main": "b.pas"}`, "b.pas"},
		{"name mit ..", `{"files": {"../x.pas": "x"}}`, "../x.pas"},
		{"name absolut", `{"files": {"/x.pas": "x"}}`, "/x.pas"},
		{"ziel", `{"files": {"p.pas": "x"}, "target": "dos"}`, "dos"},
		{"stack negativ", `{"files": {"p.pas": "x"}, "stack": -1}`, "Stack"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := decodeHandle(t, tt.line)
			if resp.Kind != KindRequest || !strings.Contains(resp.Error, tt.want) {
				t.Errorf("%+v, want kind request mit %q", resp, tt.want)
			}
		})
	}
	// Auch bei einer kaputten Anfrage kommt die id zurück, soweit lesbar.
	if resp := decodeHandle(t, `{"id": 42, "files": {"p.pas": "x"}, "foo": 1}`); string(resp.ID) != "42" {
		t.Errorf("id = %s", resp.ID)
	}
}

func TestLimitsReachCompiler(t *testing.T) {
	resp := decodeHandle(t, `{"files": {"p.pas": "program P; var x: Integer; begin x := ((((1)))) end."}, "limits": {"maxDepth": 4}}`)
	if resp.Kind != KindSource || resp.Code != 904 {
		t.Errorf("%+v", resp)
	}
	resp = decodeHandle(t, `{"files": {"p.pas": "program P; var A: array[1..2000] of Byte; begin end."}, "limits": {"maxStaticData": 1000}}`)
	if resp.Code != 49 {
		t.Errorf("%+v", resp)
	}
}

func TestNoHostFiles(t *testing.T) {
	for _, inc := range []string{"/etc/passwd", "../geheim.inc", `..\geheim.inc`} {
		src, _ := json.Marshal(map[string]any{"files": map[string]string{"p.pas": "program P; {$I " + inc + "} begin end."}})
		resp := decodeHandle(t, string(src))
		if resp.Kind != KindSource || (resp.Code != 14 && resp.Code != 15) {
			t.Errorf("{$I %s}: %+v", inc, resp)
		}
	}
}

func TestTimeout(t *testing.T) {
	src := "program P; var x: Integer; begin\n" + strings.Repeat("x := 1;\n", 300_000) + "end."
	req := Request{Files: map[string]string{"p.pas": src}, Limits: Limits{MaxTokens: -1}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if resp := Handle(ctx, req); resp.Kind != KindTimeout {
		t.Errorf("%+v", resp)
	}
}

func TestInternal(t *testing.T) {
	broken := func(context.Context, string, compiler.Options) (*compiler.Result, error) {
		return nil, fmt.Errorf("%w: kaputt", compiler.ErrInternal)
	}
	resp := handle(context.Background(), Request{Files: map[string]string{"p.pas": hallo}}, broken)
	if resp.Kind != KindInternal || !strings.Contains(resp.Error, "kaputt") {
		t.Errorf("%+v", resp)
	}
	panicky := func(context.Context, string, compiler.Options) (*compiler.Result, error) { panic("au") }
	resp = handle(context.Background(), Request{Files: map[string]string{"p.pas": hallo}}, panicky)
	if resp.Kind != KindInternal || !strings.Contains(resp.Error, "au") {
		t.Errorf("%+v", resp)
	}
}

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type serveResp struct {
	ID    json.RawMessage `json:"id"`
	Wasm  []byte          `json:"wasm"`
	Kind  string          `json:"kind"`
	Error string          `json:"error"`
	Code  int             `json:"code"`
}

func serveLines(t *testing.T, args []string, input string) ([]serveResp, int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), append([]string{"serve"}, args...), strings.NewReader(input), &stdout, &stderr)
	var resps []serveResp
	sc := bufio.NewScanner(&stdout)
	sc.Buffer(nil, 64<<20)
	for sc.Scan() {
		var r serveResp
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("keine JSON-Zeile: %q", sc.Text())
		}
		resps = append(resps, r)
	}
	return resps, code, stderr.String()
}

func req(id int, src string, extra string) string {
	s, _ := json.Marshal(src)
	return fmt.Sprintf(`{"id": %d, "files": {"p.pas": %s}%s}`, id, s, extra) + "\n"
}

const okProg = "program P; begin WriteLn('ok') end."

func TestServe(t *testing.T) {
	long := `{"id": 4, "files": {"p.pas": "` + strings.Repeat("x", 1<<20+10) + `"}}` + "\n"
	input := req(1, okProg, "") +
		req(2, "program P; begin x := 1 end.", "") +
		"kaputt\n" +
		"\n" + // Leerzeilen werden übersprungen
		long +
		req(5, okProg, `, "target": "gojs"`) +
		req(6, okProg, `, "limits": {"maxDepht": 1}`) +
		req(7, okProg, "")
	resps, code, stderr := serveLines(t, []string{"-maxrequest", "1"}, input)
	if code != 0 {
		t.Fatalf("Exit-Code %d, stderr %q", code, stderr)
	}
	want := []struct {
		id, kind string
		code     int
	}{
		{"1", "", 0}, {"2", "source", 3}, {"", "request", 0}, {"", "request", 0},
		{"5", "", 0}, {"6", "request", 0}, {"7", "", 0},
	}
	if len(resps) != len(want) {
		t.Fatalf("%d Antworten, erwartet %d: %+v", len(resps), len(want), resps)
	}
	for i, w := range want {
		r := resps[i]
		if string(r.ID) != w.id || r.Kind != w.kind || r.Code != w.code {
			t.Errorf("Antwort %d: id=%s kind=%q code=%d error=%q", i+1, r.ID, r.Kind, r.Code, r.Error)
		}
		if w.kind == "" && !bytes.HasPrefix(r.Wasm, []byte("\x00asm")) {
			t.Errorf("Antwort %d: kein Modul", i+1)
		}
	}
	if !strings.Contains(resps[3].Error, "1 MB") {
		t.Errorf("zu lange Zeile: %q", resps[3].Error)
	}
}

func TestServeTimeout(t *testing.T) {
	big := "program P; var x: Integer; begin\n" + strings.Repeat("x := 1;\n", 300_000) + "end."
	input := req(1, big, `, "limits": {"maxTokens": -1}`) + req(2, okProg, "")
	resps, code, _ := serveLines(t, []string{"-timeout", "20ms"}, input)
	if code != 0 || len(resps) != 2 {
		t.Fatalf("Exit-Code %d, %d Antworten", code, len(resps))
	}
	if resps[0].Kind != "timeout" || resps[1].Kind != "" {
		t.Errorf("%+v", resps)
	}
}

func TestServeFlags(t *testing.T) {
	if _, code, _ := serveLines(t, []string{"-gibtsnicht"}, ""); code != 2 {
		t.Errorf("unbekannte Option: Exit-Code %d", code)
	}
	if _, code, _ := serveLines(t, []string{"datei.pas"}, ""); code != 2 {
		t.Errorf("Argument: Exit-Code %d", code)
	}
	if resps, code, _ := serveLines(t, nil, ""); code != 0 || len(resps) != 0 {
		t.Errorf("leere Eingabe: Exit-Code %d, %d Antworten", code, len(resps))
	}
}

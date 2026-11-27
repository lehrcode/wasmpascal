package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lehrcode/wasmpascal/compiler"
)

// zeitProgramm gibt Datum, Wochentag und Uhrzeit aus der Unit Dos aus und die
// Dateizeit von datei.txt, falls vorhanden.
const zeitProgramm = `program Zeit;
uses Dos;
var
  J, M, T, W, H, Mi, S, S100: Word;
  F: file;
  P: LongInt;
  DT: DateTime;
begin
  GetDate(J, M, T, W);
  GetTime(H, Mi, S, S100);
  WriteLn(J, '-', M, '-', T, ' ', W, ' ', H, ':', Mi, ':', S, '.', S100);
  Assign(F, 'datei.txt');
  {$I-} Reset(F, 1); {$I+}
  if IOResult = 0 then
  begin
    GetFTime(F, P);
    UnpackTime(P, DT);
    WriteLn(DT.Year, '-', DT.Month, '-', DT.Day, ' ', DT.Hour, ':', DT.Min, ':', DT.Sec);
    Close(F)
  end
end.
`

// runZeit führt zeitProgramm mit fester Uhr (now, UTC) und Umgebung env aus.
func runZeit(t *testing.T, now time.Time, env []string, dir string) string {
	t.Helper()
	res, err := compiler.Compile(context.Background(), "zeit.pas", compiler.Options{
		FS: fstest.MapFS{"zeit.pas": {Data: []byte(zeitProgramm)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	opts := Options{
		Stdout: &out,
		Env:    env,
		walltime: func() (int64, int32) {
			return now.Unix(), int32(now.Nanosecond())
		},
	}
	if dir != "" {
		opts.Dirs = map[string]string{"/": dir}
	}
	if _, err := Run(context.Background(), res.Wasm, opts); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func utc(y int, m time.Month, d, h, mi, s, cs int) time.Time {
	return time.Date(y, m, d, h, mi, s, cs*10_000_000, time.UTC)
}

func TestOrtszeit(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		env  []string
		want string
	}{
		{"ohne Versatz UTC", utc(2026, 11, 27, 13, 58, 7, 25), nil, "2026-11-27 5 13:58:7.25\n"},
		{"ungültiger Versatz UTC", utc(2026, 11, 27, 13, 58, 7, 0), []string{"WASMPASCAL_UTC_OFFSET=abc"}, "2026-11-27 5 13:58:7.0\n"},
		{"Tageswechsel vorwärts", utc(2026, 11, 27, 23, 30, 0, 0), []string{"WASMPASCAL_UTC_OFFSET=60"}, "2026-11-28 6 0:30:0.0\n"},
		{"Jahreswechsel rückwärts", utc(2026, 1, 1, 0, 30, 0, 0), []string{"WASMPASCAL_UTC_OFFSET=-300"}, "2025-12-31 3 19:30:0.0\n"},
		{"Schaltjahr", utc(2024, 2, 29, 12, 0, 0, 99), []string{"WASMPASCAL_UTC_OFFSET=0"}, "2024-2-29 4 12:0:0.99\n"},
		{"halbe Stunden", utc(2026, 11, 27, 13, 58, 0, 0), []string{"WASMPASCAL_UTC_OFFSET=330"}, "2026-11-27 5 19:28:0.0\n"},
		{"Schaltjahr über Mitternacht", utc(2024, 2, 28, 23, 0, 0, 0), []string{"WASMPASCAL_UTC_OFFSET=120"}, "2024-2-29 4 1:0:0.0\n"},
		{"Hundertstel am Sekundenende", utc(2026, 11, 27, 13, 58, 59, 99), nil, "2026-11-27 5 13:58:59.99\n"},
		{"halbe Sekunde", utc(2026, 11, 27, 13, 58, 59, 50), nil, "2026-11-27 5 13:58:59.50\n"},
		{"Jahr 2100 ohne Schalttag", utc(2100, 2, 28, 23, 0, 0, 0), []string{"WASMPASCAL_UTC_OFFSET=120"}, "2100-3-1 1 1:0:0.0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runZeit(t, tt.now, tt.env, ""); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// GetFTime rechnet die Änderungszeit der Datei mit demselben Versatz in
// Ortszeit um; DOS speichert Sekunden nur gerade.
func TestDateizeitOrtszeit(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "datei.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mtime := utc(2026, 6, 15, 22, 30, 3, 0)
	if err := os.Chtimes(file, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	got := runZeit(t, utc(2026, 11, 27, 12, 0, 0, 0), []string{"WASMPASCAL_UTC_OFFSET=120"}, dir)
	want := "2026-11-27 5 14:0:0.0\n2026-6-16 0:30:2\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

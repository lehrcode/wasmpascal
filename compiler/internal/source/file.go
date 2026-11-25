package source

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"unicode/utf8"
)

// File ist eine geladene, gültige UTF-8-Quelldatei ohne BOM.
type File struct {
	// Name ist der Pfad in Loader.FS.
	Name string
	// Data ist der Inhalt ohne BOM.
	Data []byte
}

// Position rechnet einen Byte-Offset in Data in Zeile und Spalte um.
func (f *File) Position(off int) Pos {
	line, start := 1, 0
	for i := 0; i < off && i < len(f.Data); i++ {
		if f.Data[i] == '\n' {
			line, start = line+1, i+1
		}
	}
	end := min(off, len(f.Data))
	return Pos{File: f.Name, Line: line, Col: utf8.RuneCount(f.Data[start:end]) + 1}
}

// Grenzen für die Quelltextmenge einer Übersetzung. Sie verhindern, dass sich
// Dateien über {$I} vervielfachen (jede Einbindung zählt erneut).
const (
	MaxIncludes    = 1000     // {$I}-Direktiven insgesamt
	MaxSourceBytes = 16 << 20 // gelesene Quelltextbytes insgesamt (16 MB)
)

// Loader liest Quelldateien aus einem fs.FS und merkt sich bereits geladene.
// Ein Loader gehört zu genau einer Übersetzung und ist nicht nebenläufig nutzbar;
// er zählt auch, wie viel Quelltext die Übersetzung schon gelesen hat.
type Loader struct {
	FS           fs.FS
	IncludePaths []string
	files        map[string]*File
	includes     int // bisherige {$I}-Einbindungen
	bytes        int // bisher gelesene Quelltextbytes, mit Wiederholungen
}

// Use zählt f als gelesen, include gibt an, ob es über {$I} geschieht. Ist
// eine Grenze überschritten, liefert Use den Grund für die Fehlermeldung
// (ErrSourceTooLarge), sonst "".
func (l *Loader) Use(f *File, include bool) string {
	if include {
		l.includes++
		if l.includes > MaxIncludes {
			return fmt.Sprintf("more than %d {$I} directives", MaxIncludes)
		}
	}
	l.bytes += len(f.Data)
	if l.bytes > MaxSourceBytes {
		return fmt.Sprintf("more than %d MB of source text", MaxSourceBytes>>20)
	}
	return ""
}

// NewLoader erzeugt einen Loader für fsys mit zusätzlichen Suchpfaden.
func NewLoader(fsys fs.FS, includePaths []string) *Loader {
	return &Loader{FS: fsys, IncludePaths: includePaths, files: map[string]*File{}}
}

var bom = []byte{0xEF, 0xBB, 0xBF}

// Load liest die Datei name (exakter Pfad in FS), überspringt ein BOM und
// prüft auf gültiges UTF-8. Ein ungültiges Byte ergibt einen *Error mit Position.
func (l *Loader) Load(name string) (*File, error) {
	if f, ok := l.files[name]; ok {
		return f, nil
	}
	data, err := fs.ReadFile(l.FS, name)
	if err != nil {
		return nil, err
	}
	if len(data) >= 3 && string(data[:3]) == string(bom) {
		data = data[3:]
	}
	f := &File{Name: name, Data: data}
	if !utf8.Valid(data) {
		off := 0
		for off < len(data) {
			r, size := utf8.DecodeRune(data[off:])
			if r == utf8.RuneError && size <= 1 {
				break
			}
			off += size
		}
		return nil, Errorf(f.Position(off), ErrInvalidUTF8, "")
	}
	l.files[name] = f
	return f, nil
}

// Resolve sucht eine Datei, wie sie im Quelltext genannt ist (z. B. in {$I} oder
// später in uses): zuerst relativ zu dir, dann in den IncludePaths.
// Rückstriche gelten als Pfadtrenner, Groß-/Kleinschreibung ist egal (wie unter
// DOS); bei einem exakten Treffer wird dieser bevorzugt.
func (l *Loader) Resolve(name, dir string) (string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), `\`, "/")
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return "", errInvalidName
	}
	for _, d := range append([]string{dir}, l.IncludePaths...) {
		p := path.Join(d, name)
		if !fs.ValidPath(p) {
			continue
		}
		if found, ok := lookupFold(l.FS, p); ok {
			return found, nil
		}
	}
	return "", fs.ErrNotExist
}

var errInvalidName = errors.New("ungültiger Dateiname")

// IsInvalidName meldet, ob err von Resolve wegen eines ungültigen Namens kommt.
func IsInvalidName(err error) bool { return errors.Is(err, errInvalidName) }

// lookupFold findet p in fsys, wobei jede Pfadkomponente ohne Rücksicht auf
// Groß-/Kleinschreibung verglichen wird.
func lookupFold(fsys fs.FS, p string) (string, bool) {
	if st, err := fs.Stat(fsys, p); err == nil && !st.IsDir() {
		return p, true
	}
	cur := "."
	parts := strings.Split(p, "/")
	for i, part := range parts {
		entries, err := fs.ReadDir(fsys, cur)
		if err != nil {
			return "", false
		}
		match := ""
		for _, e := range entries {
			if e.Name() == part {
				match = part
				break
			}
			if match == "" && strings.EqualFold(e.Name(), part) {
				match = e.Name()
			}
		}
		if match == "" {
			return "", false
		}
		cur = path.Join(cur, match)
		if i == len(parts)-1 {
			st, err := fs.Stat(fsys, cur)
			return cur, err == nil && !st.IsDir()
		}
	}
	return "", false
}

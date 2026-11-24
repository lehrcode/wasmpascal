package runner

import (
	"context"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Vorläufige Hostfunktionen für Write/WriteLn (PLAN.md, Etappe 4). Sie werden
// in Etappe 6 durch die Laufzeit in Pascal ersetzt und dann entfernt.

func instantiateDevHost(ctx context.Context, rt wazero.Runtime, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	write := func(s string) { _, _ = io.WriteString(out, s) }
	_, err := rt.NewHostModuleBuilder("wasmpascal").
		NewFunctionBuilder().WithFunc(func(v, width int32) {
		write(pad(strconv.Itoa(int(v)), width))
	}).Export("write_int").
		NewFunctionBuilder().WithFunc(func(v float64, width, prec int32) {
		write(formatReal(v, width, prec))
	}).Export("write_real").
		NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, ptr, n, width uint32) {
		b, _ := m.Memory().Read(ptr, n)
		write(pad(string(b), int32(width)))
	}).Export("write_str").
		NewFunctionBuilder().WithFunc(func(c, width int32) {
		write(pad(string([]byte{byte(c)}), width))
	}).Export("write_char").
		NewFunctionBuilder().WithFunc(func(b, width int32) {
		s := "FALSE"
		if b != 0 {
			s = "TRUE"
		}
		write(pad(s, width))
	}).Export("write_bool").
		NewFunctionBuilder().WithFunc(func() { write("\n") }).Export("write_ln").
		Instantiate(ctx)
	return err
}

// pad richtet s rechtsbündig in width Zeichen aus (Bytes, wie TP).
func pad(s string, width int32) string {
	if n := int(width) - len(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

// formatReal formatiert wie TP: mit Nachkommastellen (prec >= 0) als
// Festkommazahl, sonst in Exponentialschreibweise mit Breite 17 als Vorgabe,
// z. B. " 1.5000000000E+00". Gerundet wird wie in TP: halbe Werte von null weg,
// bezogen auf den exakten Binärwert.
func formatReal(v float64, width, prec int32) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return pad(strconv.FormatFloat(v, 'g', -1, 64), width)
	}
	neg := math.Signbit(v)
	digits, exp := decimalDigits(math.Abs(v))
	if prec >= 0 {
		// Festkomma: exp Stellen vor dem Komma, prec danach
		keep := exp + int(prec)
		d, e := roundDigits(digits, keep, exp)
		intPart, frac := "0", ""
		all := d
		for len(all) < e+int(prec) {
			all += "0"
		}
		if e > 0 {
			intPart = all[:e]
			frac = all[e:]
		} else {
			frac = strings.Repeat("0", -e) + all
		}
		frac = (frac + strings.Repeat("0", int(prec)))[:prec]
		s := intPart
		if prec > 0 {
			s += "." + frac
		}
		if neg && strings.Trim(s, "0.") != "" {
			s = "-" + s
		}
		return pad(s, width)
	}
	w := width
	if w < 0 {
		w = 17
	}
	n := max(int(w)-7, 1) // Nachkommastellen der Mantisse
	d, e := roundDigits(digits, n+1, exp)
	for len(d) < n+1 {
		d += "0"
	}
	x := e - 1
	if digits == "" {
		x = 0
	}
	sign := " "
	if neg {
		sign = "-"
	}
	es := "+"
	if x < 0 {
		es, x = "-", -x
	}
	xs := strconv.Itoa(x)
	if len(xs) < 2 {
		xs = "0" + xs
	}
	return pad(sign+d[:1]+"."+d[1:n+1]+"E"+es+xs, width)
}

// decimalDigits liefert die exakten Dezimalziffern von v > 0 ohne führende
// und abschließende Nullen und den Exponenten e mit v = 0.ziffern × 10^e.
// Für v = 0 ist digits leer.
func decimalDigits(v float64) (string, int) {
	if v == 0 {
		return "", 0
	}
	s := new(big.Float).SetFloat64(v).Text('e', 1100) // exakt: float64 hat höchstens 767 signifikante Stellen
	mant, expStr, _ := strings.Cut(s, "e")
	e, _ := strconv.Atoi(expStr)
	digits := strings.TrimRight(strings.Replace(mant, ".", "", 1), "0")
	return digits, e + 1
}

// roundDigits behält keep Ziffern und rundet halbe Werte auf; ein Übertrag
// erhöht den Exponenten.
func roundDigits(digits string, keep, exp int) (string, int) {
	if keep < 0 {
		return "", exp
	}
	if keep >= len(digits) {
		return digits, exp
	}
	up := digits[keep] >= '5'
	b := []byte(digits[:keep])
	if up {
		i := len(b) - 1
		for ; i >= 0 && b[i] == '9'; i-- {
			b[i] = '0'
		}
		if i < 0 {
			b = append([]byte{'1'}, b...)
			exp++
		} else {
			b[i]++
		}
	}
	return string(b), exp
}

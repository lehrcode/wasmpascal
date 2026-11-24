program VarianteRecords;
type
  TForm = (Kreis, Rechteck, Dreieck);
  TFigur = record
    x, y: Real;
    case Art: TForm of
      Kreis: (Radius: Real);
      Rechteck: (Breite, Hoehe: Real);
      Dreieck: (a, b, c: Real)
  end;
  TWandler = record
    case Integer of
      0: (L: LongInt);
      1: (B: array[0..3] of Byte);
      2: (W1, W2: Word; case Boolean of True: (Z: Char); False: ())
  end;
var
  f: TFigur;
  w: TWandler;

function Flaeche(const f: TFigur): Real;
var
  s: Real;
begin
  case f.Art of
    Kreis: Flaeche := Pi * Sqr(f.Radius);
    Rechteck: Flaeche := f.Breite * f.Hoehe;
    Dreieck:
      begin
        s := (f.a + f.b + f.c) / 2;
        Flaeche := Sqrt(s * (s - f.a) * (s - f.b) * (s - f.c))
      end
  end
end;

begin
  f.Art := Rechteck;
  f.Breite := 3.0; f.Hoehe := 4.5;
  WriteLn(Flaeche(f):0:2);
  w.L := $01020304;
  WriteLn(w.B[0], ' ', w.B[3])
end.

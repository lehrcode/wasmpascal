program TypisierteKonstanten;
type
  TPunkt = record
    x, y: Integer
  end;
  TFarbe = (Rot, Gruen, Blau);
  TMatrix = array[1..2, 1..3] of Real;
  TLinie = record
    Von, Bis: TPunkt;
    Farbe: TFarbe
  end;
const
  Max: Integer = 100;
  Name: string[10] = 'WasmPascal';
  Ursprung: TPunkt = (x: 0; y: 0);
  Einheit: TMatrix = ((1.0, 0.0, 0.0), (0.0, 1.0, 0.0));
  Linie: TLinie = (Von: (x: 1; y: 2); Bis: (x: 3; y: 4); Farbe: Blau);
  Primzahlen: set of Byte = [2, 3, 5, 7, 11];
  Tage: array[1..12] of Byte = (31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31);
  Nichts: ^Integer = nil;
  Faktor: Real = (1 + 2) * 0.5;
  Zeichen: array[0..2] of Char = ('a', 'b', #99);
  Grenze = SizeOf(TMatrix) div 2;
begin
  WriteLn(Name, ' ', Ursprung.x, ' ', Einheit[2, 2]:0:1, ' ', Tage[2]);
  if 7 in Primzahlen then WriteLn(Ord(Linie.Farbe))
end.

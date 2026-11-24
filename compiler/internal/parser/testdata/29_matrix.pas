program Matrix;
const
  N = 3;
type
  TMatrix = array[1..N, 1..N] of Real;
  TZeile = array[1..N] of Real;
  TTabelle = array[1..N] of TZeile;
  TBits = packed array[0..7] of Boolean;
var
  a, b, c: TMatrix;
  t: TTabelle;
  Bits: TBits;
  i, j, k: Integer;
begin
  for i := 1 to N do
    for j := 1 to N do
    begin
      a[i, j] := i + j;
      if i = j then b[i, j] := 1 else b[i, j] := 0;
      t[i][j] := a[i, j]
    end;
  for i := 1 to N do
    for j := 1 to N do
    begin
      c[i, j] := 0;
      for k := 1 to N do
        c[i, j] := c[i, j] + a[i, k] * b[k, j]
    end;
  for i := 1 to N do
  begin
    for j := 1 to N do Write(c[i][j]:6:1);
    WriteLn
  end;
  FillChar(Bits, SizeOf(Bits), 0);
  Bits[3] := True
end.

program Bubblesort;
const
  N = 10;
  Daten: array[1..N] of Integer = (5, 3, 9, 1, 7, 2, 8, 6, 4, 10);
type
  TFeld = array[1..N] of Integer;
var
  Feld: TFeld;
  i: Integer;

procedure Tausche(var x, y: Integer);
var
  h: Integer;
begin
  h := x; x := y; y := h
end;

procedure Sortiere(var f: TFeld);
var
  i, j: Integer;
  getauscht: Boolean;
begin
  for i := N downto 2 do
  begin
    getauscht := False;
    for j := 1 to i - 1 do
      if f[j] > f[j + 1] then
      begin
        Tausche(f[j], f[j + 1]);
        getauscht := True
      end;
    if not getauscht then Exit
  end
end;

begin
  for i := 1 to N do Feld[i] := Daten[i];
  Sortiere(Feld);
  for i := 1 to N do Write(Feld[i]:4);
  WriteLn
end.

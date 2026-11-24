program Primzahlsieb;
{ Sieb des Eratosthenes }
const
  Max = 1000;
var
  Prim: array[2..Max] of Boolean;
  i, j, Anzahl: Integer;
begin
  for i := 2 to Max do
    Prim[i] := True;
  i := 2;
  while i * i <= Max do
  begin
    if Prim[i] then
    begin
      j := i * i;
      while j <= Max do
      begin
        Prim[j] := False;
        j := j + i
      end
    end;
    Inc(i)
  end;
  Anzahl := 0;
  for i := 2 to Max do
    if Prim[i] then
    begin
      Write(i:5);
      Anzahl := Anzahl + 1;
      if Anzahl mod 10 = 0 then WriteLn
    end;
  WriteLn;
  WriteLn(Anzahl, ' Primzahlen bis ', Max)
end.

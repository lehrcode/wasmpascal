program Zeitscheiben;
{ Viele Write-Aufrufe: alle 30 ms haelt das Programm an (Zeitscheibe),
  auch mitten in einer Rekursion. }
var
  Summe: LongInt;

procedure Rechne(Tiefe: Integer; Runden: LongInt);
var
  i: LongInt;
begin
  if Tiefe > 0 then
    Rechne(Tiefe - 1, Runden)
  else
    for i := 1 to Runden do
    begin
      Write('');
      Summe := Summe + i mod 7
    end
end;

begin
  Summe := 0;
  Rechne(20, 300000);
  WriteLn(Summe)
end.

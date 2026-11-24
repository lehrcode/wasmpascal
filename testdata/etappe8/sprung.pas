program Sprung;
{ goto: aus verschachtelten Schleifen heraus, zurueck nach oben und
  in eine Schleife hinein (irreduzibler Kontrollfluss). }
label
  1, 2, 3, Fertig;
var
  i, j, Runde: Integer;

function Suche(Ziel: Integer): Integer;
label
  Gefunden;
var
  a, b: Integer;
begin
  for a := 1 to 9 do
    for b := 1 to 9 do
      if a * 10 + b = Ziel then
        goto Gefunden;
  Suche := -1;
  Exit;
Gefunden:
  Suche := a * b
end;

begin
  Runde := 0;
1:
  Inc(Runde);
  for i := 1 to 5 do
    for j := 1 to 5 do
      if i * j = Runde * 4 then
      begin
        WriteLn('Runde ', Runde, ': ', i, '*', j);
        if Runde < 3 then
          goto 1;
        goto Fertig
      end;
  WriteLn('nie');
Fertig:
  WriteLn('fertig nach ', Runde, ' Runden');
  i := 0;
2:
  i := i + 1;
  if i < 5 then
    goto 2;
  WriteLn(i);
  i := 0;
  goto 3;
  repeat
    i := i + 10;
3:
    i := i + 1
  until i > 30;
  WriteLn(i);
  WriteLn(Suche(47), ' ', Suche(5))
end.

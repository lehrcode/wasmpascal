program Forward;

function IstGerade(n: Integer): Boolean; forward;

function IstUngerade(n: Integer): Boolean;
begin
  if n = 0 then IstUngerade := False
  else IstUngerade := IstGerade(n - 1)
end;

function IstGerade;
begin
  if n = 0 then IstGerade := True
  else IstGerade := IstUngerade(n - 1)
end;

procedure Ausgabe(x: Integer); forward;

procedure Ausgabe(x: Integer);
begin
  WriteLn(x, ' gerade: ', IstGerade(x))
end;

begin
  Ausgabe(10);
  Ausgabe(7)
end.

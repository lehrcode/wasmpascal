program Sprung;
label
  1, 99, Fertig;
var
  i, j: Integer;
begin
  i := 0;
1:
  Inc(i);
  for j := 1 to 10 do
  begin
    if i * j > 50 then goto Fertig;
    if j = i then goto 1
  end;
  goto 99;
Fertig:
  WriteLn('fertig bei ', i, ' ', j);
99:
end.

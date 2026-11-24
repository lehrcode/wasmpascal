program Anhalten;
var
  i: Integer;

procedure Pruefe(n: Integer);
begin
  if n > 2 then
  begin
    WriteLn('Abbruch bei ', n);
    Halt(3)
  end
end;

begin
  for i := 1 to 10 do
  begin
    WriteLn(i);
    Pruefe(i)
  end;
  WriteLn('nie erreicht')
end.

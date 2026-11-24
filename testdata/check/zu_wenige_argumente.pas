program ZuWenige;

function Max(a, b: Integer): Integer;
begin
  if a > b then Max := a else Max := b
end;

begin
  WriteLn(Max(3))
end.

{ RunError aus TP 5.0 }
program RunErr;

procedure Pruefe(X: Integer);
begin
  if X < 0 then
    RunError(201)
end;

begin
  Pruefe(1);
  WriteLn('ok');
  Pruefe(-1);
  WriteLn('nie')
end.

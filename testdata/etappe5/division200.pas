program Division;
var
  a, b: Integer;

function Quote(x, y: Integer): Integer;
begin
  Quote := x div y
end;

begin
  a := 10;
  b := 2;
  WriteLn(Quote(a, b));
  b := 0;
  WriteLn(Quote(a, b));
  WriteLn('nie')
end.

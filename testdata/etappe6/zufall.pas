program Zufall;
var
  i: Integer;
begin
  RandSeed := 42;
  for i := 1 to 8 do Write(Random(100), ' ');
  WriteLn;
  RandSeed := 1;
  WriteLn(Random:0:6);
  Randomize;
  i := Random(6) + 1;
  WriteLn((i >= 1) and (i <= 6))
end.

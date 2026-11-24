program GGT;
var
  a, b: Integer;

function Ggt(a, b: Integer): Integer;
var
  r: Integer;
begin
  repeat
    r := a mod b;
    a := b;
    b := r
  until r = 0;
  Ggt := a
end;

function Kgv(a, b: Integer): LongInt;
begin
  Kgv := LongInt(a) * b div Ggt(a, b)
end;

begin
  Write('Zwei Zahlen: ');
  ReadLn(a, b);
  WriteLn('ggT = ', Ggt(a, b), ', kgV = ', Kgv(a, b))
end.

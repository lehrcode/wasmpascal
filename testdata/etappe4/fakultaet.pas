program Fakultaet;
var
  i: Integer;

function Fak(n: Integer): LongInt;
begin
  if n <= 1 then
    Fak := 1
  else
    Fak := n * Fak(n - 1)
end;

function FakIterativ(n: Integer): LongInt;
var
  k: Integer;
begin
  Result := 1;
  for k := 2 to n do
    Result := Result * k
end;

begin
  for i := 0 to 12 do
    WriteLn(i:2, '! = ', Fak(i):10, FakIterativ(i):10)
end.

program Fibonacci;
const
  Anzahl = 20;
var
  a, b, c: LongInt;
  i: Integer;

function FibRek(n: Integer): LongInt;
begin
  if n < 2 then FibRek := n
  else FibRek := FibRek(n - 1) + FibRek(n - 2)
end;

begin
  a := 0; b := 1;
  for i := 1 to Anzahl do
  begin
    Write(a, ' ');
    c := a + b;
    a := b;
    b := c
  end;
  WriteLn;
  WriteLn('fib(20) rekursiv: ', FibRek(20))
end.

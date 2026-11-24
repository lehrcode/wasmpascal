program Ausdruecke;
var
  a, b, c: Integer;
  x, y: Real;
  p, q: Boolean;
  w: Word;
  s: set of Char;
begin
  a := 7; b := 3;
  c := a + b * 2 - a div b mod 2;
  c := -a * -b;
  c := (a + b) * (a - b);
  c := a shl 2 or b shr 1 and $0F xor 255;
  x := a / b + 1.0E2 - 3.5e-1;
  y := -x;
  p := not (a > b) or (b >= c) and (a <> c);
  q := (a = b) = p;
  p := not not q;
  w := Word(a) + Ord('A') + Ord(p);
  s := ['a'..'z', 'A'..'Z', '_'];
  q := 'x' in s;
  p := (c in [1..10]) xor q;
  x := +x;
  a := Trunc(x) + Round(y) + Abs(-3) + Sqr(2)
end.

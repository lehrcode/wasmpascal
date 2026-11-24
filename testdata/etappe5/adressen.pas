program Adressen;
type
  PInteger = ^Integer;
var
  x: Integer;
  p: PInteger;
  q: Pointer;
  a: array[1..3] of Integer;
begin
  x := 42;
  p := @x;
  q := Addr(x);
  WriteLn(p^, ' ', p = q, ' ', Ofs(x) = LongInt(p), ' ', Seg(x));
  p := Ptr(0, Ofs(x));
  p^ := 7;
  WriteLn(x);
  p := Ptr(Seg(a), Ofs(a[2]));
  p^ := 99;
  WriteLn(a[2]);
  WriteLn(Ofs(a[3]) - Ofs(a[1]), ' ', LongInt(@a[2]) - LongInt(@a[1]));
  WriteLn(Ofs(x) > 65535, ' ', p <> nil)
end.

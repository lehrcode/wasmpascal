program HeapVoll;
var
  p: Pointer;
begin
  GetMem(p, 1000);
  WriteLn('1000 Bytes ok');
  GetMem(p, 2147000000)
end.

program StrVal;
var
  s: string;
  k: string[4];
  i, Code: Integer;
  r: Real;
begin
  Str(42, s); WriteLn('[', s, ']');
  Str(-42:6, s); WriteLn('[', s, ']');
  Str(3.14159:8:3, s); WriteLn('[', s, ']');
  Str(1.5, s); WriteLn('[', s, ']');
  Str(1.5:10, s); WriteLn('[', s, ']');
  Str(123456:0, k); WriteLn('[', k, ']');
  Val('123', i, Code); WriteLn(i, ' ', Code);
  Val('  -77', i, Code); WriteLn(i, ' ', Code);
  Val('$FF', i, Code); WriteLn(i, ' ', Code);
  Val('12x4', i, Code); WriteLn(Code);
  Val('', i, Code); WriteLn(Code);
  Val('2.5e3', r, Code); WriteLn(r:0:1, ' ', Code);
  Val('-0.125', r, Code); WriteLn(r:0:3, ' ', Code);
  Val('1.', r, Code); WriteLn(Code);
  Val('.5', r, Code); WriteLn(Code);
  Val('1e', r, Code); WriteLn(Code);
  WriteLn(1E100, ' ', -1.5E-200:12, ' ', 0.000123:0:6, ' ', 999.996:0:2)
end.

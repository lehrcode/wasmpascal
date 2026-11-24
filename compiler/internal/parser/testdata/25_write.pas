program Formatierung;
var
  r: Real;
  i: Integer;
  b: Boolean;
  f: Text;
begin
  r := 1.5;
  i := -42;
  b := True;
  WriteLn(r);
  WriteLn(r:8:2, i:6, b:7);
  WriteLn('|', 'links':-10, '|');
  Write('ohne Zeilenende');
  WriteLn;
  Write(Output, 'Ausgabe ', i);
  WriteLn(Output);
  Assign(f, '');
  Rewrite(f);
  WriteLn(f, r:0:3, ' ', 2.5E-3:12, ' ', i * 2:4);
  Close(f)
end.

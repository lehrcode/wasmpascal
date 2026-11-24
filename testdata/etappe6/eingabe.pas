program Eingabe;
var
  Name: string[10];
  a, b: Integer;
  r: Real;
  Summe: LongInt;
  n: Integer;
  c: Char;
begin
  Write('Name? ');
  ReadLn(Name);
  WriteLn('Hallo, ', Name, '!');
  Read(a, b);
  ReadLn(r);
  WriteLn(a, ' + ', b, ' = ', a + b, ', r = ', r:0:3);
  Read(c);
  WriteLn('Zeichen: ', c, ' ', Ord(c));
  ReadLn;
  Summe := 0;
  n := 0;
  while not SeekEof do
  begin
    Read(a);
    Summe := Summe + a;
    Inc(n)
  end;
  WriteLn(n, ' Zahlen, Summe ', Summe, ' ', Eof)
end.

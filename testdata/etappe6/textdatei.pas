program Textdatei;
{ Abnahme Etappe 6: Textdateien schreiben, lesen und anhaengen }
var
  f: Text;
  Zeile: string;
  n, a, b, Code: Integer;
  r: Real;
  c: Char;
begin
  Assign(f, 'notizen.txt');
  Rewrite(f);
  WriteLn(f, 'Erste Zeile');
  WriteLn(f, 42, ' ', -7, ' ', 2.5:0:1);
  Write(f, 'ohne Ende');
  Close(f);

  Reset(f);
  n := 0;
  while not Eof(f) do
  begin
    ReadLn(f, Zeile);
    Inc(n);
    WriteLn(n, ': ', Zeile, ' (', Length(Zeile), ')')
  end;
  Close(f);

  Append(f);
  WriteLn(f);
  WriteLn(f, 'angehaengt');
  Close(f);

  Reset(f);
  ReadLn(f);
  Read(f, a, b, r);
  WriteLn(a + b, ' ', r:0:2);
  ReadLn(f);
  Read(f, c);
  Write(c);
  Read(f, c);
  WriteLn(c, ' ', Eoln(f));
  ReadLn(f, Zeile);
  WriteLn(Zeile);
  ReadLn(f, Zeile);
  WriteLn(Zeile, ' ', Eof(f));
  Close(f);
  Erase(f);
  {$I-} Reset(f); {$I+}
  Code := IOResult;
  WriteLn('nach Erase: ', Code)
end.

program Textdateien;
var
  f: Text;
  Zeile: string;
  n: Integer;
begin
  Assign(f, 'test.txt');
  Rewrite(f);
  for n := 1 to 3 do
    WriteLn(f, 'Zeile ', n);
  Close(f);

  Append(f);
  Write(f, 'angehaengt');
  WriteLn(f);
  Close(f);

  {$I-}
  Reset(f);
  {$I+}
  if IOResult <> 0 then
  begin
    WriteLn('Datei fehlt');
    Halt(1)
  end;
  n := 0;
  while not Eof(f) do
  begin
    ReadLn(f, Zeile);
    Inc(n);
    WriteLn(n:3, ': ', Zeile)
  end;
  Close(f);
  {$I-} Erase(f); {$I+}
  n := IOResult
end.

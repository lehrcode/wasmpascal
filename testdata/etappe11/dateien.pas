program Dateien;
{ Typisierte Dateien: Binaerdaten 0..255, Seek, FilePos, FileSize,
  Truncate, Eof; Append auf eine fehlende Datei; Erase und Rename einer
  geschlossenen Datei. }
var
  B: file of Byte;
  T: Text;
  X: Byte;
  I, N, Fehler: Integer;
  S: string;
begin
  Assign(B, 'bytes.dat');
  Rewrite(B);
  for I := 0 to 255 do
  begin
    X := I;
    Write(B, X)
  end;
  WriteLn('Groesse: ', FileSize(B), ' Position: ', FilePos(B));
  Seek(B, 0);
  Fehler := 0;
  for I := 0 to 255 do
  begin
    Read(B, X);
    if X <> I then
      Inc(Fehler)
  end;
  WriteLn('Abweichungen: ', Fehler, ' Eof: ', Eof(B));
  Seek(B, 200);
  Read(B, X);
  WriteLn('Satz 200: ', X, ' Position: ', FilePos(B));
  Seek(B, 100);
  Truncate(B);
  WriteLn('nach Truncate: ', FileSize(B), ' Eof: ', Eof(B));
  Seek(B, 99);
  Read(B, X);
  WriteLn('letzter Satz: ', X);
  Close(B);
  Reset(B);
  N := 0;
  while not Eof(B) do
  begin
    Read(B, X);
    Inc(N)
  end;
  Close(B);
  WriteLn('wieder gelesen: ', N);
  { Append auf eine fehlende Datei }
  Assign(T, 'fehlt.txt');
  {$I-} Append(T); {$I+} N := IOResult;
  WriteLn('Append fehlt: ', N);
  { Append an eine vorhandene Datei }
  Assign(T, 'log.txt');
  Rewrite(T); WriteLn(T, 'eins'); Close(T);
  Append(T); WriteLn(T, 'zwei'); Close(T);
  Reset(T);
  while not Eof(T) do
  begin
    ReadLn(T, S);
    WriteLn('log: ', S)
  end;
  Close(T);
  { Rename und Erase geschlossener Dateien }
  Rename(T, 'protokoll.txt');
  Reset(T); ReadLn(T, S); Close(T);
  WriteLn('nach Rename: ', S);
  Assign(T, 'log.txt');
  {$I-} Reset(T); {$I+} N := IOResult;
  WriteLn('alter Name: ', N);
  Assign(T, 'protokoll.txt');
  Erase(T);
  {$I-} Reset(T); {$I+} N := IOResult;
  WriteLn('nach Erase: ', N);
  Erase(B);
  {$I-} Erase(B); {$I+} N := IOResult;
  WriteLn('Erase doppelt: ', N)
end.

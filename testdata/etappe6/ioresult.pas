program IOFehler;
var
  f, g: Text;
  s: string;
  Code: Integer;
begin
  {$I-}
  Assign(f, 'gibtsnicht.txt');
  Reset(f);
  Code := IOResult;
  WriteLn('Reset: ', Code);
  Code := IOResult;
  WriteLn('danach: ', Code);
  ReadLn(f, s);
  Code := IOResult;
  WriteLn('ReadLn auf geschlossener Datei: ', Code);
  Close(g);
  Code := IOResult;
  WriteLn('Close ohne Assign: ', Code);
  Assign(f, 'neu.txt');
  Rewrite(f);
  ReadLn(f, s);
  Code := IOResult;
  WriteLn('Lesen aus Ausgabedatei: ', Code);
  Close(f);
  Reset(f);
  Write(f, 'x');
  Code := IOResult;
  WriteLn('Schreiben in Eingabedatei: ', Code);
  Close(f);
  MkDir('ordner');
  MkDir('ordner');
  Code := IOResult;
  WriteLn('MkDir doppelt: ', Code);
  {$I+}
  WriteLn('fertig')
end.
